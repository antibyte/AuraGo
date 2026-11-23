package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func loadChibiSenseiYAML(t *testing.T, content string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return Load(path)
}

func TestChibiSenseiDefaults(t *testing.T) {
	want := ChibiSenseiConfig{
		Enabled:   true,
		Storage:   ChibiSenseiStorageConfig{MaxGB: 20},
		Dataset:   ChibiSenseiDatasetConfig{MaxRows: 50000},
		Import:    ChibiSenseiImportConfig{MaxRows: 20000, MaxFileMB: 50},
		Generator: ChibiSenseiGeneratorConfig{MaxRowsPerJob: 500, MaxDailyRows: 5000},
		Training:  ChibiSenseiTrainingConfig{MachineShape: "NvidiaTeslaT4", DeleteRemoteAfterRun: true},
	}
	for name, content := range map[string]string{
		"empty file":    "{}\n",
		"null section":  "chibi_sensei:\n",
		"null training": "chibi_sensei:\n  training:\n",
	} {
		cfg, err := loadChibiSenseiYAML(t, content)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if cfg.ChibiSensei != want {
			t.Fatalf("%s: chibi_sensei = %+v, want %+v", name, cfg.ChibiSensei, want)
		}
	}
	cfg, err := loadChibiSenseiYAML(t, "chibi_sensei:\n  enabled: false\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ChibiSensei.Enabled {
		t.Fatal("an explicit enabled: false was overridden")
	}
}

func TestNormalizeChibiSenseiConfigRanges(t *testing.T) {
	ok := []ChibiSenseiConfig{
		{Storage: ChibiSenseiStorageConfig{MaxGB: 1}},
		{Storage: ChibiSenseiStorageConfig{MaxGB: 500}},
		{Dataset: ChibiSenseiDatasetConfig{MaxRows: 1000}},
		{Import: ChibiSenseiImportConfig{MaxRows: 100}},
		{Import: ChibiSenseiImportConfig{MaxFileMB: 500}},
		{Generator: ChibiSenseiGeneratorConfig{MaxRowsPerJob: 10}},
		{Generator: ChibiSenseiGeneratorConfig{MaxDailyRows: 100000}},
	}
	for _, c := range ok {
		c := c
		if err := NormalizeChibiSenseiConfig(&c); err != nil {
			t.Errorf("%+v rejected: %v", c, err)
		}
	}
	trim := ChibiSenseiConfig{Generator: ChibiSenseiGeneratorConfig{Provider: "  p1 "}}
	if err := NormalizeChibiSenseiConfig(&trim); err != nil || trim.Generator.Provider != "p1" {
		t.Errorf("provider trim: %q (%v)", trim.Generator.Provider, err)
	}
	bad := []ChibiSenseiConfig{
		{Storage: ChibiSenseiStorageConfig{MaxGB: 501}},
		{Storage: ChibiSenseiStorageConfig{MaxGB: -1}},
		{Dataset: ChibiSenseiDatasetConfig{MaxRows: 999}},
		{Dataset: ChibiSenseiDatasetConfig{MaxRows: 50001}},
		{Import: ChibiSenseiImportConfig{MaxRows: 99}},
		{Import: ChibiSenseiImportConfig{MaxFileMB: 501}},
		{Generator: ChibiSenseiGeneratorConfig{MaxRowsPerJob: 9}},
		{Generator: ChibiSenseiGeneratorConfig{MaxDailyRows: 99}},
	}
	for _, c := range bad {
		c := c
		if NormalizeChibiSenseiConfig(&c) == nil {
			t.Errorf("%+v accepted", c)
		}
	}
	if _, err := loadChibiSenseiYAML(t, "chibi_sensei:\n  dataset:\n    max_rows: 5\n"); err == nil {
		t.Error("Load accepted dataset.max_rows below 1000")
	}
}

// The shipped template must describe the same defaults the loader applies.
func TestChibiSenseiTemplateMatchesLoadDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse template: %v", err)
	}
	node, ok := doc["chibi_sensei"]
	if !ok {
		t.Fatal("config_template.yaml has no chibi_sensei: block")
	}
	raw, err := yaml.Marshal(&node)
	if err != nil {
		t.Fatal(err)
	}
	var tpl ChibiSenseiConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("template block does not decode into ChibiSenseiConfig: %v", err)
	}
	if err := NormalizeChibiSenseiConfig(&tpl); err != nil {
		t.Fatalf("template values rejected: %v", err)
	}
	cfg, err := loadChibiSenseiYAML(t, "{}\n")
	if err != nil {
		t.Fatal(err)
	}
	if tpl != cfg.ChibiSensei {
		t.Fatalf("template %+v differs from loader defaults %+v", tpl, cfg.ChibiSensei)
	}
}

func TestChibiSenseiGeneratorProviderReference(t *testing.T) {
	var c Config
	c.ChibiSensei.Generator.Provider = "p1"
	for _, ref := range c.ProviderReferences() {
		if ref.Path == "chibi_sensei.generator.provider" && ref.Role == "chibi_sensei_generator" && ref.ProviderID == "p1" {
			return
		}
	}
	t.Fatalf("chibi_sensei.generator.provider reference missing: %+v", c.ProviderReferences())
}

func TestChibiSenseiTrainingConfig(t *testing.T) {
	cfg, err := loadChibiSenseiYAML(t, "chibi_sensei:\n  training:\n    delete_remote_after_run: false\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ChibiSensei.Training.DeleteRemoteAfterRun || cfg.ChibiSensei.Training.MachineShape != "NvidiaTeslaT4" {
		t.Fatalf("explicit false or default shape lost: %+v", cfg.ChibiSensei.Training)
	}
	cfg, err = loadChibiSenseiYAML(t, "chibi_sensei:\n  training:\n    machine_shape: \" nvidial4 \"\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ChibiSensei.Training.MachineShape != "NvidiaL4" || !cfg.ChibiSensei.Training.DeleteRemoteAfterRun {
		t.Fatalf("shape not canonical or delete default lost: %+v", cfg.ChibiSensei.Training)
	}
	for _, shape := range []string{"NvidiaA100", "T4", "NvidiaTeslaT4,NvidiaL4"} {
		c := ChibiSenseiConfig{Training: ChibiSenseiTrainingConfig{MachineShape: shape}}
		err := NormalizeChibiSenseiConfig(&c)
		if err == nil || !strings.Contains(err.Error(), "training.machine_shape must be one of NvidiaTeslaT4, NvidiaL4") {
			t.Errorf("shape %q: %v", shape, err)
		}
	}
	if _, err := loadChibiSenseiYAML(t, "chibi_sensei:\n  training:\n    machine_shape: NvidiaA100\n"); err == nil {
		t.Error("Load accepted an unknown machine shape")
	}
	var c ChibiSenseiConfig
	if err := NormalizeChibiSenseiConfig(&c); err != nil || c.Training.MachineShape != "NvidiaTeslaT4" || c.Training.DeleteRemoteAfterRun {
		t.Fatalf("normalize must default the shape and leave the bool alone: %+v %v", c.Training, err)
	}
}
