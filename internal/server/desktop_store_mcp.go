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

// storeMCPApps are the Store apps that connect to the agent through a managed MCP server.
var storeMCPApps = []string{"n8n", "bytestash", desktopstore.VelxioAppID}

var velxioDesignTools = []string{"create_circuit", "update_circuit", "import_wokwi_json", "export_wokwi_json", "generate_code_files"}

func storeMCPName(id string) string {
	if slices.Contains(storeMCPApps, id) {
		return "aurago-store-" + id
	}
	return ""
}

func (s *Server) syncStoreMCPs() {
	for _, id := range storeMCPApps {
		_ = s.syncStoreMCP(id)
	}
}

// velxioMCPToken reads the generated MCP bearer token from the Vault.
func (s *Server) velxioMCPToken() (string, error) {
	missing := errors.New("Velxio's agent token is missing in the AuraGo vault; run a Store Update or reinstall Velxio")
	if s.Vault == nil {
		return "", missing
	}
	token, err := s.Vault.ReadSecret(desktopstore.VelxioMCPTokenKey())
	token = strings.TrimSpace(token)
	if err != nil || token == "" {
		return "", missing
	}
	security.RegisterSensitive(token)
	return token, nil
}

func (s *Server) storeMCPConfig(id string, settings desktopstore.IntegrationSettings) (*tools.MCPServerConfig, error) {
	name := storeMCPName(id)
	credential := settings.Credential
	if id == desktopstore.VelxioAppID {
		var err error
		if credential, err = s.velxioMCPToken(); err != nil {
			return nil, err
		}
	}
	if name == "" || credential == "" {
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
	} else if id == desktopstore.VelxioAppID {
		allowed = slices.Clone(velxioDesignTools)
		if settings.Execute && !cfg.VirtualDesktop.ReadOnly {
			allowed = append(allowed, "compile_project", "run_project")
		}
	} else if settings.Create && !cfg.VirtualDesktop.ReadOnly {
		allowed = append(allowed, "create_snippet")
	}
	security.RegisterSensitive(credential)
	permissions := tools.RuntimePermissionsFromConfig(cfg)
	return &tools.MCPServerConfig{Name: name, Transport: "streamable_http", URL: strings.TrimRight(settings.URL, "/") + path, Headers: map[string]string{"Authorization": "Bearer " + credential}, Enabled: true, AllowPrivateNetwork: true, AllowedTools: allowed, ManagedRevision: settings.Revision, AllowDestructive: (settings.Execute || settings.Create) && !cfg.VirtualDesktop.ReadOnly, ExecutionPermissions: &permissions, MCPEnabled: true}, nil
}

func (s *Server) probeStoreMCP(ctx context.Context, id string, settings desktopstore.IntegrationSettings) error {
	runtime, err := s.storeMCPConfig(id, settings)
	if err != nil {
		return err
	}
	names, err := storeMCPProbeTools(ctx, *runtime, s.Logger)
	if err != nil {
		if id == desktopstore.VelxioAppID {
			return errors.New("Velxio's MCP server is not reachable yet. After a start it can take a few minutes (the first start installs Arduino cores); AuraGo keeps retrying.")
		}
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
		// Fail closed: a stale live server must not outlive a config that cannot be built.
		_ = tools.SetManagedMCPServer(name, nil)
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
