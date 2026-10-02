package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/tresor"
)

func TestTresorBrowserBoundaryAndRevisions(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = strings.Repeat("s", 32)
	cfg.Directories.DataDir = t.TempDir()
	s := &Server{Cfg: cfg}
	handler := handleDesktopTresor(s)
	call := func(method, path string, body any, cookie, origin bool) *httptest.ResponseRecorder {
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, "https://example.com/api/desktop/tresor"+path, bytes.NewReader(data))
		if cookie {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		}
		if origin {
			r.Header.Set("Origin", "https://example.com")
		}
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("Tresor response may be cached")
		}
		return w
	}
	if w := call("GET", "", nil, false, false); w.Code != 401 {
		t.Fatalf("anonymous: %d", w.Code)
	}
	if w := call("GET", "", nil, true, false); w.Code != 200 || !strings.Contains(w.Body.String(), `"initialized":false`) {
		t.Fatalf("initial: %d %s", w.Code, w.Body.String())
	}
	header := tresor.Header{Salt: make([]byte, 32), PasswordEnvelope: make([]byte, 60), RecoveryEnvelope: make([]byte, 60)}
	if w := call("POST", "", header, true, false); w.Code != 403 {
		t.Fatalf("missing origin: %d", w.Code)
	}
	if w := call("POST", "", header, true, true); w.Code != 204 {
		t.Fatalf("setup: %d %s", w.Code, w.Body.String())
	}
	if w := call("POST", "", header, true, true); w.Code != 412 {
		t.Fatalf("duplicate setup: %d", w.Code)
	}
	if w := call("GET", "", nil, true, false); w.Code != 200 || !strings.Contains(w.Body.String(), `"initialized":true`) {
		t.Fatalf("read header: %d %s", w.Code, w.Body.String())
	}

	id := "a8185ea8-4ec9-447d-bd6c-88cd4ed6228b"
	item := tresor.Record{ID: id, Meta: make([]byte, 28), Body: make([]byte, 28)}
	if w := call("POST", "/items", item, true, true); w.Code != 204 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if w := call("GET", "/items/"+id, nil, true, false); w.Code != 200 || w.Header().Get("ETag") != `"1"` {
		t.Fatalf("read item: %d %s", w.Code, w.Body.String())
	}
	if w := call("GET", "/items", nil, true, false); w.Code != 200 || !strings.Contains(w.Body.String(), id) {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	item.Body[27] = 1
	mutate := func(method string, revision int) *httptest.ResponseRecorder {
		data, _ := json.Marshal(item)
		r := httptest.NewRequest(method, "https://example.com/api/desktop/tresor/items/"+id, bytes.NewReader(data))
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		r.Header.Set("Origin", "https://example.com")
		r.Header.Set("If-Match", strconv.Quote(strconv.Itoa(revision)))
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	if w := mutate("PUT", 1); w.Code != 204 {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	if w := mutate("PUT", 1); w.Code != 412 {
		t.Fatalf("stale update: %d", w.Code)
	}
	if w := mutate("DELETE", 1); w.Code != 412 {
		t.Fatalf("stale delete: %d", w.Code)
	}
	if w := mutate("DELETE", 2); w.Code != 204 {
		t.Fatalf("delete: %d", w.Code)
	}

	if validTresorRecord(tresor.Record{ID: id, Meta: make([]byte, 28), Body: make([]byte, tresorMaxBody+1)}) {
		t.Fatal("oversized encrypted file accepted")
	}
	r := httptest.NewRequest("GET", "https://example.com/api/desktop/tresor", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
	r.Header.Set("Authorization", "Bearer desktop-token")
	w := httptest.NewRecorder()
	handler(w, r)
	if w.Code != 401 {
		t.Fatalf("bearer accepted: %d", w.Code)
	}
	r = httptest.NewRequest("GET", "http://example.com/api/desktop/tresor", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
	w = httptest.NewRecorder()
	handler(w, r)
	if w.Code != 403 {
		t.Fatalf("insecure remote accepted: %d", w.Code)
	}
	r = httptest.NewRequest("GET", "http://localhost/api/desktop/tresor", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
	w = httptest.NewRecorder()
	handler(w, r)
	if w.Code != 200 {
		t.Fatalf("local loopback denied: %d", w.Code)
	}
	r.Header.Set("X-Forwarded-For", "192.0.2.1")
	w = httptest.NewRecorder()
	handler(w, r)
	if w.Code != 403 {
		t.Fatalf("insecure proxy accepted: %d", w.Code)
	}
	cfg.Auth.Enabled = false
	w = httptest.NewRecorder()
	handler(w, r)
	if w.Code != 401 {
		t.Fatalf("auth-disabled vault accepted: %d", w.Code)
	}
}
