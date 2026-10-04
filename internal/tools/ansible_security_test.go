package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnsibleLocalOperationsRequireEffectiveShellGrant(t *testing.T) {
	cfg := AnsibleLocalConfig{Context: WithRuntimePermissions(context.Background(), RuntimePermissions{}), PlaybooksDir: t.TempDir(), WorkDir: t.TempDir()}
	for _, run := range []func() string{
		func() string { return AnsibleLocalStatus(cfg) },
		func() string { return AnsibleLocalListInventory(cfg, "hosts.ini") },
		func() string { return AnsibleLocalPing(cfg, "all", "") },
		func() string { return AnsibleLocalAdhoc(cfg, "localhost", "shell", "echo unsafe", "", nil) },
		func() string { return AnsibleLocalRunPlaybook(cfg, "safe.yml", "", "", "", "", nil, true, true) },
		func() string { return AnsibleLocalGatherFacts(cfg, "all", "") },
	} {
		if result := run(); !strings.Contains(result, "disabled") {
			t.Fatalf("operation bypassed shell gate: %s", result)
		}
	}
	cfg.Context = WithRuntimePermissions(context.Background(), RuntimePermissions{AllowShell: true})
	if result := AnsibleLocalStatus(cfg); !strings.Contains(result, "allow_unsafe_host_execution") {
		t.Fatalf("host gate bypassed: %s", result)
	}
}

func TestAnsibleInputsSnapshotAndRejectUncontrolledPaths(t *testing.T) {
	root, work := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "safe.yml"), []byte("- hosts: all\n  tasks: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hosts.ini"), []byte("localhost\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := AnsibleLocalConfig{PlaybooksDir: root, WorkDir: work}
	dir, args, err := stageAnsibleInputs(context.Background(), cfg, "ansible-playbook", []string{"safe.yml", "-i", "hosts.ini"})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if !strings.HasPrefix(args[0], dir) || !strings.HasPrefix(args[2], dir) {
		t.Fatal("process received original file paths")
	}
	if err := os.WriteFile(filepath.Join(root, "safe.yml"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(args[0])
	if err != nil || strings.Contains(string(data), "changed") {
		t.Fatal("snapshot changed with source")
	}
	outside := filepath.Join(t.TempDir(), "outside.yml")
	for _, input := range []string{outside, "../outside.yml"} {
		if _, _, err := stageAnsibleInputs(context.Background(), cfg, "ansible-playbook", []string{input}); err == nil {
			t.Fatal("path escaped")
		}
	}
	if err := os.WriteFile(filepath.Join(root, "dynamic.ini"), []byte("#!/bin/sh\necho unsafe\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := stageAnsibleInputs(context.Background(), cfg, "ansible-inventory", []string{"--list", "-i", "dynamic.ini"}); err == nil {
		t.Fatal("dynamic inventory accepted")
	}
}
