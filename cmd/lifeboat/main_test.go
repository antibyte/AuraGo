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
	want := []string{"build", "-o", lifeboatMainBinaryName(), "./cmd/aurago"}
	if got := lifeboatBuildCommandArgs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("lifeboatBuildCommandArgs = %#v, want %#v", got, want)
	}
}

func TestLifeboatRestartSpec(t *testing.T) {
	exePath, args, err := lifeboatRestartSpec("ctx", func(name string) (string, error) {
		return filepath.Join("abs", name), nil
	})
	if err != nil {
		t.Fatalf("lifeboatRestartSpec returned error: %v", err)
	}
	if exePath != filepath.Join("abs", lifeboatMainBinaryName()) {
		t.Fatalf("exePath = %q", exePath)
	}
	if !reflect.DeepEqual(args, []string{"--recovery-context", "ctx"}) {
		t.Fatalf("args = %#v", args)
	}

	exePath, args, err = lifeboatRestartSpec("", func(name string) (string, error) {
		return "", errors.New("no abs")
	})
	if err == nil {
		t.Fatal("expected abs resolver error")
	}
	if exePath != "./"+lifeboatMainBinaryName() {
		t.Fatalf("fallback exePath = %q", exePath)
	}
	if len(args) != 0 {
		t.Fatalf("empty recovery args = %#v", args)
	}
}
