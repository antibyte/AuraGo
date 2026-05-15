package tools

import (
	"strings"
	"testing"
)

func TestBuildVercelDeployCommandLinksProjectBeforeDeploy(t *testing.T) {
	cmd := buildVercelDeployCommand("testseite-aurago", "prj_123", "production", VercelConfig{
		TeamID: "team_abc",
	})

	if !strings.Contains(cmd, "vercel link ") {
		t.Fatalf("expected deploy command to link the Vercel project first, got: %s", cmd)
	}
	if !strings.Contains(cmd, "--project 'prj_123'") {
		t.Fatalf("expected project reference to be used for vercel link, got: %s", cmd)
	}
	if strings.Count(cmd, "--scope 'team_abc'") != 2 {
		t.Fatalf("expected scope to be applied to link and deploy commands, got: %s", cmd)
	}

	parts := strings.SplitN(cmd, "vercel deploy ", 2)
	if len(parts) != 2 {
		t.Fatalf("expected command to include vercel deploy, got: %s", cmd)
	}
	if strings.Contains(parts[1], "--project") {
		t.Fatalf("vercel deploy must not receive the unsupported --project flag, got: %s", cmd)
	}
}

func TestBuildVercelDeployCommandSkipsLinkWithoutProjectRef(t *testing.T) {
	cmd := buildVercelDeployCommand("testseite-aurago", "", "preview", VercelConfig{})

	if strings.Contains(cmd, "vercel link ") {
		t.Fatalf("expected no link step without a project reference, got: %s", cmd)
	}
	if strings.Contains(cmd, "--project") {
		t.Fatalf("expected no --project flag without a project reference, got: %s", cmd)
	}
	if !strings.Contains(cmd, "vercel deploy --yes") {
		t.Fatalf("expected deploy command, got: %s", cmd)
	}
}

func TestHomepageVercelStrategyUsesProjectRootForFrameworks(t *testing.T) {
	candidate := homepageDeployCandidate{BuildDir: "dist", ContainerSubdir: "vite-site/dist", Kind: "spa"}
	deploySubdir, useCandidate := homepageVercelDeploySubdir("vite-site", "vite", "", candidate)
	if deploySubdir != "vite-site" {
		t.Fatalf("framework deploys should run from project root, got %q", deploySubdir)
	}
	if useCandidate {
		t.Fatal("framework deploys should not deploy the static output subdirectory by default")
	}
}

func TestHomepageVercelStrategyAllowsExplicitStaticBuildDir(t *testing.T) {
	candidate := homepageDeployCandidate{BuildDir: "dist", ContainerSubdir: "vite-site/dist", Kind: "spa"}
	deploySubdir, useCandidate := homepageVercelDeploySubdir("vite-site", "vite", "dist", candidate)
	if deploySubdir != "vite-site/dist" {
		t.Fatalf("explicit build_dir should deploy static output, got %q", deploySubdir)
	}
	if !useCandidate {
		t.Fatal("explicit build_dir should use the validated static candidate")
	}
}
