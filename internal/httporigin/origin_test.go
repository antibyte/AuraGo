package httporigin

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestNewClientRejectsCrossOriginRedirect is the live fixture for the policy
// client: server A answers a credentialed POST with a 307 to server B (another
// loopback port), and B must never see the request, its headers or its body.
func TestNewClientRejectsCrossOriginRedirect(t *testing.T) {
	var secondHits atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits.Add(1)
	}))
	defer second.Close()
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", second.URL+r.URL.Path)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer first.Close()

	client := NewClient(5 * time.Second)
	if client.Timeout != 5*time.Second {
		t.Fatalf("Timeout = %v, want 5s", client.Timeout)
	}
	req, err := http.NewRequest(http.MethodPost, first.URL+"/v1/upload", strings.NewReader("SECRET-BODY"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer sk-test")
	req.Header.Set("cf-aig-authorization", "Bearer gateway-test")
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
	if hits := secondHits.Load(); hits != 0 {
		t.Fatalf("second origin received %d request(s)", hits)
	}
	if !errors.Is(err, ErrCrossOriginRedirect) {
		t.Fatalf("error = %v, want ErrCrossOriginRedirect", err)
	}
}

func TestNewClientFollowsSameOriginRedirectWithBodyAndAuth(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/v1/upload" {
			w.Header().Set("Location", "/v2/upload")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "BODY" || r.Header.Get("Authorization") == "" {
			t.Errorf("same-origin hop lost body or auth: body=%q auth=%q", body, r.Header.Get("Authorization"))
		}
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/upload", strings.NewReader("BODY"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer sk-test")
	resp, err := NewClient(5 * time.Second).Do(req)
	if err != nil {
		t.Fatalf("same-origin redirect: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || hits.Load() != 2 {
		t.Fatalf("status = %d, hits = %d; want 200 after one followed redirect", resp.StatusCode, hits.Load())
	}
}

// NewClient(0) is the policy-only client for calls bounded by the caller's
// context: no client timeout, but the redirect policy still applies.
func TestNewClientZeroTimeoutKeepsPolicy(t *testing.T) {
	client := NewClient(0)
	if client.Timeout != 0 {
		t.Fatalf("Timeout = %v, want none", client.Timeout)
	}
	if client.CheckRedirect == nil {
		t.Fatal("NewClient(0) installed no redirect policy")
	}
	first, _ := http.NewRequest(http.MethodPost, "http://llm.lan:8080/v1/chat/completions", nil)
	cross, _ := http.NewRequest(http.MethodPost, "http://other.lan:8080/v1/chat/completions", nil)
	if err := client.CheckRedirect(cross, []*http.Request{first}); !errors.Is(err, ErrCrossOriginRedirect) {
		t.Fatalf("cross-origin redirect error = %v, want ErrCrossOriginRedirect", err)
	}
}

func TestSameOriginRedirectRequiresExactOrigin(t *testing.T) {
	original, _ := http.NewRequest(http.MethodGet, "https://example.com/api", nil)
	for name, raw := range map[string]string{
		"https downgrade": "http://example.com/api",
		"other port":      "https://example.com:444/api",
		"other host":      "https://sub.example.com/api",
		"userinfo":        "https://user@example.com/api",
	} {
		target, _ := http.NewRequest(http.MethodGet, raw, nil)
		if SameOriginRedirect(target, []*http.Request{original}) == nil {
			t.Errorf("%s: unsafe redirect accepted: %s", name, raw)
		}
	}
	same, _ := http.NewRequest(http.MethodGet, "https://EXAMPLE.com:443/other", nil)
	if err := SameOriginRedirect(same, []*http.Request{original}); err != nil {
		t.Fatalf("same-origin redirect rejected: %v", err)
	}
}

func TestSameOriginRedirectCapsHops(t *testing.T) {
	next, _ := http.NewRequest(http.MethodGet, "https://example.com/hop", nil)
	var via []*http.Request
	for i := 0; i < 9; i++ {
		hop, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("https://example.com/hop%d", i), nil)
		via = append(via, hop)
	}
	if err := SameOriginRedirect(next, via); err != nil {
		t.Fatalf("ninth same-origin redirect rejected: %v", err)
	}
	extra, _ := http.NewRequest(http.MethodGet, "https://example.com/hop9", nil)
	via = append(via, extra)
	if SameOriginRedirect(next, via) == nil {
		t.Fatal("same-origin redirect after 10 requests accepted; the policy stops after 10 redirects")
	}
	if SameOriginRedirect(next, nil) == nil {
		t.Fatal("redirect without an original request accepted")
	}
}

func TestSameOriginRedirectErrorText(t *testing.T) {
	original, _ := http.NewRequest(http.MethodGet, "https://example.com/api", nil)
	target, _ := http.NewRequest(http.MethodGet, "https://evil.example.net/api", nil)
	err := SameOriginRedirect(target, []*http.Request{original})
	if !errors.Is(err, ErrCrossOriginRedirect) {
		t.Fatalf("error = %v, want ErrCrossOriginRedirect", err)
	}
	if err.Error() != "cross-origin integration redirect rejected" {
		t.Fatalf("error text = %q, want the stable rejection text", err.Error())
	}
	wrapped := &url.Error{Op: "Get", URL: target.URL.String(), Err: err}
	if !errors.Is(wrapped, ErrCrossOriginRedirect) {
		t.Fatal("ErrCrossOriginRedirect must survive net/http's *url.Error wrapping")
	}
}

func TestSameOriginUsesEffectivePort(t *testing.T) {
	parse := func(raw string) *url.URL {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	if !SameOrigin(parse("http://lan.example:80/a"), parse("http://LAN.example/b")) {
		t.Error("explicit default HTTP port must match the implicit one")
	}
	if SameOrigin(parse("http://lan.example/a"), parse("https://lan.example/a")) {
		t.Error("scheme change must not count as the same origin")
	}
	if SameOrigin(nil, parse("https://lan.example/a")) {
		t.Error("nil URL must never match")
	}
}
