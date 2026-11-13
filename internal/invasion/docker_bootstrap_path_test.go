package invasion

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestDockerEggArchiveLoadsConfigurationAndImportsBootstrap(t *testing.T) {
	for _, dir := range []string{"./data", ".", "state/private"} {
		t.Run(dir, func(t *testing.T) {
			root := t.TempDir()
			cfgBytes := []byte("directories:\n  data_dir: " + dir + "\n")
			writeArchive := func(secret string) {
				bootstrap, _ := json.Marshal(EggBootstrap{Version: 1, Secrets: map[string]string{"egg_shared_key": strings.Repeat("c", 64), "provider_main_api_key": secret}})
				archive, err := buildDockerEggConfigArchive(cfgBytes, bootstrap)
				if err != nil {
					t.Fatal(err)
				}
				reader := tar.NewReader(bytes.NewReader(archive))
				for {
					header, err := reader.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					if strings.HasPrefix(header.Name, "/") || strings.Contains(header.Name, "..") {
						t.Fatal("unsafe archive path")
					}
					target := filepath.Join(root, filepath.FromSlash(header.Name))
					if header.Typeflag == tar.TypeDir {
						if err := os.MkdirAll(target, 0700); err != nil {
							t.Fatal(err)
						}
						continue
					}
					if header.Mode != 0600 || header.Uid != dockerEggConfigUID {
						t.Fatal("bootstrap/config permissions changed")
					}
					data, err := io.ReadAll(reader)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(target, data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			writeArchive("first-bootstrap-fixture")
			cfg, err := config.Load(filepath.Join(root, dockerEggConfigFileName))
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Clean(filepath.Join(root, dir))
			if cfg.Directories.DataDir != want {
				t.Fatalf("resolved data directory = %s, want %s", cfg.Directories.DataDir, want)
			}
			vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(want, "vault.bin"))
			if err != nil {
				t.Fatal(err)
			}
			if err := vault.WriteSecret("unrelated", "retained-vault-fixture"); err != nil {
				t.Fatal(err)
			}
			statePath := filepath.Join(want, "existing-state.db")
			if err := os.WriteFile(statePath, []byte("existing-state-fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"first-bootstrap-fixture", "reconfigured-bootstrap-fixture"} {
				if secret != "first-bootstrap-fixture" {
					writeArchive(secret)
				}
				if err := ApplyEggBootstrap(cfg.Directories.DataDir, vault); err != nil {
					t.Fatal(err)
				}
				if got, err := vault.ReadSecret("provider_main_api_key"); err != nil || got != secret {
					t.Fatal("bootstrap was not imported at the resolved data directory", err)
				}
				if got, err := vault.ReadSecret("unrelated"); err != nil || got != "retained-vault-fixture" {
					t.Fatal("existing Vault entry changed")
				}
				if _, err := os.Stat(filepath.Join(want, EggBootstrapFile)); !os.IsNotExist(err) {
					t.Fatal("applied private input not retired")
				}
			}
			if data, err := os.ReadFile(statePath); err != nil || string(data) != "existing-state-fixture" {
				t.Fatal("existing data moved or overwritten")
			}
			if _, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(want, "vault.bin")); err != nil {
				t.Fatal("local Vault master key changed", err)
			}
		})
	}
}

func TestDockerEggRejectsArchiveEscapeBeforeStopping(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(500) }))
	defer server.Close()
	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(server.URL, "http://"))
	nest := NestRecord{ID: "12345678-fixture", DeployMethod: "docker_local"}
	for _, cfg := range []string{"directories:\n  data_dir: ../../outside\n", "directories:\n  data_dir: /etc\n", "directories:\n  data_dir: config.yaml/child\n", "directories: [invalid"} {
		if err := (&DockerConnector{}).Deploy(context.Background(), nest, nil, EggDeployPayload{ConfigYAML: []byte(cfg), Bootstrap: []byte("private-fixture")}); err == nil {
			t.Fatal("invalid archive accepted")
		}
		if err := (&DockerConnector{}).Reconfigure(context.Background(), nest, nil, []byte(cfg), []byte("private-fixture")); err == nil {
			t.Fatal("invalid reconfiguration accepted")
		}
	}
	if calls != 0 {
		t.Fatalf("invalid paths reached Engine before validation: %d requests", calls)
	}
}

func TestPrivateDockerBootstrapTransportBoundaries(t *testing.T) {
	for _, tc := range []struct {
		host    string
		allowed bool
	}{
		{"unix:///var/run/docker.sock", true}, {"npipe:////./pipe/docker_engine", true}, {"npipe://./pipe/docker_engine", true},
		{"tcp://127.0.0.1:2375", true}, {"tcp://[::1]:2375", true}, {"tcp://localhost:2375", true},
		{"tcp://192.0.2.1:2375", false}, {"tcp://0.0.0.0:2375", false}, {"tcp://docker.example:2375", false},
		{"tcp://localhost.evil.example:2375", false}, {"npipe://remote/pipe/docker_engine", false}, {"unix://remote/var/run/docker.sock", false},
		{"tcp://user@127.0.0.1:2375", false},
	} {
		t.Run(tc.host, func(t *testing.T) {
			t.Setenv("DOCKER_HOST", tc.host)
			err := requirePrivateDockerBootstrap(NestRecord{DeployMethod: "docker_local"})
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v: %v", tc.allowed, err)
			}
		})
	}
	for _, mode := range []string{"", "invalid", "tls", "mtls"} {
		err := requirePrivateDockerBootstrap(NestRecord{DeployMethod: "docker_remote", DockerTLS: mode})
		if (err == nil) != (mode == "tls" || mode == "mtls") {
			t.Fatalf("remote mode %q: %v", mode, err)
		}
	}
}
