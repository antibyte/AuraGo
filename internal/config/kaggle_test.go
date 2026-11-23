package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestKaggleDefaultsAndSecretOmission(t *testing.T) {
	c := DefaultKaggleConfig()
	if c.Enabled || !c.ReadOnly || c.Local.Enabled || c.MaxTransferMB != 2048 || c.MaxStorageMB != 10240 || c.Local.MemoryMB != 4096 || c.Local.IdleMinutes != 15 {
		t.Fatalf("unsafe defaults: %+v", c)
	}
	c.Token = "fixture-secret"
	c.LegacyKey = "fixture-secret"
	b, _ := yaml.Marshal(c)
	if strings.Contains(string(b), "fixture-secret") {
		t.Fatal("secret persisted in YAML")
	}
	b, _ = json.Marshal(c)
	if strings.Contains(string(b), "fixture-secret") {
		t.Fatal("secret exposed in JSON")
	}
}

func TestKaggleMachineShapesNormalize(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want []string
	}{
		{"nil", nil, []string{}},
		{"canonical order", []string{"NvidiaL4", "NvidiaTeslaT4"}, []string{"NvidiaTeslaT4", "NvidiaL4"}},
		{"trim, case and duplicates", []string{" nvidial4 ", "NvidiaL4", "NVIDIATESLAT4"}, []string{"NvidiaTeslaT4", "NvidiaL4"}},
		{"unknown names dropped", []string{"NvidiaA100", "", "TpuV38"}, []string{}},
	} {
		c := KaggleConfig{AllowedMachineShapes: tc.in}
		NormalizeKaggleConfig(&c)
		if !reflect.DeepEqual(c.AllowedMachineShapes, tc.want) {
			t.Errorf("%s: %v, want %v", tc.name, c.AllowedMachineShapes, tc.want)
		}
	}
	b, _ := json.Marshal(DefaultKaggleConfig())
	if !strings.Contains(string(b), `"allowed_machine_shapes":[]`) {
		t.Fatalf("default must serialize an empty list: %s", b)
	}
	if shape, ok := CanonicalKaggleMachineShape(" nvidiateslat4 "); !ok || shape != "NvidiaTeslaT4" {
		t.Fatalf("CanonicalKaggleMachineShape = %q %v", shape, ok)
	}
	if _, ok := CanonicalKaggleMachineShape("NvidiaA100"); ok {
		t.Fatal("unknown shape accepted")
	}
}

func TestKaggleMachineShapesLoadSaveAndTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("kaggle:\n  allowed_machine_shapes: [\" NvidiaL4\", NvidiaA100, NvidiaTeslaT4, NvidiaL4]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"NvidiaTeslaT4", "NvidiaL4"}; !reflect.DeepEqual(cfg.Kaggle.AllowedMachineShapes, want) {
		t.Fatalf("loaded %v, want %v", cfg.Kaggle.AllowedMachineShapes, want)
	}
	template, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(template, &doc); err != nil {
		t.Fatal(err)
	}
	node := doc["kaggle"]
	raw, _ := yaml.Marshal(&node)
	var tpl KaggleConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("template kaggle block: %v", err)
	}
	if len(tpl.AllowedMachineShapes) != 0 {
		t.Fatalf("template must grant no machine shape: %v", tpl.AllowedMachineShapes)
	}
	full := filepath.Join(dir, "full.yaml")
	if err := os.WriteFile(full, template, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(full)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Kaggle.AllowedMachineShapes = []string{"NvidiaL4"}
	if err := cfg.Save(full); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(full)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reloaded.Kaggle.AllowedMachineShapes, []string{"NvidiaL4"}) {
		t.Fatalf("saved list not reloaded: %v", reloaded.Kaggle.AllowedMachineShapes)
	}
}
