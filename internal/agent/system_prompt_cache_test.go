package agent

import (
	"testing"

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
			name:       "ReuseContext changes cache key",
			modify:     func(f *prompts.ContextFlags) { f.ReuseContext = "reuse this cheatsheet first" },
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
