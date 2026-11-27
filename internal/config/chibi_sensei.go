package config

import (
	"fmt"
	"slices"
	"strings"
)

// ChibiSenseiConfig configures the Chibi Sensei desktop app (small-model training studio). Enabled,
// Training.DeleteRemoteAfterRun and Runtime.Enabled default to true in the loader, because a YAML bool cannot
// tell "absent" from false. A later phase adds Agent as a sibling field; none of the fields below is renamed then.
type ChibiSenseiConfig struct {
	Enabled   bool                       `yaml:"enabled" json:"enabled"`
	Storage   ChibiSenseiStorageConfig   `yaml:"storage" json:"storage"`
	Dataset   ChibiSenseiDatasetConfig   `yaml:"dataset" json:"dataset"`
	Import    ChibiSenseiImportConfig    `yaml:"import" json:"import"`
	Generator ChibiSenseiGeneratorConfig `yaml:"generator" json:"generator"`
	Training  ChibiSenseiTrainingConfig  `yaml:"training" json:"training"`
	Runtime   ChibiSenseiRuntimeConfig   `yaml:"runtime" json:"runtime"`
}

// ChibiSenseiRuntimeBackends lists the accepted runtime.backend values.
var ChibiSenseiRuntimeBackends = []string{"auto", "cpu", "vulkan", "cuda"}

// ChibiSenseiRuntimeConfig controls the local llama.cpp chat of the Test step.
type ChibiSenseiRuntimeConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`           // default true (set by the loader); nothing downloads until an administrator confirms
	Backend     string `yaml:"backend" json:"backend"`           // one of ChibiSenseiRuntimeBackends; default auto
	IdleMinutes int    `yaml:"idle_minutes" json:"idle_minutes"` // 1..120, default 10
	ContextSize int    `yaml:"context_size" json:"context_size"` // 1024..32768, default 4096
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

// DefaultChibiSenseiMachineShape is the Kaggle machine shape of a training run when none is set.
const DefaultChibiSenseiMachineShape = "NvidiaTeslaT4"

// ChibiSenseiTrainingConfig controls remote training runs on Kaggle.
type ChibiSenseiTrainingConfig struct {
	MachineShape         string `yaml:"machine_shape" json:"machine_shape"`                     // one of KaggleMachineShapes; default NvidiaTeslaT4
	DeleteRemoteAfterRun bool   `yaml:"delete_remote_after_run" json:"delete_remote_after_run"` // default true (set by the loader)
}

// NormalizeChibiSenseiConfig trims the provider and model, applies defaults for zero values and
// rejects values outside their range or an unknown training machine shape. It changes neither
// Enabled nor Training.DeleteRemoteAfterRun; the loader sets their defaults before decoding.
func NormalizeChibiSenseiConfig(c *ChibiSenseiConfig) error {
	c.Generator.Provider = strings.TrimSpace(c.Generator.Provider)
	c.Generator.Model = strings.TrimSpace(c.Generator.Model)
	if strings.TrimSpace(c.Training.MachineShape) == "" {
		c.Training.MachineShape = DefaultChibiSenseiMachineShape
	} else if shape, ok := CanonicalKaggleMachineShape(c.Training.MachineShape); ok {
		c.Training.MachineShape = shape
	} else {
		return fmt.Errorf("chibi_sensei: training.machine_shape must be one of %s", strings.Join(KaggleMachineShapes, ", "))
	}
	c.Runtime.Backend = strings.ToLower(strings.TrimSpace(c.Runtime.Backend))
	if c.Runtime.Backend == "" {
		c.Runtime.Backend = "auto"
	}
	if !slices.Contains(ChibiSenseiRuntimeBackends, c.Runtime.Backend) {
		return fmt.Errorf("chibi_sensei: runtime.backend must be one of %s", strings.Join(ChibiSenseiRuntimeBackends, ", "))
	}
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
		{&c.Runtime.IdleMinutes, 10, 1, 120, "runtime.idle_minutes"},
		{&c.Runtime.ContextSize, 4096, 1024, 32768, "runtime.context_size"},
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
