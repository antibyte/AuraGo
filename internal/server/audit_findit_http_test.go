package server

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"aurago/internal/findit"
)

func TestFindItOversizedJSONReturnsRequestTooLarge(t *testing.T) {
	s, _ := findItHTTPFixture(t)
	body := strings.Repeat(" ", (128<<10)+1) + `{}`
	r := httptest.NewRequest("POST", "http://localhost"+findItAPI+"/query", strings.NewReader(body))
	r.Header.Set("Origin", "http://localhost")
	w := httptest.NewRecorder()
	handleFindIt(s).ServeHTTP(w, r)
	if w.Code != 413 || !strings.Contains(w.Body.String(), `"code":"request_too_large"`) {
		t.Fatalf("oversized JSON response = %d %s, want 413 request_too_large", w.Code, w.Body)
	}
}

func TestFindItReadErrorsAreSuppressedAfterGrantRevocation(t *testing.T) {
	s, _ := findItHTTPFixture(t)
	admitted := s.ConfigSnapshot()
	next := admitted.Clone()
	next.FindIt.Enabled = false
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(next)
	s.CfgMu.Unlock()

	r := httptest.NewRequest("GET", "http://localhost"+findItAPI+"/catalog", nil)
	w := httptest.NewRecorder()
	s.findItReadError(w, r, admitted, findit.ErrNotFound)
	if w.Code != 403 || !strings.Contains(w.Body.String(), `"code":"policy_changed"`) {
		t.Fatalf("revoked read error = %d %s, want 403 policy_changed", w.Code, w.Body)
	}
}

func TestFindItConcurrentSettingsSavesUseObservedRevision(t *testing.T) {
	s, call := findItHTTPFixture(t)
	revision := findItConfigRevision(s.ConfigSnapshot())
	start := make(chan struct{})
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for _, body := range []map[string]any{
		{"enabled": true, "agent_access": true, "readonly": false, "expected_version": revision},
		{"enabled": false, "agent_access": false, "readonly": false, "expected_version": revision},
	} {
		wg.Add(1)
		go func(body map[string]any) {
			defer wg.Done()
			<-start
			results <- call("POST", "/settings", body).Code
		}(body)
	}
	close(start)
	wg.Wait()
	close(results)
	var saved, conflicted int
	for status := range results {
		switch status {
		case 200:
			saved++
		case 409:
			conflicted++
		default:
			t.Errorf("settings save returned %d, want 200 or 409", status)
		}
	}
	if saved != 1 || conflicted != 1 {
		t.Fatalf("concurrent save outcomes = %d saved, %d conflicted; want one each", saved, conflicted)
	}
}

func TestFindItSettingsCanToggleReadOnlyAndRestoreWriteAccess(t *testing.T) {
	s, call := findItHTTPFixture(t)
	save := func(readonly bool) *httptest.ResponseRecorder {
		return call("POST", "/settings", map[string]any{
			"enabled": true, "agent_access": false, "readonly": readonly,
			"expected_version": findItConfigRevision(s.ConfigSnapshot()),
		})
	}

	if got := save(true); got.Code != 200 {
		t.Fatalf("enable FindIt readonly: %d %s", got.Code, got.Body)
	}
	if cfg := s.ConfigSnapshot(); !cfg.VirtualDesktop.Enabled || !cfg.FindIt.ReadOnly {
		t.Fatalf("readonly save changed unrelated Desktop state: desktop=%v findit_readonly=%v", cfg.VirtualDesktop.Enabled, cfg.FindIt.ReadOnly)
	}
	if got := call("POST", "/query", map[string]any{"operation": "search", "entity": "items"}); got.Code != 200 {
		t.Fatalf("FindIt readonly blocked query: %d %s", got.Code, got.Body)
	}

	if got := save(false); got.Code != 200 {
		t.Fatalf("disable FindIt readonly: %d %s", got.Code, got.Body)
	}
	if cfg := s.ConfigSnapshot(); !cfg.VirtualDesktop.Enabled || cfg.FindIt.ReadOnly {
		t.Fatalf("readonly restore changed unrelated Desktop state: desktop=%v findit_readonly=%v", cfg.VirtualDesktop.Enabled, cfg.FindIt.ReadOnly)
	}
	if got := call("POST", "/query", map[string]any{"operation": "search", "entity": "items"}); got.Code != 200 {
		t.Fatalf("FindIt query after readonly restore: %d %s", got.Code, got.Body)
	}
}
