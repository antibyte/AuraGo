package config

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestLocalWikipediaTemplateMatchesLoadDefaults pins the local_wikipedia block
// of config_template.yaml against Load's defaults, so neither drifts alone.
func TestLocalWikipediaTemplateMatchesLoadDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse config_template.yaml: %v", err)
	}
	node, ok := doc["local_wikipedia"]
	if !ok {
		t.Fatal("config_template.yaml has no local_wikipedia: block")
	}
	raw, err := yaml.Marshal(&node)
	if err != nil {
		t.Fatalf("re-encode local_wikipedia block: %v", err)
	}
	var tpl LocalWikipediaConfig
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&tpl); err != nil {
		t.Fatalf("decode local_wikipedia block: %v", err)
	}
	var keys map[string]interface{}
	if err := yaml.Unmarshal(raw, &keys); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(keys))
	for key := range keys {
		got = append(got, key)
	}
	sort.Strings(got)
	if want := "agent_access,data_dir,enabled,language,update_check,variant"; strings.Join(got, ",") != want {
		t.Fatalf("template keys = %v, want %s", got, want)
	}
	if loaded := loadLocalWikipediaYAML(t, "{}\n").LocalWikipedia; tpl != loaded {
		t.Fatalf("template = %+v, Load defaults = %+v", tpl, loaded)
	}
	if tpl.Enabled {
		t.Fatal("the template must not enable Local Wikipedia")
	}
}
