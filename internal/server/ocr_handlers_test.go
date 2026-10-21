package server

import (
	"archive/zip"
	"aurago/internal/config"
	"aurago/internal/ocr"
	"aurago/internal/tools"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type serverOCRRuntime struct{ starts atomic.Int32 }

func (r *serverOCRRuntime) Start(context.Context, bool) error { r.starts.Add(1); return nil }
func (*serverOCRRuntime) Stop(context.Context) error          { return nil }
func (*serverOCRRuntime) Convert(_ context.Context, _ string, _ string, _ string, emit func(ocr.Page) error) error {
	return emit(ocr.Page{Index: 0, Text: "Hello <script>external</script>", Source: "ocr"})
}
func TestOCRDesktopHTTPAuthorityExportAndPassiveStatus(t *testing.T) {
	cfg := &config.Config{OCR: config.DefaultOCRConfig()}
	cfg.OCR.Enabled = true
	rt := &serverOCRRuntime{}
	m, err := ocr.New(filepath.Join(t.TempDir(), "jobs"), ocr.Policy{Enabled: true}, rt)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close(context.Background())
	s := &Server{Cfg: cfg, OCR: m}
	handler := handleDesktopOCR(s)
	call := func(method, path, cookie, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/desktop/ocr/"+path, strings.NewReader(body))
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie})
		}
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	if w := call("GET", "status", "a", ""); w.Code != 200 || rt.starts.Load() != 0 {
		t.Fatal("status started runtime", w.Body.String())
	}
	if err = m.Start(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	f, err := m.Stage("desktop:"+desktopRequestSessionHash(func() *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "a"})
		return r
	}()), "hello.pdf", bytes.NewBufferString("%PDF-1.7\nfixture"))
	if err != nil {
		t.Fatal(err)
	}
	w := call("POST", "jobs", "a", `{"uploads":["`+f.ID+`"],"mode":"auto"}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var job ocr.Job
	if err = json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		w = call("GET", "jobs/"+job.ID, "a", "")
		_ = json.Unmarshal(w.Body.Bytes(), &job)
		if job.FinishedAt != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	for _, tail := range []string{"", "/result?file_id=" + f.ID, "/source?file_id=" + f.ID, "/export?format=zip", "/cancel"} {
		method := "GET"
		if tail == "/cancel" {
			method = "POST"
		}
		w = call(method, "jobs/"+job.ID+tail, "b", "")
		if w.Code != 404 {
			t.Fatal("foreign job accessible", tail, w.Code)
		}
	}
	w = call("GET", "jobs/"+job.ID+"/export?format=zip", "a", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	archive, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, entry := range archive.File {
		found[entry.Name] = true
	}
	if !found["LICENSE-MODELS.txt"] || !found["NOTICE.txt"] || len(found) != 5 {
		t.Fatal(found)
	}
	cfg.VirtualDesktop.ReadOnly = true
	if w = call("POST", "jobs", "a", `{"uploads":[]}`); w.Code != 403 {
		t.Fatal("readonly accepted", w.Code)
	}
	if w = call("POST", "jobs/"+job.ID+"/cancel", "a", "{}"); w.Code != 200 {
		t.Fatal("cleanup blocked", w.Code)
	}
	cfg.OCR.Enabled = false
	if w = call("GET", "status", "a", ""); w.Code != 200 {
		t.Fatal("disabled status hidden")
	}
	if w = call("GET", "jobs", "a", ""); w.Code != 403 {
		t.Fatal("disabled jobs allowed")
	}
}
func TestOCRDesktopUploadStricterLimit(t *testing.T) {
	cfg := &config.Config{OCR: config.DefaultOCRConfig()}
	cfg.OCR.Enabled = true
	cfg.VirtualDesktop.MaxFileSizeMB = 1
	m, err := ocr.New(filepath.Join(t.TempDir(), "jobs"), ocr.Policy{Enabled: true}, &serverOCRRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close(context.Background())
	_ = m.Start(context.Background(), true)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "test.pdf")
	_, _ = io.WriteString(part, "%PDF-1.7\n")
	_, _ = part.Write(make([]byte, 1<<20))
	_ = writer.Close()
	r := httptest.NewRequest("POST", "/api/desktop/ocr/uploads", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	handleDesktopOCR(&Server{Cfg: cfg, OCR: m})(w, r)
	if w.Code != 413 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestOCRWebAttachmentPromptIsIsolated(t *testing.T) {
	cfg := &config.Config{OCR: config.DefaultOCRConfig()}
	cfg.OCR.Enabled = true
	cfg.Directories.WorkspaceDir = t.TempDir()
	attachments := filepath.Join(cfg.Directories.WorkspaceDir, "attachments")
	if err := os.MkdirAll(attachments, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachments, "page.pdf"), []byte("%PDF-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := ocr.New(filepath.Join(t.TempDir(), "jobs"), ocr.Policy{Enabled: true}, &serverOCRRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close(context.Background())
	_ = m.Start(context.Background(), true)
	tools.PublishOCR(m)
	defer tools.WithdrawOCR(m)
	if got := ocrWebAttachments(context.Background(), cfg, "session", "agent_workspace/workdir/attachments/../../secret.pdf"); got != "" {
		t.Fatal("invalid reference enriched", got)
	}
	if got := ocrWebAttachments(context.Background(), cfg, "session", "agent_workspace/workdir/attachments/page.pdf"); !strings.Contains(got, "external_data") || !strings.Contains(got, "job_id") {
		t.Fatal("authorized attachment not isolated", got)
	}
	if !mcpFeatureFlags(&Server{Cfg: cfg}).OCREnabled {
		t.Fatal("MCP bridge lost enabled OCR")
	}
	cfg.OCR.AgentAccess = false
	if mcpFeatureFlags(&Server{Cfg: cfg}).OCREnabled {
		t.Fatal("MCP bridge bypassed revocation")
	}
}
