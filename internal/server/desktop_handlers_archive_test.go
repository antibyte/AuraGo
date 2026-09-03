package server

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"aurago/internal/desktop"
)

func writeDesktopTestZip(t *testing.T, svc *desktop.Service, dest string, files map[string][]byte) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		writer, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := svc.WriteFileBytes(context.Background(), dest, buf.Bytes(), desktop.SourceUser); err != nil {
		t.Fatalf("WriteFileBytes %s: %v", dest, err)
	}
}

func TestDesktopArchiveEntryStreamsPreviewableFile(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}
	writeDesktopTestZip(t, svc, "Documents/preview.zip", map[string][]byte{
		"notes/hello.txt": []byte("inside zip"),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/archive/entry?path="+url.QueryEscape("Documents/preview.zip")+"&entry="+url.QueryEscape("notes/hello.txt"), nil)
	resp := httptest.NewRecorder()
	handleDesktopArchiveEntry(s).ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Body.String(); got != "inside zip" {
		t.Fatalf("body = %q", got)
	}
	if ct := resp.Header().Get("Content-Type"); ct == "" {
		t.Fatal("missing Content-Type")
	}
}

func TestDesktopArchiveEntryRejectsTraversalAndExecutables(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}
	writeDesktopTestZip(t, svc, "Documents/preview.zip", map[string][]byte{
		"notes/hello.txt": []byte("inside zip"),
		"notes/run.exe":   []byte("MZ"),
	})

	slip := httptest.NewRequest(http.MethodGet, "/api/desktop/archive/entry?path="+url.QueryEscape("Documents/preview.zip")+"&entry="+url.QueryEscape("../hello.txt"), nil)
	slipResp := httptest.NewRecorder()
	handleDesktopArchiveEntry(s).ServeHTTP(slipResp, slip)
	if slipResp.Code != http.StatusBadRequest {
		t.Fatalf("traversal status = %d body = %s", slipResp.Code, slipResp.Body.String())
	}

	exe := httptest.NewRequest(http.MethodGet, "/api/desktop/archive/entry?path="+url.QueryEscape("Documents/preview.zip")+"&entry="+url.QueryEscape("notes/run.exe"), nil)
	exeResp := httptest.NewRecorder()
	handleDesktopArchiveEntry(s).ServeHTTP(exeResp, exe)
	if exeResp.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("exe status = %d body = %s", exeResp.Code, exeResp.Body.String())
	}
}

func TestDesktopViewerContentReadsArchiveMarkdown(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}
	writeDesktopTestZip(t, svc, "Documents/docs.zip", map[string][]byte{
		"guide.md": []byte("# Hello"),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/viewer/content?path="+url.QueryEscape("Documents/docs.zip")+"&entry="+url.QueryEscape("guide.md"), nil)
	resp := httptest.NewRecorder()
	handleDesktopViewerContent(s).ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !bytes.Contains(resp.Body.Bytes(), []byte(`"type":"markdown"`)) && !bytes.Contains(resp.Body.Bytes(), []byte(`"type": "markdown"`)) {
		t.Fatalf("expected markdown viewer payload, got %s", body)
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte("# Hello")) {
		t.Fatalf("missing markdown content: %s", body)
	}
}
