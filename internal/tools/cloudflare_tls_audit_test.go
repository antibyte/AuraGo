package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type cloudflareAuditTransport func(*http.Request) (*http.Response, error)

func (f cloudflareAuditTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCloudflareAPIExceptionAppliesOnlyToAuraGoOrigin(t *testing.T) {
	original := `{"success":true,"result":{"config":{"warp-routing":{"enabled":true},"originRequest":{"noTLSVerify":true,"connectTimeout":12},"ingress":[{"hostname":"ui.example","service":"https://localhost:8443","originRequest":{"httpHostHeader":"preserve.example"}},{"hostname":"other.example","service":"https://remote.example:8443"},{"service":"http_status:404"}]}}}`
	var saved map[string]any
	old := cfHTTPClient
	cfHTTPClient = &http.Client{Transport: cloudflareAuditTransport(func(r *http.Request) (*http.Response, error) {
		body := original
		if r.Method == http.MethodPut {
			if err := json.NewDecoder(r.Body).Decode(&saved); err != nil {
				t.Fatal(err)
			}
			body = `{"success":true}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	defer func() { cfHTTPClient = old }()
	applyNoTLSVerifyViaAPI(context.Background(), CloudflareTunnelConfig{AccountID: "fixture", TunnelID: "fixture", HTTPSEnabled: true, HTTPSPort: 8443}, "fixture-only", slogDiscard())
	if saved == nil {
		t.Fatal("legacy global TLS exception was not corrected")
	}
	cfg := saved["config"].(map[string]any)
	origin := cfg["originRequest"].(map[string]any)
	if origin["noTLSVerify"] == true || origin["connectTimeout"] != float64(12) {
		t.Fatalf("global policy changed incorrectly: %v", origin)
	}
	if cfg["warp-routing"] == nil {
		t.Fatal("unrelated Cloudflare configuration was discarded")
	}
	rules := cfg["ingress"].([]any)
	local := rules[0].(map[string]any)["originRequest"].(map[string]any)
	if local["noTLSVerify"] != true || local["httpHostHeader"] != "preserve.example" {
		t.Fatalf("local origin policy: %v", local)
	}
	if rules[1].(map[string]any)["originRequest"] != nil {
		t.Fatal("remote origin TLS verification was disabled")
	}
}

func TestCloudflareNamedExceptionAppliesOnlyToAuraGoOrigin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	cfg := CloudflareTunnelConfig{TunnelName: "fixture", HTTPSEnabled: true, HTTPSPort: 8443, ExposeWebUI: true, WebUIPort: 8080, CustomIngress: []CloudflareIngress{{Hostname: "other.example", Service: "https://remote.example:8443"}}}
	if err := writeNamedTunnelConfig(cfg, "credentials.json", path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["originRequest"] != nil {
		t.Fatal("global TLS exception remains")
	}
	rules := got["ingress"].([]any)
	if rules[0].(map[string]any)["originRequest"] != nil {
		t.Fatal("remote service received local TLS exception")
	}
	local := rules[1].(map[string]any)["originRequest"].(map[string]any)
	if local["noTLSVerify"] != true {
		t.Fatal("local self-signed origin exception missing")
	}
}
