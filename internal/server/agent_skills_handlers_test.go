package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func newTestAgentSkillServer(t *testing.T) (*Server, *http.ServeMux) {
	t.Helper()
	tmp := t.TempDir()
	db, err := tools.InitAgentSkillsDB(filepath.Join(tmp, "skills.db"))
	if err != nil {
		t.Fatalf("InitAgentSkillsDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mgr := tools.NewAgentSkillManager(db, filepath.Join(tmp, "agent_skills"), filepath.Join(tmp, "workspace"), slog.Default())
	cfg := &config.Config{}
	cfg.WebConfig.Enabled = true
	cfg.Tools.SkillManager.Enabled = true
	cfg.Tools.SkillManager.AllowUploads = true
	cfg.Tools.SkillManager.MaxUploadSizeMB = 1
	s := &Server{
		Cfg:               cfg,
		Logger:            slog.Default(),
		AgentSkillManager: mgr,
	}
	mux := http.NewServeMux()
	s.registerToolAPIRoutes(mux)
	return s, mux
}

func decodeAgentSkillResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return out
}

func writeAgentSkillZip(dst io.Writer, name string) error {
	zw := zip.NewWriter(dst)
	files := map[string]string{
		name + "/SKILL.md":       "---\nname: " + name + "\ndescription: Zip API skill. Use when importing Agent Skills.\n---\n# Zip\n",
		name + "/scripts/run.py": "print('zip')\n",
	}
	for path, content := range files {
		w, err := zw.Create(path)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

func TestAgentSkillAPICreateListFileVerifyAndEnable(t *testing.T) {
	_, mux := newTestAgentSkillServer(t)

	createBody := `{"name":"api-skill","description":"API test skill. Use when testing Agent Skills.","body":"# API Skill\nRun scripts/echo.py."}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills", strings.NewReader(createBody)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	created := decodeAgentSkillResponse(t, rec)
	skill := created["skill"].(map[string]interface{})
	id := skill["id"].(string)
	if skill["enabled"].(bool) {
		t.Fatal("new Agent Skill should be disabled")
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills/"+id+"/files", strings.NewReader(`{"path":"scripts/echo.py","content":"print('ok')\n"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("write file status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills/"+id+"/verify", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("verify status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/agent-skills/"+id, strings.NewReader(`{"enabled":true}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("enable status=%d body=%s", rec.Code, rec.Body.String())
	}
	updated := decodeAgentSkillResponse(t, rec)["skill"].(map[string]interface{})
	if !updated["enabled"].(bool) {
		t.Fatal("clean Agent Skill should enable")
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/agent-skills?enabled=true", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	listed := decodeAgentSkillResponse(t, rec)
	if int(listed["count"].(float64)) != 1 {
		t.Fatalf("count=%v, want 1", listed["count"])
	}
}

func TestAgentSkillAPIWarningApprovalAndZipImport(t *testing.T) {
	_, mux := newTestAgentSkillServer(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills", strings.NewReader(`{"name":"warn-api","description":"Warn API skill. Use when warning.","body":"# Warn\nReads /etc/passwd."}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create warning status=%d body=%s", rec.Code, rec.Body.String())
	}
	id := decodeAgentSkillResponse(t, rec)["skill"].(map[string]interface{})["id"].(string)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/agent-skills/"+id, strings.NewReader(`{"enabled":true}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("warning enable status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills/"+id+"/approve-warning", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("approve status=%d body=%s", rec.Code, rec.Body.String())
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "zip-api.zip")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if err := writeAgentSkillZip(fw, "zip-api"); err != nil {
		t.Fatalf("writeAgentSkillZip: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/agent-skills/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("zip import status=%d body=%s", rec.Code, rec.Body.String())
	}
	imported := decodeAgentSkillResponse(t, rec)["skill"].(map[string]interface{})
	if imported["name"] != "zip-api" {
		t.Fatalf("imported name=%v, want zip-api", imported["name"])
	}
}

func TestAgentSkillAPICreateAutoEnableClean(t *testing.T) {
	s, mux := newTestAgentSkillServer(t)
	s.Cfg.Tools.SkillManager.AutoEnableClean = true
	createBody := `{"name":"auto-clean","description":"Auto enable test. Use when testing auto enable.","body":"# Clean\nNo risky content."}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills", strings.NewReader(createBody)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	created := decodeAgentSkillResponse(t, rec)
	skill := created["skill"].(map[string]interface{})
	if !skill["enabled"].(bool) {
		t.Fatal("expected skill.enabled true with AutoEnableClean")
	}
}

func TestAgentSkillAPICreateAutoEnableCleanOffLeavesDisabled(t *testing.T) {
	s, mux := newTestAgentSkillServer(t)
	s.Cfg.Tools.SkillManager.AutoEnableClean = false
	createBody := `{"name":"auto-off","description":"Auto off test. Use when testing auto enable off.","body":"# Clean\nNo risky content."}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills", strings.NewReader(createBody)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	created := decodeAgentSkillResponse(t, rec)
	skill := created["skill"].(map[string]interface{})
	if skill["enabled"].(bool) {
		t.Fatal("expected skill.enabled false when AutoEnableClean off")
	}
}

func TestAgentSkillAPIVerifyAutoEnableClean(t *testing.T) {
	s, mux := newTestAgentSkillServer(t)
	s.Cfg.Tools.SkillManager.AutoEnableClean = false
	createBody := `{"name":"verify-auto","description":"Verify auto test. Use when testing verify auto.","body":"# Clean\nNo risky content."}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills", strings.NewReader(createBody)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	id := decodeAgentSkillResponse(t, rec)["skill"].(map[string]interface{})["id"].(string)
	s.Cfg.Tools.SkillManager.AutoEnableClean = true
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills/"+id+"/verify", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("verify status=%d body=%s", rec.Code, rec.Body.String())
	}
	skill := decodeAgentSkillResponse(t, rec)["skill"].(map[string]interface{})
	if !skill["enabled"].(bool) {
		t.Fatal("expected skill.enabled true after verify with AutoEnableClean")
	}
}

func TestAgentSkillAPITestRespectsDangerZonePolicy(t *testing.T) {
	s, mux := newTestAgentSkillServer(t)
	s.Cfg.Agent.AllowPython = false

	createBody := `{"name":"test-policy","description":"Policy test. Use when testing Agent Skill script policy.","body":"# Policy\nRun scripts/echo.py."}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills", strings.NewReader(createBody)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	id := decodeAgentSkillResponse(t, rec)["skill"].(map[string]interface{})["id"].(string)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills/"+id+"/files", strings.NewReader(`{"path":"scripts/echo.py","content":"print('ok')\n"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("write file status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills/"+id+"/verify", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("verify status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/agent-skills/"+id, strings.NewReader(`{"enabled":true}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("enable status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/agent-skills/"+id+"/test", strings.NewReader(`{"script":"scripts/echo.py","args":{}}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("test status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := decodeAgentSkillResponse(t, rec)
	if body["status"] != "error" || !strings.Contains(body["message"].(string), "agent.allow_python") {
		t.Fatalf("test response = %+v, want agent.allow_python policy error", body)
	}
}
