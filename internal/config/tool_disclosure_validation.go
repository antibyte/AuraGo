package config

import (
	"fmt"
	"math"
	"strings"
)

// ValidateToolDisclosureSettings is shared by file loading and pre-write API validation.
func ValidateToolDisclosureSettings(cfg *Config) error {
	if cfg.Agent.ToolOutputLimit < 0 {
		return fmt.Errorf("agent.tool_output_limit must be zero (automatic) or positive bytes")
	}
	if math.IsNaN(cfg.MemoryAnalysis.AutoConfirm) || math.IsInf(cfg.MemoryAnalysis.AutoConfirm, 0) || cfg.MemoryAnalysis.AutoConfirm < 0 || cfg.MemoryAnalysis.AutoConfirm > 1 {
		return fmt.Errorf("memory_analysis.auto_confirm_threshold must be between 0 and 1")
	}
	if day := strings.TrimSpace(cfg.MemoryAnalysis.ReflectionDay); day != "" {
		switch day {
		case "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday":
		default:
			return fmt.Errorf("memory_analysis.reflection_day must be a weekday name")
		}
	}
	return nil
}
