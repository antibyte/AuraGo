package main

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"aurago/internal/config"
)

func TestLifeboatPathHelpers(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = "data"
	cfg.Directories.WorkspaceDir = "workspace"
	cfg.Directories.ToolsDir = "tools"
	cfg.Directories.PromptsDir = "prompts"
	cfg.Directories.SkillsDir = "skills"
	cfg.Directories.VectorDBDir = "vectordb"
	cfg.Logging.LogDir = "logs"

	if got := lifeboatLogPath(cfg); got != filepath.Join("logs", "lifeboat.log") {
		t.Fatalf("lifeboatLogPath = %q", got)
	}
	if got := lifeboatBusyFilePath(cfg); got != filepath.Join("data", "maintenance.lock") {
		t.Fatalf("lifeboatBusyFilePath = %q", got)
	}

	wantDirs := []string{"data", "workspace", "tools", "prompts", "skills", "vectordb", "logs"}
	if got := lifeboatRuntimeDirs(cfg); !reflect.DeepEqual(got, wantDirs) {
		t.Fatalf("lifeboatRuntimeDirs = %#v, want %#v", got, wantDirs)
	}
}

func TestLifeboatBuildCommandArgs(t *testing.T) {
	target := filepath.Join("bin", "aurago_linux")
	want := []string{"build", "-o", target, "./cmd/aurago"}
	if got := lifeboatBuildCommandArgsForTarget(target); !reflect.DeepEqual(got, want) {
		t.Fatalf("lifeboatBuildCommandArgsForTarget = %#v, want %#v", got, want)
	}
}

func TestLifeboatSelectMainBinaryPath(t *testing.T) {
	got := lifeboatSelectMainBinaryPath("linux", func(name string) bool {
		return name == filepath.Join("bin", "aurago_linux")
	})
	if got != filepath.Join("bin", "aurago_linux") {
		t.Fatalf("linux preferred path = %q", got)
	}

	got = lifeboatSelectMainBinaryPath("linux", func(name string) bool {
		return name == filepath.Join("bin", "aurago")
	})
	if got != filepath.Join("bin", "aurago") {
		t.Fatalf("linux fallback path = %q", got)
	}

	got = lifeboatSelectMainBinaryPath("windows", func(name string) bool {
		return name == "aurago.exe"
	})
	if got != "aurago.exe" {
		t.Fatalf("windows fallback path = %q", got)
	}
}

func TestLifeboatRestartSpec(t *testing.T) {
	target := filepath.Join("bin", "aurago_linux")
	exePath, args, err := lifeboatRestartSpecForTarget(target, "ctx", func(name string) (string, error) {
		return filepath.Join("abs", name), nil
	})
	if err != nil {
		t.Fatalf("lifeboatRestartSpec returned error: %v", err)
	}
	if exePath != filepath.Join("abs", target) {
		t.Fatalf("exePath = %q", exePath)
	}
	if !reflect.DeepEqual(args, []string{"--recovery-context", "ctx"}) {
		t.Fatalf("args = %#v", args)
	}

	exePath, args, err = lifeboatRestartSpecForTarget(target, "", func(name string) (string, error) {
		return "", errors.New("no abs")
	})
	if err == nil {
		t.Fatal("expected abs resolver error")
	}
	if exePath != "."+string(filepath.Separator)+target {
		t.Fatalf("fallback exePath = %q", exePath)
	}
	if len(args) != 0 {
		t.Fatalf("empty recovery args = %#v", args)
	}
}

func TestLifeboatSidecarTokenAuthorization(t *testing.T) {
	if !lifeboatCommandAuthorized("secret-token", "secret-token") {
		t.Fatal("expected matching token to authorize")
	}
	for _, tc := range []struct {
		name string
		want string
		got  string
	}{
		{name: "wrong token", want: "secret-token", got: "other"},
		{name: "missing expected token", want: "", got: "secret-token"},
		{name: "missing supplied token", want: "secret-token", got: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if lifeboatCommandAuthorized(tc.want, tc.got) {
				t.Fatalf("expected authorization to fail for want=%q got=%q", tc.want, tc.got)
			}
		})
	}
}
