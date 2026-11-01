package config

// FindItConfig separates human inventory access from the native agent read grant.
type FindItConfig struct {
	Enabled     bool `yaml:"enabled" json:"enabled"`
	AgentAccess bool `yaml:"agent_access" json:"agent_access"`
	ReadOnly    bool `yaml:"readonly" json:"readonly"`
}
