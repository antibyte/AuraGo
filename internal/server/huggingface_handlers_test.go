package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	hf "aurago/internal/huggingface"
)

func TestHandleHuggingFaceStatusMasksToken(t *testing.T) {
	s := &Server{Cfg: &config.Config{
		HuggingFace: config.HuggingFaceConfig{
			Enabled:   true,
			ReadOnly:  true,
			AllowJobs: true,
			Token:     "hf-secret",
		},
	}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/huggingface/status", nil)
	handleHuggingFaceStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["status"] != "ready" || got["configured"] != true {
		t.Fatalf("unexpected status payload: %#v", got)
	}
	if _, leaked := got["token"]; leaked {
		t.Fatalf("status leaked token: %#v", got)
	}
	if strings.Contains(rec.Body.String(), "hf-secret") {
		t.Fatalf("status body leaked token: %s", rec.Body.String())
	}
}

func TestHandleHuggingFaceJobsReturnsLocalLedger(t *testing.T) {
	dataDir := t.TempDir()
	s := &Server{Cfg: &config.Config{
		HuggingFace: config.HuggingFaceConfig{Enabled: true, ReadOnly: true},
	}}
	s.Cfg.Directories.DataDir = dataDir

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/huggingface/jobs", nil)
	handleHuggingFaceJobs(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Jobs []interface{} `json:"jobs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Jobs) != 0 {
		t.Fatalf("jobs = %#v, want empty ledger", got.Jobs)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "huggingface.db")); err != nil {
		t.Fatalf("expected huggingface.db ledger: %v", err)
	}
}

func TestHandleHuggingFaceJobsHonorsLimit(t *testing.T) {
	dataDir := t.TempDir()
	store, err := hf.OpenJobStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordJob(context.Background(), hf.JobRecord{HFJobID: "job-1", Operation: "job_run_python"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordJob(context.Background(), hf.JobRecord{HFJobID: "job-2", Operation: "job_run_python"}); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	s := &Server{Cfg: &config.Config{HuggingFace: config.HuggingFaceConfig{Enabled: true}}}
	s.Cfg.Directories.DataDir = dataDir

	rec := httptest.NewRecorder()
	handleHuggingFaceJobs(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/huggingface/jobs?limit=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Jobs []hf.JobRecord `json:"jobs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Jobs) != 1 {
		t.Fatalf("jobs = %#v", got.Jobs)
	}
}

func TestHandleHuggingFaceJobsRejectsInvalidLimit(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	for _, value := range []string{"0", "201", "invalid"} {
		rec := httptest.NewRecorder()
		handleHuggingFaceJobs(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/huggingface/jobs?limit="+value, nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("limit %q status = %d, body=%s", value, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleHuggingFaceTestChecksPublicHub(t *testing.T) {
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models" {
			t.Fatalf("path = %s, want /api/models", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{{"id": "org/model"}})
	}))
	defer hub.Close()

	s := &Server{Cfg: &config.Config{HuggingFace: config.HuggingFaceConfig{
		Enabled: true, ReadOnly: true, HubBaseURL: hub.URL, DatasetBaseURL: hub.URL, JobsBaseURL: hub.URL,
	}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/huggingface/test", nil)
	handleHuggingFaceTest(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Public Hub access successful") {
		t.Fatalf("status code = %d, body=%s", rec.Code, rec.Body.String())
	}
}
