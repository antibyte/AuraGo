package tools

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnsibleManagedPolicyChecksImageBeforeReplacingContainer(t *testing.T) {
	for _, policy := range []string{"2", "3"} {
		for _, rotate := range []bool{false, true} {
			t.Run(policy+"/rotate="+map[bool]string{false: "false", true: "true"}[rotate], func(t *testing.T) {
				exists, stopped, deleted, created := true, false, false, false
				var payload struct {
					Env        []string
					HostConfig struct{ Binds []string }
				}
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					path := r.URL.Path
					switch {
					case path == "/version":
						_, _ = w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.24"}`))
					case strings.Contains(path, "/images/"):
						_ = json.NewEncoder(w).Encode(map[string]interface{}{"Config": map[string]interface{}{"Labels": map[string]string{ansibleImagePolicyLabel: policy}}})
					case r.Method == http.MethodGet && strings.Contains(path, "/containers/"):
						if !exists {
							w.WriteHeader(http.StatusNotFound)
							return
						}
						_ = json.NewEncoder(w).Encode(map[string]interface{}{"State": map[string]bool{"Running": true}, "Config": map[string]interface{}{"Env": []string{"AURAGO_ANSIBLE_POLICY=2", "ANSIBLE_READONLY=false", "DEFAULT_INVENTORY="}, "Labels": map[string]string{ansibleImagePolicyLabel: "2"}}})
					case r.Method == http.MethodPost && strings.Contains(path, "/stop"):
						stopped = true
						w.WriteHeader(http.StatusNoContent)
					case r.Method == http.MethodDelete:
						deleted, exists = true, false
						w.WriteHeader(http.StatusNoContent)
					case r.Method == http.MethodPost && strings.Contains(path, "/containers/create"):
						created = true
						if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
							t.Error(err)
						}
						w.WriteHeader(http.StatusCreated)
						_, _ = w.Write([]byte(`{"Id":"fixture"}`))
					default:
						w.WriteHeader(http.StatusNoContent)
					}
				}))
				defer srv.Close()
				cfg := AnsibleSidecarConfig{Image: "fixture-ansible:3", Token: "fixture-token", PlaybooksDir: t.TempDir(), InventoryDir: t.TempDir()}
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				host := "tcp://" + strings.TrimPrefix(srv.URL, "http://")
				if rotate {
					ReapplyAnsibleToken(host, cfg, logger)
				} else {
					EnsureAnsibleSidecarRunning(host, cfg, logger)
				}
				if policy == "2" {
					if stopped || deleted || created {
						t.Fatal("obsolete image caused an existing container to be replaced")
					}
					return
				}
				if !stopped || !deleted || !created || len(payload.HostConfig.Binds) < 2 {
					t.Fatalf("policy upgrade did not preserve configured mounts: stopped=%v deleted=%v created=%v binds=%v", stopped, deleted, created, payload.HostConfig.Binds)
				}
				if !strings.Contains(strings.Join(payload.Env, "\n"), "AURAGO_ANSIBLE_POLICY=3") || !strings.Contains(strings.Join(payload.Env, "\n"), "ANSIBLE_API_TOKEN=fixture-token") {
					t.Fatal("replacement lost the current policy or token")
				}
			})
		}
	}
}
