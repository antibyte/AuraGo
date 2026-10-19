package config

import (
	"fmt"
	"strings"
)

// ActionPolicyConfig supplements capability gates with explicit action grants.
// Read-only settings and other hard denials always take precedence.
type ActionPolicyConfig struct {
	Enabled bool               `yaml:"enabled" json:"enabled"`
	Allow   []ActionPolicyRule `yaml:"allow" json:"allow"`
	Deny    []ActionPolicyRule `yaml:"deny" json:"deny"`
}

// ActionPolicyRule matches canonical names and stable, non-secret target IDs.
// An explicit "*" target permits every target; an empty target never does.
type ActionPolicyRule struct {
	Tool          string `yaml:"tool" json:"tool"`
	Operation     string `yaml:"operation" json:"operation"`
	Target        string `yaml:"target" json:"target"`
	RunType       string `yaml:"run_type,omitempty" json:"run_type,omitempty"`
	ScopeID       string `yaml:"scope_id,omitempty" json:"scope_id,omitempty"`
	ArgumentsHash string `yaml:"arguments_hash,omitempty" json:"arguments_hash,omitempty"`
}

func ValidateActionPolicy(policy ActionPolicyConfig) error {
	if len(policy.Allow)+len(policy.Deny) > 1000 {
		return fmt.Errorf("action policy permits at most 1000 rules")
	}
	for _, rules := range [][]ActionPolicyRule{policy.Allow, policy.Deny} {
		for i, rule := range rules {
			if strings.TrimSpace(rule.Tool) == "" || strings.TrimSpace(rule.Operation) == "" || strings.TrimSpace(rule.Target) == "" {
				return fmt.Errorf("action policy rule %d requires tool, operation and target", i+1)
			}
			if len(rule.Tool) > 128 || len(rule.Operation) > 128 || len(rule.Target) > 2048 || len(rule.ScopeID) > 256 {
				return fmt.Errorf("action policy rule %d exceeds its field limit", i+1)
			}
			if rule.ArgumentsHash != "" {
				if len(rule.ArgumentsHash) != 64 || strings.ContainsAny(rule.ArgumentsHash, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
					return fmt.Errorf("action policy rule %d requires a lowercase SHA-256 arguments hash", i+1)
				}
				for _, c := range rule.ArgumentsHash {
					if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
						return fmt.Errorf("action policy rule %d has an invalid arguments hash", i+1)
					}
				}
			}
			switch rule.RunType {
			case "", "interactive", "mission", "flow", "maintenance", "coagent", "background":
			default:
				return fmt.Errorf("action policy rule %d has an unknown run type", i+1)
			}
		}
	}
	return nil
}
