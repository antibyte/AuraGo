package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestIndexingDirectoryMediaOptInSerialization(t *testing.T) {
	on := IndexingDirectory{Path: "./photos", Collection: "pics", AllowMediaAnalysis: true}

	raw, err := yaml.Marshal(on)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if !strings.Contains(string(raw), "allow_media_analysis: true") {
		t.Fatalf("enabled flag missing from YAML:\n%s", raw)
	}
	var back IndexingDirectory
	if err := yaml.Unmarshal(raw, &back); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if back != on {
		t.Fatalf("YAML round trip = %+v, want %+v", back, on)
	}

	// The default stays out of the file so untouched installs keep their config byte-identical.
	rawOff, err := yaml.Marshal(IndexingDirectory{Path: "./docs"})
	if err != nil {
		t.Fatalf("yaml.Marshal(off): %v", err)
	}
	if strings.Contains(string(rawOff), "allow_media_analysis") {
		t.Fatalf("disabled flag leaked into YAML:\n%s", rawOff)
	}

	// Path and Collection keep their untagged JSON names; the UI reads them that way.
	jsonRaw, err := json.Marshal(on)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(jsonRaw, &fields); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if fields["Path"] != "./photos" || fields["Collection"] != "pics" || fields["allow_media_analysis"] != true {
		t.Fatalf("JSON fields = %v", fields)
	}
	jsonOff, _ := json.Marshal(IndexingDirectory{Path: "./docs"})
	if !strings.Contains(string(jsonOff), `"allow_media_analysis":false`) {
		t.Fatalf("disabled flag must still be reported in JSON: %s", jsonOff)
	}
}

func TestLoadIndexingDirectoriesDefaultMediaOptInToOff(t *testing.T) {
	cases := map[string]string{
		"legacy bare string": "indexing:\n  enabled: true\n  directories:\n    - ./knowledge\n",
		"mapping without flag": "indexing:\n  enabled: true\n  directories:\n" +
			"    - path: ./knowledge\n      collection: docs\n",
		"no directories at all": "indexing:\n  enabled: true\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(cfg.Indexing.Directories) == 0 {
				t.Fatal("expected at least the default indexing directory")
			}
			for _, dir := range cfg.Indexing.Directories {
				if dir.AllowMediaAnalysis {
					t.Fatalf("directory %q must not allow media analysis by default", dir.Path)
				}
			}
		})
	}
}

func TestLoadKeepsExplicitIndexingMediaOptIn(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := "indexing:\n  enabled: true\n  directories:\n" +
		"    - path: ./photos\n      allow_media_analysis: true\n" +
		"    - path: ./docs\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Indexing.Directories) != 2 {
		t.Fatalf("directories = %+v", cfg.Indexing.Directories)
	}
	if !cfg.Indexing.Directories[0].AllowMediaAnalysis || cfg.Indexing.Directories[1].AllowMediaAnalysis {
		t.Fatalf("flags = %v / %v, want true / false",
			cfg.Indexing.Directories[0].AllowMediaAnalysis, cfg.Indexing.Directories[1].AllowMediaAnalysis)
	}
}
