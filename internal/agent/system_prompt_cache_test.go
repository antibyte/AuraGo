package agent

import (
	"strings"
	"testing"
	"time"

	"aurago/internal/prompts"
)

func TestBuildSystemPromptCacheKey_DifferentFlags(t *testing.T) {
	baseFlags := prompts.ContextFlags{
		Tier:        "full",
		TokenBudget: 1000,
		IsMission:   false,
	}
	baseKey, err := buildSystemPromptCacheKey("/prompts", &baseFlags, "", "hint")
	if err != nil {
		t.Fatalf("failed to build base cache key: %v", err)
	}

	tests := []struct {
		name       string
		modify     func(*prompts.ContextFlags)
		wantNewKey bool
	}{
		{
			name:       "IsErrorState changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.IsErrorState = true },
			wantNewKey: true,
		},
		{
			name:       "RequiresCoding changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.RequiresCoding = true },
			wantNewKey: true,
		},
		{
			name:       "SystemLanguage changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.SystemLanguage = "de" },
			wantNewKey: true,
		},
		{
			name:       "CorePersonality changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.CorePersonality = "punk" },
			wantNewKey: true,
		},
		{
			name:       "AdditionalPrompt changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.AdditionalPrompt = "extra instructions" },
			wantNewKey: true,
		},
		{
			name:       "InnerVoice changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.InnerVoice = "think carefully" },
			wantNewKey: true,
		},
		{
			name:       "LearnedRulesContext changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.LearnedRulesContext = "avoid stale docker state" },
			wantNewKey: true,
		},
		{
			name:       "SurgeryPlan changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.IsMaintenanceMode = true; f.SurgeryPlan = "restart service safely" },
			wantNewKey: true,
		},
		{
			name:       "SpecialistsSuggestion changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.SpecialistsSuggestion = "Delegate frontend audit" },
			wantNewKey: true,
		},
		{
			name:       "IsCoAgent changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.IsCoAgent = true },
			wantNewKey: true,
		},
		{
			name:       "IsEgg changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.IsEgg = true },
			wantNewKey: true,
		},
		{
			name: "SpaceAgentPublicURL changes cache key",
			modify: func(f *prompts.ContextFlags) {
				f.SpaceAgentEnabled = true
				f.SpaceAgentPublicURL = "https://space.example/"
			},
			wantNewKey: true,
		},
		{
			name:       "ToolsDir changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.ToolsDir = "/workspace/tools" },
			wantNewKey: true,
		},
		{
			name:       "SkillsDir changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.SkillsDir = "/workspace/skills" },
			wantNewKey: true,
		},
		{
			name:       "PredictedGuides changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.PredictedGuides = []string{"guide1", "guide2"} },
			wantNewKey: true,
		},
		{
			name:       "HighPriorityNotes changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.HighPriorityNotes = "URGENT" },
			wantNewKey: true,
		},
		{
			name:       "PlannerContext changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.PlannerContext = "Open todos: 2" },
			wantNewKey: true,
		},
		{
			name:       "OperationalIssueReminder changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.OperationalIssueReminder = "System issue: background failure" },
			wantNewKey: true,
		},
		{
			name:       "ReuseContext changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.ReuseContext = "reuse this cheatsheet first" },
			wantNewKey: true,
		},
		{
			name:       "TaskRules changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.TaskRules = "# Rule\nUse homepage tool." },
			wantNewKey: true,
		},
		{
			name:       "TaskRuleIDs changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.TaskRuleIDs = []string{"homepage"} },
			wantNewKey: true,
		},
		{
			name:       "HomepageDesignSystem changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.HomepageDesignSystem = "## Colors\nUse tokens." },
			wantNewKey: true,
		},
		{
			name:       "SessionTodoItems changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.SessionTodoItems = "task1, task2" },
			wantNewKey: true,
		},
		{
			name:       "SkipIntegrationTools changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.SkipIntegrationTools = []string{"docker"} },
			wantNewKey: true,
		},
		{
			name:       "WebhooksDefinitions changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.WebhooksDefinitions = "webhook1, webhook2" },
			wantNewKey: true,
		},
		{
			name:       "Unchanged flags produce same cache key",
			modify:     func(f *prompts.ContextFlags) { /* no-op */ },
			wantNewKey: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := baseFlags
			tt.modify(&flags)
			key, err := buildSystemPromptCacheKey("/prompts", &flags, "", "hint")
			if err != nil {
				t.Fatalf("failed to build cache key: %v", err)
			}
			if tt.wantNewKey && key == baseKey {
				t.Errorf("expected cache key to change but it stayed the same")
			}
			if !tt.wantNewKey && key != baseKey {
				t.Errorf("expected cache key to stay the same but it changed")
			}
		})
	}
}

func TestBuildSystemPromptCacheKey_SkipIntegrationToolsOrderInsensitive(t *testing.T) {
	flagsA := prompts.ContextFlags{
		Tier:                 "full",
		SkipIntegrationTools: []string{"github", "docker"},
	}
	flagsB := prompts.ContextFlags{
		Tier:                 "full",
		SkipIntegrationTools: []string{"docker", "github"},
	}

	keyA, err := buildSystemPromptCacheKey("/prompts", &flagsA, "", "")
	if err != nil {
		t.Fatalf("build key A: %v", err)
	}
	keyB, err := buildSystemPromptCacheKey("/prompts", &flagsB, "", "")
	if err != nil {
		t.Fatalf("build key B: %v", err)
	}
	if keyA != keyB {
		t.Fatalf("expected SkipIntegrationTools order-insensitive cache key")
	}
}

func TestBuildSystemPromptCacheKey_TaskRuleIDsOrderInsensitive(t *testing.T) {
	flagsA := prompts.ContextFlags{
		Tier:        "full",
		TaskRuleIDs: []string{"homepage", "shell"},
	}
	flagsB := prompts.ContextFlags{
		Tier:        "full",
		TaskRuleIDs: []string{"shell", "homepage"},
	}

	keyA, err := buildSystemPromptCacheKey("/prompts", &flagsA, "", "")
	if err != nil {
		t.Fatalf("build key A: %v", err)
	}
	keyB, err := buildSystemPromptCacheKey("/prompts", &flagsB, "", "")
	if err != nil {
		t.Fatalf("build key B: %v", err)
	}
	if keyA != keyB {
		t.Fatalf("expected TaskRuleIDs order-insensitive cache key")
	}
}

func TestRefreshCachedSystemPromptNowUpdatesNowLine(t *testing.T) {
	prompt := "# SYSTEM\nstable\n\n# NOW\n2026-06-08 10:11\n> **Channel:** Web Chat\n"
	got := refreshCachedSystemPromptNow(prompt, time.Date(2026, 6, 8, 12, 34, 56, 0, time.UTC))

	if got == prompt {
		t.Fatal("expected # NOW line to be refreshed")
	}
	if want := "# NOW\n2026-06-08 12:34\n"; !strings.Contains(got, want) {
		t.Fatalf("refreshed prompt missing %q:\n%s", want, got)
	}
	if !strings.Contains(got, "> **Channel:** Web Chat") {
		t.Fatalf("refresh dropped content after # NOW:\n%s", got)
	}
}
