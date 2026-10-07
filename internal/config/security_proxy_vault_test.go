package config

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestApplyVaultSecretsLoadsSecurityProxyBasicAuth(t *testing.T) {
	cfg := &Config{}
	cfg.SecurityProxy.BasicAuth.Enabled = true
	vault := &testSecretVault{data: map[string]string{
		ProxyBasicAuthUserVaultKey:     "admin",
		ProxyBasicAuthPasswordVaultKey: "vault-password",
	}}

	cfg.ApplyVaultSecrets(vault)

	if got := cfg.SecurityProxy.BasicAuth.Username; got != "admin" {
		t.Fatalf("basic auth user = %q, want admin", got)
	}
	if got := cfg.SecurityProxy.BasicAuth.Password; got != "vault-password" {
		t.Fatalf("basic auth password = %q, want the Vault value", got)
	}
}

func TestSecurityProxyBasicAuthCredentialsStayVaultOnly(t *testing.T) {
	cfg := &Config{}
	cfg.SecurityProxy.BasicAuth.Enabled = true
	cfg.SecurityProxy.BasicAuth.Username = "admin"
	cfg.SecurityProxy.BasicAuth.Password = "vault-password"

	rawYAML, err := yaml.Marshal(cfg.SecurityProxy)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	rawJSON, err := json.Marshal(cfg.SecurityProxy)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	for name, raw := range map[string]string{"yaml": string(rawYAML), "json": string(rawJSON)} {
		if strings.Contains(raw, "vault-password") || strings.Contains(raw, "admin") {
			t.Fatalf("%s serialization exposes Vault-only credentials:\n%s", name, raw)
		}
	}
}
