package server

import (
	"testing"

	"github.com/sashabaranov/go-openai"

	"aurago/internal/config"
	"aurago/internal/flows"
)

func fnTool(name string) openai.Tool {
	return openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
		Name: name, Description: name + " tool",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{"operation": map[string]any{"type": "string"}}},
	}}
}

func TestFlowToolAvailability(t *testing.T) {
	cfg := &config.Config{}
	names := map[string]bool{"filesystem": true, "execute_skill": true, "document_creator": true}
	check := func(tool, state, section string) {
		t.Helper()
		a := flowToolAvailability(cfg, names, tool)
		if a.State != state || (section != "" && a.ConfigSection != section) {
			t.Errorf("%s = %+v, want %s/%s", tool, a, state, section)
		}
	}
	check("filesystem", flows.AvailableState, "")
	check("send_telegram", flows.NeedsSetupState, "telegram")
	check(flows.BraveSearchTool, flows.NeedsSetupState, "brave_search")
	cfg.BraveSearch.Enabled, cfg.BraveSearch.APIKey = true, "key"
	check(flows.BraveSearchTool, flows.AvailableState, "")
	check(flows.PDFExtractorTool, flows.NeedsSetupState, "tools")
	cfg.Tools.PDFExtractor.Enabled = true
	check(flows.PDFExtractorTool, flows.AvailableState, "")
	check(flows.GotenbergTool, flows.NeedsSetupState, "document_creator")
	cfg.Tools.DocumentCreator.Backend = "Gotenberg"
	check(flows.GotenbergTool, flows.AvailableState, "")
	if a := flowToolAvailability(nil, names, "filesystem"); a.State != flows.NeedsSetupState {
		t.Fatalf("a nil config = %+v", a)
	}
}

func TestFlowCatalogEnvRefreshesGenericToolsPerConfig(t *testing.T) {
	cfg1, cfg2 := &config.Config{}, &config.Config{}
	current := cfg1
	calls := 0
	env := &flowCatalogEnv{
		current: func() *config.Config { return current },
		schemas: func(cfg *config.Config) []openai.Tool {
			calls++
			list := []openai.Tool{fnTool("filesystem"), fnTool("discover_tools")}
			if cfg == cfg2 {
				list = append(list, fnTool("proxmox"))
			}
			return list
		},
	}
	reg := flows.NewRegistry()
	env.refreshRegistry(reg, cfg1)
	env.refreshRegistry(reg, cfg1)
	if calls != 1 {
		t.Fatalf("schemas were built %d times for one configuration", calls)
	}
	if _, ok := reg.Lookup(flows.GenericTypePrefix + "filesystem"); !ok {
		t.Fatal("tool.filesystem is missing")
	}
	if _, ok := reg.Lookup(flows.GenericTypePrefix + "discover_tools"); ok {
		t.Fatal("excluded tools must not become nodes")
	}
	if env.ToolAvailability("proxmox").State != flows.NeedsSetupState {
		t.Fatal("proxmox is not configured in cfg1")
	}
	current = cfg2
	env.refreshRegistry(reg, cfg2)
	if _, ok := reg.Lookup(flows.GenericTypePrefix + "proxmox"); !ok {
		t.Fatal("tool.proxmox must appear after the configuration changed")
	}
	if env.ToolAvailability("proxmox").State != flows.AvailableState {
		t.Fatal("proxmox is configured in cfg2")
	}
}
