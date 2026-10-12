package config

import "fmt"

// DecisionsConfig controls AuraGo's optional local decision-model service.
// Model and runtime pins are deliberately owned by the application rather
// than being overridable through user configuration.
type DecisionsConfig struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	Deployment         string `yaml:"deployment" json:"deployment"`
	CPUFallbackAllowed bool   `yaml:"cpu_fallback_allowed" json:"cpu_fallback_allowed"`
}

const (
	DecisionsDeploymentAuto   = "auto"
	DecisionsDeploymentNative = "native"
	DecisionsDeploymentDocker = "docker"
)

// Normalize applies safe defaults without enabling the service or CPU use.
func (c DecisionsConfig) Normalize() DecisionsConfig {
	if c.Deployment == "" {
		c.Deployment = DecisionsDeploymentAuto
	}
	return c
}

// NormalizeDecisionsConfig applies safe defaults in place.
func NormalizeDecisionsConfig(c *DecisionsConfig) {
	if c == nil {
		return
	}
	*c = c.Normalize()
}

// Validate rejects deployment modes that cannot be safely managed.
func (c DecisionsConfig) Validate() error {
	c = c.Normalize()
	switch c.Deployment {
	case DecisionsDeploymentAuto, DecisionsDeploymentNative, DecisionsDeploymentDocker:
		return nil
	default:
		return fmt.Errorf("unsupported decisions deployment %q", c.Deployment)
	}
}

// ValidateDecisionsConfig checks the selected managed deployment policy.
func ValidateDecisionsConfig(c DecisionsConfig) error {
	return c.Validate()
}
