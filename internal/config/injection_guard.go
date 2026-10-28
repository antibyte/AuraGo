package config

import "fmt"

func ValidateInjectionGuardConfig(cfg *Config) error {
	if cfg.Guardian.InjectionGuard.Action == "" {
		cfg.Guardian.InjectionGuard.Action = "quarantine"
	}
	switch cfg.Guardian.InjectionGuard.Action {
	case "quarantine", "datamark":
		return nil
	}
	return fmt.Errorf("guardian.injection_guard.action must be quarantine or datamark")
}
