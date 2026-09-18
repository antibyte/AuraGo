package tools

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
)

func TestRenderReportHTMLPDFConfiguredRenderer(t *testing.T) {
	const content = "<!doctype html><title>Research</title><p>Source-backed report</p>"
	var validPDF atomic.Bool
	validPDF.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/forms/chromium/convert/html" || r.Method != "POST" {
			t.Errorf("unexpected renderer request: %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		file, header, err := r.FormFile("files")
		if err != nil {
			t.Error(err)
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if header.Filename != "index.html" || string(data) != content || r.FormValue("preferCssPageSize") != "true" {
			t.Error("report content or print options changed")
		}
		if validPDF.Load() {
			io.WriteString(w, "%PDF-1.7\nfixture")
		} else {
			io.WriteString(w, "<html>renderer unavailable</html>")
		}
	}))
	defer srv.Close()
	cfg := &config.GotenbergConfig{URL: srv.URL, Timeout: 5}
	if _, err := RenderReportHTMLPDF(context.Background(), cfg, content); err != nil {
		t.Fatal(err)
	}
	validPDF.Store(false)
	if _, err := RenderReportHTMLPDF(context.Background(), cfg, content); err == nil {
		t.Fatal("accepted HTML as PDF")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RenderReportHTMLPDF(ctx, cfg, content); err == nil {
		t.Fatal("cancelled export succeeded")
	}
}
