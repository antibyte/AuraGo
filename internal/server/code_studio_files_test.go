package server

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type codeStudioWriteDocker struct {
	fakeCodeStudioDockerAPI
	archive   []byte
	commands  [][]string
	exitCode  int
	uploadErr error
}

func (d *codeStudioWriteDocker) PutArchive(_ context.Context, _ string, data []byte) error {
	d.archive = append([]byte(nil), data...)
	return d.uploadErr
}

func (d *codeStudioWriteDocker) Exec(_ context.Context, _ string, cmd []string, _ time.Duration) (codeStudioExecResult, error) {
	d.commands = append(d.commands, append([]string(nil), cmd...))
	return codeStudioExecResult{ExitCode: d.exitCode, Output: "file already exists"}, nil
}

func TestCodeStudioWriteUsesBinaryTransferAndCreateConflict(t *testing.T) {
	for _, tc := range []struct {
		name   string
		size   int
		create bool
		exit   int
		want   int
	}{
		{"save100KiB", 100 * 1024, false, 0, http.StatusOK},
		{"saveAtLimit", 1024 * 1024, false, 0, http.StatusOK},
		{"overLimit", 1024*1024 + 1, false, 0, http.StatusRequestEntityTooLarge},
		{"createConflict", 0, true, 73, http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testCodeStudioServerWithFakeCodeContainer(t, 1)
			d := &codeStudioWriteDocker{exitCode: tc.exit}
			content := strings.Repeat("x", tc.size)
			body, _ := json.Marshal(map[string]interface{}{"path": "/workspace/main.go", "content": content, "create_only": tc.create})
			rec := httptest.NewRecorder()
			codeStudioHandlers{server: s, docker: d}.handleWriteFile(rec, httptest.NewRequest(http.MethodPut, "/api/code-studio/file", bytes.NewReader(body)))
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == http.StatusRequestEntityTooLarge {
				if len(d.commands) != 0 || len(d.archive) != 0 {
					t.Fatal("oversize write reached Docker")
				}
				return
			}
			assertCodeStudioArchive(t, d.archive, []byte(content))
			for _, cmd := range d.commands {
				for _, arg := range cmd {
					if len(arg) > 16384 {
						t.Fatal("file bytes leaked into exec arguments")
					}
				}
			}
		})
	}
}

func TestCodeStudioUploadUsesBinaryTransfer(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	d := &codeStudioWriteDocker{}
	content := bytes.Repeat([]byte{0, 255, 1, 128}, 32*1024)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("path", "/workspace")
	f, _ := w.CreateFormFile("file", "data.bin")
	_, _ = f.Write(content)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/code-studio/upload", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	codeStudioHandlers{server: s, docker: d}.handleUpload(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload = %d: %s", rec.Code, rec.Body.String())
	}
	assertCodeStudioArchive(t, d.archive, content)
}

func TestCodeStudioFailedTransferDoesNotInstallFile(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	d := &codeStudioWriteDocker{uploadErr: fmt.Errorf("transfer interrupted")}
	rec := httptest.NewRecorder()
	codeStudioHandlers{server: s, docker: d}.handleWriteFile(rec, httptest.NewRequest(http.MethodPut, "/api/code-studio/file", strings.NewReader(`{"path":"/workspace/main.go","content":"new"}`)))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d", rec.Code)
	}
	for _, cmd := range d.commands {
		if len(cmd) > 0 && cmd[0] != "rm" {
			t.Fatalf("failed transfer installed file: %q", cmd)
		}
	}
}

func assertCodeStudioArchive(t *testing.T, data, want []byte) {
	t.Helper()
	r := tar.NewReader(bytes.NewReader(data))
	h, err := r.Next()
	if err != nil {
		t.Fatalf("read uploaded archive: %v", err)
	}
	if strings.ContainsAny(h.Name, "/\\") || h.Typeflag != tar.TypeReg || h.Mode != 0600 {
		t.Fatalf("unsafe staging entry: %+v", h)
	}
	got, err := io.ReadAll(r)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("archive bytes differ: got %d, want %d: %v", len(got), len(want), err)
	}
	if _, err := r.Next(); err != io.EOF {
		t.Fatalf("unexpected additional archive entry: %v", err)
	}
}

func TestCodeStudioInstallFileScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("container install uses POSIX directory descriptors; run on Linux")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 required for container script acceptance")
	}
	root := t.TempDir()
	staged := filepath.Join(t.TempDir(), "payload")
	content := bytes.Repeat([]byte("payload\x00"), 16384)
	if err := os.WriteFile(staged, content, 0600); err != nil {
		t.Fatal(err)
	}
	run := func(target string, create bool, digest string) int {
		cmd := exec.Command(python, "-c", codeStudioInstallFileScript, root, target, staged, strconv.FormatBool(create), digest, strconv.Itoa(len(content)))
		if err := cmd.Run(); err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				return exit.ExitCode()
			}
			return -1
		}
		return 0
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(content))
	target := filepath.Join(root, "nested", "main.sh")
	var wg sync.WaitGroup
	results := make(chan int, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- run(target, true, digest) }()
	}
	wg.Wait()
	close(results)
	wins := 0
	for code := range results {
		if code == 0 {
			wins++
		} else if code != codeStudioFileExists {
			t.Fatalf("create exit = %d", code)
		}
	}
	if wins != 1 {
		t.Fatalf("exclusive create winners = %d", wins)
	}
	if err := os.Chmod(target, 0755); err != nil {
		t.Fatal(err)
	}
	if code := run(target, false, digest); code != 0 {
		t.Fatalf("replace exit = %d", code)
	}
	info, _ := os.Stat(target)
	if info.Mode().Perm() != 0755 {
		t.Fatal("executable mode lost")
	}
	if code := run(target, false, "invalid"); code == 0 {
		t.Fatal("incomplete transfer accepted")
	}
	got, _ := os.ReadFile(target)
	if !bytes.Equal(got, content) {
		t.Fatal("failed write changed target")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if code := run(filepath.Join(root, "escape", "bad"), false, digest); code == 0 {
		t.Fatal("symlink parent accepted")
	}
	if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if code := run(filepath.Join(root, "link"), false, digest); code == 0 {
		t.Fatal("symlink target accepted")
	}
	if code := run(filepath.Join(root, "..", "escape"), false, digest); code == 0 {
		t.Fatal("workspace escape accepted")
	}
	entries, _ := os.ReadDir(filepath.Dir(target))
	if len(entries) != 1 {
		t.Fatalf("staging files leaked: %v", entries)
	}
}
