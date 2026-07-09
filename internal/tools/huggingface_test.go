package tools

import (
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
	req := HuggingFaceRequest{Operation: "job_run_script", Hardware: "cpu-basic"}

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
