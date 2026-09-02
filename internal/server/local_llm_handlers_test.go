package server

import (
	"aurago/internal/config"
	"aurago/internal/localllm"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSetupLocalLLMRequestStripsControlEnvelopeAndEnablesSavedConfig(t *testing.T) {
	patch := map[string]interface{}{
		"local_llm": map[string]interface{}{"enabled": false, "backend": "sycl"},
		"_local_llm_setup": map[string]interface{}{
			"enabled": true, "role": "primary", "regular_provider": "cloud",
		},
	}
	request := parseSetupLocalLLMRequest(patch)
	if !request.Enabled || request.Role != "primary" || request.RegularProvider != "cloud" {
		t.Fatalf("request = %#v", request)
	}
	if _, exists := patch["_local_llm_setup"]; exists {
		t.Fatal("setup-only control envelope would be persisted")
	}
	local := patch["local_llm"].(map[string]interface{})
	if local["enabled"] != true {
		t.Fatalf("saved local_llm patch = %#v", local)
	}
}

func TestSetupLocalLLMJobTokenIsolationAndSecretExclusion(t *testing.T) {
	job := &setupLocalLLMJob{
		ID: "job-a", Token: "secret-job-token", State: "queued", Progress: 0.25,
		Role: "test_only", ExpiresAt: time.Now().Add(time.Hour),
	}
	server := &Server{SetupLocalLLMJobs: map[string]*setupLocalLLMJob{job.ID: job}}
	handler := handleSetupLocalLLMJob(server)

	for _, test := range []struct {
		name   string
		token  string
		status int
	}{
		{name: "missing", status: http.StatusNotFound},
		{name: "wrong", token: "another-token", status: http.StatusNotFound},
		{name: "matching", token: job.Token, status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/setup/local-llm/job?id="+job.ID, nil)
			request.Header.Set("X-Setup-Job-Token", test.token)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, test.status, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), job.Token) || strings.Contains(recorder.Body.String(), "expires_at") {
				t.Fatalf("job response leaked token or expiry: %s", recorder.Body.String())
			}
		})
	}
}

func TestSetupLocalLLMJobExpiryIsBoundedAndTerminalWindowIsFifteenMinutes(t *testing.T) {
	now := time.Now()
	server := &Server{SetupLocalLLMJobs: map[string]*setupLocalLLMJob{
		"expired": {ID: "expired", Token: "token", State: "installing", ExpiresAt: now.Add(-time.Second)},
		"active":  {ID: "active", Token: "token", State: "installing", ExpiresAt: now.Add(6 * time.Hour)},
	}}
	server.SetupLocalLLMJobsMu.Lock()
	pruneSetupLocalLLMJobsLocked(server, now)
	if server.SetupLocalLLMJobs["expired"] != nil || server.SetupLocalLLMJobs["active"] == nil {
		t.Fatalf("pruned jobs=%#v", server.SetupLocalLLMJobs)
	}
	server.SetupLocalLLMJobsMu.Unlock()

	updateSetupLocalLLMJob(server, "active", "completed", "", 1)
	server.SetupLocalLLMJobsMu.Lock()
	remaining := time.Until(server.SetupLocalLLMJobs["active"].ExpiresAt)
	server.SetupLocalLLMJobsMu.Unlock()
	if remaining < 14*time.Minute || remaining > 15*time.Minute+time.Second {
		t.Fatalf("terminal token lifetime=%s", remaining)
	}
}

