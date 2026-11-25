package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestCodeStudioSSHBackendFiles(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	b, _ := newCodeStudioTestSSHBackend(t, f, "/home/dev/proj")
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(b.Write(ctx, "/workspace/src/main.go", []byte("package main\n"), false))
	if err := b.Write(ctx, "/workspace/src/main.go", []byte("x"), true); !errors.Is(err, errCodeStudioExists) {
		t.Fatalf("createOnly: %v", err)
	}
	must(b.Write(ctx, "/workspace/src/main.go", []byte("package main // v2\n"), false))
	must(b.Write(ctx, "/workspace/README.md", []byte("# hi\n"), true))
	entries, err := b.List(ctx, "/workspace")
	must(err)
	if len(entries) != 2 || entries[0].Path != "/workspace/src" || entries[0].Type != "directory" || entries[1].Name != "README.md" {
		t.Fatalf("%+v", entries)
	}
	src, _ := b.List(ctx, "/workspace/src")
	for _, e := range src {
		if strings.HasPrefix(e.Name, ".aurago-save-") {
			t.Fatal("temp file left")
		}
	}
	probe, err := b.Probe(ctx, "/workspace/src/main.go")
	must(err)
	if probe.Path != "/workspace/src/main.go" || probe.Size != int64(len("package main // v2\n")) {
		t.Fatalf("%+v", probe)
	}
	data, err := b.Read(ctx, probe.Path, 1<<20)
	must(err)
	if string(data) != "package main // v2\n" {
		t.Fatalf("%q", data)
	}
	var se *codeStudioStatusError
	if _, err := b.Read(ctx, probe.Path, 3); !errors.As(err, &se) || se.Status != 413 {
		t.Fatalf("limit: %v", err)
	}
	var rej codeStudioProbeRejection
	if _, err := b.Probe(ctx, "/workspace/src"); !errors.As(err, &rej) {
		t.Fatalf("dir probe: %v", err)
	}
	must(b.Mkdir(ctx, "/workspace/empty/nested"))
	must(b.Move(ctx, "/workspace/src/main.go", "/workspace/cmd/app/main.go"))
	if _, err := b.Probe(ctx, "/workspace/cmd/app/main.go"); err != nil {
		t.Fatal(err)
	}
	must(b.Delete(ctx, "/workspace/cmd"))
	if err := b.Delete(ctx, "/workspace"); err == nil {
		t.Fatal("root delete allowed")
	}
	entries, _ = b.List(ctx, "/workspace")
	for _, e := range entries {
		if e.Name == "cmd" {
			t.Fatal("recursive delete left cmd")
		}
	}
}

func TestCodeStudioSSHBackendExecGitSearch(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	f.exec = func(cmd string, stdin []byte) (string, uint32) {
		s := unwrapShC(cmd)
		switch {
		case strings.Contains(s, "uname"):
			return "os=Linux\narch=aarch64\nshell=/bin/zsh\nhas=git\nhas=grep\nhas=clangd\n", 0
		case strings.Contains(s, "'grep'"):
			return "/home/dev/proj/src/main.go\x0012:func main() {\n/home/dev/proj/README.md\x003:main\n", 0
		case strings.Contains(s, "'gofmt'"):
			return strings.ToUpper(string(stdin)), 3
		}
		return "ok\n", 0
	}
	b, _ := newCodeStudioTestSSHBackend(t, f, "/home/dev/proj")
	ctx := context.Background()
	res, err := b.Exec(ctx, []string{"gofmt"}, "", []byte("package  main\n"), 5*time.Second)
	if err != nil || res.ExitCode != 3 || res.Output != "PACKAGE  MAIN\n" {
		t.Fatalf("%+v %v", res, err)
	}
	if got := unwrapShC(f.lastExec()); got != "cd '/home/dev/proj' && exec 'gofmt'" {
		t.Fatalf("%q", got)
	}
	if _, err := b.Exec(ctx, []string{"sh", "-c", "make"}, "/workspace/src", nil, time.Second); err != nil {
		t.Fatal(err)
	}
	if got := unwrapShC(f.lastExec()); !strings.HasPrefix(got, "cd '/home/dev/proj/src' && exec 'sh' '-c' 'make'") {
		t.Fatalf("%q", got)
	}
	if _, err := b.Git(ctx, time.Second, "status", "--porcelain=v1", "-z"); err != nil {
		t.Fatal(err)
	}
	if got := unwrapShC(f.lastExec()); !strings.Contains(got, "'git' '-C' '/home/dev/proj' '--literal-pathspecs' 'status'") || !strings.Contains(got, "'LC_ALL=C'") {
		t.Fatalf("%q", got)
	}
	results, truncated, err := b.Search(ctx, codeStudioSearchOptions{Query: "main", Path: "/workspace"})
	if err != nil || truncated || len(results) != 2 || results[0].Path != "/workspace/src/main.go" || results[0].Line != 12 || results[1].Path != "/workspace/README.md" {
		t.Fatalf("%+v %v %v", results, truncated, err)
	}
	if got := unwrapShC(f.lastExec()); !strings.Contains(got, "'--null'") || !strings.Contains(got, "'--' '/home/dev/proj'") {
		t.Fatalf("%q", got)
	}
	caps, err := b.probeCapabilities(ctx)
	if err != nil || caps.Arch != "aarch64" || !caps.Git || !caps.LSP.CPP || caps.LSP.Go {
		t.Fatalf("%+v %v", caps, err)
	}
}

