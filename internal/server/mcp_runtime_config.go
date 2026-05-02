package server

import (
	"aurago/internal/config"
	"aurago/internal/tools"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
)

var mcpAliasRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

type mcpSecretStatus struct {
	Alias       string `json:"alias"`
	Label       string `json:"label"`
	Description string `json:"description"`
	HasValue    bool   `json:"has_value"`
}

func normalizeMCPSecretAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

func mcpSecretVaultKey(alias string) string {
	return "mcp_secret_" + normalizeMCPSecretAlias(alias)
}

func resolveMCPHostWorkdir(cfg *config.Config, srv config.MCPServer, logger *slog.Logger) string {
	workspaceRoot := strings.TrimSpace(cfg.Directories.WorkspaceDir)
	baseDir := filepath.Join(workspaceRoot, "mcp")
	safeName := strings.ToLower(strings.TrimSpace(srv.Name))
	safeName = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, safeName)
	safeName = strings.Trim(safeName, "-")
	if safeName == "" {
		safeName = "server"
	}

	hostWorkdir := strings.TrimSpace(srv.HostWorkdir)
	if hostWorkdir == "" {
		return filepath.Join(baseDir, safeName)
	}
	if !filepath.IsAbs(hostWorkdir) {
		hostWorkdir = filepath.Join(workspaceRoot, hostWorkdir)
	}
	absHost, err := filepath.Abs(hostWorkdir)
	if err != nil {
		return filepath.Join(baseDir, safeName)
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return filepath.Join(baseDir, safeName)
	}
	rel, err := filepath.Rel(absBase, absHost)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		if logger != nil {
			logger.Warn("[MCP] Host workdir outside allowed MCP workspace root; using safe default", "server", srv.Name, "requested", absHost, "allowed_root", absBase)
		}
		return filepath.Join(absBase, safeName)
	}
	return absHost
}

func buildRuntimeMCPConfigs(cfg *config.Config, vault config.SecretReader, logger *slog.Logger) []tools.MCPServerConfig {
	if cfg == nil {
		return nil
	}

	secretValues := make(map[string]string, len(cfg.MCP.Secrets))
	for _, secret := range cfg.MCP.Secrets {
		alias := normalizeMCPSecretAlias(secret.Alias)
		if alias == "" || !mcpAliasRe.MatchString(alias) {
			continue
		}
		if vault == nil {
			continue
		}
		if value, err := vault.ReadSecret(mcpSecretVaultKey(alias)); err == nil && strings.TrimSpace(value) != "" {
			secretValues[alias] = strings.TrimSpace(value)
		}
	}

	runtimeConfigs := make([]tools.MCPServerConfig, 0, len(cfg.MCP.Servers))
	for _, srv := range cfg.MCP.Servers {
		runtimeConfigs = append(runtimeConfigs, tools.MCPServerConfig{
			Name:               srv.Name,
			Transport:          strings.ToLower(strings.TrimSpace(srv.Transport)),
			URL:                strings.TrimSpace(srv.URL),
			Headers:            mapsCloneStringString(srv.Headers),
			Command:            srv.Command,
			Args:               append([]string(nil), srv.Args...),
			Env:                mapsCloneStringString(srv.Env),
			Enabled:            srv.Enabled,
			Runtime:            strings.ToLower(strings.TrimSpace(srv.Runtime)),
			DockerImage:        strings.TrimSpace(srv.DockerImage),
			DockerCommand:      strings.TrimSpace(srv.DockerCommand),
			AllowLocalFallback: srv.AllowLocalFallback,
			HostWorkdir:        resolveMCPHostWorkdir(cfg, srv, logger),
			ContainerWorkdir:   strings.TrimSpace(srv.ContainerWorkdir),
			AllowedTools:       append([]string(nil), srv.AllowedTools...),
			AllowDestructive:   srv.AllowDestructive,
			Secrets:            mapsCloneStringString(secretValues),
		})
	}
	return runtimeConfigs
}

func buildMCPSecretStatuses(cfg *config.Config, vault config.SecretReader) []mcpSecretStatus {
	if cfg == nil {
		return nil
	}
	statuses := make([]mcpSecretStatus, 0, len(cfg.MCP.Secrets))
	for _, secret := range cfg.MCP.Secrets {
		alias := normalizeMCPSecretAlias(secret.Alias)
		if alias == "" || !mcpAliasRe.MatchString(alias) {
			continue
		}
		hasValue := false
		if vault != nil {
			if value, err := vault.ReadSecret(mcpSecretVaultKey(alias)); err == nil && strings.TrimSpace(value) != "" {
				hasValue = true
			}
		}
		statuses = append(statuses, mcpSecretStatus{
			Alias:       alias,
			Label:       secret.Label,
			Description: secret.Description,
			HasValue:    hasValue,
		})
	}
	return statuses
}

func mapsCloneStringString(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
