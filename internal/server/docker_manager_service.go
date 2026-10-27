package server

import (
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/config"
	"aurago/internal/dockerhost"
	"aurago/internal/dockerstacks"
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
	s.initDockerStacks(store)
}

// initDockerStacks opens the stacks service on the store's database handle (its
// operation journal lives in docker_manager.db). Operations that were running
// when AuraGo last stopped are reattached by Open in the background. A failure
// is logged and leaves DockerStacks nil: the stack routes then answer 503
// stacks_unavailable while hosts, containers and the other resources keep working.
func (s *Server) initDockerStacks(store *dockerhost.Store) {
	stacks, err := dockerstacks.Open(store.DB(), s.DockerHosts, dockerstacks.Options{Logger: s.Logger})
	if err != nil {
		s.logDockerManager("Docker stacks service could not be opened", err)
		return
	}
	s.DockerStacks = stacks
}

// closeDockerHosts stops the background status checks, stops the stacks
// service (running operations keep going in their runner containers and are
// reattached by the next start), releases the cached Engine connections and
// closes the store. The stacks service goes first: its followers use the
// registry. It is safe to call without a registry.
func (s *Server) closeDockerHosts() {
	s.dockerStatus.close()
	if s.DockerStacks != nil {
		s.DockerStacks.Close()
	}
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