func TestCodeStudioSSHBackendTerminalAndProcess(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	b, _ := newCodeStudioTestSSHBackend(t, f, "/home/dev/proj")
	ctx := context.Background()
	readUntil := func(r io.Reader, want string) {
		t.Helper()
		got := make(chan string, 1)
		go func() { buf := make([]byte, len(want)); _, _ = io.ReadFull(r, buf); got <- string(buf) }()
		select {
		case s := <-got:
			if s != want {
				t.Fatalf("read %q want %q", s, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("read timeout")
		}
	}
	term, err := b.OpenTerminal(ctx, 100, 40)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := term.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	readUntil(term, "hello")
	if err := term.Resize(ctx, 120, 50); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		f.mu.Lock()
		w := append([][2]int(nil), f.windows...)
		f.mu.Unlock()
		if len(w) >= 2 && w[0] == [2]int{100, 40} && w[len(w)-1] == [2]int{120, 50} {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("windows %v", w)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(f.lastExec(), "aurago-terminal") {
		t.Fatal(f.lastExec())
	}
	_ = term.Close()
	proc, err := b.StartProcess(ctx, []string{"aurago-echo"}, "")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = io.Copy(io.Discard, proc.Stderr()) }()
	_, _ = proc.Write([]byte("ping\n"))
	readUntil(proc, "ping\n")
	_ = proc.CloseWrite()
	if err := proc.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestCodeStudioBackendForSSHWorkspaceServesHandlers(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	_, remote := newCodeStudioTestSSHBackend(t, f, "/home/dev/proj")
	h := codeStudioHandlers{server: s, docker: &recordingCodeStudioDockerAPI{}, remote: remote}
	store, _ := h.workspaceStore(context.Background())
	rec, _ := store.Upsert(context.Background(), "dev1", "/home/dev/proj", "", nil)
	if err := f.adminSFTP(t).MkdirAll("/home/dev/proj/pkg"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/code-studio/files?path=/workspace", nil)
	req.Header.Set(codeStudioWorkspaceHeader, rec.ID)
	w := httptest.NewRecorder()
	h.handleFiles(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"/workspace/pkg"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder() // query form, write + read back
	h.handleFile(w, httptest.NewRequest(http.MethodPut, "/api/code-studio/file?ws="+rec.ID, strings.NewReader(`{"path":"/workspace/a.txt","content":"hi"}`)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.handleFile(w, httptest.NewRequest(http.MethodGet, "/api/code-studio/file?ws="+rec.ID+"&path=/workspace/a.txt", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"content":"hi"`) {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	h.handleLSP(w, httptest.NewRequest(http.MethodGet, "/api/code-studio/lsp?lang=go&ws="+rec.ID, nil))
	if w.Code == 200 || w.Header().Get("Upgrade") != "" {
		t.Fatal("LSP over SSH must not upgrade in Plan A")
	}
}

func TestCodeStudioBackendByIDResolvesWithoutDialing(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	dials := 0
	remote := newCodeStudioRemote(func(context.Context, string, string) (*ssh.Client, codeStudioSSHTarget, error) {
		dials++
		return nil, codeStudioSSHTarget{}, errors.New("must not dial")
	}, slog.Default())
	t.Cleanup(remote.pool.closeAll)
	h := &codeStudioHandlers{server: s, docker: &recordingCodeStudioDockerAPI{}, remote: remote}
	ctx := context.Background()
	for _, id := range []string{"", "local"} {
		b, err := h.codeStudioBackendByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := b.(*codeStudioLocalBackend); !ok {
			t.Fatalf("%q -> %T", id, b)
		}
	}
	store, err := h.workspaceStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := store.Upsert(ctx, "dev1", "/home/dev/proj", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := h.codeStudioBackendByID(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if info := b.Info(); info.ID != rec.ID || info.Kind != codeStudioWorkspaceKindSSH || info.Connected || info.Root != "/home/dev/proj" {
		t.Fatalf("%+v", info)
	}
	if dials != 0 {
		t.Fatalf("resolving dialed %d times", dials)
	}
	_, err = h.codeStudioBackendByID(ctx, "ssh_missing")
	var re *codeStudioRemoteError
	if !errors.Is(err, errCodeStudioWorkspaceNotFound) || !errors.As(err, &re) || re.Status != http.StatusNotFound || re.Code != "workspace_not_found" {
		t.Fatalf("unknown id: %v", err)
	}
}
