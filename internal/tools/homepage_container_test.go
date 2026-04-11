package tools

import (
	"strings"
	"testing"
)

func TestHomepageContainerVersionsPinned(t *testing.T) {
	if homepageWebImage != "caddy:2.11.2-alpine" {
		t.Fatalf("expected pinned caddy image, got %q", homepageWebImage)
	}

	requiredSnippets := []string{
		"FROM mcr.microsoft.com/playwright:v1.59.1-noble",
		"ARG CLOUDFLARED_VERSION=2026.3.0",
		`apt-get install -y --no-install-recommends`,
		`arch="$(dpkg --print-architecture)"`,
		`amd64) cloudflared_arch="amd64" ;;`,
		`arm64) cloudflared_arch="arm64" ;;`,
		`npm cache clean --force`,
		`ENV NODE_ENV=development`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(homepageDockerfile, snippet) {
			t.Fatalf("expected homepageDockerfile to contain %q", snippet)
		}
	}
}
