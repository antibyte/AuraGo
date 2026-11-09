package server

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

// These tests change the package-level storeRelayGOOS seam, so none of them is
// parallel.
func setStoreRelayGOOS(t *testing.T, goos string) {
	t.Helper()
	previous := storeRelayGOOS
	storeRelayGOOS = goos
	t.Cleanup(func() { storeRelayGOOS = previous })
}

func relayProviderServer(mutate func(*config.Config)) *Server {
	return newStoreRelayServer(&relayFakeClient{}, func(c *config.Config) {
		c.Server.Host = "0.0.0.0"
		c.Auth.Enabled, c.Auth.PasswordHash = true, "configured"
		if mutate != nil {
			mutate(c)
		}
	})
}

func TestStoreRelayProviderNativeDefault(t *testing.T) {
	setStoreRelayGOOS(t, "linux")
	info, err := storeLLMRelayProvider{s: relayProviderServer(nil)}.RelayInfo(context.Background(), "")
	if err != nil {
		t.Fatalf("RelayInfo: %v", err)
	}
	if info.BaseURL != "http://host.docker.internal:8088" {
		t.Fatalf("BaseURL = %q, want the default port 8088 behind the Docker host alias", info.BaseURL)
	}
	if len(info.ExtraHosts) != 1 || info.ExtraHosts[0] != "host.docker.internal:host-gateway" {
		t.Fatalf("ExtraHosts = %v", info.ExtraHosts)
	}
	if info.Vision || info.EmbeddingsAvailable || info.ContextWindow != 0 {
		t.Fatalf("flags must default to off/unknown: %+v", info)
	}
	// The Host the container sends is the one the relay accepts with login off.
	if storeRelayHostGateway != storeLLMRelayDockerHostAlias+":host-gateway" || !strings.HasPrefix(info.BaseURL, "http://"+storeLLMRelayDockerHostAlias+":") {
		t.Fatalf("provider and relay Host alias drifted: %q vs %q", info.BaseURL, storeLLMRelayDockerHostAlias)
	}

	// A configured port is used, and the network mode is ignored in v1.
	info, err = storeLLMRelayProvider{s: relayProviderServer(func(c *config.Config) { c.Server.Port = 9123 })}.RelayInfo(context.Background(), "some-shared-network")
	if err != nil || info.BaseURL != "http://host.docker.internal:9123" || len(info.ExtraHosts) != 1 {
		t.Fatalf("custom port: %+v err=%v", info, err)
	}
}

func TestStoreRelayProviderLoopbackOnlyDependsOnThePlatform(t *testing.T) {
	setStoreRelayGOOS(t, "linux")
	for _, host := range []string{"", "127.0.0.1", "localhost", "LOCALHOST", "::1", "[::1]", "127.0.0.2"} {
		storeRelayGOOS = "linux"
		_, err := storeLLMRelayProvider{s: relayProviderServer(func(c *config.Config) { c.Server.Host = host })}.RelayInfo(context.Background(), "")
		if err == nil || !strings.Contains(err.Error(), "server.host") || !strings.Contains(err.Error(), "0.0.0.0") {
			t.Fatalf("linux host %q: err = %v, want the server.host hint", host, err)
		}
		for _, goos := range []string{"windows", "darwin"} {
			storeRelayGOOS = goos
			info, err := storeLLMRelayProvider{s: relayProviderServer(func(c *config.Config) { c.Server.Host = host })}.RelayInfo(context.Background(), "")
			if err != nil || info.BaseURL != "http://host.docker.internal:8088" {
				t.Fatalf("%s host %q: info = %+v err = %v, Docker Desktop must accept loopback", goos, host, info, err)
			}
		}
	}
	// A wildcard or LAN bind is fine on Linux.
	storeRelayGOOS = "linux"
	for _, host := range []string{"0.0.0.0", "::", "192.168.1.20"} {
		if _, err := (storeLLMRelayProvider{s: relayProviderServer(func(c *config.Config) { c.Server.Host = host })}).RelayInfo(context.Background(), ""); err != nil {
			t.Fatalf("linux host %q: %v", host, err)
		}
	}
}

