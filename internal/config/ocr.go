package config

// OCRConfig controls the optional, locally managed Indra OCR service.
type OCRConfig struct {
	Enabled           bool `yaml:"enabled" json:"enabled"`
	AgentAccess       bool `yaml:"agent_access" json:"agent_access"`
	AutomaticChat     bool `yaml:"automatic_chat" json:"automatic_chat"`
	AutomaticIndexing bool `yaml:"automatic_indexing" json:"automatic_indexing"`
}

func DefaultOCRConfig() OCRConfig {
	return OCRConfig{AgentAccess: true, AutomaticChat: true, AutomaticIndexing: true}
}
