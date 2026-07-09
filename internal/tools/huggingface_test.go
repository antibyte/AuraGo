package tools

import (
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
