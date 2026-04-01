package agent

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"aurago/internal/config"
)

// ══════════════════════════════════════════════
// extractSpecialistRole
// ══════════════════════════════════════════════

func TestExtractSpecialistRole(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		want      string
	}{
		{"generic co-agent", "coagent-5", ""},
		{"researcher", "specialist-researcher-1", "researcher"},
		{"coder", "specialist-coder-23", "coder"},
		{"designer", "specialist-designer-100", "designer"},
		{"security", "specialist-security-3", "security"},
		{"writer", "specialist-writer-42", "writer"},
		{"empty string", "", ""},
		{"main session", "main", ""},
		{"prefix only", "specialist-", ""},
		{"no counter", "specialist-coder", "coder"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSpecialistRole(tt.sessionID)
			if got != tt.want {
				t.Errorf("extractSpecialistRole(%q) = %q, want %q", tt.sessionID, got, tt.want)
			}
		})
	}
}

// ══════════════════════════════════════════════
// checkSpecialistToolRestriction
// ══════════════════════════════════════════════

func TestCheckSpecialistToolRestriction(t *testing.T) {
	tests := []struct {
		name      string
		role      string
		action    string
		operation string
		blocked   bool
	}{
		// Researcher
		{"researcher can search", "researcher", "api_request", "", false},
		{"researcher blocked shell", "researcher", "execute_shell", "", true},
		{"researcher blocked imagegen", "researcher", "image_generation", "", true},
		{"researcher blocked remote", "researcher", "remote_control", "", true},
		{"researcher blocked homepage", "researcher", "homepage", "", true},
		{"researcher can query memory", "researcher", "query_memory", "", false},
		{"researcher can use python", "researcher", "execute_python", "", false},

		// Coder
		{"coder can shell", "coder", "execute_shell", "", false},
		{"coder can python", "coder", "execute_python", "", false},
		{"coder can filesystem", "coder", "filesystem", "write", false},
		{"coder blocked imagegen", "coder", "image_generation", "", true},
		{"coder blocked remote", "coder", "remote_control", "", true},

		// Designer
		{"designer can imagegen", "designer", "image_generation", "", false},
		{"designer blocked shell", "designer", "execute_shell", "", true},
		{"designer blocked python", "designer", "execute_python", "", true},
		{"designer blocked remote", "designer", "remote_control", "", true},
		{"designer can filesystem", "designer", "filesystem", "read", false},

		// Security
		{"security can shell", "security", "execute_shell", "", false},
		{"security can python", "security", "execute_python", "", false},
		{"security blocked imagegen", "security", "image_generation", "", true},
		{"security blocked remote", "security", "remote_control", "", true},
		{"security can read fs", "security", "filesystem", "read", false},
		{"security blocked write fs", "security", "filesystem", "write", true},
		{"security blocked delete fs", "security", "filesystem", "delete", true},
		{"security blocked move fs", "security", "filesystem", "move", true},
		{"security blocked copy fs", "security", "filesystem", "copy", true},

		// Writer
		{"writer can query memory", "writer", "query_memory", "", false},
		{"writer can filesystem read", "writer", "filesystem", "read", false},
		{"writer can filesystem write", "writer", "filesystem", "write", false},
		{"writer blocked shell", "writer", "execute_shell", "", true},
		{"writer blocked python", "writer", "execute_python", "", true},
		{"writer blocked imagegen", "writer", "image_generation", "", true},
		{"writer blocked remote", "writer", "remote_control", "", true},
		{"writer blocked homepage", "writer", "homepage", "", true},

		// Unknown role passes everything
		{"unknown role passes", "unknown", "execute_shell", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkSpecialistToolRestriction(tt.role, tt.action, tt.operation)
			gotBlocked := result != ""
			if gotBlocked != tt.blocked {
				t.Errorf("checkSpecialistToolRestriction(%q, %q, %q) blocked=%v, want blocked=%v (msg=%q)",
					tt.role, tt.action, tt.operation, gotBlocked, tt.blocked, result)
			}
		})
	}
}

