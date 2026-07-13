package manus

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
)

func TestUploadUsesValidatedHandleAfterPathReplacement(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, "report.txt")
	if err := os.WriteFile(path, []byte("validated"), 0o600); err != nil {
		t.Fatal(err)
	}
	local, err := ResolveUploadPath(workspace, "report.txt", 1024)
	if err != nil {
		t.Fatal(err)
	}
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		oldPath := filepath.Join(workspace, "old-report.txt")
		if err := os.Rename(path, oldPath); err != nil {
			t.Skipf("atomic file replacement unavailable: %v", err)
		}
		if err := os.WriteFile(path, []byte("replacement"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"ok":true,"file":{"id":"file-1","filename":"report.txt"},"upload_url":"https://203.0.113.10/upload"}`))
	}))
	defer apiServer.Close()
	fileClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "validated" {
			t.Fatalf("upload body = %q, want validated handle content", body)
		}
		return responseWithBody(http.StatusOK, ""), nil
	})}
	client, _ := NewClient("secret", ClientConfig{BaseURL: apiServer.URL, FileHTTPClient: fileClient})
	if _, err := client.UploadLocalFile(context.Background(), local); err != nil {
		t.Fatalf("UploadLocalFile() error = %v", err)
	}
}

func TestResolveUploadPathRejectsExecutableContentModeAndExtensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		file     string
		content  []byte
		mode     os.FileMode
		unixOnly bool
	}{
		{name: "executable mode", file: "notes.txt", content: []byte("plain"), mode: 0o700, unixOnly: true},
		{name: "shebang", file: "notes.txt", content: []byte("#!/bin/sh\necho unsafe"), mode: 0o600},
		{name: "PE magic", file: "notes.txt", content: append([]byte("MZ"), make([]byte, 20)...), mode: 0o600},
		{name: "ELF magic", file: "notes.txt", content: []byte{0x7f, 'E', 'L', 'F', 0}, mode: 0o600},
		{name: "Mach-O magic", file: "notes.txt", content: []byte{0xfe, 0xed, 0xfa, 0xcf, 0}, mode: 0o600},
		{name: "Java magic", file: "notes.txt", content: []byte{0xca, 0xfe, 0xba, 0xbe, 0}, mode: 0o600},
		{name: "Windows script", file: "notes.vbs", content: []byte("safe-looking"), mode: 0o600},
		{name: "Java archive", file: "notes.jar", content: []byte("safe-looking"), mode: 0o600},
		{name: "configuration", file: "notes.yaml", content: []byte("safe: false"), mode: 0o600},
		{name: "private key", file: "notes.pem", content: []byte("key"), mode: 0o600},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.unixOnly && goruntime.GOOS == "windows" {
				t.Skip("Windows does not expose POSIX executable mode bits")
			}
			workspace := t.TempDir()
			if err := os.WriteFile(filepath.Join(workspace, tc.file), tc.content, tc.mode); err != nil {
				t.Fatal(err)
			}
			if _, err := ResolveUploadPath(workspace, tc.file, 1024); err == nil {
				t.Fatal("ResolveUploadPath() error = nil, want rejection")
			}
		})
	}
}

func TestRuntimeDownloadRejectsSymlinkedWorkspaceRoot(t *testing.T) {
	t.Parallel()

	workspaceParent := t.TempDir()
	outside := t.TempDir()
	workspace := filepath.Join(workspaceParent, "workspace")
	if err := os.Symlink(outside, workspace); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"task_id":"task-1","messages":[{"id":"event-1","type":"assistant_message","assistant_message":{"attachments":[{"type":"file","filename":"result.txt","url":"https://203.0.113.12/result"}]}}]}`))
	}))
	defer server.Close()
	fileClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("result")), ContentLength: 6, Header: make(http.Header)}, nil
	})}
	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL, FileHTTPClient: fileClient})
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "manus.db"))
	defer ledger.Close()
	_ = ledger.Upsert(context.Background(), TaskRecord{TaskID: "task-1"})
	runtime := NewRuntime(client, ledger, RuntimeConfig{
		Policy: Policy{AllowFileDownloads: true}, WorkspaceDir: workspace,
		DownloadRoot: filepath.Join(workspace, "workdir", "manus"), MaxFileBytes: 1024,
	})

	if _, err := runtime.DownloadAttachments(context.Background(), "task-1", ""); err == nil {
		t.Fatal("DownloadAttachments() accepted a symlinked workspace root")
	}
	if _, err := os.Stat(filepath.Join(outside, "workdir", "manus", "task-1", "result.txt")); !os.IsNotExist(err) {
		t.Fatalf("attachment escaped workspace root: %v", err)
	}
}

