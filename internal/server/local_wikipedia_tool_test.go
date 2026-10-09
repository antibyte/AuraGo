package server

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"aurago/internal/localwiki"
	"aurago/internal/tools"
)

// openWikiSource reports an open edition without serving requests; that is
// enough for availability checks.
type openWikiSource struct{}

func (openWikiSource) AcquireLibrary() (tools.LocalWikipediaLibrary, func(), bool) {
	return nil, func() {}, true
}

func TestPublishLocalWikipediaToolNilWithdrawsSource(t *testing.T) {
	t.Cleanup(func() { tools.SetLocalWikipediaSource(nil) })
	tools.SetLocalWikipediaSource(openWikiSource{})
	publishLocalWikipediaTool(nil)
	if tools.LocalWikipediaAvailable() {
		t.Fatal("publishing nil must withdraw the edition source")
	}
}

func TestWithdrawLocalWikipediaToolKeepsAnotherSource(t *testing.T) {
	t.Cleanup(func() { tools.SetLocalWikipediaSource(nil) })
	tools.SetLocalWikipediaSource(openWikiSource{})
	withdrawLocalWikipediaTool(nil)
	withdrawLocalWikipediaTool(localwiki.NewManager(localwiki.Deps{}))
	if !tools.LocalWikipediaAvailable() {
		t.Fatal("shutdown withdrew an edition source this server never published")
	}
}

func TestMCPFeatureFlagsIncludeLocalWikipediaOnlyWithOpenEdition(t *testing.T) {
	t.Cleanup(func() { tools.SetLocalWikipediaSource(nil) })
	cfg := &config.Config{}
	cfg.LocalWikipedia.Enabled = true
	cfg.LocalWikipedia.AgentAccess = true
	tools.SetLocalWikipediaSource(nil)
	if mcpFeatureFlags(&Server{Cfg: cfg}).LocalWikipediaEnabled {
		t.Fatal("local_wikipedia exposed without an open edition")
	}
	tools.SetLocalWikipediaSource(openWikiSource{})
	if !mcpFeatureFlags(&Server{Cfg: cfg}).LocalWikipediaEnabled {
		t.Fatal("local_wikipedia hidden although an edition is open")
	}
	cfg.LocalWikipedia.AgentAccess = false
	if mcpFeatureFlags(&Server{Cfg: cfg}).LocalWikipediaEnabled {
		t.Fatal("local_wikipedia exposed without agent access")
	}
}

func TestPythonToolBridgeOffersLocalWikipediaGroup(t *testing.T) {
	groups := pythonToolBridgeBuildCatalogGroups(map[string]bool{"local_wikipedia": true})
	if len(groups) != 1 || groups[0].Key != "local_wikipedia" || len(groups[0].Tools) != 1 || groups[0].Tools[0] != "local_wikipedia" {
		t.Fatalf("groups = %#v", groups)
	}
}

// A telephone config that allows local_wikipedia keeps working while no
// edition is open (first load, data-dir change, deletion): the tool answers
// needs_setup instead of every call failing the tool-scope check.
func TestSIPToolScopeAcceptsLocalWikipediaWithoutAnOpenEdition(t *testing.T) {
	t.Cleanup(func() { tools.SetLocalWikipediaSource(nil) })
	tools.SetLocalWikipediaSource(nil)
	cfg := &config.Config{}
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.LocalWikipedia.Enabled = true
	cfg.LocalWikipedia.AgentAccess = true
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if mcpFeatureFlags(s).LocalWikipediaEnabled {
		t.Fatal("MCP must still hide local_wikipedia without an open edition")
	}
	if err := validateSIPAgentToolScope(s, cfg, []string{"local_wikipedia"}); err != nil {
		t.Fatalf("configured local_wikipedia rejected while no edition is open: %v", err)
	}
	found := false
	for _, option := range sipAgentToolCatalog(s, cfg) {
		found = found || option.ID == "local_wikipedia"
	}
	if !found {
		t.Fatal("telephone tool catalog hides an enabled local_wikipedia")
	}
	cfg.LocalWikipedia.AgentAccess = false
	if err := validateSIPAgentToolScope(s, cfg, []string{"local_wikipedia"}); err == nil {
		t.Fatal("local_wikipedia accepted without agent access")
	}
}
