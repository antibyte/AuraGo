package tools

import (
	"aurago/internal/testutil"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestWebDAVURLPathEscapesSegments(t *testing.T) {
	got, err := webdavURL(WebDAVConfig{URL: "https://dav.example.test/root"}, "/folder with spaces/report?.txt")
	if err != nil {
		t.Fatalf("webdavURL returned error: %v", err)
	}
	want := "https://dav.example.test/root/folder%20with%20spaces/report%3F.txt"
	if got != want {
		t.Fatalf("webdavURL() = %q, want %q", got, want)
	}
}

func TestWebDAVURLEmptyPathKeepsTrailingSlash(t *testing.T) {
	got, err := webdavURL(WebDAVConfig{URL: "https://dav.example.test/root/"}, "")
	if err != nil {
		t.Fatalf("webdavURL returned error: %v", err)
	}
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

func TestWebDAVRejectsUnsafePathsBeforeHTTP(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	var calls int32
	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := WebDAVConfig{URL: srv.URL}
	tests := []struct {
		name string
		run  func() string
	}{
		{"read parent traversal", func() string { return WebDAVRead(cfg, "../secret.txt") }},
		{"list dot segment", func() string { return WebDAVList(cfg, "./folder") }},
		{"mkdir backslash", func() string { return WebDAVMkdir(cfg, `folder\secret`) }},
		{"delete empty segment", func() string { return WebDAVDelete(cfg, "folder//secret") }},
		{"read ambiguous root", func() string { return WebDAVRead(cfg, "///") }},
		{"move unsafe destination", func() string { return WebDAVMove(cfg, "safe.txt", "../secret.txt") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.run()
			if !strings.Contains(out, `"status":"error"`) {
				t.Fatalf("expected error output, got %s", out)
			}
			if !strings.Contains(strings.ToLower(out), "invalid path") {
				t.Fatalf("expected invalid path message, got %s", out)
			}
		})
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("unsafe paths made %d HTTP calls, want 0", got)
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

func TestWebDAVWriteAllowsEmptyContent(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	var body []byte
	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	out := WebDAVWrite(WebDAVConfig{URL: srv.URL}, "/empty.txt", "")
	if !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("expected success output, got %s", out)
	}
	if len(body) != 0 {
		t.Fatalf("PUT body length = %d, want 0", len(body))
	}
}

func TestWebDAVListSkipsSelfResponseByHrefNotPosition(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Fatalf("method = %s, want PROPFIND", r.Method)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(207)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>/root/folder/child%20one.txt</d:href>
    <d:propstat>
      <d:prop>
        <d:getcontentlength>12</d:getcontentlength>
        <d:getcontenttype>text/plain</d:getcontenttype>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
  <d:response>
    <d:href>/root/folder/</d:href>
    <d:propstat>
      <d:prop><d:displayname>folder</d:displayname><d:resourcetype><d:collection/></d:resourcetype></d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`)
	}))
	defer srv.Close()

	out := WebDAVList(WebDAVConfig{URL: srv.URL + "/root"}, "/folder")
	var result FSResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal output %s: %v", out, err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, output = %s", result.Status, out)
	}
	items, ok := result.Data.([]interface{})
	if !ok {
		t.Fatalf("data = %T, want []interface{}", result.Data)
	}
	if len(items) != 1 {
		t.Fatalf("items = %#v, want one child entry", items)
	}
	item, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatalf("item = %T, want map", items[0])
	}
	if item["name"] != "child one.txt" {
		t.Fatalf("name = %v, want child one.txt", item["name"])
	}
}

func TestWebDAVInfoAllowsEmptyRootPath(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Fatalf("method = %s, want PROPFIND", r.Method)
		}
		if got := r.Header.Get("Depth"); got != "0" {
			t.Fatalf("Depth = %q, want 0", got)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(207)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>/root/</d:href>
    <d:propstat>
      <d:prop><d:displayname>root</d:displayname><d:resourcetype><d:collection/></d:resourcetype></d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`)
	}))
	defer srv.Close()

	out := WebDAVInfo(WebDAVConfig{URL: srv.URL + "/root"}, "")
	if !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("expected success output, got %s", out)
	}
}