func TestStoreRelayProviderRejectsHTTPSOnlyListeners(t *testing.T) {
	https := func(port, httpsPort, httpPort, tunnel int) func(*config.Config) {
		return func(c *config.Config) {
			c.Server.HTTPS.Enabled = true
			c.Server.Port, c.Server.HTTPS.HTTPSPort, c.Server.HTTPS.HTTPPort = port, httpsPort, httpPort
			c.CloudflareTunnel.LoopbackPort = tunnel
		}
	}
	for name, mutate := range map[string]func(*config.Config){
		"port is the TLS port":      https(8443, 8443, 0, 0),
		"port is the redirect port": https(80, 443, 80, 0),
		"default port, TLS on 443":  https(0, 0, 0, 0),
		"tunnel loopback only":      https(8088, 8443, 0, 18088),
	} {
		for _, goos := range []string{"linux", "windows"} {
			setStoreRelayGOOS(t, goos)
			_, err := storeLLMRelayProvider{s: relayProviderServer(mutate)}.RelayInfo(context.Background(), "")
			if err == nil || !strings.Contains(err.Error(), "plain-HTTP") || !strings.Contains(err.Error(), "server.https.enabled") {
				t.Fatalf("%s on %s: err = %v, want the plain-HTTP explanation", name, goos, err)
			}
		}
	}

	// A separate server.port gets the plain 127.0.0.1 listener: fine on Docker
	// Desktop, unreachable from containers on Linux.
	setStoreRelayGOOS(t, "windows")
	info, err := storeLLMRelayProvider{s: relayProviderServer(https(8088, 8443, 0, 0))}.RelayInfo(context.Background(), "")
	if err != nil || info.BaseURL != "http://host.docker.internal:8088" {
		t.Fatalf("dedicated listener on Docker Desktop: %+v err=%v", info, err)
	}
	setStoreRelayGOOS(t, "linux")
	_, err = storeLLMRelayProvider{s: relayProviderServer(https(8088, 8443, 0, 0))}.RelayInfo(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1 only") {
		t.Fatalf("dedicated listener on Linux: err = %v", err)
	}
}

func TestStoreRelayProviderNeedsAMainLLM(t *testing.T) {
	setStoreRelayGOOS(t, "linux")
	for name, server := range map[string]*Server{
		"no model":  relayProviderServer(func(c *config.Config) { c.LLM.Model = " " }),
		"no client": newStoreRelayServer(nil, func(c *config.Config) {
			c.Server.Host, c.Auth.Enabled, c.Auth.PasswordHash = "0.0.0.0", true, "configured"
		}),
	} {
		_, err := storeLLMRelayProvider{s: server}.RelayInfo(context.Background(), "")
		if err == nil || !strings.Contains(err.Error(), "Config > Providers") {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	if _, err := (storeLLMRelayProvider{}).RelayInfo(context.Background(), ""); err == nil {
		t.Fatal("a provider without a server must fail")
	}
}

func TestStoreRelayProviderReportsModelCapabilities(t *testing.T) {
	setStoreRelayGOOS(t, "linux")
	server := relayProviderServer(func(c *config.Config) {
		c.LLM.Provider, c.LLM.Multimodal = "main", true
		c.Providers = []config.ProviderEntry{
			{ID: "other", ContextWindow: 8000},
			{ID: "main", ContextWindow: 131072},
		}
		c.Embeddings.Provider, c.Embeddings.BaseURL = "emb", "https://embed.example.test/v1"
	})
	info, err := storeLLMRelayProvider{s: server}.RelayInfo(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Vision || !info.EmbeddingsAvailable || info.ContextWindow != 131072 {
		t.Fatalf("info = %+v, want vision, embeddings and the main provider's 131072 window", info)
	}

	// Unknown or unset provider entries mean an unknown window.
	server = relayProviderServer(func(c *config.Config) {
		c.LLM.Provider = "missing"
		c.Providers = []config.ProviderEntry{{ID: "main", ContextWindow: 131072}}
	})
	if info, err = (storeLLMRelayProvider{s: server}).RelayInfo(context.Background(), ""); err != nil || info.ContextWindow != 0 {
		t.Fatalf("unknown provider: info = %+v err = %v", info, err)
	}
}

func TestStoreRelayProviderNeedsAnEffectiveLogin(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "darwin"} {
		setStoreRelayGOOS(t, goos)
		for name, mutate := range map[string]func(*config.Config){
			"login disabled":           func(c *config.Config) { c.Auth.Enabled, c.Auth.PasswordHash = false, "configured" },
			"login disabled, no hash":  func(c *config.Config) { c.Auth.Enabled, c.Auth.PasswordHash = false, "" },
			"enabled without password": func(c *config.Config) { c.Auth.Enabled, c.Auth.PasswordHash = true, "" },
		} {
			_, err := storeLLMRelayProvider{s: relayProviderServer(mutate)}.RelayInfo(context.Background(), "")
			if err == nil || !strings.Contains(err.Error(), "login") || !strings.Contains(err.Error(), "Config > Auth") {
				t.Fatalf("%s on %s: err = %v, want the login requirement", name, goos, err)
			}
		}
		info, err := storeLLMRelayProvider{s: relayProviderServer(nil)}.RelayInfo(context.Background(), "")
		if err != nil || info.BaseURL == "" {
			t.Fatalf("login enabled with a password on %s: info = %+v err = %v", goos, info, err)
		}
	}
}
