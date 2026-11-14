package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestDesktopWriterPDFDownloadsAndSaves(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	previous := desktopWriterPDFRender
	defer func() { desktopWriterPDFRender = previous }()
	var pages []string
	var renderErr error
	desktopWriterPDFRender = func(ctx context.Context, cfg *config.Config, page string) ([]byte, error) {
		pages = append(pages, page)
		if renderErr != nil {
			return nil, renderErr
		}
		return []byte(fmt.Sprintf("%%PDF-1.7 render %d", len(pages))), nil
	}
	handler := handleDesktopWriterPDF(s)
	post := func(query, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, "/api/desktop/office/pdf"+query, strings.NewReader(body)))
		return rec
	}
	stored := func(path string) string {
		svc, _, err := s.getDesktopService(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		data, _, err := svc.ReadFileBytes(context.Background(), path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(data)
	}

	// Without a path the response is the PDF.
	got := post("", "<html>pages</html>")
	if got.Code != http.StatusOK || got.Header().Get("Content-Type") != "application/pdf" || got.Body.String() != "%PDF-1.7 render 1" || pages[0] != "<html>pages</html>" {
		t.Fatalf("download: %d %q %q", got.Code, got.Header().Get("Content-Type"), got.Body)
	}
	if got.Header().Get("Cache-Control") != "no-store" || got.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("download headers: %v", got.Header())
	}
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/office/pdf", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d", rec.Code)
	}
	if got := post("", "  \n"); got.Code != http.StatusBadRequest || len(pages) != 1 {
		t.Fatalf("empty page: %d after %d renders", got.Code, len(pages))
	}

	// With a path the PDF becomes a file: create only, then with overwrite.
	target := "?path=Documents/Exports/report.pdf"
	if got := post(target, "<html>one</html>"); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"path":"Documents/Exports/report.pdf"`) {
		t.Fatalf("save: %d %s", got.Code, got.Body)
	}
	if stored("Documents/Exports/report.pdf") != "%PDF-1.7 render 2" {
		t.Fatalf("saved bytes: %q", stored("Documents/Exports/report.pdf"))
	}
	if got := post(target, "<html>two</html>"); got.Code != http.StatusConflict || !strings.Contains(got.Body.String(), "pdf_exists") || len(pages) != 2 {
		t.Fatalf("existing file: %d %s after %d renders", got.Code, got.Body, len(pages))
	}
	if got := post(target+"&overwrite=1", "<html>three</html>"); got.Code != http.StatusOK || stored("Documents/Exports/report.pdf") != "%PDF-1.7 render 3" {
		t.Fatalf("overwrite: %d %s", got.Code, got.Body)
	}
	if got := post("?path=Documents/Exports/report.docx", "<html>four</html>"); got.Code != http.StatusBadRequest || len(pages) != 3 {
		t.Fatalf("wrong extension: %d after %d renders", got.Code, len(pages))
	}
	if got := post("?path=../outside.pdf", "<html>five</html>"); got.Code != http.StatusBadRequest {
		t.Fatalf("path outside the workspace: %d %s", got.Code, got.Body)
	}

	// No renderer is its own answer; another failure is not.
	renderErr = fmt.Errorf("%w (browser: no binary)", tools.ErrHTMLPDFUnavailable)
	if got := post("", "<html>six</html>"); got.Code != http.StatusServiceUnavailable || !strings.Contains(got.Body.String(), `"error":"pdf_unavailable"`) {
		t.Fatalf("no renderer: %d %s", got.Code, got.Body)
	}
	renderErr = errors.New("render broke")
	if got := post("", "<html>seven</html>"); got.Code != http.StatusBadGateway || !strings.Contains(got.Body.String(), `"error":"pdf_failed"`) || strings.Contains(got.Body.String(), "render broke") {
		t.Fatalf("failed render: %d %s", got.Code, got.Body)
	}
}

func TestDesktopWriterPDFReadOnlyDesktopStillDownloads(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	previous := desktopWriterPDFRender
	defer func() { desktopWriterPDFRender = previous }()
	renders := 0
	desktopWriterPDFRender = func(ctx context.Context, cfg *config.Config, page string) ([]byte, error) {
		renders++
		return []byte("%PDF-1.7"), nil
	}
	// The server's own switch decides; the desktop service follows it on the next request.
	s.CfgMu.Lock()
	s.Cfg.VirtualDesktop.ReadOnly = true
	s.CfgMu.Unlock()
	handler := handleDesktopWriterPDF(s)
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/api/desktop/office/pdf", strings.NewReader("<html>pages</html>")))
	if rec.Code != http.StatusOK || rec.Body.String() != "%PDF-1.7" {
		t.Fatalf("download on a read-only desktop: %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/api/desktop/office/pdf?path=Documents/report.pdf", strings.NewReader("<html>pages</html>")))
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "desktop_readonly") || renders != 1 {
		t.Fatalf("save on a read-only desktop: %d %s after %d renders", rec.Code, rec.Body, renders)
	}
}
