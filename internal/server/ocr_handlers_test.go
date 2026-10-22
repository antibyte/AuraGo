package server

import (
	"archive/zip"
	"aurago/internal/config"
	"aurago/internal/ocr"
	"aurago/internal/tools"
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type serverOCRRuntime struct {
	starts atomic.Int32
	fail   bool
}

func (r *serverOCRRuntime) Start(context.Context, bool) error { r.starts.Add(1); return nil }
func (*serverOCRRuntime) Stop(context.Context) error          { return nil }
func (r *serverOCRRuntime) Convert(_ context.Context, _ string, _ string, _ string, emit func(ocr.Page) error) error {
	if err := emit(ocr.Page{Index: 0, Text: "Hello <script>external</script>", Markdown: "**Hello**", Source: "ocr", Note: "Recognition warning"}); err != nil {
		return err
	}
	if r.fail {
		return errors.New("second page failed")
	}
	return nil
}

func TestOCRPartialExportsPreserveWarnings(t *testing.T) {
	m, err := ocr.New(filepath.Join(t.TempDir(), "jobs"), ocr.Policy{Enabled: true}, &serverOCRRuntime{fail: true})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close(context.Background())
	if err = m.Start(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	file, err := m.Stage("owner", "scan.pdf", bytes.NewBufferString("%PDF-1.7\nfixture"))
	if err != nil {
		t.Fatal(err)
	}
	job, err := m.Submit("owner", []string{file.ID}, "auto")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err = tools.WaitOCRText(ctx, m, "owner", job.ID); err == nil || ctx.Err() != nil {
		t.Fatal("expected a completed partial job", err)
	}
	check := func(name, text string) {
		t.Helper()
		for _, want := range []string{"OCR incomplete", "state=partial", "ocr_runtime_failed", "returned_pages=1", "page=1", "Recognition warning", "Hello"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s export omitted %q: %s", name, want, text)
			}
		}
	}
	for _, format := range []string{"text", "md", "zip"} {
		t.Run(format, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/export?format="+format, nil)
			exportOCR(w, r, m, "owner", job.ID, file.ID)
			if w.Code != http.StatusOK {
				t.Fatal(w.Code, w.Body.String())
			}
			if format != "zip" {
				check(format, w.Body.String())
				return
			}
			archive, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
			if err != nil {
				t.Fatal(err)
			}
			checked := 0
			for _, entry := range archive.File {
				if !strings.HasSuffix(entry.Name, ".txt") && !strings.HasSuffix(entry.Name, ".md") || entry.Name == "NOTICE.txt" || entry.Name == "LICENSE-MODELS.txt" {
					continue
				}
				source, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(source)
				source.Close()
				if err != nil {
					t.Fatal(err)
				}
				check(entry.Name, string(data))
				checked++
			}
			if checked != 2 {
				t.Fatal("missing text/Markdown ZIP entries", checked)
			}
		})
	}
}

func TestOCRTextExportKeepsFailureAndPageNotes(t *testing.T) {
	for _, state := range []string{"error", "cancelled", "partial"} {
		file := ocr.File{State: state, Error: "ocr_timeout"}
		if got := ocrFileText(file, "text"); !strings.Contains(got, "state="+state) || !strings.Contains(got, "ocr_timeout") {
			t.Fatal("empty incomplete export lost its status", got)
		}
	}
	file := ocr.File{State: "completed", Pages: []ocr.Page{{Index: 1, Source: "error"}}}
	if got := ocrFileText(file, "md"); !strings.Contains(got, "page=2; source=error") {
		t.Fatal("failed page omitted without a note", got)
	}
	file = ocr.File{State: "completed", Pages: []ocr.Page{{Index: 0, Text: "complete", Markdown: "**complete**"}}}
	if got := ocrFileText(file, "text"); got != "complete\n\n" {
		t.Fatal("complete export changed", got)
	}
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
