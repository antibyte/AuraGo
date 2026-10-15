package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

// runCapturingStdout runs the merger and returns what it printed to stdout.
func runCapturingStdout(t *testing.T, sourcePath, templatePath, outputPath string) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	printed := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		printed <- string(b)
	}()
	original := os.Stdout
	os.Stdout = w
	func() {
		defer func() { os.Stdout = original }()
		run(sourcePath, templatePath, outputPath)
	}()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return <-printed
}

// mergedFileMap reads the merger output back the way the next run would.
func mergedFileMap(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := readNormalized(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := parseYAMLMap(data)
	if err != nil {
		t.Fatalf("merger output is not valid YAML: %v", err)
	}
	return m
}

func videoStudioTemplatePath() string {
	return filepath.Join("..", "..", "config_template.yaml")
}

func TestRunRaisesUntouchedVideoStudioLimits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		section string
		want    map[string]interface{}
	}{
		{
			name:    "earlier template defaults",
			section: preProVideoStudioSection,
			want:    map[string]interface{}{"enabled": false, "readonly": false, "ffmpeg_path": ""},
		},
		{
			name:    "earlier defaults next to other customized keys",
			section: "video_studio:\n    enabled: true\n    readonly: true\n    ffmpeg_path: /opt/ffmpeg/bin/ffmpeg\n    max_asset_size_mb: 1024\n    max_project_size_mb: 4096\n    render_timeout_seconds: 3600\n",
			want:    map[string]interface{}{"enabled": true, "readonly": true, "ffmpeg_path": "/opt/ffmpeg/bin/ffmpeg"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte("server:\n    port: 8099\n"+tc.section), 0o600); err != nil {
				t.Fatal(err)
			}

			// In place, as update.sh and docker-entrypoint.sh run the merger.
			printed := runCapturingStdout(t, path, videoStudioTemplatePath(), path)
			if !strings.Contains(printed, "Raised the Video Studio limits") {
				t.Fatalf("merger output %q does not report the migration", printed)
			}

			out := mergedFileMap(t, path)
			for dotted, want := range map[string]interface{}{
				"video_studio.max_asset_size_mb":      4096,
				"video_studio.max_project_size_mb":    16384,
				"video_studio.render_timeout_seconds": 7200,
				"video_studio.max_export_size_mb":     8192,
				"server.port":                         8099,
			} {
				if got := mergedValue(out, dotted); got != want {
					t.Fatalf("%s = %#v, want %#v", dotted, got, want)
				}
			}
			for key, want := range tc.want {
				if got := mergedValue(out, "video_studio."+key); got != want {
					t.Fatalf("video_studio.%s = %#v, want %#v", key, got, want)
				}
			}

			cfg, err := config.Load(path)
			if err != nil {
				t.Fatalf("Load(merged): %v", err)
			}
			vs := cfg.VideoStudio
			if vs.MaxAssetSizeMB != 4096 || vs.MaxProjectSizeMB != 16384 || vs.MaxExportSizeMB != 8192 || vs.RenderTimeoutSeconds != 7200 {
				t.Fatalf("loaded limits = %d/%d/%d/%d s, want 4096/16384/8192 MB and 7200 s",
					vs.MaxAssetSizeMB, vs.MaxProjectSizeMB, vs.MaxExportSizeMB, vs.RenderTimeoutSeconds)
			}
		})
	}
}

func TestRunKeepsCustomizedVideoStudioLimits(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		section                     string
		asset, project, timeout, ex int
	}{
		{"raised project limit", "max_asset_size_mb: 1024\n    max_project_size_mb: 8192\n    render_timeout_seconds: 3600", 1024, 8192, 3600, 8192},
		{"raised asset limit", "max_asset_size_mb: 2048\n    max_project_size_mb: 4096\n    render_timeout_seconds: 3600", 2048, 4096, 3600, 4096},
		{"shorter render timeout", "max_asset_size_mb: 1024\n    max_project_size_mb: 4096\n    render_timeout_seconds: 1800", 1024, 4096, 1800, 4096},
		// Not all three are written: the missing one is the template's, the others stay.
		{"timeout not written", "max_asset_size_mb: 1024\n    max_project_size_mb: 4096", 1024, 4096, 7200, 4096},
		{"already the new defaults", "max_asset_size_mb: 4096\n    max_project_size_mb: 16384\n    render_timeout_seconds: 7200", 4096, 16384, 7200, 8192},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte("video_studio:\n    enabled: true\n    "+tc.section+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			printed := runCapturingStdout(t, path, videoStudioTemplatePath(), path)
			if strings.Contains(printed, "Raised the Video Studio limits") {
				t.Fatalf("merger output %q reports a migration for a customized section", printed)
			}

			out := mergedFileMap(t, path)
			for key, want := range map[string]int{
				"max_asset_size_mb":      tc.asset,
				"max_project_size_mb":    tc.project,
				"render_timeout_seconds": tc.timeout,
			} {
				if got := mergedValue(out, "video_studio."+key); got != want {
					t.Fatalf("video_studio.%s = %#v, want %d", key, got, want)
				}
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatalf("Load(merged): %v", err)
			}
			if cfg.VideoStudio.MaxExportSizeMB != tc.ex {
				t.Fatalf("export limit = %d, want %d", cfg.VideoStudio.MaxExportSizeMB, tc.ex)
			}
		})
	}
}

func TestRunVideoStudioMigrationRunsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n    port: 8099\n"+preProVideoStudioSection), 0o600); err != nil {
		t.Fatal(err)
	}
	if printed := runCapturingStdout(t, path, videoStudioTemplatePath(), path); !strings.Contains(printed, "Raised the Video Studio limits") {
		t.Fatalf("first run printed %q, want the migration reported", printed)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	printed := runCapturingStdout(t, path, videoStudioTemplatePath(), path)
	if strings.Contains(printed, "Raised the Video Studio limits") || !strings.Contains(printed, "Config is up to date") {
		t.Fatalf("second run printed %q, want no change", printed)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("second run changed the file:\n%s\n---\n%s", first, second)
	}
}

// The update flow merges in place. A file that only differs from the template
// in the earlier Video Studio limits has no missing top-level section, so only
// the migration can make the merger rewrite it; a customized copy stays untouched.
func TestVideoStudioMigrationForcesTheInPlaceWrite(t *testing.T) {
	templateText, _ := repositoryTemplate(t)
	current := "    max_asset_size_mb: 4096\n    max_project_size_mb: 16384\n    max_export_size_mb: 8192\n    render_timeout_seconds: 7200\n"
	if strings.Count(templateText, current) != 1 {
		t.Fatalf("the template's video_studio limits changed; update this test: %q", current)
	}
	for _, tc := range []struct {
		name      string
		limits    string
		migrated  bool
		wantAsset int
	}{
		{"earlier defaults", "    max_asset_size_mb: 1024\n    max_project_size_mb: 4096\n    render_timeout_seconds: 3600\n", true, 4096},
		{"customized", "    max_asset_size_mb: 1024\n    max_project_size_mb: 8192\n    render_timeout_seconds: 3600\n", false, 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(templateText, current, tc.limits, 1)
			_, tmplMap := repositoryTemplate(t)
			srcMap, err := parseYAMLMap(source)
			if err != nil {
				t.Fatal(err)
			}
			result := mergeUserConfig(tmplMap, srcMap)
			if len(result.missing) != 0 || result.safetyAdjusted || result.typeFixed || result.sanitized {
				t.Fatalf("test source needs a write for another reason: missing=%v safety=%v types=%v sanitized=%v",
					result.missing, result.safetyAdjusted, result.typeFixed, result.sanitized)
			}
			if result.migrated != tc.migrated || result.needsWrite() != tc.migrated {
				t.Fatalf("migrated=%v needsWrite=%v, want both %v", result.migrated, result.needsWrite(), tc.migrated)
			}

			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			printed := runCapturingStdout(t, path, videoStudioTemplatePath(), path)
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if rewritten := string(after) != source; rewritten != tc.migrated {
				t.Fatalf("file rewritten = %v, want %v (merger printed %q)", rewritten, tc.migrated, printed)
			}
			if got := mergedValue(mergedFileMap(t, path), "video_studio.max_asset_size_mb"); got != tc.wantAsset {
				t.Fatalf("video_studio.max_asset_size_mb = %#v, want %d", got, tc.wantAsset)
			}
		})
	}
}

// A salvaged section is migrated like a parsed one.
func TestRecoverCorruptedConfigMigratesVideoStudioLimits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		section string
		asset   int
		project int
	}{
		{"earlier defaults", preProVideoStudioSection, 4096, 16384},
		{"customized", "video_studio:\n    max_asset_size_mb: 1024\n    max_project_size_mb: 8192\n    render_timeout_seconds: 3600\n", 1024, 8192},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, tmplMap := repositoryTemplate(t)
			salvaged, err := parseYAMLMap(tc.section)
			if err != nil {
				t.Fatal(err)
			}
			merged := recoverCorruptedConfig(tmplMap, salvaged)
			if got := mergedValue(merged, "video_studio.max_asset_size_mb"); got != tc.asset {
				t.Fatalf("video_studio.max_asset_size_mb = %#v, want %d", got, tc.asset)
			}
			if got := mergedValue(merged, "video_studio.max_project_size_mb"); got != tc.project {
				t.Fatalf("video_studio.max_project_size_mb = %#v, want %d", got, tc.project)
			}
		})
	}
}

// The migration targets are the defaults the template and the loader ship.
func TestVideoStudioMigrationTargetsAreTheCurrentDefaults(t *testing.T) {
	_, tmplMap := repositoryTemplate(t)
	var defaults config.VideoStudioConfig
	if err := config.NormalizeVideoStudioConfig(&defaults); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key        string
		target     int
		loaderDflt int
	}{
		{"max_asset_size_mb", videoStudioAssetMB, defaults.MaxAssetSizeMB},
		{"max_project_size_mb", videoStudioProjectMB, defaults.MaxProjectSizeMB},
		{"render_timeout_seconds", videoStudioTimeoutSecond, defaults.RenderTimeoutSeconds},
	} {
		if got := mergedValue(tmplMap, "video_studio."+tc.key); got != tc.target || tc.loaderDflt != tc.target {
			t.Fatalf("%s: template %#v, loader default %d, migration target %d must agree", tc.key, got, tc.loaderDflt, tc.target)
		}
	}
}
