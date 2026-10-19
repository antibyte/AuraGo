package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActionPolicyRejectsIncompleteOrAmbiguousRules(t *testing.T) {
	valid := ActionPolicyRule{Tool: "docker", Operation: "remove", Target: "container_id:fixture"}
	for _, test := range []struct {
		name string
		edit func(*ActionPolicyRule)
	}{
		{"tool", func(r *ActionPolicyRule) { r.Tool = " " }},
		{"operation", func(r *ActionPolicyRule) { r.Operation = "" }},
		{"target", func(r *ActionPolicyRule) { r.Target = "" }},
		{"run type", func(r *ActionPolicyRule) { r.RunType = "anything" }},
		{"hash case", func(r *ActionPolicyRule) { r.ArgumentsHash = strings.Repeat("A", 64) }},
		{"hash alphabet", func(r *ActionPolicyRule) { r.ArgumentsHash = strings.Repeat("z", 64) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			rule := valid
			test.edit(&rule)
			if err := ValidateActionPolicy(ActionPolicyConfig{Allow: []ActionPolicyRule{rule}}); err == nil {
				t.Fatal("invalid action rule was accepted")
			}
		})
	}
	valid.RunType, valid.ScopeID, valid.ArgumentsHash = "mission", "fixture", strings.Repeat("a", 64)
	if err := ValidateActionPolicy(ActionPolicyConfig{Allow: []ActionPolicyRule{valid}}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateActionPolicy(ActionPolicyConfig{Deny: make([]ActionPolicyRule, 1001)}); err == nil {
		t.Fatal("unbounded action rules were accepted")
	}
}

func TestAuditDefaultsPreserveExplicitExistingConfiguration(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "defaults", true: "existing overrides"}[explicit], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			data := "llm:\n  model: fixture\n"
			if explicit {
				data += "agent:\n  action_policy:\n    enabled: false\n  max_cron_jobs: 1700\n  background_tasks:\n    max_active_tasks: 32\nsandbox:\n  enabled: false\nbudget:\n  enabled: false\n  daily_limit_usd: 11\n  enforcement: block\n"
			}
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if explicit {
				if cfg.Agent.ActionPolicy.Enabled || cfg.Sandbox.Enabled || cfg.Budget.Enabled || cfg.Agent.MaxCronJobs != 1700 || cfg.Agent.BackgroundTasks.MaxActiveTasks != 32 || cfg.Budget.DailyLimitUSD != 11 {
					t.Fatal("existing explicit settings were changed")
				}
				return
			}
			if !cfg.Agent.ActionPolicy.Enabled || !cfg.Sandbox.Enabled || cfg.Agent.MaxCronJobs != 1000 || cfg.Agent.BackgroundTasks.MaxActiveTasks != 256 || cfg.Sandbox.MemoryMB != 512 || cfg.Sandbox.PIDsLimit != 64 {
				t.Fatal("bounded execution defaults were not applied")
			}
		})
	}
}
