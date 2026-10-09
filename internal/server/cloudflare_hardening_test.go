package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestCloudflareConfigSaveRejectsInvalidPortsBeforeWriting(t *testing.T) {
	for _, setup := range []bool{false, true} {
		for _, port := range []interface{}{-1, 65536, 1.5, 8080} {
			root := t.TempDir()
			path := filepath.Join(root, "config.yaml")
			initial := "server:\n  host: 127.0.0.1\n  port: 8080\ncloudflare_tunnel:\n  mode: native\n"
			if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			cfg.ConfigPath = path
			s := &Server{Cfg: cfg, Logger: slog.Default()}
			patch := map[string]interface{}{"cloudflare_tunnel": map[string]interface{}{"metrics_port": port}}
			if setup {
				if _, err := applyConfigPatch(s, patch); err == nil {
					t.Fatalf("setup accepted %v", port)
				}
			} else {
				body, _ := json.Marshal(patch)
				rec := httptest.NewRecorder()
				handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(string(body))))
				if rec.Code != 400 {
					t.Fatalf("config accepted %v: %d %s", port, rec.Code, rec.Body.String())
				}
			}
			after, _ := os.ReadFile(path)
			if string(after) != initial {
				t.Fatalf("invalid port %v changed config", port)
			}
		}
	}
}

func TestCloudflareConfigRevocationStopsBeforeDockerPermissionsChange(t *testing.T) {
	previous, configured := tools.CurrentRuntimePermissionsForTest()
	defer func() {
		tools.SetRuntimePermissionResolver(nil)
		if configured {
			tools.ConfigureRuntimePermissions(previous)
		} else {
			tools.ClearRuntimePermissionsForTest()
		}
	}()
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			var exists atomic.Bool
			exists.Store(true)
			var stops atomic.Int32
			var s *Server
			id := strings.Repeat("a", 64)
			engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/version":
					_, _ = w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`))
				case strings.HasSuffix(r.URL.Path, "/json"):
					if !exists.Load() {
						w.WriteHeader(404)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"Id": id, "Name": "/aurago-cloudflared", "Config": map[string]interface{}{"Image": "cloudflare/cloudflared:latest", "Cmd": []string{"tunnel", "run"}}, "State": map[string]bool{"Running": true}})
				case strings.HasSuffix(r.URL.Path, "/stop"):
					stops.Add(1)
					perms, _ := tools.CurrentRuntimePermissionsForTest()
					if !perms.DockerEnabled || perms.DockerReadOnly || !s.ConfigSnapshot().CloudflareTunnel.Enabled {
						t.Error("permissions were published before the stop")
					}
					if fail {
						w.WriteHeader(500)
					} else {
						w.WriteHeader(204)
					}
				case r.Method == http.MethodDelete:
					exists.Store(false)
					w.WriteHeader(204)
				default:
					t.Errorf("unexpected engine request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(500)
				}
			}))
			defer engine.Close()
			path := filepath.Join(t.TempDir(), "config.yaml")
			initial := fmt.Sprintf("server: {host: 127.0.0.1, port: 8080}\ndocker: {enabled: true, host: %q}\ncloudflare_tunnel: {enabled: true, mode: docker, auth_method: token}\n", "tcp://"+strings.TrimPrefix(engine.URL, "http://"))
			if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			cfg.ConfigPath = path
			vault, err := security.NewVault(strings.Repeat("ab", 32), filepath.Join(t.TempDir(), "vault.bin"))
			if err != nil {
				t.Fatal(err)
			}
			s = &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}
			s.bindRuntimePermissions()
			defer func() {
				s.replaceConfigSnapshot(cfg)
				exists.Store(false)
				_ = tools.CloudflareTunnelShutdown(tools.CloudflareTunnelConfigFromConfig(cfg), nil, s.Logger, false)
				_ = tools.CloudflareTunnelReconcile(tools.CloudflareTunnelConfig{Enabled: true, Mode: "native"}, s.Logger)
			}()
			if result := tools.CloudflareTunnelReconcile(s.buildTunnelConfig(), s.Logger); !strings.Contains(result, `"status":"ok"`) {
				t.Fatal(result)
			}
			rec := httptest.NewRecorder()
			handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"cloudflare_tunnel":{"enabled":false},"docker":{"enabled":false,"readonly":true}}`)))
			after, _ := os.ReadFile(path)
			if stops.Load() != 1 {
				t.Fatalf("stops = %d", stops.Load())
			}
			if fail {
				if rec.Code != 409 || string(after) != initial || !s.ConfigSnapshot().Docker.Enabled {
					t.Fatalf("failed stop changed config: %d %s", rec.Code, rec.Body.String())
				}
			} else {
				if rec.Code != 200 || s.ConfigSnapshot().Docker.Enabled || s.ConfigSnapshot().CloudflareTunnel.Enabled {
					t.Fatalf("revocation failed: %d %s", rec.Code, rec.Body.String())
				}
			}
		})
	}
}

func TestCloudflareActionsPreserveMethodsReadOnlyAndAdminPolicy(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.CloudflareTunnel.Enabled = true
	s.Cfg.CloudflareTunnel.ReadOnly = true
	s.Cfg.CloudflareTunnel.Mode = "native"
	for name, handler := range map[string]http.HandlerFunc{"start": handleCloudflareTunnelStart(s), "stop": handleCloudflareTunnelStop(s), "restart": handleCloudflareTunnelRestart(s)} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cloudflare-tunnel/"+name, nil))
		if rec.Code != 405 {
			t.Fatalf("%s accepted GET", name)
		}
		for _, auth := range []string{"token", "named", "quick"} {
			s.Cfg.CloudflareTunnel.AuthMethod = auth
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/cloudflare-tunnel/"+name, nil))
			if !strings.Contains(rec.Body.String(), "read-only") {
				t.Fatalf("%s/%s bypassed read-only: %s", name, auth, rec.Body.String())
			}
		}
	}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "cloudflare-admin-fixture"
	mux := http.NewServeMux()
	s.registerInfrastructureRoutes(mux, make(chan struct{}))
	for _, action := range []string{"start", "stop", "restart", "status"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/cloudflare-tunnel/"+action, nil))
		if rec.Code != 403 && rec.Code != 401 {
			t.Fatalf("%s lacks admin gate: %d", action, rec.Code)
		}
	}
}

func TestCloudflareStatusAPIsShareObservedStateWhenDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.CloudflareTunnel.Mode = "native"
	legacy, current := httptest.NewRecorder(), httptest.NewRecorder()
	handleTunnelStatus(s).ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/api/tunnel/status", nil))
	handleCloudflareTunnelStatus(s).ServeHTTP(current, httptest.NewRequest(http.MethodGet, "/api/cloudflare-tunnel/status", nil))
	var envelope map[string]interface{}
	if err := json.Unmarshal(current.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["tunnel"] != strings.TrimSpace(legacy.Body.String()) {
		t.Fatal("status APIs diverged")
	}
	if !strings.Contains(legacy.Body.String(), "state_known") {
		t.Fatal("disabled integration hid observed state")
	}
}