func TestDispatchInnerBlocksCoAgentSensitiveAliases(t *testing.T) {
	tests := []struct {
		name      string
		tc        ToolCall
		wantParts []string
	}{
		{
			name:      "blocks secrets vault access",
			tc:        ToolCall{Action: "secrets_vault", Operation: "get", Key: "api_key"},
			wantParts: []string{"error", "cannot access the secrets vault"},
		},
		{
			name:      "blocks remember writes",
			tc:        ToolCall{Action: "remember", Content: "store this"},
			wantParts: []string{"error", "cannot store new facts or memories"},
		},
		{
			name:      "blocks knowledge alias mutation",
			tc:        ToolCall{Action: "manage_knowledge", Operation: "add"},
			wantParts: []string{"error", "cannot modify the knowledge graph"},
		},
		{
			name:      "blocks notes alias mutation",
			tc:        ToolCall{Action: "notes", Operation: "add"},
			wantParts: []string{"error", "cannot modify notes"},
		},
		{
			name:      "blocks journal alias mutation",
			tc:        ToolCall{Action: "journal", Operation: "create"},
			wantParts: []string{"error", "cannot modify journal entries"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dispatchInner(context.Background(), tt.tc, &DispatchContext{SessionID: "specialist-researcher-1"})
			for _, want := range tt.wantParts {
				if !strings.Contains(strings.ToLower(got), strings.ToLower(want)) {
					t.Fatalf("dispatchInner() = %q, want substring %q", got, want)
				}
			}
		})
	}
}

// ══════════════════════════════════════════════
// stripYAMLFrontmatter
// ══════════════════════════════════════════════

