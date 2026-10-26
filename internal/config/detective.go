package config

import "strings"

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

// NormalizeDetective trims the administrator-owned operation allowlist, drops empty
// entries and empty tools so a typo cannot smuggle a blank operation into the policy.
// Load and the config-save path call it, so an API round-trip is stable.
func (c *DetectiveConfig) Normalize() {
	if c.ExtraReadOperations == nil {
		return
	}
	cleaned := make(map[string][]string, len(c.ExtraReadOperations))
	for tool, operations := range c.ExtraReadOperations {
		tool = strings.TrimSpace(tool)
		if tool == "" {
			continue
		}
		kept := make([]string, 0, len(operations))
		seen := map[string]bool{}
		for _, operation := range operations {
			operation = strings.TrimSpace(operation)
			if operation == "" || seen[operation] {
				continue
			}
			seen[operation] = true
			kept = append(kept, operation)
		}
		if len(kept) > 0 {
			cleaned[tool] = kept
		}
	}
	if len(cleaned) == 0 {
		c.ExtraReadOperations = nil
		return
	}
	c.ExtraReadOperations = cleaned
}
