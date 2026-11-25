package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCodeStudioWorkspaceIDFromRequest(t *testing.T) {
	cases := []struct {
		header, query, want string
		wantErr             bool
	}{
		{"", "", "local", false}, {"local", "", "local", false}, {"", "ssh_ab12", "ssh_ab12", false},
		{"ssh_h", "ssh_q", "ssh_h", false}, {"../x", "", "", true}, {"", strings.Repeat("a", 65), "", true},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/api/code-studio/files?ws="+url.QueryEscape(c.query), nil)
		if c.header != "" {
			r.Header.Set(codeStudioWorkspaceHeader, c.header)
		}
		got, err := codeStudioWorkspaceID(r)
		if (err != nil) != c.wantErr || got != c.want {
			t.Fatalf("%+v: got %q err %v", c, got, err)
		}
	}
}

func TestCodeStudioErrorWritersShapes(t *testing.T) {
	rec := httptest.NewRecorder()
	writeCodeStudioOpError(rec, &codeStudioRemoteError{Status: 409, Code: "unknown_host_key", Message: "Unknown SSH host key for h", Host: "h", KeyType: "ssh-ed25519", Fingerprint: "SHA256:x"})
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != 409 || body["code"] != "unknown_host_key" || body["fingerprint"] != "SHA256:x" || body["key_type"] != "ssh-ed25519" || body["host"] != "h" {
		t.Fatalf("%d %v", rec.Code, body)
	}
	rec = httptest.NewRecorder()
	writeCodeStudioOpError(rec, codeStudioExistsError("file already exists"))
	if rec.Code != 409 || strings.TrimSpace(rec.Body.String()) != `{"error":"file already exists"}` {
		t.Fatal(rec.Body.String())
	}
	if !errors.Is(codeStudioExistsError("x"), errCodeStudioExists) {
		t.Fatal("exists sentinel")
	}
	rec = httptest.NewRecorder()
	writeCodeStudioBackendError(rec, codeStudioUnavailableError{errors.New("docker down")})
	if rec.Code != 503 {
		t.Fatal(rec.Code)
	}
}

func TestCodeStudioWorkspaceChangeHook(t *testing.T) {
	var got []string
	unregister := registerCodeStudioWorkspaceChangeHook(func(ws string, p []string) { got = append(got, ws+":"+strings.Join(p, ",")) })
	codeStudioHandlers{}.notifyWorkspaceChange("local", "/workspace/a")
	unregister()
	codeStudioHandlers{}.notifyWorkspaceChange("local", "/workspace/b")
	if len(got) != 1 || got[0] != "local:/workspace/a" {
		t.Fatal(got)
	}
}

func TestCodeStudioExecArgvMatchesLegacyNormalize(t *testing.T) {
	for _, c := range []struct {
		cmd  string
		args []string
		cwd  string
	}{
		{"go test", nil, ""}, {"go test", nil, "/workspace/pkg"}, {"", []string{"gofmt"}, "/workspace/x"},
	} {
		legacy, _ := normalizeCodeStudioExecCommand(c.cmd, c.args, c.cwd)
		argv, vcwd, err := codeStudioExecArgv(c.cmd, c.args, c.cwd)
		if err != nil {
			t.Fatal(err)
		}
		got := codeStudioLocalExecCommand(argv, vcwd)
		if !reflect.DeepEqual(got, legacy) {
			t.Fatalf("got %q want %q", got, legacy)
		}
	}
}

func TestCodeStudioBackendForSelectsLocalOrRejects(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	h := codeStudioHandlers{server: s, docker: &recordingCodeStudioDockerAPI{}}
	for _, ws := range []string{"", "local"} {
		r := httptest.NewRequest(http.MethodGet, "/api/code-studio/files", nil)
		if ws != "" {
			r.Header.Set(codeStudioWorkspaceHeader, ws)
		}
		b, err := h.backendFor(r)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := b.(*codeStudioLocalBackend); !ok || b.Info().ID != "local" {
			t.Fatalf("%T", b)
		}
	}
	rec := httptest.NewRecorder()
	h.handleFiles(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/files?ws=ssh_missing", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"workspace_not_found"`) {
		t.Fatal(rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.handleFiles(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/files?ws=..%2Fx", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatal(rec.Code)
	}
}

func TestCodeStudioRemotePathMapping(t *testing.T) {
	cases := []struct {
		root, v, want string
		err           bool
	}{
		{"/home/u/p", "/workspace", "/home/u/p", false}, {"/home/u/p", "/workspace/src/a.go", "/home/u/p/src/a.go", false},
		{"/", "/workspace/etc", "/etc", false}, {"/home/u/p", "/workspace/../x", "", true}, {"/home/u/p", "/etc/passwd", "", true},
	}
	for _, c := range cases {
		got, err := codeStudioRemotePath(c.root, c.v)
		if (err != nil) != c.err || got != c.want {
			t.Fatalf("%+v -> %q %v", c, got, err)
		}
	}
	if v, _ := codeStudioVirtualPath("/home/u/p", "/home/u/p/src/a.go"); v != "/workspace/src/a.go" {
		t.Fatal(v)
	}
	if _, err := codeStudioVirtualPath("/home/u/p", "/home/u/proj2/a"); err == nil {
		t.Fatal("prefix sibling accepted")
	}
	if r, _ := normalizeCodeStudioRemoteRoot(`~/code\x`, "/home/u"); r != "/home/u/code/x" {
		t.Fatal(r)
	}
	if _, err := normalizeCodeStudioRemoteRoot("rel/dir", "/home/u"); err == nil {
		t.Fatal("relative accepted")
	}
}

func TestCodeStudioCapabilitiesAndGrepRewrite(t *testing.T) {
	caps := parseCodeStudioCapabilities("os=Linux\narch=x86_64\nshell=/bin/bash\nhas=git\nhas=gopls\nhas=gofmt\n", time.Unix(1, 0))
	if caps.OS != "Linux" || !caps.Git || caps.Grep || !caps.LSP.Go || !caps.Formatters["gofmt"] || caps.LSP.CPP {
		t.Fatalf("%+v", caps)
	}
	out := rewriteCodeStudioGrepPaths("/home/u/p", "/home/u/p/a.go\x003:x\n/etc/passwd\x001:root\n/home/u/p/b\x00partial")
	if out != "/workspace/a.go\x003:x\n" {
		t.Fatalf("%q", out)
	}
	args := strings.Join(codeStudioSSHGrepArgs(codeStudioSearchOptions{Query: "q", Exclude: "node_modules"}, "/d"), " ")
	for _, want := range []string{"--null", "-H", "-F", "-i", "--exclude-dir=node_modules", "-e q -- /d"} {
		if !strings.Contains(args, want) {
			t.Fatal(args)
		}
	}
}
