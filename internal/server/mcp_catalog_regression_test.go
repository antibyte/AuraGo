package server

import (
	"aurago/internal/config"
	"testing"
)

func TestMCPCatalogSharesIntegrationFlagsAndExplicitGrants(t *testing.T) {
	s := auditMCPServer(t)
	s.Cfg.S3.Enabled = true
	s.Cfg.WebDAV.Enabled = true
	s.Cfg.MeshCore.Enabled = true
	s.Cfg.AgentMail.Enabled = true
	flags := mcpFeatureFlags(s)
	if !flags.S3Enabled || !flags.WebDAVEnabled || !flags.MeshCoreEnabled || !flags.AgentMailEnabled {
		t.Fatalf("canonical integration flags lost")
	}
	if flags.CoAgentEnabled {
		t.Fatal("MCP exposed co-agents")
	}
	if !mcpToolAvailable(s, "s3_storage") {
		t.Fatal("enabled S3 missing from MCP")
	}
	if len(mcpBuildToolList(s)) != 0 {
		t.Fatal("catalog bypassed explicit grants")
	}
	s.Cfg.MCPServer.AllowedTools = []string{"s3_storage"}
	list := mcpBuildToolList(s)
	if len(list) != 1 || list[0].Name != "s3_storage" {
		t.Fatalf("explicit S3 export: %#v", list)
	}
	s.Cfg.S3.Enabled = false
	if len(mcpBuildToolList(s)) != 0 {
		t.Fatal("disabled S3 exported")
	}
	s.Cfg.Tools.Contacts.Enabled = true
	if mcpFeatureFlags(s).ContactsEnabled {
		t.Fatal("missing runtime handle accepted")
	}
}

func TestMCPCatalogUsesPublishedSnapshot(t *testing.T) {
	s := auditMCPServer(t)
	current := *s.Cfg
	current.S3.Enabled = true
	s.cfgSnapshot.Store(&current)
	if !mcpToolAvailable(s, "s3_storage") {
		t.Fatal("stale Cfg pointer used instead of snapshot")
	}
	s.cfgSnapshot.Store(&config.Config{})
	if mcpToolAvailable(s, "s3_storage") {
		t.Fatal("revoked integration remained visible")
	}
}
