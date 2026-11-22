package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/chibisensei"
)

// chibiWriteFile puts a file into the desktop workspace through the same adapter the routes use.
func chibiWriteFile(t *testing.T, s *Server, path, content string) {
	t.Helper()
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := (chibiWorkspace{svc}).WriteFile(context.Background(), path, strings.NewReader(content), 64<<20, true); err != nil {
		t.Fatal(err)
	}
}

func chibiReadFile(t *testing.T, s *Server, path string) string {
	t.Helper()
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	data, err := (chibiWorkspace{svc}).ReadFile(context.Background(), path, 64<<20)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestChibiSenseiImportExportDocuments(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant"})
	base := "/projects/" + pid
	chibiWriteFile(t, s, "Imports/data.csv", "question,answer\nq1,a1\nq2,\n")

	w := chibiDo(t, s, token, http.MethodPost, base+"/imports/preview", map[string]any{"workspace_path": "Imports/data.csv", "format": "auto"})
	chibiWant(t, w, http.StatusOK, "")
	preview := chibiBody(t, w)
	cols := preview["columns"].([]any)
	mapping := preview["mapping"].(map[string]any)
	if preview["format"] != "csv" || len(cols) != 2 || mapping["shape"] != "columns" ||
		len(preview["sample"].([]any)) != 1 || len(preview["issues"].([]any)) != 1 {
		t.Fatalf("preview = %s", w.Body.String())
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/imports/preview", map[string]any{"workspace_path": "Imports/missing.csv", "format": "csv"}),
		http.StatusNotFound, "not_found")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/imports/preview", map[string]any{"workspace_path": "Imports/data.pdf"}),
		http.StatusUnprocessableEntity, "unsupported_file")

	// Format "auto" and a missing mapping are refused before a job exists.
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/imports", map[string]any{"workspace_path": "Imports/data.csv", "format": "auto", "mapping": mapping}),
		http.StatusUnprocessableEntity, "validation_failed")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/imports", map[string]any{"workspace_path": "Imports/data.csv", "format": "csv"}),
		http.StatusUnprocessableEntity, "validation_failed")

	// A running import blocks the next one of the project.
	rt, release, err := s.acquireChibiSensei()
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := rt.jobs.Start(context.Background(), nil, pid, "import_file", nil, 0,
		func(ctx context.Context, _ func(chibisensei.Progress)) (chibisensei.JobResult, error) {
			<-ctx.Done()
			return chibisensei.JobResult{}, ctx.Err()
		})
	release()
	if err != nil {
		t.Fatal(err)
	}
	importBody := map[string]any{"workspace_path": "Imports/data.csv", "format": "csv", "mapping": mapping}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/imports", importBody), http.StatusConflict, "job_active")
	w = chibiDo(t, s, token, http.MethodPost, "/jobs/"+blocker.ID+"/cancel", nil)
	chibiWant(t, w, http.StatusOK, "")
	if job := waitChibiJob(t, s, token, blocker.ID); job.State != chibisensei.JobCancelled {
		t.Fatalf("cancelled job state = %s", job.State)
	}

	w = chibiDo(t, s, token, http.MethodPost, base+"/imports", importBody)
	chibiWant(t, w, http.StatusAccepted, "")
	jobID := chibiBody(t, w)["job"].(map[string]any)["id"].(string)
	job := waitChibiJob(t, s, token, jobID)
	if job.State != chibisensei.JobSucceeded || job.Result.RowsAdded != 1 || job.Result.RowsSkipped != 1 {
		t.Fatalf("import job = %+v", job)
	}
	w = chibiDo(t, s, token, http.MethodGet, base+"/jobs?active=1", nil)
	chibiWant(t, w, http.StatusOK, "")
	if jobs := chibiBody(t, w)["jobs"].([]any); len(jobs) != 0 {
		t.Fatalf("active jobs = %v", jobs)
	}

	// A file over import.max_file_mb is refused with 413.
	cfg := *s.ConfigSnapshot()
	cfg.ChibiSensei.Import.MaxFileMB = 1
	s.replaceConfigSnapshot(&cfg)
	chibiWriteFile(t, s, "Imports/big.csv", "question,answer\n"+strings.Repeat("q,a\n", 600<<10))
	big := map[string]any{"workspace_path": "Imports/big.csv", "format": "csv", "mapping": mapping}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/imports", big), http.StatusRequestEntityTooLarge, "file_too_large")
	cfg.ChibiSensei.Import.MaxFileMB = 50
	s.replaceConfigSnapshot(&cfg)

	// Documents take TXT and Markdown only.
	chibiWriteFile(t, s, "Documents/faq.md", "# FAQ\n\nHow does it work? Like this.\n")
	chibiWriteFile(t, s, "Documents/faq.pdf", "%PDF-1.4")
	w = chibiDo(t, s, token, http.MethodPost, base+"/documents", map[string]any{"workspace_path": "Documents/faq.md"})
	chibiWant(t, w, http.StatusCreated, "")
	doc := chibiBody(t, w)["document"].(map[string]any)
	if doc["chars"].(float64) < 10 || doc["name"] != "faq.md" {
		t.Fatalf("document = %v", doc)
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/documents", map[string]any{"workspace_path": "Documents/faq.pdf"}),
		http.StatusUnprocessableEntity, "unsupported_file")
	w = chibiDo(t, s, token, http.MethodGet, base+"/documents", nil)
	if docs := chibiBody(t, w)["documents"].([]any); len(docs) != 1 {
		t.Fatalf("documents = %s", w.Body.String())
	}
	if w := chibiDo(t, s, token, http.MethodDelete, base+"/documents/"+doc["id"].(string), nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete document = %d", w.Code)
	}

	// Exports: create-only unless the UI confirmed an overwrite.
	export := func(overwrite bool) chibisensei.Job {
		t.Helper()
		w := chibiDo(t, s, token, http.MethodPost, base+"/exports",
			map[string]any{"workspace_path": "Exports/out.jsonl", "which": "all", "overwrite": overwrite})
		chibiWant(t, w, http.StatusAccepted, "")
		return waitChibiJob(t, s, token, chibiBody(t, w)["job"].(map[string]any)["id"].(string))
	}
	if job := export(false); job.State != chibisensei.JobSucceeded || job.Result.Rows != 1 {
		t.Fatalf("export = %+v", job)
	}
	if lines := strings.Split(strings.TrimSpace(chibiReadFile(t, s, "Exports/out.jsonl")), "\n"); len(lines) != 1 {
		t.Fatalf("export file has %d lines", len(lines))
	}
	if job := export(false); job.State != chibisensei.JobFailed || job.ErrorCode != "file_exists" {
		t.Fatalf("second export = %+v", job)
	}
	if job := export(true); job.State != chibisensei.JobSucceeded {
		t.Fatalf("overwrite export = %+v", job)
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/exports", map[string]any{"workspace_path": "Exports/out.txt"}),
		http.StatusUnprocessableEntity, "validation_failed")

	// The event stream greets, then reports the current data revision.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, chibiSenseiAPIBase+base+"/events", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handleChibiSensei(s).ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.HasPrefix(body, ": chibi-sensei events\n\n") || !strings.Contains(body, "event: rows\ndata: {\"data_rev\":") {
		t.Fatalf("event stream = %q", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type = %q", ct)
	}
}

// failingReader delivers some bytes and then fails, like an export whose data changed halfway.
type failingReader struct{ sent bool }

func (f *failingReader) Read(p []byte) (int, error) {
	if !f.sent {
		f.sent = true
		return copy(p, "partial\n"), nil
	}
	return 0, errors.New("stream aborted")
}

// The desktop service stages a stream in a temporary file and renames it only after success
// (internal/desktop/service_file_stream.go), so a failed export keeps the user's previous file.
func TestChibiSenseiFailedWriteKeepsPreviousFile(t *testing.T) {
	s, _, _ := chibiAPIFixture(t)
	chibiWriteFile(t, s, "Exports/keep.jsonl", "previous\n")
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := (chibiWorkspace{svc}).WriteFile(context.Background(), "Exports/keep.jsonl", &failingReader{}, 64<<20, true); err == nil {
		t.Fatal("a failing stream was reported as written")
	}
	if got := chibiReadFile(t, s, "Exports/keep.jsonl"); got != "previous\n" {
		t.Fatalf("file after a failed write = %q", got)
	}
}
