package main

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

// Retro-Net defaults on: an upgraded config without retronet_enabled picks up
// the template's true, and an explicit false survives the merge. config.Load
// gives the same answer for the user's own file, so the merge changes nothing.
func TestRepositoryTemplateMergeRetroNetDefault(t *testing.T) {
	for _, tc := range []struct {
		name string
		user string
		want bool
	}{
		{"section missing", "server:\n    port: 8088\n", true},
		{"key missing", "server:\n    port: 8088\nvirtual_desktop:\n    enabled: true\n", true},
		{"explicit false", "server:\n    port: 8088\nvirtual_desktop:\n    enabled: true\n    retronet_enabled: false\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, tmplMap := repositoryTemplate(t)
			srcMap, err := parseYAMLMap(tc.user)
			if err != nil {
				t.Fatalf("parse user config: %v", err)
			}
			res := mergeUserConfig(tmplMap, srcMap)
			if got := mergedValue(res.merged, "virtual_desktop.retronet_enabled"); got != tc.want {
				t.Fatalf("merged virtual_desktop.retronet_enabled = %#v, want %v", got, tc.want)
			}

			dir := t.TempDir()
			mergedPath := filepath.Join(dir, "merged.yaml")
			atomicWriteYAML(mergedPath, res.merged)
			merged, err := config.Load(mergedPath)
			if err != nil {
				t.Fatalf("Load(merged): %v", err)
			}
			sourcePath := filepath.Join(dir, "source.yaml")
			if err := os.WriteFile(sourcePath, []byte(tc.user), 0o600); err != nil {
				t.Fatal(err)
			}
			source, err := config.Load(sourcePath)
			if err != nil {
				t.Fatalf("Load(source): %v", err)
			}
			if merged.VirtualDesktop.RetroNetEnabled != tc.want || source.VirtualDesktop.RetroNetEnabled != tc.want {
				t.Fatalf("Load: merged=%v source=%v, want %v", merged.VirtualDesktop.RetroNetEnabled, source.VirtualDesktop.RetroNetEnabled, tc.want)
			}
		})
	}
}
