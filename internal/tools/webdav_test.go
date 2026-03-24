package tools

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebDAVRequestBasicAuth(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Fatal("expected basic auth header")
		}
		if user != "alice" || pass != "secret" {
			t.Fatalf("unexpected credentials: %q / %q", user, pass)
		}
		if got := r.Header.Get("Authorization"); got == "" {
			t.Fatal("expected Authorization header to be set")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := webdavRequest(WebDAVConfig{
		AuthType: "basic",
		Username: "alice",
		Password: "secret",
	}, http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatalf("webdavRequest returned error: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
}

func TestWebDAVRequestBearerAuth(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("unexpected Authorization header: %q", got)
		}
		if _, _, ok := r.BasicAuth(); ok {
			t.Fatal("did not expect basic auth for bearer mode")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := webdavRequest(WebDAVConfig{
		AuthType: "bearer",
		Token:    "token-123",
	}, http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatalf("webdavRequest returned error: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
}
