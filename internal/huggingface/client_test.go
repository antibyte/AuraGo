package huggingface

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "job-1", "stage": "RUNNING"})
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		HubBaseURL: server.URL, DatasetBaseURL: server.URL, JobsBaseURL: server.URL + "/api/jobs",
		JobNamespace: "alice", Token: "hf_test",
	})
	if _, err := client.JobRunContainer(context.Background(), JobRunOptions{
		Image: "python:3.12", Command: "python -c print(1)", Hardware: "cpu-basic", TimeoutMinutes: 10,
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
		Image: "python:3.12", Command: "python -c print(1)", Hardware: "cpu-basic", Scheduled: true, Schedule: "@hourly",
	}); err != nil {
		t.Fatalf("scheduled JobRunContainer() error = %v", err)
	}
}
