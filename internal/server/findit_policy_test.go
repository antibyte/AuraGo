package server

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/findit"
)

func TestFindItBearerScopes(t *testing.T) {
	s, _ := findItHTTPFixture(t)
	auth, readToken, writeToken := testDesktopPermissionServer(t)
	s.TokenManager = auth.TokenManager
	s.Cfg.Auth = auth.Cfg.Auth
	adminToken, _, err := s.TokenManager.Create("FindIt desktop admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, token, path, body string
		want                    int
	}{
		{"read query", readToken, "/query", `{"operation":"search","entity":"items"}`, 200},
		{"read mutation", readToken, "/command", `{}`, 403},
		{"write cannot read", writeToken, "/query", `{"operation":"search","entity":"items"}`, 403},
		{"desktop admin cannot configure global grant", adminToken, "/settings", `{}`, 403},
		{"invalid bearer", "invalid", "/query", `{}`, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", findItAPI+tc.path, strings.NewReader(tc.body))
			r.Header.Set("Authorization", "Bearer "+tc.token)
			w := httptest.NewRecorder()
			handleFindIt(s).ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
		})
	}
}

type findItCallbackReader struct {
	io.Reader
	callback func()
}

func (r *findItCallbackReader) Read(p []byte) (int, error) {
	if r.callback != nil {
		callback := r.callback
		r.callback = nil
		callback()
	}
	return r.Reader.Read(p)
}

func TestFindItRevocationDuringRequest(t *testing.T) {
	for _, operation := range []string{"query", "command"} {
		t.Run(operation, func(t *testing.T) {
			s, _ := findItHTTPFixture(t)
			payload := `{"operation":"search","entity":"items"}`
			if operation == "command" {
				payload = `{"operation":"create","entity":"items","request_id":"revoked-create","data":{"name":"Must not be committed","unit":"piece"}}`
			}
			body := &findItCallbackReader{Reader: strings.NewReader(payload), callback: func() {
				next := s.ConfigSnapshot().Clone()
				next.FindIt.Enabled = false
				s.CfgMu.Lock()
				s.replaceConfigSnapshot(next)
				s.CfgMu.Unlock()
			}}
			r := httptest.NewRequest("POST", "http://localhost"+findItAPI+"/"+operation, body)
			r.Header.Set("Origin", "http://localhost")
			w := httptest.NewRecorder()
			handleFindIt(s).ServeHTTP(w, r)
			if w.Code < 400 {
				t.Fatalf("revoked %s returned %d %s", operation, w.Code, w.Body)
			}
			result, err := s.findIt.store.Query(context.Background(), findit.QueryRequest{Operation: "search", Entity: "items"})
			if err != nil {
				t.Fatal(err)
			}
			if result.(findit.SearchResult).Total != 0 {
				t.Fatal("revoked write committed an item")
			}
		})
	}
}

func TestFindItImagesRejectDirectoryLinksAndTraversal(t *testing.T) {
	s, call := findItHTTPFixture(t)
	for _, ref := range []string{"../config.yaml", "a/../../config.yaml", strings.Repeat("a", 32) + "/../config.yaml", strings.Repeat("a", 32) + "/other"} {
		if got := call("GET", "/images/"+ref, nil); got.Code != 404 {
			t.Fatalf("invalid image reference %q: %d", ref, got.Code)
		}
	}
	outside := t.TempDir()
	id := strings.Repeat("a", 32)
	if err := os.WriteFile(filepath.Join(outside, id+".png"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(s.findIt.dataDir, "findit-images")); err != nil {
		t.Skip("directory symlinks unavailable:", err)
	}
	if got := call("GET", "/images/"+id, nil); got.Code != 404 {
		t.Fatal("image directory symlink exposed external data")
	}
}

func TestFindItRuntimeRootChangeRequiresRestart(t *testing.T) {
	s, call := findItHTTPFixture(t)
	s.Cfg.Directories.DataDir = t.TempDir()
	status := s.findItStatus(httptest.NewRequest("GET", findItAPI+"/status", nil))
	if status["ready"] != false || status["can_write"] != false {
		t.Fatalf("stale runtime advertised writable: %#v", status)
	}
	if got := call("POST", "/query", map[string]any{"operation": "search", "entity": "items"}); got.Code != 503 {
		t.Fatalf("old root returned after data root change: %d", got.Code)
	}
}
