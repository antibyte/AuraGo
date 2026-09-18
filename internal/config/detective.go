package config

// DetectiveProfile overrides one server-owned research effort profile.
type DetectiveProfile struct {
	Seconds    int `yaml:"seconds" json:"seconds"`
	Tools      int `yaml:"tools" json:"tools"`
	Iterations int `yaml:"iterations" json:"iterations"`
	Tokens     int `yaml:"tokens" json:"tokens"`
}

type DetectiveConfig struct {
	Enabled  bool                        `yaml:"enabled" json:"enabled"`
	ReadOnly bool                        `yaml:"readonly" json:"readonly"`
	Profiles map[string]DetectiveProfile `yaml:"profiles" json:"profiles"`
	// ExtraReadOperations is an administrator-owned exact operation allowlist.
	// Connected private services also require selection in the research case.
	ExtraReadOperations map[string][]string `yaml:"extra_read_operations" json:"extra_read_operations"`
}