func TestRuntimeDownloadRejectsTaskSymlinkOutsideWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	downloadRoot := filepath.Join(workspace, "workdir", "manus")
	if err := os.MkdirAll(downloadRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(downloadRoot, "task-1")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	runtime, cleanup := downloadTestRuntime(t, workspace)
	defer cleanup()

	if _, err := runtime.DownloadAttachments(context.Background(), "task-1", ""); err == nil {
		t.Fatal("DownloadAttachments() accepted a task directory symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "result.txt")); !os.IsNotExist(err) {
		t.Fatalf("attachment escaped through task symlink: %v", err)
	}
}

func TestRuntimeDownloadUsesExclusiveSanitizedNames(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	runtime, cleanup := downloadTestRuntime(t, workspace)
	defer cleanup()

	first, err := runtime.DownloadAttachments(context.Background(), "task-1", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := runtime.DownloadAttachments(context.Background(), "task-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || first[0] == second[0] {
		t.Fatalf("download paths = %#v, %#v", first, second)
	}
	if !strings.HasSuffix(filepath.ToSlash(first[0]), "/task-1/result_.txt") || !strings.HasSuffix(filepath.ToSlash(second[0]), "/task-1/result_-1.txt") {
		t.Fatalf("sanitized paths = %#v, %#v", first, second)
	}
	if goruntime.GOOS != "windows" {
		info, err := os.Stat(first[0])
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("download mode = %v, %v", info.Mode().Perm(), err)
		}
	}
}

func TestClientReturnsAttachmentBytesWithoutWritingFiles(t *testing.T) {
	t.Parallel()

	fileClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("downloaded"))), ContentLength: 10, Header: make(http.Header)}, nil
	})}
	client, _ := NewClient("secret", ClientConfig{BaseURL: "https://api.manus.ai", FileHTTPClient: fileClient})
	payload, err := client.DownloadAttachment(context.Background(), TaskAttachment{
		Filename: "result.txt", URL: "https://203.0.113.11/download",
	}, 1024)
	if err != nil || string(payload) != "downloaded" {
		t.Fatalf("DownloadAttachment() = %q, %v", payload, err)
	}
}

func downloadTestRuntime(t *testing.T, workspace string) (*Runtime, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"task_id":"task-1","messages":[{"id":"event-1","type":"assistant_message","assistant_message":{"attachments":[{"type":"file","filename":"../result?.txt","url":"https://203.0.113.12/result"}]}}]}`))
	}))
	fileClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("result")), ContentLength: 6, Header: make(http.Header)}, nil
	})}
	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL, FileHTTPClient: fileClient})
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "manus.db"))
	_ = ledger.Upsert(context.Background(), TaskRecord{TaskID: "task-1"})
	runtime := NewRuntime(client, ledger, RuntimeConfig{
		Policy: Policy{AllowFileDownloads: true}, WorkspaceDir: workspace,
		DownloadRoot: filepath.Join(workspace, "workdir", "manus"), MaxFileBytes: 1024,
	})
	return runtime, func() {
		_ = ledger.Close()
		server.Close()
	}
}
