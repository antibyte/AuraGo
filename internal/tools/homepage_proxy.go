package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const homepageNetworkName = "aurago-homepage-net"

// ProxyRoute describes a single reverse_proxy route that the Caddy web container
// should forward to the dev container.  Routes are persisted as a JSON file in
// the workspace so they survive container restarts.
type ProxyRoute struct {
	Path        string `json:"path"`
	Port        int    `json:"port"`
	StripPrefix bool   `json:"strip_prefix,omitempty"`
}

var (
	proxyRoutesMu sync.Mutex
)

// proxyRoutesPath returns the path to the persistent proxy-routes JSON file.
func proxyRoutesPath(workspacePath string) string {
	return filepath.Join(workspacePath, ".aurago-proxy-routes.json")
}

// loadProxyRoutes reads the persisted proxy routes from disk.
// Returns an empty slice (never nil) when the file does not exist.
func loadProxyRoutes(workspacePath string) []ProxyRoute {
	proxyRoutesMu.Lock()
	defer proxyRoutesMu.Unlock()

	path := proxyRoutesPath(workspacePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var routes []ProxyRoute
	if err := json.Unmarshal(data, &routes); err != nil {
		return nil
	}
	return routes
}

// saveProxyRoutes persists the given routes to disk.
func saveProxyRoutes(workspacePath string, routes []ProxyRoute) error {
	proxyRoutesMu.Lock()
	defer proxyRoutesMu.Unlock()

	data, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal proxy routes: %w", err)
	}
	return os.WriteFile(proxyRoutesPath(workspacePath), data, 0644)
}

// RegisterProxyRoute adds or updates a proxy route and persists it.
// If a route with the same path already exists it is replaced.
func RegisterProxyRoute(workspacePath string, route ProxyRoute) error {
	routes := loadProxyRoutes(workspacePath)
	replaced := false
	for i, r := range routes {
		if r.Path == route.Path {
			routes[i] = route
			replaced = true
			break
		}
	}
	if !replaced {
		routes = append(routes, route)
	}
	return saveProxyRoutes(workspacePath, routes)
}

// RemoveProxyRoute removes a proxy route by path prefix.
func RemoveProxyRoute(workspacePath, path string) error {
	routes := loadProxyRoutes(workspacePath)
	var filtered []ProxyRoute
	for _, r := range routes {
		if r.Path != path {
			filtered = append(filtered, r)
		}
	}
	return saveProxyRoutes(workspacePath, filtered)
}

// ensureHomepageNetwork creates the shared Docker network if it does not exist.
// It is idempotent — calling it when the network already exists is a no-op.
func ensureHomepageNetwork(dockerCfg DockerConfig, logger *slog.Logger) {
	data, code, err := dockerRequest(dockerCfg, "GET", "/networks/"+url.PathEscape(homepageNetworkName), "")
	if err == nil && code == 200 {
		_ = data
		return
	}
	result := DockerCreateNetwork(dockerCfg, homepageNetworkName, "bridge")
	logger.Info("[Homepage] Created shared Docker network", "network", homepageNetworkName, "result", truncateStr(result, 200))
}

// connectContainerToNetwork connects a container to the shared homepage network.
// Errors are logged but not fatal — the network may already be connected.
func connectContainerToNetwork(dockerCfg DockerConfig, containerName string, logger *slog.Logger) {
	result := DockerConnectNetwork(dockerCfg, containerName, homepageNetworkName)
	var r map[string]interface{}
	if json.Unmarshal([]byte(result), &r) == nil {
		if s, _ := r["status"].(string); s == "ok" {
			logger.Info("[Homepage] Connected container to shared network", "container", containerName, "network", homepageNetworkName)
		}
	}
}

// buildCaddyfileWithProxies builds a Caddyfile that serves static files from
// /srv AND reverse_proxies registered routes to the dev container.
//
// Static file serving is the default (catch-all).  Proxy routes are matched
// first via `handle` blocks.  The dev container is reachable inside the shared
// Docker network as `aurago-homepage`.
func buildCaddyfileWithProxies(domain string, port int, routes []ProxyRoute) string {
	devHost := homepageContainerName

	var sb strings.Builder

	if domain != "" {
		sb.WriteString(domain)
	} else {
		sb.WriteString(fmt.Sprintf(":%d", port))
	}
	sb.WriteString(" {\n")
	sb.WriteString("\troot * /srv\n")
	sb.WriteString("\tencode gzip\n\n")

	for _, r := range routes {
		proxyPath := r.Path
		if !strings.HasSuffix(proxyPath, "*") {
			proxyPath += "*"
		}
		upstream := fmt.Sprintf("%s:%d", devHost, r.Port)
		if r.StripPrefix {
			sb.WriteString(fmt.Sprintf("\thandle_path %s {\n", proxyPath))
		} else {
			sb.WriteString(fmt.Sprintf("\thandle %s {\n", proxyPath))
		}
		sb.WriteString(fmt.Sprintf("\t\treverse_proxy %s\n", upstream))
		sb.WriteString("\t}\n\n")
	}

	sb.WriteString("\thandle {\n")
	sb.WriteString("\t\tfile_server\n")
	sb.WriteString("\t\ttry_files {path} /index.html\n")
	sb.WriteString("\t}\n")

	sb.WriteString("}\n")

	return sb.String()
}

// reloadCaddy sends a SIGHUP to the Caddy process inside the web container so
// it re-reads the Caddyfile.  This is a lightweight alternative to recreating
// the container when only the config changed.
func reloadCaddy(dockerCfg DockerConfig, logger *slog.Logger) {
	cmd := "caddy reload --config /etc/caddy/Caddyfile 2>&1 || true"
	result := DockerExec(dockerCfg, homepageWebContainer, cmd, "")
	logger.Info("[Homepage] Reloaded Caddy config", "output", truncateStr(extractOutput(result), 300))
}

// rewriteCaddyfileWithProxyRoutes rewrites the Caddyfile on disk (inside the
// container mount) and reloads Caddy.  Used when a dev server starts/stops
// and the proxy routes change while the web container is already running.
func rewriteCaddyfileWithProxyRoutes(cfg HomepageConfig, dockerCfg DockerConfig, routes []ProxyRoute, logger *slog.Logger) error {
	caddyfile := buildCaddyfileWithProxies(cfg.WebServerDomain, cfg.WebServerPort, routes)
	caddyfilePath := filepath.Join(cfg.WorkspacePath, ".aurago-Caddyfile")
	if err := os.WriteFile(caddyfilePath, []byte(caddyfile), 0644); err != nil {
		return fmt.Errorf("failed to write Caddyfile: %w", err)
	}

	ensureHomepageNetwork(dockerCfg, logger)
	connectContainerToNetwork(dockerCfg, homepageWebContainer, logger)

	reloadCaddy(dockerCfg, logger)
	return nil
}
