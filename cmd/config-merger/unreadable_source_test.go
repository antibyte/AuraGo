package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnreadableExistingSourceDoesNotOverwriteOutput(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	template := filepath.Join(dir, "template.yaml")
	output := filepath.Join(dir, "output.yaml")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(template, []byte("agent:\n  allow_shell: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	sentinel := []byte("preserve me")
	if err := os.WriteFile(output, sentinel, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(source, template, output); err == nil {
		t.Fatal("unreadable existing source was accepted")
	}
	after, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(sentinel) {
		t.Fatal("output was overwritten")
	}
}

func TestMissingSourceStillCreatesFreshDefaults(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "fresh.yaml")
	template := filepath.Join(dir, "template.yaml")
	if err := os.WriteFile(template, []byte("agent:\n  allow_shell: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(source, template, source); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("fresh defaults missing")
	}
}
