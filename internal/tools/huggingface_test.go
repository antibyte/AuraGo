package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestHuggingFacePolicyAllowsPublicReadsWithoutToken(t *testing.T) {
	cfg := config.HuggingFaceConfig{
		Enabled:        true,
		ReadOnly:       true,
		MaxDatasetRows: 100,
	}
	req := HuggingFaceRequest{Operation: "search_models", Query: "bert", Limit: 5}

	if err := EvaluateHuggingFacePolicy(cfg, req, ""); err != nil {
		t.Fatalf("EvaluateHuggingFacePolicy() error = %v", err)
	}
}

func TestHuggingFacePolicyBlocksWritesWhenReadOnly(t *testing.T) {
	cfg := config.HuggingFaceConfig{Enabled: true, ReadOnly: true, AllowWrites: true}
	req := HuggingFaceRequest{Operation: "create_repo", RepoID: "owner/repo"}

	err := EvaluateHuggingFacePolicy(cfg, req, "hf_token")
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}
}

func TestHuggingFacePolicyBlocksJobsByDefault(t *testing.T) {
	cfg := config.HuggingFaceConfig{
		Enabled:         true,
		ReadOnly:        false,
		AllowedHardware: []string{"cpu-basic"},
	}
	req := HuggingFaceRequest{Operation: "job_run_python", Hardware: "cpu-basic"}

	err := EvaluateHuggingFacePolicy(cfg, req, "hf_token")
	if err == nil || !strings.Contains(err.Error(), "allow_jobs") {
		t.Fatalf("expected allow_jobs error, got %v", err)
	}
}

func TestHuggingFacePolicyAllowsAuthenticatedJobReadsInReadOnlyMode(t *testing.T) {
	cfg := config.HuggingFaceConfig{Enabled: true, ReadOnly: true, AllowJobs: true, AllowedHardware: []string{"a10g-small"}}
	for _, operation := range []string{"jobs_list", "job_get", "job_logs"} {
		if err := EvaluateHuggingFacePolicy(cfg, HuggingFaceRequest{Operation: operation}, "hf_token"); err != nil {
			t.Fatalf("%s blocked in read-only mode: %v", operation, err)
		}
		if err := EvaluateHuggingFacePolicy(cfg, HuggingFaceRequest{Operation: operation}, ""); err == nil {
			t.Fatalf("%s must require a token", operation)
		}
	}
}

func TestRunHuggingFacePassesConfiguredJobNamespace(t *testing.T) {
	var sawNamespace bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawNamespace = r.URL.Path == "/api/jobs/team"
		if r.Header.Get("Authorization") != "Bearer hf_token" {
			t.Fatalf("missing bearer token")
		}
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	cfg := config.HuggingFaceConfig{
		Enabled: true, ReadOnly: true, AllowJobs: true,
		JobsBaseURL: server.URL + "/api/jobs", JobNamespace: "team",
	}
	output := RunHuggingFace(context.Background(), cfg, "hf_token", t.TempDir(), t.TempDir(), HuggingFaceRequest{Operation: "jobs_list"})
	if !sawNamespace || !strings.Contains(output, `"status":"success"`) {
		t.Fatalf("namespace was not passed to Jobs client: path=%v output=%s", sawNamespace, output)
	}
}

func TestHuggingFacePolicyKeepsJobMutationsBehindReadOnly(t *testing.T) {
	cfg := config.HuggingFaceConfig{Enabled: true, ReadOnly: true, AllowJobs: true, AllowedHardware: []string{"cpu-basic"}}
	for _, operation := range []string{"job_run_python", "job_cancel"} {
		err := EvaluateHuggingFacePolicy(cfg, HuggingFaceRequest{Operation: operation}, "hf_token")
		if err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Fatalf("%s should be blocked in read-only mode: %v", operation, err)
		}
	}
}

func TestHuggingFacePolicyDoesNotApplyHardwareGateToCancel(t *testing.T) {
	cfg := config.HuggingFaceConfig{Enabled: true, ReadOnly: false, AllowJobs: true, AllowedHardware: []string{"a10g-small"}}
	if err := EvaluateHuggingFacePolicy(cfg, HuggingFaceRequest{Operation: "job_cancel"}, "hf_token"); err != nil {
		t.Fatalf("job_cancel unexpectedly used the hardware gate: %v", err)
	}
}

