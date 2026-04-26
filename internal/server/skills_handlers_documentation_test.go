package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// newTestSkillDocServer creates a minimal Server with a fully initialised
// SkillManager backed by a temp dir.  Both ReadOnly and AllowUploads are
// configured through the returned *config.Config so individual tests can
// toggle them.
func newTestSkillDocServer(t *testing.T) (*Server, *tools.SkillManager) {
	t.Helper()
	dbPath := t.TempDir() + "/skills.db"
	db, err := tools.InitSkillsDB(dbPath)
	if err != nil {
		t.Fatalf("InitSkillsDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	skillsDir := t.TempDir()
	mgr := tools.NewSkillManager(db, skillsDir, slog.Default())

	var cfg config.Config
	cfg.Tools.SkillManager.ReadOnly = false
	cfg.Tools.SkillManager.AllowUploads = true

	s := &Server{
		SkillManager: mgr,
		Cfg:          &cfg,
		Logger:       slog.Default(),
	}
	return s, mgr
}

// createTestSkill is a helper that inserts a user-type skill and returns its ID.
func createTestSkillForDoc(t *testing.T, mgr *tools.SkillManager, name string) string {
	t.Helper()
	code := "import sys, json\njson.dump({'ok': True}, sys.stdout)\n"
	entry, err := mgr.CreateSkillEntry(name, "test skill for doc", code, tools.SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry(%q): %v", name, err)
	}
	return entry.ID
}

// ─── GET ──────────────────────────────────────────────────────────────────────

func TestHandleSkillDocumentation_GetEmpty(t *testing.T) {
	t.Parallel()
	s, mgr := newTestSkillDocServer(t)
	id := createTestSkillForDoc(t, mgr, "get_empty")

	req := httptest.NewRequest(http.MethodGet, "/api/skills/"+id+"/documentation", nil)
	rec := httptest.NewRecorder()
	handleSkillDocumentation(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["has_documentation"] != false {
		t.Fatalf("has_documentation: want false, got %v", resp["has_documentation"])
	}
	if resp["content"] != "" {
		t.Fatalf("content: want empty, got %v", resp["content"])
	}
}

func TestHandleSkillDocumentation_GetNotFound(t *testing.T) {
	t.Parallel()
	s, _ := newTestSkillDocServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/skills/nonexistent_id/documentation", nil)
	rec := httptest.NewRecorder()
	handleSkillDocumentation(s).ServeHTTP(rec, req)

	// Unknown ID means GetSkillDocumentation returns a not-found error → 404
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

// ─── PUT ──────────────────────────────────────────────────────────────────────

func TestHandleSkillDocumentation_PutAndGetRoundtrip(t *testing.T) {
	t.Parallel()
	s, mgr := newTestSkillDocServer(t)
	id := createTestSkillForDoc(t, mgr, "put_roundtrip")
	manual := "## My Skill\nDoes stuff.\n"
	body, _ := json.Marshal(map[string]string{"content": manual})

	req := httptest.NewRequest(http.MethodPut, "/api/skills/"+id+"/documentation", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleSkillDocumentation(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	// GET to verify
	req2 := httptest.NewRequest(http.MethodGet, "/api/skills/"+id+"/documentation", nil)
	rec2 := httptest.NewRecorder()
	handleSkillDocumentation(s).ServeHTTP(rec2, req2)

	var resp map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal GET response: %v", err)
	}
	if resp["content"] != manual {
		t.Fatalf("content: want %q, got %v", manual, resp["content"])
	}
	if resp["has_documentation"] != true {
		t.Fatalf("has_documentation: want true, got %v", resp["has_documentation"])
	}
}

func TestHandleSkillDocumentation_Put413(t *testing.T) {
	t.Parallel()
	s, mgr := newTestSkillDocServer(t)
	id := createTestSkillForDoc(t, mgr, "put_413")
	oversize := strings.Repeat("x", tools.MaxSkillDocumentationBytes+1)
	body, _ := json.Marshal(map[string]string{"content": oversize})

	req := httptest.NewRequest(http.MethodPut, "/api/skills/"+id+"/documentation", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleSkillDocumentation(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("PUT status = %d, want 413; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSkillDocumentation_Put403ReadOnly(t *testing.T) {
	t.Parallel()
	s, mgr := newTestSkillDocServer(t)
	id := createTestSkillForDoc(t, mgr, "put_403")

	s.CfgMu.Lock()
	s.Cfg.Tools.SkillManager.ReadOnly = true
	s.CfgMu.Unlock()

	body, _ := json.Marshal(map[string]string{"content": "# manual\n"})
	req := httptest.NewRequest(http.MethodPut, "/api/skills/"+id+"/documentation", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleSkillDocumentation(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

// ─── DELETE ───────────────────────────────────────────────────────────────────

func TestHandleSkillDocumentation_Delete(t *testing.T) {
	t.Parallel()
	s, mgr := newTestSkillDocServer(t)
	id := createTestSkillForDoc(t, mgr, "delete_doc")
	manual := "# Manual\nSome content.\n"
	body, _ := json.Marshal(map[string]string{"content": manual})

	// PUT first
	putReq := httptest.NewRequest(http.MethodPut, "/api/skills/"+id+"/documentation", bytes.NewReader(body))
	putReq.Header.Set("Content-Type", "application/json")
	httptest.NewRecorder() // discard
	handleSkillDocumentation(s).ServeHTTP(httptest.NewRecorder(), putReq)

	// DELETE
	delReq := httptest.NewRequest(http.MethodDelete, "/api/skills/"+id+"/documentation", nil)
	delRec := httptest.NewRecorder()
	handleSkillDocumentation(s).ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200; body=%s", delRec.Code, delRec.Body.String())
	}

	// GET should now show empty
	getReq := httptest.NewRequest(http.MethodGet, "/api/skills/"+id+"/documentation", nil)
	getRec := httptest.NewRecorder()
	handleSkillDocumentation(s).ServeHTTP(getRec, getReq)

	var resp map[string]any
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal GET after delete: %v", err)
	}
	if resp["has_documentation"] != false {
		t.Fatalf("has_documentation: want false after delete, got %v", resp["has_documentation"])
	}
}

func TestHandleSkillDocumentation_Delete403ReadOnly(t *testing.T) {
	t.Parallel()
	s, mgr := newTestSkillDocServer(t)
	id := createTestSkillForDoc(t, mgr, "delete_403")

	s.CfgMu.Lock()
	s.Cfg.Tools.SkillManager.ReadOnly = true
	s.CfgMu.Unlock()

	req := httptest.NewRequest(http.MethodDelete, "/api/skills/"+id+"/documentation", nil)
	rec := httptest.NewRecorder()
	handleSkillDocumentation(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

// ─── UPLOAD ───────────────────────────────────────────────────────────────────

// buildMultipartRequest creates a multipart/form-data POST with a single
// "file" field.
func buildMultipartRequest(t *testing.T, url, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("createFormFile: %v", err)
	}
	if _, err := io.Copy(fw, bytes.NewReader(content)); err != nil {
		t.Fatalf("copy content: %v", err)
	}
	w.Close()
	req := httptest.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestHandleUploadSkillDocumentation_Success(t *testing.T) {
	t.Parallel()
	s, mgr := newTestSkillDocServer(t)
	id := createTestSkillForDoc(t, mgr, "upload_ok")
	manual := []byte("# Manual\nUploaded content.\n")

	req := buildMultipartRequest(t, "/api/skills/"+id+"/documentation/upload", "my_skill.md", manual)
	rec := httptest.NewRecorder()
	handleUploadSkillDocumentation(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	// Verify the content was actually stored
	got, err := mgr.GetSkillDocumentation(id)
	if err != nil {
		t.Fatalf("GetSkillDocumentation: %v", err)
	}
	if got != string(manual) {
		t.Fatalf("content after upload: want %q, got %q", string(manual), got)
	}
}

func TestHandleUploadSkillDocumentation_WrongExtension(t *testing.T) {
	t.Parallel()
	s, mgr := newTestSkillDocServer(t)
	id := createTestSkillForDoc(t, mgr, "upload_badext")

	req := buildMultipartRequest(t, "/api/skills/"+id+"/documentation/upload", "skill.py", []byte("print('hi')"))
	rec := httptest.NewRecorder()
	handleUploadSkillDocumentation(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleUploadSkillDocumentation_TooLarge(t *testing.T) {
	t.Parallel()
	s, mgr := newTestSkillDocServer(t)
	id := createTestSkillForDoc(t, mgr, "upload_big")
	oversize := bytes.Repeat([]byte("x"), tools.MaxSkillDocumentationBytes+1)

	req := buildMultipartRequest(t, "/api/skills/"+id+"/documentation/upload", "big.md", oversize)
	rec := httptest.NewRecorder()
	handleUploadSkillDocumentation(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleUploadSkillDocumentation_Forbidden_ReadOnly(t *testing.T) {
	t.Parallel()
	s, mgr := newTestSkillDocServer(t)
	id := createTestSkillForDoc(t, mgr, "upload_readonly")

	s.CfgMu.Lock()
	s.Cfg.Tools.SkillManager.ReadOnly = true
	s.CfgMu.Unlock()

	req := buildMultipartRequest(t, "/api/skills/"+id+"/documentation/upload", "skill.md", []byte("# hi\n"))
	rec := httptest.NewRecorder()
	handleUploadSkillDocumentation(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleUploadSkillDocumentation_Forbidden_NoUploads(t *testing.T) {
	t.Parallel()
	s, mgr := newTestSkillDocServer(t)
	id := createTestSkillForDoc(t, mgr, "upload_noupload")

	s.CfgMu.Lock()
	s.Cfg.Tools.SkillManager.AllowUploads = false
	s.CfgMu.Unlock()

	req := buildMultipartRequest(t, "/api/skills/"+id+"/documentation/upload", "skill.md", []byte("# hi\n"))
	rec := httptest.NewRecorder()
	handleUploadSkillDocumentation(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

// ─── edge: no SkillManager ───────────────────────────────────────────────────

func TestHandleSkillDocumentation_NoSkillManager(t *testing.T) {
	t.Parallel()
	s := &Server{Logger: slog.Default()}

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/skills/abc/documentation", strings.NewReader("{}"))
			rec := httptest.NewRecorder()
			handleSkillDocumentation(s).ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("%s: status = %d, want 503", method, rec.Code)
			}
		})
	}
}

func TestHandleUploadSkillDocumentation_NoSkillManager(t *testing.T) {
	t.Parallel()
	s := &Server{Logger: slog.Default()}

	req := buildMultipartRequest(t, "/api/skills/abc/documentation/upload", "skill.md", []byte("# hi\n"))
	rec := httptest.NewRecorder()
	handleUploadSkillDocumentation(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
}

// Ensure fmt is used (Go will error on unused imports otherwise).
var _ = fmt.Sprintf
