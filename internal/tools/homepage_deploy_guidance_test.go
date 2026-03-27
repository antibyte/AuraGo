package tools

import (
	"strings"
	"testing"
)

func TestDecorateHomepageBuildFailureMissingBuildScript(t *testing.T) {
	raw := `{"status":"error","output":"npm error Missing script: \"build\""}`
	result := decorateHomepageBuildFailure(raw, "ki-news")
	if !strings.Contains(result, `no 'build' script`) {
		t.Fatalf("expected missing-build guidance, got: %s", result)
	}
	if !strings.Contains(result, "ki-news") {
		t.Fatalf("expected project_dir to appear in guidance, got: %s", result)
	}
}
