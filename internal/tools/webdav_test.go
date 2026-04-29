package tools

import (
	"aurago/internal/testutil"
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestWebDAVURLPathEscapesSegments(t *testing.T) {
	got := webdavURL(WebDAVConfig{URL: "https://dav.example.test/root"}, "/folder with spaces/report?.txt")
	want := "https://dav.example.test/root/folder%20with%20spaces/report%3F.txt"
	if got != want {
		t.Fatalf("webdavURL() = %q, want %q", got, want)
	}
}

func TestWebDAVURLEmptyPathKeepsTrailingSlash(t *testing.T) {
	got := webdavURL(WebDAVConfig{URL: "https://dav.example.test/root/"}, "")
	if got != "https://dav.example.test/root/" {
		t.Fatalf("webdavURL() = %q, want trailing slash", got)
	}
}

func TestWebDAVReadOnlyBlocksDirectMutations(t *testing.T) {
	cfg := WebDAVConfig{URL: "https://dav.example.test/root", ReadOnly: true}

	for name, got := range map[string]string{
		"write":  WebDAVWrite(cfg, "note.txt", "content"),
		"mkdir":  WebDAVMkdir(cfg, "folder"),
		"delete": WebDAVDelete(cfg, "note.txt"),
		"move":   WebDAVMove(cfg, "old.txt", "new.txt"),
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(got, "read-only mode") {
				t.Fatalf("response = %s, want read-only denial", got)
			}
		})
	}
}

func TestWebDAVRequestBasicAuth(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestWebDAVReadRejectsOversizeResponse(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("x"), int(maxHTTPResponseSize+1)))
	}))
	defer srv.Close()

	out := WebDAVRead(WebDAVConfig{URL: srv.URL}, "/big.txt")
	if !strings.Contains(out, `"status":"error"`) {
		t.Fatalf("expected error output, got %s", out)
	}
	if !strings.Contains(out, "exceeds limit") {
		t.Fatalf("expected oversize message, got %s", out)
	}
}

func TestWebDAVWriteRejectsOversizeErrorBody(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write(bytes.Repeat([]byte("y"), int(maxHTTPResponseSize+1)))
	}))
	defer srv.Close()

	out := WebDAVWrite(WebDAVConfig{URL: srv.URL}, "/file.txt", "hello")
	if !strings.Contains(out, `"status":"error"`) {
		t.Fatalf("expected error output, got %s", out)
	}
	if !strings.Contains(out, "exceeds limit") {
		t.Fatalf("expected oversize message, got %s", out)
	}
}
