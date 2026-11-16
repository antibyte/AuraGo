package server

import (
	"aurago/internal/config"
	"aurago/internal/desktopstore"
	"aurago/internal/tools"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setStoreRelayGOOS(t *testing.T, value string) {
	t.Helper()
	before := storeRelayGOOS
	storeRelayGOOS = value
	t.Cleanup(func() { storeRelayGOOS = before })
}
func relayProviderServer(mutate func(*config.Config)) *Server {
	return newStoreRelayServer(&relayFakeClient{}, func(c *config.Config) {
		c.Auth.Enabled, c.Auth.PasswordHash = true, "configured"
		if mutate != nil {
			mutate(c)
		}
	})
}
func relayTopologyFixture(t *testing.T) {
	t.Helper()
	beforeResolve, beforeListen := storeRelayResolveTopology, storeRelayListen
	storeRelayResolveTopology = func(context.Context, *config.Config, string) (storeRelayTopology, error) {
		return storeRelayTopology{bind: "127.0.0.1", host: "host.docker.internal"}, nil
	}
	storeRelayListen = func(network, address string) (net.Listener, error) { return net.Listen(network, "127.0.0.1:0") }
	t.Cleanup(func() { storeRelayResolveTopology = beforeResolve; storeRelayListen = beforeListen })
}
func TestStoreRelayProviderPreservesHTTPS(t *testing.T) {
	relayTopologyFixture(t)
	for _, platform := range []string{"linux", "windows", "darwin"} {
		t.Run(platform, func(t *testing.T) {
			setStoreRelayGOOS(t, platform)
			s := relayProviderServer(func(c *config.Config) {
				c.Server.HTTPS.Enabled = true
				c.Server.Host = "127.0.0.1"
				c.Server.Port = 8443
			})
			t.Cleanup(func() { s.revokeStoreRelay("", true) })
			info, err := (storeLLMRelayProvider{s}).RelayInfo(t.Context(), "")
			if err != nil || info.BaseURL != "http://host.docker.internal:8099" || !s.ConfigSnapshot().Server.HTTPS.Enabled {
				t.Fatalf("isolated relay: %+v %v", info, err)
			}
		})
	}
}
func TestStoreRelayProviderCapabilitiesAndAdmission(t *testing.T) {
	relayTopologyFixture(t)
	s := relayProviderServer(func(c *config.Config) {
		c.StoreLLMRelay.Port = 9010
		c.LLM.Multimodal = true
		c.LLM.Provider = "main"
		c.Providers = []config.ProviderEntry{{ID: "main", ContextWindow: 131072}}
		c.Embeddings.Provider = "emb"
		c.Embeddings.BaseURL = "https://embedding.test/v1"
	})
	t.Cleanup(func() { s.revokeStoreRelay("", true) })
	info, err := (storeLLMRelayProvider{s}).RelayInfo(t.Context(), "")
	if err != nil || !info.Vision || !info.EmbeddingsAvailable || info.ContextWindow != 131072 || !strings.HasSuffix(info.BaseURL, ":9010") {
		t.Fatalf("capabilities: %+v %v", info, err)
	}
	for _, mutate := range []func(*config.Config){func(c *config.Config) { c.Auth.Enabled = false }, func(c *config.Config) { c.Auth.PasswordHash = "" }, func(c *config.Config) { c.LLM.Model = "" }, func(c *config.Config) { c.StoreLLMRelay.Port = 65536 }} {
		if _, err := (storeLLMRelayProvider{relayProviderServer(mutate)}).RelayInfo(t.Context(), ""); err == nil {
			t.Fatal("unsafe setup accepted")
		}
	}
	storeRelayListen = func(string, string) (net.Listener, error) { return nil, errors.New("in use") }
	if _, err := (storeLLMRelayProvider{relayProviderServer(nil)}).RelayInfo(t.Context(), ""); err == nil || !strings.Contains(err.Error(), "port 8099") {
		t.Fatalf("port conflict: %v", err)
	}
}
func TestStoreRelayListenerOnlyExposesFixedAppsAndRevokesCalls(t *testing.T) {
	s := relayProviderServer(nil)
	mux := s.storeRelayMux()
	for path, want := range map[string]int{"/api/config": 404, "/health": 404, "/api/store-llm/other/v1/models": 404, desktopstore.RelayPath(desktopstore.OpenWebUIAppID) + "/models": 401, storeLLMRelayPrefix + "/models": 401} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != want {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	first, release := s.storeRelayContext(t.Context(), desktopstore.OpenWebUIAppID)
	defer release()
	other, releaseOther := s.storeRelayContext(t.Context(), desktopstore.DeepTutorAppID)
	defer releaseOther()
	s.revokeStoreRelay(desktopstore.OpenWebUIAppID, false)
	if first.Err() == nil || other.Err() != nil {
		t.Fatal("revocation crossed app scope")
	}
	s.revokeStoreRelay("", true)
	if other.Err() == nil {
		t.Fatal("shutdown did not cancel relay")
	}
}
func TestOpenWebUIConnectionPreservesProvidersAndTestDoesNotWrite(t *testing.T) {
	relayTopologyFixture(t)
	s := relayProviderServer(nil)
	vaultServer, _ := newProviderTestServer(t, "")
	s.Vault = vaultServer.Vault
	t.Cleanup(func() { s.revokeStoreRelay("", true) })
	if err := s.Vault.WriteSecret(desktopstore.RelayTokenKey(desktopstore.OpenWebUIAppID), "own-relay-token"); err != nil {
		t.Fatal(err)
	}
	state := openWebUIProviders{Enabled: true, URLs: []string{"https://existing.test/v1"}, Keys: []string{"existing-key"}, Configs: map[string]json.RawMessage{"0": json.RawMessage(`{"custom":{"keep":true}}`)}}
	writes := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer administrator-token" {
			t.Error("missing admin token")
			w.WriteHeader(401)
			return
		}
		if r.Method == "POST" {
			writes++
			if r.URL.Path != "/openai/config/update" {
				t.Error("unexpected mutation")
			}
			if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
				t.Error(err)
			}
		} else if r.URL.Path != "/openai/config" {
			t.Error("unexpected read")
		}
		_ = json.NewEncoder(w).Encode(state)
	}))
	defer upstream.Close()
	settings := desktopstore.IntegrationSettings{URL: upstream.URL, Credential: "administrator-token", Read: true}
	if err := s.connectOpenWebUI(t.Context(), settings, false); err == nil || writes != 0 {
		t.Fatal("probe configured provider")
	}
	if err := s.connectOpenWebUI(t.Context(), settings, true); err != nil {
		t.Fatal(err)
	}
	if writes != 1 || len(state.URLs) != 2 || state.URLs[0] != "https://existing.test/v1" || state.Keys[0] != "existing-key" || string(state.Configs["0"]) != `{"custom":{"keep":true}}` {
		t.Fatalf("existing provider changed: writes=%d", writes)
	}
	if err := s.connectOpenWebUI(t.Context(), settings, false); err != nil || writes != 1 {
		t.Fatalf("probe: %v", err)
	}
	if err := s.connectOpenWebUI(t.Context(), settings, true); err != nil || writes != 1 {
		t.Fatal("duplicate provider write")
	}
}

