package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	promptsembed "aurago/prompts"
)

func TestLoadCatalogIncludesEmbeddedHomepageRuleAndDesign(t *testing.T) {
	t.Parallel()

	catalog, err := LoadCatalog(LoadOptions{
		PromptsDir: t.TempDir(),
		EmbeddedFS: promptsembed.FS,
	})
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	rule, ok := catalog.Rule("homepage")
	if !ok {
		t.Fatal("expected embedded homepage rule")
	}
	if !rule.Enabled {
		t.Fatal("embedded homepage rule should be enabled")
	}
	if !contains(rule.Tools, "homepage") {
		t.Fatalf("homepage rule tools = %v, want homepage", rule.Tools)
	}
	if !contains(rule.Workflows, "homepage") {
		t.Fatalf("homepage rule workflows = %v, want homepage", rule.Workflows)
	}
	if !strings.Contains(rule.Body, "Use the homepage tool") {
		t.Fatalf("homepage rule body missing tool guidance:\n%s", rule.Body)
	}

	design, ok := catalog.Design("homepage")
	if !ok {
		t.Fatal("expected embedded homepage DESIGN.md")
	}
	for _, marker := range []string{"name: Atmospheric Glass", "glass-card-standard", "## Brand & Style", "## Colors", "## Typography", "## Layout & Spacing", "## Elevation & Depth", "## Homepage Usage Guidelines"} {
		if !strings.Contains(design.Content, marker) {
			t.Fatalf("homepage design missing marker %q:\n%s", marker, design.Content)
		}
	}
}

func TestLoadCatalogUsesDiskOverrideBeforeEmbeddedRule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ruleDir := filepath.Join(dir, "rules", "homepage")
	if err := os.MkdirAll(ruleDir, 0755); err != nil {
		t.Fatal(err)
	}
	override := `---
id: homepage
title: Custom Homepage
enabled: true
priority: 99
tools: [homepage]
workflows: [homepage]
keywords: [custom-homepage]
---

Use the local house style.`
	if err := os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte(override), 0644); err != nil {
		t.Fatal(err)
	}

	catalog, err := LoadCatalog(LoadOptions{PromptsDir: dir, EmbeddedFS: promptsembed.FS})
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	rule, ok := catalog.Rule("homepage")
	if !ok {
		t.Fatal("expected homepage rule")
	}
	if rule.Title != "Custom Homepage" || rule.Priority != 99 {
		t.Fatalf("disk override not used: %+v", rule)
	}
	if !strings.Contains(rule.Body, "local house style") {
		t.Fatalf("disk override body not used: %q", rule.Body)
	}
}

func TestCatalogMatchSelectsRulesByToolWorkflowAndKeyword(t *testing.T) {
	t.Parallel()

	catalog := Catalog{Rules: []Rule{
		{ID: "low", Title: "Low", Enabled: true, Priority: 1, Tools: []string{"filesystem"}, Body: "low"},
		{ID: "homepage", Title: "Homepage", Enabled: true, Priority: 50, Tools: []string{"homepage"}, Workflows: []string{"homepage"}, Keywords: []string{"landing page"}, Body: "homepage"},
		{ID: "disabled", Title: "Disabled", Enabled: false, Priority: 100, Tools: []string{"homepage"}, Body: "disabled"},
	}}

	selected := catalog.Match(MatchContext{
		Prompt:    "Please build a landing page",
		Tools:     []string{"homepage"},
		Workflows: []string{"homepage"},
	})
	if len(selected.Rules) != 1 {
		t.Fatalf("selected %d rules, want 1: %+v", len(selected.Rules), selected.Rules)
	}
	if selected.Rules[0].ID != "homepage" {
		t.Fatalf("selected rule = %q, want homepage", selected.Rules[0].ID)
	}
}

func TestValidateRuleIDRejectsTraversalAndUnsafeNames(t *testing.T) {
	t.Parallel()

	valid := []string{"homepage", "remote-control", "web_hooks_1"}
	for _, id := range valid {
		if err := ValidateRuleID(id); err != nil {
			t.Fatalf("ValidateRuleID(%q) unexpected error: %v", id, err)
		}
	}
	invalid := []string{"", "../homepage", "home/page", "home.page", strings.Repeat("a", 65)}
	for _, id := range invalid {
		if err := ValidateRuleID(id); err == nil {
			t.Fatalf("ValidateRuleID(%q) expected error", id)
		}
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
