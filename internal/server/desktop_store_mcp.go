package server

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"aurago/internal/desktopstore"
	"aurago/internal/security"
	"aurago/internal/tools"
)

var storeMCPProbeTools = tools.ProbeMCPServerTools

func storeMCPName(id string) string {
	if id == "n8n" || id == "bytestash" {
		return "aurago-store-" + id
	}
	return ""
}

func (s *Server) storeMCPConfig(id string, settings desktopstore.IntegrationSettings) (*tools.MCPServerConfig, error) {
	name := storeMCPName(id)
	if name == "" || settings.Credential == "" {
		return nil, errors.New("Enter this app's MCP access token")
	}
	cfg := s.ConfigSnapshot()
	for _, manual := range cfg.MCP.Servers {
		if strings.EqualFold(manual.Name, name) {
			return nil, errors.New("A manual MCP connection uses this Store connection name; rename it before connecting the Store instance")
		}
	}
	path := "/mcp"
	allowed := []string{"list_snippets", "get_snippet", "list_metadata"}
	if id == "n8n" {
		path = "/mcp-server/http"
		allowed = []string{"search_workflows", "get_workflow_details"}
		if settings.Execute && !cfg.VirtualDesktop.ReadOnly {
			allowed = append(allowed, "execute_workflow")
		}
	} else if settings.Create && !cfg.VirtualDesktop.ReadOnly {
		allowed = append(allowed, "create_snippet")
	}
	security.RegisterSensitive(settings.Credential)
	permissions := tools.RuntimePermissionsFromConfig(cfg)
	return &tools.MCPServerConfig{Name: name, Transport: "streamable_http", URL: strings.TrimRight(settings.URL, "/") + path, Headers: map[string]string{"Authorization": "Bearer " + settings.Credential}, Enabled: true, AllowPrivateNetwork: true, AllowedTools: allowed, ManagedRevision: settings.Revision, AllowDestructive: (settings.Execute || settings.Create) && !cfg.VirtualDesktop.ReadOnly, ExecutionPermissions: &permissions, MCPEnabled: true}, nil
}

func (s *Server) probeStoreMCP(ctx context.Context, id string, settings desktopstore.IntegrationSettings) error {
	runtime, err := s.storeMCPConfig(id, settings)
	if err != nil {
		return err
	}
	names, err := storeMCPProbeTools(ctx, *runtime, s.Logger)
	if err != nil {
		return errors.New("MCP connection failed; check the saved token and enable the official MCP endpoint in the app")
	}
	for _, name := range runtime.AllowedTools {
		if !slices.Contains(names, name) {
			return errors.New("Store update required: the official MCP endpoint lacks a requested capability")
		}
	}
	return nil
}

func (s *Server) syncStoreMCP(id string) error {
	name := storeMCPName(id)
	if name == "" {
		return nil
	}
	cfg := s.ConfigSnapshot()
	if !cfg.Agent.AllowMCP || !cfg.MCP.Enabled || !s.storeApplicationEnabled(id) {
		return tools.SetManagedMCPServer(name, nil)
	}
	s.storeIntegrations.Lock()
	binding, ok := s.storeIntegrations.bindings[id]
	s.storeIntegrations.Unlock()
	if !ok {
		return tools.SetManagedMCPServer(name, nil)
	}
	runtime, err := s.storeMCPConfig(id, binding.settings)
	if err != nil {
		return err
	}
	runtime.CheckAccess = func() error {
		cfg := s.ConfigSnapshot()
		if !cfg.Agent.AllowMCP || !cfg.MCP.Enabled {
			return errors.New("MCP is disabled")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, release, err := s.acquireStoreAccess(ctx, id, binding.settings.URL, false)
		defer release()
		if binding.ctx.Err() != nil {
			return errors.New("Store connection was revoked")
		}
		return err
	}
	return tools.SetManagedMCPServer(name, runtime)
}

func revokeStoreMCP(id string) {
	if name := storeMCPName(id); name != "" {
		_ = tools.SetManagedMCPServer(name, nil)
	}
}
