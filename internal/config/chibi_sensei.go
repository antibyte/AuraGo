package config

import (
	"fmt"
	"strings"
)

// ChibiSenseiConfig configures the Chibi Sensei desktop app (small-model training studio). Enabled
// defaults to true in the loader. Later phases add Agent, Training and Runtime as sibling fields;
// none of the fields below is renamed then.
type ChibiSenseiConfig struct {
	Enabled   bool                       `yaml:"enabled" json:"enabled"`
	Storage   ChibiSenseiStorageConfig   `yaml:"storage" json:"storage"`
	Dataset   ChibiSenseiDatasetConfig   `yaml:"dataset" json:"dataset"`
	Import    ChibiSenseiImportConfig    `yaml:"import" json:"import"`
	Generator ChibiSenseiGeneratorConfig `yaml:"generator" json:"generator"`
}

// ChibiSenseiStorageConfig caps everything Chibi Sensei stores under the data directory.
type ChibiSenseiStorageConfig struct {
	MaxGB int `yaml:"max_gb" json:"max_gb"` // 1..500, default 20
}

// ChibiSenseiDatasetConfig bounds one project's dataset.
type ChibiSenseiDatasetConfig struct {
	MaxRows int `yaml:"max_rows" json:"max_rows"` // 1000..50000, default 50000
}

// ChibiSenseiImportConfig bounds one workspace or Hugging Face import.
type ChibiSenseiImportConfig struct {
	MaxRows   int `yaml:"max_rows" json:"max_rows"`       // 100..50000, default 20000
	MaxFileMB int `yaml:"max_file_mb" json:"max_file_mb"` // 1..500, default 50
}

// ChibiSenseiGeneratorConfig is the model slot of generation and cleaning jobs.
type ChibiSenseiGeneratorConfig struct {
	Provider      string `yaml:"provider" json:"provider"`                 // empty = main route
	Model         string `yaml:"model" json:"model"`                       // empty = provider's model
	MaxRowsPerJob int    `yaml:"max_rows_per_job" json:"max_rows_per_job"` // 10..5000, default 500
	MaxDailyRows  int    `yaml:"max_daily_rows" json:"max_daily_rows"`     // 100..100000, default 5000
}

// NormalizeChibiSenseiConfig trims the provider and model, applies defaults for zero values and
// rejects values outside their range. It does not change Enabled.
func NormalizeChibiSenseiConfig(c *ChibiSenseiConfig) error {
	c.Generator.Provider = strings.TrimSpace(c.Generator.Provider)
	c.Generator.Model = strings.TrimSpace(c.Generator.Model)
	for _, f := range []struct {
		p           *int
		def, lo, hi int
		key         string
	}{
		{&c.Storage.MaxGB, 20, 1, 500, "storage.max_gb"},
		{&c.Dataset.MaxRows, 50000, 1000, 50000, "dataset.max_rows"},
		{&c.Import.MaxRows, 20000, 100, 50000, "import.max_rows"},
		{&c.Import.MaxFileMB, 50, 1, 500, "import.max_file_mb"},
		{&c.Generator.MaxRowsPerJob, 500, 10, 5000, "generator.max_rows_per_job"},
		{&c.Generator.MaxDailyRows, 5000, 100, 100000, "generator.max_daily_rows"},
	} {
		if *f.p == 0 {
			*f.p = f.def
		}
		if *f.p < f.lo || *f.p > f.hi {
			return fmt.Errorf("chibi_sensei: %s must be %d–%d", f.key, f.lo, f.hi)
		}
	}
	return nil
}
