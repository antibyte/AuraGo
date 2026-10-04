package tools

import (
	"aurago/internal/config"
	"testing"
)

func TestHFRepositoryIdentityCannotEscapeNamespace(t *testing.T) {
	cfg := config.HuggingFaceConfig{Enabled: true, AllowWrites: true, AllowedNamespaces: []string{"allowed"}}
	for _, id := range []string{"allowed/../../other/repo", "allowed/../other", "allowed/%2e%2e/other", "allowed/repo%2f..%2fother", "allowed\\other", "/allowed/repo", "allowed//repo", "allowed/repo?target=other", "allowed/.", "allowed/repo.git"} {
		if err := EvaluateHuggingFacePolicy(cfg, HuggingFaceRequest{Operation: "upload_file", RepoID: id}, "fixture-only"); err == nil {
			t.Errorf("unsafe repository identity accepted: %q", id)
		}
	}
}
