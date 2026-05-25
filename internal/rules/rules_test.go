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
	if !strings.Contains(rule.Body, "Use the `homepage` tool") {
		t.Fatalf("homepage rule body missing tool guidance:\n%s", rule.Body)
	}
	for _, marker := range []string{"Web Interface Quality Bar", "aria-label", "prefers-reduced-motion", "transition: all", "homepage_registry", "generic dark purple/blue card UI"} {
		if !strings.Contains(rule.Body, marker) {
			t.Fatalf("homepage rule body missing marker %q:\n%s", marker, rule.Body)
		}
	}

	design, ok := catalog.Design("homepage")
	if !ok {
		t.Fatal("expected embedded homepage DESIGN.md")
	}
	for _, marker := range []string{"name: Atmospheric Glass", "glass-card-standard", "## Brand & Style", "## Colors", "## Typography", "## Layout & Spacing", "## Elevation & Depth", "## Homepage Usage Guidelines", "generic opaque dark cards"} {
		if !strings.Contains(design.Content, marker) {
			t.Fatalf("homepage design missing marker %q:\n%s", marker, design.Content)
		}
	}
}

func TestLoadCatalogIncludesEmbeddedPDFRule(t *testing.T) {
	t.Parallel()

	catalog, err := LoadCatalog(LoadOptions{
		PromptsDir: t.TempDir(),
		EmbeddedFS: promptsembed.FS,
	})
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	rule, ok := catalog.Rule("pdf")
	if !ok {
		t.Fatal("expected embedded pdf rule")
	}
	if !rule.Enabled {
		t.Fatal("embedded pdf rule should be enabled")
	}
	if !contains(rule.Tools, "document_creator") {
		t.Fatalf("pdf rule tools = %v, want document_creator", rule.Tools)
	}
	for _, marker := range []string{"PDF Creation Workflow", "Gotenberg", "Maroto", "visual", "No final PDF may contain placeholders", "If verification is impossible"} {
		if !strings.Contains(rule.Body, marker) {
			t.Fatalf("pdf rule body missing marker %q:\n%s", marker, rule.Body)
		}
	}
}

func TestLoadCatalogIncludesEmbeddedVirtualDesktopRule(t *testing.T) {
	t.Parallel()

	catalog, err := LoadCatalog(LoadOptions{
		PromptsDir: t.TempDir(),
		EmbeddedFS: promptsembed.FS,
	})
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	rule, ok := catalog.Rule("virtual_desktop")
	if !ok {
		t.Fatal("expected embedded virtual_desktop rule")
	}
	if !rule.Enabled {
		t.Fatal("embedded virtual_desktop rule should be enabled")
	}
	if !contains(rule.Tools, "virtual_desktop") {
		t.Fatalf("virtual_desktop rule tools = %v, want virtual_desktop", rule.Tools)
	}
	for _, marker := range []string{
		"Generated App And Widget Creation Workflow",
		"Call `status` before creating",
		"Use `install_app` for generated apps",
		"diagnose_app",
		"diagnose_widget",
		"Do not store secrets",
	} {
		if !strings.Contains(rule.Body, marker) {
			t.Fatalf("virtual_desktop rule body missing marker %q:\n%s", marker, rule.Body)
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

func TestCatalogMatchSelectsPDFRuleByToolAndKeyword(t *testing.T) {
	t.Parallel()

	catalog, err := LoadCatalog(LoadOptions{
		PromptsDir: t.TempDir(),
		EmbeddedFS: promptsembed.FS,
	})
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	byTool := catalog.Match(MatchContext{Tools: []string{"document_creator"}})
	if len(byTool.Rules) == 0 || byTool.Rules[0].ID != "pdf" {
		t.Fatalf("document_creator should select pdf rule first, got %+v", byTool.Rules)
	}

	byKeyword := catalog.Match(MatchContext{Prompt: "Bitte ein PDF erstellen"})
	if len(byKeyword.Rules) == 0 || byKeyword.Rules[0].ID != "pdf" {
		t.Fatalf("PDF keyword should select pdf rule first, got %+v", byKeyword.Rules)
	}
}

func TestCatalogMatchSelectsVirtualDesktopRuleByToolAndKeyword(t *testing.T) {
	t.Parallel()

	catalog, err := LoadCatalog(LoadOptions{
		PromptsDir: t.TempDir(),
		EmbeddedFS: promptsembed.FS,
	})
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	byTool := catalog.Match(MatchContext{Tools: []string{"virtual_desktop"}})
	if len(byTool.Rules) == 0 || byTool.Rules[0].ID != "virtual_desktop" {
		t.Fatalf("virtual_desktop tool should select virtual_desktop rule first, got %+v", byTool.Rules)
	}

	byKeyword := catalog.Match(MatchContext{Prompt: "Bitte im virtuellen Desktop eine App mit Widget erstellen"})
	if len(byKeyword.Rules) == 0 || byKeyword.Rules[0].ID != "virtual_desktop" {
		t.Fatalf("German virtual desktop keyword should select virtual_desktop rule first, got %+v", byKeyword.Rules)
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
