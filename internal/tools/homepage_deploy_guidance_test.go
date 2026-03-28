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

func TestHomepageWebServerStartMissingSourcePathExplainsSrvRoot(t *testing.T) {
	cfg := HomepageConfig{WorkspacePath: t.TempDir(), WebServerPort: 8080}
	result := HomepageWebServerStart(cfg, "missing-site", ".", nil)
	if !strings.Contains(result, "aurago-homepage-web") {
		t.Fatalf("expected Caddy container guidance, got: %s", result)
	}
	if !strings.Contains(result, "/srv") {
		t.Fatalf("expected /srv document root guidance, got: %s", result)
	}
	if !strings.Contains(result, "/var/www/html") {
		t.Fatalf("expected /var/www/html correction hint, got: %s", result)
	}
}
