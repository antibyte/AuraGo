package server

import (
	"testing"

	"aurago/internal/config"
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
