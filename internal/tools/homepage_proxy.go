package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"aurago/internal/fileutil"
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
// It returns no routes and no error when the file does not exist, and an error
// when the file cannot be read or parsed. A damaged route file is never
// reported as an empty route set.
func loadProxyRoutes(workspacePath string) ([]ProxyRoute, error) {
	data, err := os.ReadFile(proxyRoutesPath(workspacePath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read proxy routes: %w", err)
	}
	var routes []ProxyRoute
	if err := json.Unmarshal(data, &routes); err != nil {
		return nil, fmt.Errorf("failed to parse proxy routes %s: %w", proxyRoutesPath(workspacePath), err)
	}
	return routes, nil
}

// saveProxyRoutes persists the given routes to disk. The file is replaced
// atomically: a synced temporary file in the same directory is renamed over
// the target, so a crashed write never leaves a half-written route file.
//
// The caller owns the routes lock.
func saveProxyRoutes(workspacePath string, routes []ProxyRoute) error {
	data, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal proxy routes: %w", err)
	}
	if err := fileutil.WriteFileContext(context.Background(), proxyRoutesPath(workspacePath), data, 0644); err != nil {
		return fmt.Errorf("failed to write proxy routes: %w", err)
	}
	return nil
}

// homepageReadProxyRoutes returns the persisted proxy routes under the routes
// lock.
func homepageReadProxyRoutes(workspacePath string) ([]ProxyRoute, error) {
	proxyRoutesMu.Lock()
	defer proxyRoutesMu.Unlock()
	return loadProxyRoutes(workspacePath)
}

// homepageMutateProxyRoutes applies mutate to the persisted routes and
// persists the result. The whole load → modify → save sequence runs under one
// lock, so concurrent callers can never lose an update.
func homepageMutateProxyRoutes(workspacePath string, mutate func([]ProxyRoute) []ProxyRoute) ([]ProxyRoute, error) {
	proxyRoutesMu.Lock()
	defer proxyRoutesMu.Unlock()
	routes, err := loadProxyRoutes(workspacePath)
	if err != nil {
		return nil, err
	}
	updated := mutate(routes)
	if err := saveProxyRoutes(workspacePath, updated); err != nil {
		return nil, err
	}
	return updated, nil
}

// RegisterProxyRoute adds or updates a proxy route and persists it.
// If a route with the same path already exists it is replaced.
func RegisterProxyRoute(workspacePath string, route ProxyRoute) error {
	_, err := homepageMutateProxyRoutes(workspacePath, func(routes []ProxyRoute) []ProxyRoute {
		for i, r := range routes {
			if r.Path == route.Path {
				routes[i] = route
				return routes
			}
		}
		return append(routes, route)
	})
	return err
}

// RemoveProxyRoute removes a proxy route by path prefix.
func RemoveProxyRoute(workspacePath, path string) error {
	_, err := homepageMutateProxyRoutes(workspacePath, func(routes []ProxyRoute) []ProxyRoute {
		filtered := make([]ProxyRoute, 0, len(routes))
		for _, r := range routes {
			if r.Path != path {
				filtered = append(filtered, r)
			}
		}
		return filtered
	})
	return err
}

func filterProxyRoutesForStaticProject(routes []ProxyRoute, projectDir string) []ProxyRoute {
	conflicts := staticProjectProxyRouteConflicts(projectDir)
	if len(routes) == 0 || len(conflicts) == 0 {
		return routes
	}

	filtered := make([]ProxyRoute, 0, len(routes))
	for _, route := range routes {
		if _, blocked := conflicts[normalizeProxyRoutePath(route.Path)]; blocked {
			continue
		}
		filtered = append(filtered, route)
	}
	if len(filtered) == len(routes) {
		return routes
	}
	return filtered
}

func staticProjectProxyRouteConflicts(projectDir string) map[string]struct{} {
	clean := strings.TrimSpace(filepath.ToSlash(projectDir))
	clean = strings.Trim(clean, "/")
	if clean == "" || clean == "." {
		return nil
	}

	conflicts := map[string]struct{}{
		"/" + clean: {},
	}
	if strings.HasSuffix(clean, "-static") {
		withoutSuffix := strings.TrimSuffix(clean, "-static")
		if withoutSuffix != "" {
			conflicts["/"+withoutSuffix] = struct{}{}
		}
	}
	return conflicts
}

func normalizeProxyRoutePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimSuffix(path, "*")
	path = strings.Trim(path, "/")
	if path == "" || path == "." {
		return ""
	}
	return "/" + path
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
	if logger != nil {
		logger.Info("[Homepage] Created shared Docker network", "network", homepageNetworkName, "result", truncateStr(result, 200))
	}
}

// connectContainerToNetwork connects a container to the shared homepage network.
// Errors are logged but not fatal — the network may already be connected.
func connectContainerToNetwork(dockerCfg DockerConfig, containerName string, logger *slog.Logger) {
	result := DockerConnectNetwork(dockerCfg, containerName, homepageNetworkName)
	var r map[string]interface{}
	if json.Unmarshal([]byte(result), &r) == nil {
		if s, _ := r["status"].(string); s == "ok" {
			if logger != nil {
				logger.Info("[Homepage] Connected container to shared network", "container", containerName, "network", homepageNetworkName)
			}
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
	sb.WriteString("\tencode gzip\n")
	sb.WriteString("\theader {\n")
	sb.WriteString("\t\tAccess-Control-Allow-Origin \"*\"\n")
	sb.WriteString("\t\tAccess-Control-Allow-Methods \"GET, HEAD, OPTIONS\"\n")
	sb.WriteString("\t\tAccess-Control-Allow-Headers \"*\"\n")
	sb.WriteString("\t}\n")
	sb.WriteString("\t@cors_preflight method OPTIONS\n")
	sb.WriteString("\trespond @cors_preflight 204\n\n")

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
func reloadCaddy(dockerCfg DockerConfig, logger *slog.Logger) (string, error) {
	cmd := "caddy reload --config /etc/caddy/Caddyfile 2>&1"
	result := homepageDockerExecFunc(dockerCfg, homepageWebContainer, cmd, "")
	output := strings.TrimSpace(extractOutput(result))
	if homepageBuildSucceeded(result) {
		if logger != nil {
			logger.Info("[Homepage] Reloaded Caddy config", "output", truncateStr(output, 300))
		}
		return output, nil
	}
	if output == "" {
		output = strings.TrimSpace(result)
	}
	if logger != nil {
		logger.Warn("[Homepage] Caddy config reload failed", "output", truncateStr(output, 500))
	}
	return output, fmt.Errorf("caddy reload failed: %s", truncateStr(output, 500))
}

// writeHomepageBindMountedFile writes a file that is bind-mounted into a
// container (the Caddyfile). The container binds the file by inode, so
// replacing it with a rename would leave the running container on the
// previous content. The new content is therefore staged in a synced temporary
// file in the same directory and copied into the existing file, which keeps the
// bind mount valid.
func writeHomepageBindMountedFile(path string, content []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".aurago-publish-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	staged, err := os.ReadFile(tmpName)
	if err != nil {
		return err
	}
	target, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer target.Close()
	if err := target.Truncate(0); err != nil {
		return err
	}
	if _, err := target.WriteAt(staged, 0); err != nil {
		return err
	}
	return target.Sync()
}

// rewriteCaddyfileWithProxyRoutes rewrites the Caddyfile on disk (inside the
// container mount) and reloads Caddy.  Used when a dev server starts/stops
// and the proxy routes change while the web container is already running.
func rewriteCaddyfileWithProxyRoutes(cfg HomepageConfig, dockerCfg DockerConfig, routes []ProxyRoute, logger *slog.Logger) error {
	caddyfile := buildCaddyfileWithProxies(cfg.WebServerDomain, cfg.WebServerPort, routes)
	caddyfilePath := filepath.Join(cfg.WorkspacePath, ".aurago-Caddyfile")
	if err := writeHomepageBindMountedFile(caddyfilePath, []byte(caddyfile)); err != nil {
		return fmt.Errorf("failed to write Caddyfile: %w", err)
	}

	ensureHomepageNetwork(dockerCfg, logger)
	connectContainerToNetwork(dockerCfg, homepageWebContainer, logger)

	reloadOutput, err := reloadCaddy(dockerCfg, logger)
	if err != nil {
		return fmt.Errorf("failed to reload Caddy after writing proxy routes: host_caddyfile_path=%s container_config_path=/etc/caddy/Caddyfile webserver_container=%s port=%d output=%s: %w",
			caddyfilePath, homepageWebContainer, cfg.WebServerPort, truncateStr(reloadOutput, 500), err)
	}
	return nil
}
