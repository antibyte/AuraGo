package server

import (
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/config"
	"aurago/internal/dockerhost"
	"aurago/internal/security"
)

// dockerManagerDBFile is the Docker Manager's SQLite store inside the data directory.
const dockerManagerDBFile = "docker_manager.db"

// The server vault is the registry's secret store as it is: a missing key
// answers security.ErrSecretNotFound.
var _ dockerhost.SecretVault = (*security.Vault)(nil)

// dockerManagerDBPath places docker_manager.db in the configured data directory.
func dockerManagerDBPath(cfg *config.Config) string {
	dir := "data"
	if cfg != nil && strings.TrimSpace(cfg.Directories.DataDir) != "" {
		dir = cfg.Directories.DataDir
	}
	return filepath.Join(dir, dockerManagerDBFile)
}

// dockerLocalEndpoint is the dockerhost.LocalEndpointFunc: the configured
// docker.host ("" means the platform default), read under the config lock
// exactly as the Containers API does.
func (s *Server) dockerLocalEndpoint() string {
	cfg, _, _ := containerDockerConfig(s)
	return cfg.Host
}

// initDockerHosts opens docker_manager.db and builds the host registry. A
// failure is logged and leaves DockerHosts nil: the Docker Manager routes then
// answer 503 docker_hosts_unavailable while every other feature keeps running.
func (s *Server) initDockerHosts() {
	if s.Cfg == nil || s.DockerHosts != nil {
		return
	}
	path := dockerManagerDBPath(s.Cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		s.logDockerManager("Docker host store directory is not usable", err)
		return
	}
	store, err := dockerhost.OpenStore(path)
	if err != nil {
		s.logDockerManager("Docker host store could not be opened", err)
		return
	}
	// A nil *security.Vault must not become a non-nil interface: the registry
	// tests its vault against nil (hosts without secrets then still work).
	var vault dockerhost.SecretVault
	if s.Vault != nil {
		vault = s.Vault
	}
	s.dockerHostStore = store
	s.DockerHosts = dockerhost.NewRegistry(store, vault, s.dockerLocalEndpoint)
}

// closeDockerHosts stops the background status checks, releases the cached
// Engine connections and closes the store. It is safe to call without a
// registry.
func (s *Server) closeDockerHosts() {
	s.dockerStatus.close()
	if s.DockerHosts != nil {
		s.DockerHosts.Close()
	}
	if s.dockerHostStore != nil {
		_ = s.dockerHostStore.Close()
	}
}

func (s *Server) logDockerManager(message string, err error) {
	if s.Logger != nil {
		s.Logger.Warn("[DockerManager] "+message, "error", err)
	}
}
