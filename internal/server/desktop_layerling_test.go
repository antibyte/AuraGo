package server

import (
	"aurago/internal/desktop"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestLayerlingFilesAndPolicy(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	s.Cfg.VirtualDesktop.Layerling.Enabled = true
	s.Cfg.VirtualDesktop.Layerling.AgentAccess = "write"
	s.Cfg.VirtualDesktop.AllowAgentControl = true
	s.Cfg.Tools.VirtualDesktop.Enabled = true
	mux := http.NewServeMux()
	h := registerLayerlingRoutes(mux, s)
	folder := httptest.NewRecorder()
	mux.ServeHTTP(folder, httptest.NewRequest("GET", layerlingBase+"files?path=Documents/Layerling", nil))
	if folder.Code != 200 || !strings.Contains(folder.Body.String(), `"files":[]`) {
		t.Fatal("first-save folder listing", folder.Code, folder.Body.String())
	}
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ListFiles(context.Background(), "Documents/Layerling"); err == nil {
		t.Fatal("folder created before first save")
	}
	call := func(method, path, match string, data []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, layerlingBase+"file?path="+url.QueryEscape(path), bytes.NewReader(data))
		if method == "PUT" {
			if match == "" {
				r.Header.Set("If-None-Match", "*")
			} else {
				r.Header.Set("If-Match", match)
			}
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	original := []byte("original fixture bytes")
	target := "Documents/Layerling/test.lyl"
	if w := call("PUT", target, "", original); w.Code != 200 {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	read := call("GET", target, "", nil)
	if read.Code != 200 || !bytes.Equal(read.Body.Bytes(), original) {
		t.Fatal("read failed")
	}
	version := read.Header().Get("ETag")
	if version != desktop.NoteVersion(original) {
		t.Fatal("wrong etag")
	}
	if w := call("PUT", target, "", []byte("lost")); w.Code != 412 {
		t.Fatalf("create conflict %d", w.Code)
	}
	if w := call("PUT", target, version, []byte("new")); w.Code != 200 {
		t.Fatalf("conditional write %d %s", w.Code, w.Body.String())
	}
	if w := call("PUT", target, version, original); w.Code != 412 {
		t.Fatal("stale write accepted")
	}
	for _, path := range []string{"../outside.lyl", "Documents/Notes/unsafe.lyl", "test.html"} {
		if w := call("PUT", path, "", original); w.Code == 200 {
			t.Fatalf("unsafe path %s accepted", path)
		}
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	if w := call("PUT", "readonly.lyl", "", original); w.Code != 403 {
		t.Fatalf("readonly %d", w.Code)
	}
	if w := call("GET", target, "", nil); w.Code != 200 {
		t.Fatal("readonly blocked read")
	}
	if h.allowed(httptest.NewRequest("GET", "/", nil), true) {
		t.Fatal("agent bypassed readonly")
	}
	s.Cfg.VirtualDesktop.ReadOnly = false
	s.Cfg.VirtualDesktop.Layerling.AgentAccess = "read"
	if !h.allowed(httptest.NewRequest("GET", "/", nil), false) || h.allowed(httptest.NewRequest("GET", "/", nil), true) {
		t.Fatal("agent read policy")
	}
	s.Cfg.VirtualDesktop.Layerling.AgentAccess = "off"
	if h.allowed(httptest.NewRequest("GET", "/", nil), false) {
		t.Fatal("agent off policy")
	}
	s.Cfg.VirtualDesktop.Layerling.Enabled = false
	if w := call("GET", target, "", nil); w.Code != 403 {
		t.Fatal("disabled read accepted")
	}
	svc, _, _ = s.getDesktopService(context.Background())
	data, _, err := svc.ReadFileBytes(context.Background(), target)
	if err != nil || string(data) != "new" {
		t.Fatal("rejected write changed file")
	}
}

type layerlingRevokingReader struct {
	io.Reader
	revoke func()
}

func (r *layerlingRevokingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if r.revoke != nil {
		r.revoke()
		r.revoke = nil
	}
	return n, err
}

func TestLayerlingInterruptedWritesPreserveFile(t *testing.T) {
	for _, mode := range []string{"cancel", "readonly", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			s := newDesktopFilesystemTestServer(t)
			s.Cfg.VirtualDesktop.Layerling.Enabled = true
			h := registerLayerlingRoutes(http.NewServeMux(), s)
			target := layerlingBase + "file?path=Documents/Layerling/protected.lyl"
			create := httptest.NewRequest("PUT", target, strings.NewReader("original"))
			create.Header.Set("If-None-Match", "*")
			w := httptest.NewRecorder()
			h.serve(w, create)
			if w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			body := &layerlingRevokingReader{Reader: strings.NewReader("replacement"), revoke: func() {
				if mode == "cancel" {
					cancel()
					return
				}
				s.CfgMu.Lock()
				defer s.CfgMu.Unlock()
				if mode == "readonly" {
					s.Cfg.VirtualDesktop.ReadOnly = true
				} else {
					s.Cfg.VirtualDesktop.Layerling.Enabled = false
				}
			}}
			r := httptest.NewRequest("PUT", target, body).WithContext(ctx)
			r.Header.Set("If-Match", desktop.NoteVersion([]byte("original")))
			w = httptest.NewRecorder()
			h.serve(w, r)
			if w.Code == 200 {
				t.Fatal("revoked write accepted")
			}
			svc, _, err := s.getDesktopService(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			data, _, err := svc.ReadFileBytes(context.Background(), "Documents/Layerling/protected.lyl")
			if err != nil || string(data) != "original" {
				t.Fatal("interruption damaged original", err)
			}
		})
	}
}
