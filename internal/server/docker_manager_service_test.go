package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitDockerHostsOpensTheStoreInTheDataDir(t *testing.T) {
	s := testContainerServer(true, false) // no vault: hosts without secrets still work
	dir := t.TempDir()
	s.Cfg.Directories.DataDir = dir
	s.Cfg.Docker.Host = "tcp://10.0.0.5:2375"

	s.initDockerHosts()
	t.Cleanup(s.closeDockerHosts)
	if s.DockerHosts == nil {
		t.Fatal("initDockerHosts left DockerHosts nil")
	}
	if _, err := os.Stat(filepath.Join(dir, "docker_manager.db")); err != nil {
		t.Fatalf("docker_manager.db is not in the data directory: %v", err)
	}
	hosts, err := s.DockerHosts.Hosts(context.Background())
	if err != nil || len(hosts) != 1 || hosts[0].ID != "local" || !strings.Contains(hosts[0].Address, "10.0.0.5") {
		t.Fatalf("hosts = %+v, %v; want only the local host following docker.host", hosts, err)
	}
	if s.DockerHosts.SecretSet("local") {
		t.Fatal("the local host reports a secret")
	}

	// docker.host is read on every use, so a config reload needs no restart.
	s.CfgMu.Lock()
	s.Cfg.Docker.Host = "tcp://10.0.0.9:2375"
	s.CfgMu.Unlock()
	hosts, err = s.DockerHosts.Hosts(context.Background())
	if err != nil || !strings.Contains(hosts[0].Address, "10.0.0.9") {
		t.Fatalf("local host after a docker.host change = %+v, %v", hosts, err)
	}
}

func TestInitDockerHostsLeavesTheRegistryNilWhenTheStoreCannotOpen(t *testing.T) {
	s := testContainerServer(true, false)
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.Cfg.Directories.DataDir = blocker // MkdirAll under a file fails

	s.initDockerHosts()
	t.Cleanup(s.closeDockerHosts)
	if s.DockerHosts != nil {
		t.Fatal("DockerHosts is set although the store directory is unusable")
	}
}
