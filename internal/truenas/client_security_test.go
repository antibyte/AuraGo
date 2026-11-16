package truenas

import (
	"aurago/internal/config"
	"aurago/internal/security"
	"strings"
	"testing"
)

func TestTrueNASClientValidatesHostAndRegistersKey(t *testing.T) {
	cfg := config.TrueNASConfig{Enabled: true, Host: "nas.local", UseHTTPS: true, APIKey: "fixture-truenas-secret"}
	client, err := NewClient(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if strings.Contains(security.Scrub(cfg.APIKey), cfg.APIKey) {
		t.Error("key not registered")
	}
	if client.httpClient.CheckRedirect == nil {
		t.Error("missing redirect policy")
	}
	for _, host := range []string{"evil.test/path", "user@evil.test", "nas.local?query", "nas.local#fragment", "nas.local:1234", "nas.local%2fsecret"} {
		cfg.Host = host
		if _, err := NewClient(cfg, nil); err == nil {
			t.Errorf("accepted host %s", host)
		}
	}
	cfg.Host = "::1"
	c, err := NewClient(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if !strings.Contains(c.baseURL, "[::1]:443") {
		t.Fatalf("IPv6 base URL=%s", c.baseURL)
	}
}
