package config

import "strings"

// KaggleConfig separates remote resource grants from local notebook execution.
type KaggleConfig struct {
	Enabled               bool              `yaml:"enabled" json:"enabled"`
	ReadOnly              bool              `yaml:"read_only" json:"read_only"`
	AuthMode              string            `yaml:"auth_mode" json:"auth_mode"`
	Username              string            `yaml:"username" json:"username"`
	Token                 string            `yaml:"-" json:"-" vault:"kaggle_api_token"`
	LegacyKey             string            `yaml:"-" json:"-" vault:"kaggle_legacy_key"`
	AllowDownload         bool              `yaml:"allow_download" json:"allow_download"`
	AllowCreate           bool              `yaml:"allow_create" json:"allow_create"`
	AllowUpdate           bool              `yaml:"allow_update" json:"allow_update"`
	AllowPublish          bool              `yaml:"allow_publish" json:"allow_publish"`
	AllowDelete           bool              `yaml:"allow_delete" json:"allow_delete"`
	AllowSubmit           bool              `yaml:"allow_submit" json:"allow_submit"`
	AllowRemoteExecute    bool              `yaml:"allow_remote_execute" json:"allow_remote_execute"`
	AllowGPU              bool              `yaml:"allow_gpu" json:"allow_gpu"`
	AllowTPU              bool              `yaml:"allow_tpu" json:"allow_tpu"`
	AllowInternet         bool              `yaml:"allow_internet" json:"allow_internet"`
	AllowedOwners         []string          `yaml:"allowed_owners" json:"allowed_owners"`
	MaxTransferMB         int               `yaml:"max_transfer_mb" json:"max_transfer_mb"`
	MaxStorageMB          int               `yaml:"max_storage_mb" json:"max_storage_mb"`
	MaxRemoteJobs         int               `yaml:"max_remote_jobs" json:"max_remote_jobs"`
	RemoteTimeoutMinutes  int               `yaml:"remote_timeout_minutes" json:"remote_timeout_minutes"`
	RequestTimeoutSeconds int               `yaml:"request_timeout_seconds" json:"request_timeout_seconds"`
	Local                 KaggleLocalConfig `yaml:"local" json:"local"`
}

type KaggleLocalConfig struct {
	Enabled            bool `yaml:"enabled" json:"enabled"`
	AllowExecute       bool `yaml:"allow_execute" json:"allow_execute"`
	AllowInternet      bool `yaml:"allow_internet" json:"allow_internet"`
	MaxSessions        int  `yaml:"max_sessions" json:"max_sessions"`
	CPUCores           int  `yaml:"cpu_cores" json:"cpu_cores"`
	MemoryMB           int  `yaml:"memory_mb" json:"memory_mb"`
	IdleMinutes        int  `yaml:"idle_minutes" json:"idle_minutes"`
	CellTimeoutMinutes int  `yaml:"cell_timeout_minutes" json:"cell_timeout_minutes"`
}

func DefaultKaggleConfig() KaggleConfig {
	c := KaggleConfig{ReadOnly: true}
	NormalizeKaggleConfig(&c)
	return c
}

func NormalizeKaggleConfig(c *KaggleConfig) {
	c.AuthMode = strings.ToLower(strings.TrimSpace(c.AuthMode))
	if c.AuthMode == "" {
		c.AuthMode = "token"
	}
	c.Username = strings.TrimSpace(c.Username)
	for i := range c.AllowedOwners {
		c.AllowedOwners[i] = strings.TrimSpace(c.AllowedOwners[i])
	}
	bounded := func(p *int, fallback, max int) {
		if *p <= 0 {
			*p = fallback
		}
		if *p > max {
			*p = max
		}
	}
	bounded(&c.MaxTransferMB, 2048, 1048576)
	bounded(&c.MaxStorageMB, 10240, 10485760)
	bounded(&c.MaxRemoteJobs, 1, 16)
	bounded(&c.RemoteTimeoutMinutes, 60, 720)
	bounded(&c.RequestTimeoutSeconds, 60, 300)
	bounded(&c.Local.MaxSessions, 1, 8)
	bounded(&c.Local.CPUCores, 2, 64)
	bounded(&c.Local.MemoryMB, 4096, 262144)
	bounded(&c.Local.IdleMinutes, 15, 240)
	bounded(&c.Local.CellTimeoutMinutes, 30, 720)
}
