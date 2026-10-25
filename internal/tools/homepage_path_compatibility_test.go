package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHomepageCompatiblePathsLocal(t *testing.T) {
	cfg := HomepageConfig{WorkspacePath: t.TempDir(), DockerHost: "unit-test-compatible-paths"}
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[cfg.DockerHost] = dockerAvailabilityEntry{available: false, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
	t.Cleanup(func() { invalidateDockerAvailabilityCache(cfg.DockerHost) })
	for _, path := range []string{
		"site/Über uns.txt", "site/Cafe\u0301.txt", "site/日本語.txt",
		"site/app/[slug]/page.tsx", "site/app/[[...slug]]/page.tsx",
		"site/app/(marketing)/page.tsx", "site/app/@modal/(.)photo/page.tsx",
	} {
		t.Run(path, func(t *testing.T) {
			if result := HomepageWriteFile(cfg, path, "before", slogDiscard()); !strings.Contains(result, `"status":"ok"`) {
				t.Fatal(result)
			}
			if result := HomepageEditFile(cfg, path, "str_replace", "before", "after", "", "", 0, 0, slogDiscard()); strings.Contains(result, `"status":"error"`) {
				t.Fatal(result)
			}
			var read struct{ Content string }
			if result := HomepageReadFile(cfg, path, slogDiscard()); json.Unmarshal([]byte(result), &read) != nil || read.Content != "after" {
				t.Fatalf("read did not preserve the filename: %s", result)
			}
		})
	}
	if result := HomepageListFiles(cfg, "site", slogDiscard()); !strings.Contains(result, "Über uns.txt") {
		t.Fatalf("Unicode filename missing from listing: %s", result)
	}
	t.Run("symlink escape", func(t *testing.T) {
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(cfg.WorkspacePath, "site", "(link)")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if result := HomepageWriteFile(cfg, "site/(link)/Über uns.txt", "escaped", slogDiscard()); !strings.Contains(result, `"status":"error"`) {
			t.Fatalf("symlink escape allowed: %s", result)
		}
		if _, err := os.Stat(filepath.Join(outside, "Über uns.txt")); !os.IsNotExist(err) {
			t.Fatalf("outside path was written: %v", err)
		}
	})
	for _, tc := range []struct {
		name, source string
		edit         func(string) string
	}{
		{"données.json", `{"message":"before"}`, func(path string) string {
			return HomepageJsonEdit(cfg, path, "set", "message", "after", "", slogDiscard())
		}},
		{"données.yaml", "message: before\n", func(path string) string { return HomepageYamlEdit(cfg, path, "set", "message", "after", slogDiscard()) }},
		{"données.xml", "<root><message>before</message></root>", func(path string) string {
			return HomepageXmlEdit(cfg, path, "set_text", "./root/message", "after", slogDiscard())
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "site/(group)/[slug]/" + tc.name
			if result := HomepageWriteFile(cfg, path, tc.source, slogDiscard()); !strings.Contains(result, `"status":"ok"`) {
				t.Fatal(result)
			}
			if result := tc.edit(path); strings.Contains(result, `"status":"error"`) {
				t.Fatal(result)
			}
			data, err := os.ReadFile(filepath.Join(cfg.WorkspacePath, filepath.FromSlash(path)))
			if err != nil || !strings.Contains(string(data), "after") {
				t.Fatalf("structured edit failed: %s: %v", data, err)
			}
		})
	}
}

func TestHomepagePermissionCommandsQuotePaths(t *testing.T) {
	project := "Über uns/[slug]/(group)"
	want := "'/workspace/" + project + "'"
	for _, command := range []string{
		homepageProjectWritableCheckCommand(project),
		homepageProjectWritableRepairCommand("1001:1001", project),
		homepageProjectNodeArtifactsCheckCommand(project),
		homepageProjectNodeArtifactsRepairCommand("1001:1001", project),
	} {
		if strings.Count(command, want) != strings.Count(command, "/workspace/") {
			t.Errorf("unquoted project path: %s", command)
		}
	}
}
