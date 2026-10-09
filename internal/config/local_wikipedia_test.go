package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func loadLocalWikipediaYAML(t *testing.T, content string) *Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestLocalWikipediaDefaultsWhenSectionIsAbsent(t *testing.T) {
	for name, content := range map[string]string{
		"empty file":     "{}\n",
		"null section":   "local_wikipedia:\n",
		"null values":    "local_wikipedia:\n  agent_access:\n  update_check:\n  variant:\n",
		"empty section":  "local_wikipedia: {}\n",
		"other sections": "server:\n  port: 8088\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := loadLocalWikipediaYAML(t, content).LocalWikipedia
			want := LocalWikipediaConfig{AgentAccess: true, Variant: "nopic", UpdateCheck: true}
			if got != want {
				t.Fatalf("defaults = %+v, want %+v", got, want)
			}
		})
	}
}

func TestLocalWikipediaExplicitValuesAndNormalization(t *testing.T) {
	got := loadLocalWikipediaYAML(t, "local_wikipedia:\n  enabled: true\n  agent_access: false\n  update_check: false\n  language: \" DE \"\n  variant: \" MAXI \"\n  data_dir: \"  /srv/wiki  \"\n").LocalWikipedia
	want := LocalWikipediaConfig{Enabled: true, Language: "de", Variant: "maxi", DataDir: "/srv/wiki"}
	if got != want {
		t.Fatalf("loaded = %+v, want %+v", got, want)
	}
	unknown := loadLocalWikipediaYAML(t, "local_wikipedia:\n  language: xx\n  variant: mini\n").LocalWikipedia
	if unknown.Language != "" || unknown.Variant != "nopic" {
		t.Fatalf("unknown values must fall back to system language and nopic: %+v", unknown)
	}
}

func TestValidateLocalWikipediaConfig(t *testing.T) {
	for _, valid := range []LocalWikipediaConfig{
		{}, {Language: "de", Variant: "nopic"}, {Language: "NO", Variant: "MAXI"}, {Language: "zh"},
	} {
		if err := ValidateLocalWikipediaConfig(valid); err != nil {
			t.Fatalf("ValidateLocalWikipediaConfig(%+v) = %v", valid, err)
		}
	}
	for _, invalid := range []LocalWikipediaConfig{
		{Language: "xx"}, {Language: "nb"}, {Variant: "mini"}, {Variant: "all"},
	} {
		if err := ValidateLocalWikipediaConfig(invalid); err == nil {
			t.Fatalf("ValidateLocalWikipediaConfig(%+v) accepted an unsupported value", invalid)
		}
	}
}

func TestLocalWikipediaLanguageCodesAreTheSixteenUILanguages(t *testing.T) {
	want := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	got := LocalWikipediaLanguageCodes()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("codes = %v, want %v", got, want)
	}
	got[0] = "xx"
	if LocalWikipediaLanguageCodes()[0] != "cs" {
		t.Fatal("LocalWikipediaLanguageCodes must return a copy")
	}
}