func TestHuggingFacePolicyRequiresTwoPartTokenInjectionOptIn(t *testing.T) {
	cfg := config.HuggingFaceConfig{
		Enabled: true, ReadOnly: false, AllowJobs: true, AllowedHardware: []string{"cpu-basic"},
	}
	req := HuggingFaceRequest{Operation: "job_run_python", Script: "print('ok')", InjectToken: true}
	if err := EvaluateHuggingFacePolicy(cfg, req, "hf_token"); err == nil || !strings.Contains(err.Error(), "allow_job_token_injection") {
		t.Fatalf("expected config token-injection gate, got %v", err)
	}
	cfg.AllowJobTokenInjection = true
	if err := EvaluateHuggingFacePolicy(cfg, req, "hf_token"); err != nil {
		t.Fatalf("two-part token opt-in was rejected: %v", err)
	}
	withToken := hfJobRunOptions(cfg, req, "hf_token")
	if withToken.Secrets["HF_TOKEN"] != "hf_token" {
		t.Fatalf("HF_TOKEN was not injected after both opt-ins: %#v", withToken.Secrets)
	}
	withoutToken := hfJobRunOptions(cfg, HuggingFaceRequest{}, "hf_token")
	if len(withoutToken.Secrets) != 0 {
		t.Fatalf("HF_TOKEN was injected without request opt-in: %#v", withoutToken.Secrets)
	}
}

func TestHuggingFacePolicyRequiresHardwareAllowlist(t *testing.T) {
	cfg := config.HuggingFaceConfig{
		Enabled:         true,
		ReadOnly:        false,
		AllowJobs:       true,
		AllowedHardware: []string{"cpu-basic"},
	}
	req := HuggingFaceRequest{Operation: "job_run_container", Hardware: "a10g-small"}

	err := EvaluateHuggingFacePolicy(cfg, req, "hf_token")
	if err == nil || !strings.Contains(err.Error(), "hardware") {
		t.Fatalf("expected hardware allowlist error, got %v", err)
	}
}

func TestHuggingFacePolicyRejectsOversizedDatasetRows(t *testing.T) {
	cfg := config.HuggingFaceConfig{Enabled: true, ReadOnly: true, MaxDatasetRows: 50}
	req := HuggingFaceRequest{Operation: "dataset_rows", Dataset: "org/data", Split: "train", Length: 100}

	err := EvaluateHuggingFacePolicy(cfg, req, "")
	if err == nil || !strings.Contains(err.Error(), "max_dataset_rows") {
		t.Fatalf("expected max_dataset_rows error, got %v", err)
	}
}

func TestHuggingFaceWorkspacePathsRejectEscapes(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	inside := filepath.Join(workspace, "inside.txt")
	if err := os.WriteFile(inside, []byte("workspace"), 0o600); err != nil {
		t.Fatalf("write inside fixture: %v", err)
	}

	if _, err := validateHuggingFaceUploadSource(workspace, outside, 1); err == nil {
		t.Fatal("expected absolute path outside workspace to be rejected")
	}
	if _, err := validateHuggingFaceUploadSource(workspace, inside, 1); err == nil {
		t.Fatal("expected absolute path inside workspace to be rejected")
	}
	if _, err := resolveHuggingFaceWorkspaceFile(workspace, filepath.Join("..", "..", "outside.txt")); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestHuggingFaceWorkspacePathsRejectSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "agent_workspace", "workdir")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	link := filepath.Join(workspace, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := resolveHuggingFaceWorkspaceFile(workspace, filepath.Join("link", "file.txt")); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestHuggingFaceUploadSourceRejectsDirectoriesAndSize(t *testing.T) {
	workspace := t.TempDir()
	if _, err := validateHuggingFaceUploadSource(workspace, ".", 1); err == nil {
		t.Fatal("expected directory upload to be rejected")
	}
	large := filepath.Join(workspace, "large.bin")
	file, err := os.Create(large)
	if err != nil {
		t.Fatalf("create large fixture: %v", err)
	}
	if err := file.Truncate(1024*1024 + 1); err != nil {
		_ = file.Close()
		t.Fatalf("truncate large fixture: %v", err)
	}
	_ = file.Close()
	if _, err := validateHuggingFaceUploadSource(workspace, "large.bin", 1); err == nil {
		t.Fatal("expected one MB upload limit to reject fixture")
	}
	if _, err := validateHuggingFaceUploadSource(workspace, "large.bin", 0); err != nil {
		t.Fatalf("unexpected default upload limit error: %v", err)
	}
}

func TestHuggingFaceLedgerFailureProducesSafeWarning(t *testing.T) {
	blockingFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := map[string]interface{}{"id": "job-1"}
	err := recordHuggingFaceJob(context.Background(), blockingFile, "job_run_python", "cpu-basic", HuggingFaceRequest{}, result, nil)
	if err == nil {
		t.Fatal("expected local ledger failure")
	}
	attachHuggingFaceLedgerWarning(result, err)
	if result["ledger_warning"] == nil || strings.Contains(result["ledger_warning"].(string), blockingFile) {
		t.Fatalf("unsafe or missing ledger warning: %#v", result)
	}
}
