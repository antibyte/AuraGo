package huggingface

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientAddsBearerTokenAndSearchesModels(t *testing.T) {
	var sawAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models" {
			t.Fatalf("path = %s, want /api/models", r.URL.Path)
		}
		if r.URL.Query().Get("search") != "llama" || r.URL.Query().Get("limit") != "3" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		sawAuth = r.Header.Get("Authorization") == "Bearer hf_test"
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{{"id": "org/model"}})
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		HubBaseURL:     server.URL,
		DatasetBaseURL: server.URL,
		JobsBaseURL:    server.URL,
		Token:          "hf_test",
	})
	got, err := client.SearchModels(context.Background(), SearchOptions{Query: "llama", Limit: 3})
	if err != nil {
		t.Fatalf("SearchModels() error = %v", err)
	}
	if !sawAuth {
		t.Fatal("expected bearer token on Hugging Face request")
	}
	if len(got) != 1 || got[0]["id"] != "org/model" {
		t.Fatalf("SearchModels() = %#v", got)
	}
}

func TestDatasetRowsClampsLengthToConfiguredMaximum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rows" {
			t.Fatalf("path = %s, want /rows", r.URL.Path)
		}
		if r.URL.Query().Get("dataset") != "org/data" || r.URL.Query().Get("split") != "train" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("length") != "25" {
			t.Fatalf("length = %q, want 25", r.URL.Query().Get("length"))
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"rows": []interface{}{}})
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		HubBaseURL:       server.URL,
		DatasetBaseURL:   server.URL,
		JobsBaseURL:      server.URL,
		MaxDatasetRows:   25,
		MaxDownloadMB:    1,
		RequestUserAgent: "aurago-test",
	})
	if _, err := client.DatasetRows(context.Background(), DatasetRowsOptions{
		Dataset: "org/data",
		Split:   "train",
		Length:  500,
	}); err != nil {
		t.Fatalf("DatasetRows() error = %v", err)
	}
}

func TestClientSurfacesRateLimitResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{HubBaseURL: server.URL, DatasetBaseURL: server.URL, JobsBaseURL: server.URL})
	_, err := client.GetModel(context.Background(), "org/model")
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if apiErr, ok := err.(*APIError); !ok || apiErr.StatusCode != http.StatusTooManyRequests || apiErr.RetryAfter != "30" {
		t.Fatalf("error = %#v, want APIError 429 with Retry-After", err)
	}
}

func TestJobsUseNamespaceAndOfficialPayloadFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/jobs/alice" || r.Method != http.MethodPost {
			t.Fatalf("request = %s %s, want POST /api/jobs/alice", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer hf_test" {
			t.Fatalf("authorization header missing")
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["flavor"] != "cpu-basic" || payload["timeoutSeconds"] != float64(600) {
			t.Fatalf("unexpected job payload: %#v", payload)
		}
		if _, ok := payload["environment"].(map[string]interface{}); !ok {
			t.Fatalf("expected environment payload: %#v", payload)
		}
		if _, ok := payload["secrets"].(map[string]interface{}); !ok {
			t.Fatalf("expected secrets payload: %#v", payload)
		}
		command, ok := payload["command"].([]interface{})
		if !ok || len(command) != 4 || command[3] != "" {
			t.Fatalf("command array was not preserved: %#v", payload["command"])
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "job-1", "stage": "RUNNING"})
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		HubBaseURL: server.URL, DatasetBaseURL: server.URL, JobsBaseURL: server.URL + "/api/jobs",
		JobNamespace: "alice", Token: "hf_test",
	})
	if _, err := client.JobRunContainer(context.Background(), JobRunOptions{
		Image: "python:3.12", Command: []string{"python", "-c", "print(1)", ""}, Hardware: "cpu-basic", TimeoutMinutes: 10,
		Env: map[string]string{"MODE": "test"}, Secrets: map[string]string{"HF_TOKEN": "hf_test"},
	}); err != nil {
		t.Fatalf("JobRunContainer() error = %v", err)
	}
}

func TestScheduledJobsUseDedicatedEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/scheduled-jobs/alice" || r.Method != http.MethodPost {
			t.Fatalf("request = %s %s, want POST /api/scheduled-jobs/alice", r.Method, r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["schedule"] != "@hourly" || payload["jobSpec"] == nil {
			t.Fatalf("unexpected scheduled payload: %#v", payload)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "schedule-1"})
	}))
	defer server.Close()

	client := NewClient(ClientConfig{JobsBaseURL: server.URL + "/api/jobs", JobNamespace: "alice"})
	if _, err := client.JobRunContainer(context.Background(), JobRunOptions{
		Image: "python:3.12", Command: []string{"python", "-c", "print(1)"}, Hardware: "cpu-basic", Scheduled: true, Schedule: "@hourly",
	}); err != nil {
		t.Fatalf("scheduled JobRunContainer() error = %v", err)
	}
}

