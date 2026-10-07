package security

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/httporigin"
)

// Fixtures for the SSRF-protected same-origin client. Server A is
// the configured provider and answers with a redirect to server B, a second
// origin on another loopback port. Both are loopback, so the tests enable the
// AURAGO_SSRF_ALLOW_LOOPBACK escape hatch; the SSRF pinning itself still runs.

func TestSSRFProtectedSameOriginClientRejectsCrossOriginRedirect(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	var secondHits atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer second.Close()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", second.URL+r.URL.Path)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer provider.Close()

	req, err := http.NewRequest(http.MethodPost, provider.URL+"/v1/images", strings.NewReader("SECRET-PROMPT"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer sk-test")
	resp, err := NewSSRFProtectedHTTPClientSameOrigin(5 * time.Second).Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
		t.Fatalf("error = %v, want httporigin.ErrCrossOriginRedirect", err)
	}
	if hits := secondHits.Load(); hits != 0 {
		t.Fatalf("second origin received %d request(s)", hits)
	}
}

func TestSSRFProtectedSameOriginClientFollowsSameOriginRedirect(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/v1/images" {
			w.Header().Set("Location", "/v2/images")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "SECRET-PROMPT" || r.Header.Get("Authorization") == "" {
			t.Errorf("same-origin hop lost body or auth: body=%q auth=%q", body, r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/images", strings.NewReader("SECRET-PROMPT"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer sk-test")
	resp, err := NewSSRFProtectedHTTPClientSameOrigin(5 * time.Second).Do(req)
	if err != nil {
		t.Fatalf("same-origin redirect: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("server hits = %d, want 2 (redirect + followed request)", got)
	}
}

// The origin check runs before the SSRF re-check, so a cross-origin target is
// refused without resolving it, and a same-origin hop is still re-validated
// and re-pinned exactly as by NewSSRFProtectedHTTPClient.
func TestSSRFProtectedSameOriginClientChecksOriginThenSSRF(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "0")
	check := NewSSRFProtectedHTTPClientSameOrigin(time.Second).CheckRedirect
	original, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/a", nil)

	crossOrigin, _ := http.NewRequest(http.MethodGet, "http://unresolvable.invalid/b", nil)
	if err := check(crossOrigin, []*http.Request{original}); !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
		t.Fatalf("cross-origin hop: error = %v, want httporigin.ErrCrossOriginRedirect", err)
	}

	sameOrigin, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/b", nil)
	err := check(sameOrigin, []*http.Request{original})
	if err == nil || errors.Is(err, httporigin.ErrCrossOriginRedirect) || !strings.Contains(err.Error(), "SSRF protection") {
		t.Fatalf("same-origin hop to a blocked address: error = %v, want the SSRF rejection", err)
	}
}

func TestSSRFProtectedSameOriginClientKeepsSSRFDialBlocking(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "0")
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer server.Close()

	resp, err := NewSSRFProtectedHTTPClientSameOrigin(5 * time.Second).Get(server.URL)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "SSRF protection") {
		t.Fatalf("loopback request: error = %v, want the SSRF rejection", err)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("blocked loopback server received %d request(s)", got)
	}
}