func TestLocalLLMConfigRevisionAndErrorsAreSanitized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("local_llm:\n  enabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := configFileRevision(path)
	if err != nil || len(first) != 64 {
		t.Fatalf("first revision = %q, err = %v", first, err)
	}
	if err := os.WriteFile(path, []byte("local_llm:\n  enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := configFileRevision(path)
	if err != nil || first == second {
		t.Fatalf("second revision = %q, first = %q, err = %v", second, first, err)
	}
	if got := safeLocalLLMErrorCode(&testLocalLLMError{message: `download failed: C:\secret\model.gguf`}); got != "local_llm_error" {
		t.Fatalf("unsafe error code = %q", got)
	}
	if got := safeLocalLLMErrorCode(&testLocalLLMError{message: "gpu_offload_not_verified: detail"}); got != "gpu_offload_not_verified" {
		t.Fatalf("safe error code = %q", got)
	}

	payload, err := json.Marshal(setupLocalLLMJob{ID: "id", Token: "secret", ExpiresAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "secret") || strings.Contains(string(payload), "ExpiresAt") {
		t.Fatalf("serialized setup job leaked secret fields: %s", payload)
	}
}

func TestLocalLLMRoleRejectsStaleConfigRevisionBeforeMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("local_llm:\n  enabled: false\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ConfigPath: path}
	server := &Server{Cfg: cfg}
	request := httptest.NewRequest(http.MethodPost, "/api/local-llm/role", strings.NewReader(
		`{"role":"test_only","regular_provider":"main","config_revision":"stale"}`,
	))
	recorder := httptest.NewRecorder()
	handleLocalLLMRole(server).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusConflict, recorder.Body.String())
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(original) {
		t.Fatalf("config changed after stale revision: %q, err=%v", after, err)
	}
}

func TestLocalLLMRoleMapping(t *testing.T) {
	cfg := &config.Config{}
	if got := localLLMRole(cfg); got != "test_only" {
		t.Fatalf("default role = %q", got)
	}
	cfg.FallbackLLM.Enabled = true
	cfg.FallbackLLM.Provider = config.LocalLLMProviderID
	if got := localLLMRole(cfg); got != "fallback" {
		t.Fatalf("fallback role = %q", got)
	}
	cfg.LLM.Provider = config.LocalLLMProviderID
	if got := localLLMRole(cfg); got != "primary" {
		t.Fatalf("primary role = %q", got)
	}
}

func TestLocalLLMRoutingRequiresSameDesiredAppliedAndVerifiedFingerprint(t *testing.T) {
	status := localllm.Status{
		State: "running", Backend: "sycl", GPUOffloadVerified: true, MemoryProfileVerified: true, ToolCallVerified: true,
		DesiredFingerprint: "current", AppliedFingerprint: "current", VerifiedFingerprint: "current",
	}
	if !localLLMStatusRoutingReady(status) {
		t.Fatalf("verified status was rejected: %#v", status)
	}
	status.VerifiedFingerprint = "old"
	if localLLMStatusRoutingReady(status) {
		t.Fatal("stale verification was accepted")
	}
	status.VerifiedFingerprint = "current"
	status.PendingRestart = true
	if localLLMStatusRoutingReady(status) {
		t.Fatal("pending restart was accepted")
	}
	status.PendingRestart = false
	status.OperationInProgress = true
	if localLLMStatusRoutingReady(status) {
		t.Fatal("in-progress temporary runtime plan was accepted")
	}
	status.OperationInProgress = false
	status.MemoryProfileVerified = false
	if localLLMStatusRoutingReady(status) {
		t.Fatal("unverified memory profile was accepted")
	}
}

func TestReservedLocalLLMProviderIDIsRejectedBeforeProviderPersistence(t *testing.T) {
	if err := validateProviderIDForSave(config.LocalLLMProviderID, map[string]bool{}); err == nil ||
		!strings.Contains(err.Error(), "reserved") {
		t.Fatalf("reserved provider validation error = %v", err)
	}
}

func TestSetupLocalLLMProbeRequiresCSRFBeforeManagerAccess(t *testing.T) {
	server := &Server{Cfg: &config.Config{}}
	request := httptest.NewRequest(http.MethodPost, "/api/setup/local-llm/probe", nil)
	recorder := httptest.NewRecorder()
	handleSetupLocalLLMProbe(server).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestSetupLocalLLMProbeRejectsUnknownModelBeforeHardwareAccess(t *testing.T) {
	server := &Server{Cfg: &config.Config{}, LocalLLM: &localllm.Manager{}}
	addSetupCSRFTokenForTest(server, "family-test-token")
	request := httptest.NewRequest(http.MethodPost, "/api/setup/local-llm/probe", strings.NewReader(`{"backend":"cuda","model_family":"unknown"}`))
	request.Header.Set("X-CSRF-Token", "family-test-token")
	recorder := httptest.NewRecorder()
	handleSetupLocalLLMProbe(server).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
}

type testLocalLLMError struct{ message string }

func (e *testLocalLLMError) Error() string { return e.message }
