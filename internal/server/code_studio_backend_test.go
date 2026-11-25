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
	t.Skip("Task 3")
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