func TestStripYAMLFrontmatter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"no frontmatter",
			"Hello world",
			"Hello world",
		},
		{
			"with frontmatter",
			"---\ntitle: test\n---\n# Content here",
			"# Content here",
		},
		{
			"frontmatter with CRLF",
			"---\r\ntitle: test\r\n---\r\n# Content",
			"# Content",
		},
		{
			"unclosed frontmatter",
			"---\ntitle: test\nno close",
			"---\ntitle: test\nno close",
		},
		{
			"empty frontmatter passes through",
			"---\n---\nBody",
			"---\n---\nBody",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripYAMLFrontmatter(tt.input)
			if got != tt.want {
				t.Errorf("stripYAMLFrontmatter() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ══════════════════════════════════════════════
// specialistsAvailable / buildSpecialistsStatus
// ══════════════════════════════════════════════

func TestSpecialistsAvailable(t *testing.T) {
	cfg := specialistTestConfig()
	cfg.CoAgents.Enabled = false
	if specialistsAvailable(cfg) {
		t.Error("expected false when co_agents disabled")
	}

	cfg.CoAgents.Enabled = true
	if specialistsAvailable(cfg) {
		t.Error("expected false when no specialists enabled")
	}

	cfg.CoAgents.Specialists.Researcher.Enabled = true
	if !specialistsAvailable(cfg) {
		t.Error("expected true when researcher enabled")
	}
}

func TestBuildSpecialistsStatus(t *testing.T) {
	cfg := specialistTestConfig()
	cfg.CoAgents.Enabled = false
	if s := buildSpecialistsStatus(cfg); s != "" {
		t.Errorf("expected empty when disabled, got %q", s)
	}

	cfg.CoAgents.Enabled = true
	s := buildSpecialistsStatus(cfg)
	if s != "No specialists are currently enabled." {
		t.Errorf("expected no-specialists message, got %q", s)
	}

	cfg.CoAgents.Specialists.Coder.Enabled = true
	cfg.CoAgents.Specialists.Security.Enabled = true
	s = buildSpecialistsStatus(cfg)
	if s == "" || s == "No specialists are currently enabled." {
		t.Errorf("expected specialist listing, got %q", s)
	}
	if !contains(s, "coder") || !contains(s, "security") {
		t.Errorf("expected coder and security in status, got %q", s)
	}
	if contains(s, "researcher") || contains(s, "designer") || contains(s, "writer") {
		t.Errorf("expected only enabled specialists, got %q", s)
	}
}

// ══════════════════════════════════════════════
// CoAgentRegistry with specialists
// ══════════════════════════════════════════════

func TestRegistrySpecialistPrefix(t *testing.T) {
	logger := specialistTestLogger()
	reg := NewCoAgentRegistry(5, logger)

	id, err := reg.RegisterWithPrefix("specialist-researcher", "research task", nil)
	if err != nil {
		t.Fatalf("RegisterWithPrefix failed: %v", err)
	}
	if id != "specialist-researcher-1" {
		t.Errorf("expected specialist-researcher-1, got %s", id)
	}

	// Check the specialist field
	reg.mu.RLock()
	info := reg.agents[id]
	reg.mu.RUnlock()
	if info.Specialist != "researcher" {
		t.Errorf("expected specialist=researcher, got %q", info.Specialist)
	}

	// Generic co-agent should have empty specialist
	id2, err := reg.Register("generic task", nil)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	reg.mu.RLock()
	info2 := reg.agents[id2]
	reg.mu.RUnlock()
	if info2.Specialist != "" {
		t.Errorf("expected empty specialist for generic, got %q", info2.Specialist)
	}
}

func TestRegistryList(t *testing.T) {
	logger := specialistTestLogger()
	reg := NewCoAgentRegistry(5, logger)

	reg.RegisterWithPrefix("specialist-coder", "code task", nil)
	reg.Register("general task", nil)

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(list))
	}

	// Check specialist field in list output
	foundSpecialist := false
	foundGeneric := false
	for _, m := range list {
		if m["specialist"] == "coder" {
			foundSpecialist = true
		}
		if m["specialist"] == "" {
			foundGeneric = true
		}
	}
	if !foundSpecialist {
		t.Error("expected coder specialist in list output")
	}
	if !foundGeneric {
		t.Error("expected generic co-agent in list output")
	}
}

func TestRegistryQueuesAndPromotesByPriority(t *testing.T) {
	logger := specialistTestLogger()
	reg := NewCoAgentRegistry(1, logger)

	cancel1 := func() {}
	cancel2 := func() {}
	cancel3 := func() {}
	id1, state1, err := reg.RegisterWithPriority("coagent", "first", cancel1, 2)
	if err != nil {
		t.Fatalf("first RegisterWithPriority failed: %v", err)
	}
	if state1 != CoAgentRunning {
		t.Fatalf("first co-agent state = %s, want running", state1)
	}
	id2, state2, err := reg.RegisterWithPriority("coagent", "second", cancel2, 1)
	if err != nil {
		t.Fatalf("second RegisterWithPriority failed: %v", err)
	}
	id3, state3, err := reg.RegisterWithPriority("coagent", "third", cancel3, 3)
	if err != nil {
		t.Fatalf("third RegisterWithPriority failed: %v", err)
	}
	if state2 != CoAgentQueued || state3 != CoAgentQueued {
		t.Fatalf("expected queued states, got %s and %s", state2, state3)
	}

	reg.Complete(id1, "done", 10, 1)

	if err := reg.WaitForStart(id3, context.Background()); err != nil {
		t.Fatalf("queued high-priority co-agent should have started: %v", err)
	}
	reg.mu.RLock()
	info3 := reg.agents[id3]
	info2 := reg.agents[id2]
	reg.mu.RUnlock()
	info3.mu.Lock()
	got3 := info3.State
	info3.mu.Unlock()
	info2.mu.Lock()
	got2 := info2.State
	pos2 := info2.QueuePosition
	info2.mu.Unlock()
	if got3 != CoAgentRunning {
		t.Fatalf("high-priority queued co-agent state = %s, want running", got3)
	}
	if got2 != CoAgentQueued {
		t.Fatalf("remaining queued co-agent state = %s, want queued", got2)
	}
	if pos2 != 1 {
		t.Fatalf("remaining queued co-agent position = %d, want 1", pos2)
	}
}

func TestNormalizeCoAgentRequest(t *testing.T) {
	cfg := specialistTestConfig()
	cfg.CoAgents.MaxContextHints = 2
	cfg.CoAgents.MaxContextHintChars = 8

	req := normalizeCoAgentRequest(cfg, CoAgentRequest{
		Task:         "  Review logs  ",
		Specialist:   " CODER ",
		ContextHints: []string{" errors ", "errors", "long-hint-value", "  "},
		Priority:     9,
	})

	if req.Task != "Review logs" {
		t.Fatalf("task = %q, want trimmed value", req.Task)
	}
	if req.Specialist != "coder" {
		t.Fatalf("specialist = %q, want coder", req.Specialist)
	}
	if len(req.ContextHints) != 2 {
		t.Fatalf("len(context_hints) = %d, want 2", len(req.ContextHints))
	}
	if req.ContextHints[0] != "errors" {
		t.Fatalf("first hint = %q, want errors", req.ContextHints[0])
	}
	if req.ContextHints[1] != "long-hin" {
		t.Fatalf("second hint = %q, want truncated value", req.ContextHints[1])
	}
}

func TestBuildSpecialistDelegationHint(t *testing.T) {
	cfg := specialistTestConfig()
	cfg.CoAgents.Enabled = true
	cfg.CoAgents.Specialists.Researcher.Enabled = true
	cfg.CoAgents.Specialists.Writer.Enabled = true

	hint := buildSpecialistDelegationHint(cfg, "Research the topic and write a final report with sources")
	if hint == "" {
		t.Fatal("expected delegation hint for multi-domain query")
	}
	if !contains(hint, "researcher") || !contains(hint, "writer") {
		t.Fatalf("expected researcher and writer in hint, got %q", hint)
	}
}

func TestBuildSpecialistDelegationHintSkipsSimpleSingleDomainPrompt(t *testing.T) {
	cfg := specialistTestConfig()
	cfg.CoAgents.Enabled = true
	cfg.CoAgents.Specialists.Writer.Enabled = true

	hint := buildSpecialistDelegationHint(cfg, "Write a short report about the meeting")
	if hint != "" {
		t.Fatalf("expected no hint for simple single-domain request, got %q", hint)
	}
}

func TestBuildSpecialistDelegationHintSupportsExplicitSingleRoleDelegation(t *testing.T) {
	cfg := specialistTestConfig()
	cfg.CoAgents.Enabled = true
	cfg.CoAgents.Specialists.Coder.Enabled = true

	hint := buildSpecialistDelegationHint(cfg, "Please delegate this implementation task to a specialist coder")
	if hint == "" {
		t.Fatal("expected explicit delegation hint")
	}
	if !contains(hint, "coder") {
		t.Fatalf("expected coder in hint, got %q", hint)
	}
}

func TestSelectCoAgentLLMForDesignerFallsBackFromImageOnlyModel(t *testing.T) {
	cfg := specialistTestConfig()
	cfg.CoAgents.LLM.Model = "openai/gpt-4.1-mini"
	cfg.CoAgents.Specialists.Designer.Enabled = true
	cfg.CoAgents.Specialists.Designer.LLM.Model = "bytedance-seed/seedream-4.5"

	selection, reason := selectCoAgentLLMForRole(cfg, "designer")
	if selection.Model != "openai/gpt-4.1-mini" {
		t.Fatalf("selection.Model = %q, want fallback text model", selection.Model)
	}
	if selection.Source != "co_agents" {
		t.Fatalf("selection.Source = %q, want co_agents", selection.Source)
	}
	if reason == "" {
		t.Fatal("expected non-empty fallback reason")
	}
}

func TestSelectCoAgentLLMForDesignerKeepsTextModel(t *testing.T) {
	cfg := specialistTestConfig()
	cfg.CoAgents.LLM.Model = "openai/gpt-4.1-mini"
	cfg.CoAgents.Specialists.Designer.Enabled = true
	cfg.CoAgents.Specialists.Designer.LLM.Model = "openai/gpt-4.1"

	selection, reason := selectCoAgentLLMForRole(cfg, "designer")
	if selection.Model != "openai/gpt-4.1" {
		t.Fatalf("selection.Model = %q, want specialist text model", selection.Model)
	}
	if selection.Source != "specialist" {
		t.Fatalf("selection.Source = %q, want specialist", selection.Source)
	}
	if reason != "" {
		t.Fatalf("unexpected fallback reason: %q", reason)
	}
}

// ══════════════════════════════════════════════
// helpers
// ══════════════════════════════════════════════

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func specialistTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.CoAgents.MaxConcurrent = 3
	cfg.CoAgents.MaxContextHints = 6
	cfg.CoAgents.MaxContextHintChars = 180
	return cfg
}

func specialistTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}
