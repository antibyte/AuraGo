package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget(t *testing.T) {
	t.Parallel()

	root := filepath.Join("js", "desktop")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".js" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Count(string(data), "\n")
		if len(data) > 0 && data[len(data)-1] != '\n' {
			lines++
		}
		if lines >= 1000 {
			t.Errorf("%s has %d lines, want < 1000", filepath.ToSlash(path), lines)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVirtualDesktopJSUsesSemanticChunkNames(t *testing.T) {
	t.Parallel()

	root := filepath.Join("js", "desktop")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if strings.Contains(name, "-part-") || strings.HasPrefix(name, "part-") {
			t.Errorf("%s uses a mechanical chunk name", filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