func TestStoreRelayTopologyRejectsRemoteAndControlNetworks(t *testing.T) {
	setStoreRelayGOOS(t, "linux")
	before := storeRelayEngineRequest
	t.Cleanup(func() { storeRelayEngineRequest = before })
	storeRelayEngineRequest = func(_ context.Context, _ tools.DockerConfig, method, path, body string) ([]byte, int, error) {
		if method != "GET" {
			t.Fatal("topology performed a write")
		}
		if strings.HasPrefix(path, "/containers/") {
			return []byte(`{"Name":"/aurago","NetworkSettings":{"Networks":{"apps":{"IPAddress":"127.0.0.2"},"control":{"IPAddress":"127.0.0.3"}}}}`), 200, nil
		}
		if path == "/networks/apps" {
			return []byte(`{"Driver":"bridge","Internal":false,"IPAM":{"Config":[{"Subnet":"127.0.0.0/8","Gateway":"127.0.0.1"}]}}`), 200, nil
		}
		return []byte(`{"Driver":"bridge","Internal":true}`), 200, nil
	}
	cfg := &config.Config{}
	cfg.Docker.Host = "tcp://remote.example:2375"
	if _, err := resolveStoreRelayTopology(t.Context(), cfg, ""); err == nil {
		t.Fatal("remote Docker accepted")
	}
	cfg.Docker.Host = ""
	cfg.Runtime.IsDocker = true
	for _, network := range []string{"", "host", "control", "missing"} {
		if _, err := resolveStoreRelayTopology(t.Context(), cfg, network); err == nil {
			t.Fatalf("unsafe network %q", network)
		}
	}
	// The chosen IP must exist locally; no wildcard/LAN fallback is permitted.
	if _, err := resolveStoreRelayTopology(t.Context(), cfg, "apps"); err == nil {
		t.Fatal("non-local container IP accepted")
	}
}