func TestHubWritePayloadsMatchOfficialAPI(t *testing.T) {
	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "hello.txt")
	if err := os.WriteFile(localPath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write upload fixture: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload for %s %s: %v", r.Method, r.URL.Path, err)
		}
		switch r.URL.Path {
		case "/api/repos/create":
			if r.Method != http.MethodPost || payload["name"] != "repo" || payload["organization"] != "alice" || payload["type"] != "model" || payload["private"] != true {
				t.Fatalf("unexpected create payload: %s %#v", r.Method, payload)
			}
		case "/api/models/alice/repo/commit/main":
			if r.Method != http.MethodPost || payload["summary"] != "Upload file via AuraGo" {
				t.Fatalf("unexpected upload request: %s %#v", r.Method, payload)
			}
			files, ok := payload["files"].([]interface{})
			if !ok || len(files) != 1 {
				t.Fatalf("files payload = %#v", payload["files"])
			}
			file := files[0].(map[string]interface{})
			decoded, err := base64.StdEncoding.DecodeString(file["content"].(string))
			if err != nil || string(decoded) != "hello" || file["path"] != "README.md" || file["encoding"] != "base64" {
				t.Fatalf("unexpected file payload: %#v", file)
			}
		case "/api/repos/delete":
			if r.Method != http.MethodDelete || payload["name"] != "repo" || payload["organization"] != "alice" || payload["type"] != "model" {
				t.Fatalf("unexpected delete request: %s %#v", r.Method, payload)
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer server.Close()

	client := NewClient(ClientConfig{HubBaseURL: server.URL, DatasetBaseURL: server.URL, JobsBaseURL: server.URL, MaxUploadMB: 1, Token: "hf_test"})
	if _, err := client.CreateRepo(context.Background(), CreateRepoOptions{RepoID: "alice/repo", Type: "model", Private: true}); err != nil {
		t.Fatalf("CreateRepo() error = %v", err)
	}
	if _, err := client.UploadFile(context.Background(), UploadFileOptions{RepoType: "model", RepoID: "alice/repo", Path: "README.md", LocalPath: localPath}); err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}
	if _, err := client.DeleteRepo(context.Background(), "model", "alice/repo"); err != nil {
		t.Fatalf("DeleteRepo() error = %v", err)
	}
}

func TestJobLogsParsesSSESnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/jobs/alice/job-1/logs" || r.URL.Query().Get("tail") != "2" {
			t.Fatalf("request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("Accept = %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(": keep-alive\n\ndata: ===== Job started\n\ndata: first\n\ndata: second\ndata: line\n\n"))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{HubBaseURL: server.URL, DatasetBaseURL: server.URL, JobsBaseURL: server.URL + "/api/jobs", JobNamespace: "alice", Token: "hf_test"})
	result, err := client.JobLogs(context.Background(), "job-1", JobLogsOptions{Tail: 2})
	if err != nil {
		t.Fatalf("JobLogs() error = %v", err)
	}
	logs, ok := result["logs"].([]string)
	if !ok || len(logs) != 2 || logs[0] != "first" || logs[1] != "second\nline" {
		t.Fatalf("logs = %#v", result["logs"])
	}
	if result["truncated"] != false {
		t.Fatalf("truncated = %#v", result["truncated"])
	}
}

func TestJobLogsTruncatesOversizedSSELine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: this line is longer than the response limit\n\n"))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		HubBaseURL: server.URL, DatasetBaseURL: server.URL, JobsBaseURL: server.URL + "/api/jobs",
		JobNamespace: "alice", MaxResultBytes: 16,
	})
	result, err := client.JobLogs(context.Background(), "job-1", JobLogsOptions{})
	if err != nil {
		t.Fatalf("JobLogs() error = %v", err)
	}
	if result["truncated"] != true {
		t.Fatalf("truncated = %#v, want true", result["truncated"])
	}
}

func TestJobRunUVScriptUsesBoundedBootstrap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["dockerImage"] != "astral-sh/uv:python3.12-bookworm" {
			t.Fatalf("dockerImage = %#v", payload["dockerImage"])
		}
		command := payload["command"].([]interface{})
		if command[0] != "sh" || !strings.Contains(command[2].(string), "uv run /tmp/aurago_script.py") {
			t.Fatalf("command = %#v", command)
		}
		if payload["arguments"].([]interface{})[0] != "--sample" {
			t.Fatalf("arguments = %#v", payload["arguments"])
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "job-uv"})
	}))
	defer server.Close()

	client := NewClient(ClientConfig{HubBaseURL: server.URL, DatasetBaseURL: server.URL, JobsBaseURL: server.URL + "/api/jobs", JobNamespace: "alice", Token: "hf_test"})
	if _, err := client.JobRunUVScript(context.Background(), JobRunOptions{Script: "print('ok')", Arguments: []string{"--sample"}, Hardware: "cpu-basic"}); err != nil {
		t.Fatalf("JobRunUVScript() error = %v", err)
	}
}
