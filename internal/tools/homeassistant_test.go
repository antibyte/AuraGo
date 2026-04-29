package tools

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestHAEntityStateEndpointEscapesEntityID(t *testing.T) {
	got := haEntityStateEndpoint("sensor.office/temperature")
	want := "/api/states/sensor.office%2Ftemperature"
	if got != want {
		t.Fatalf("haEntityStateEndpoint() = %q, want %q", got, want)
	}
}

func TestHAServiceEndpointEscapesDomainAndService(t *testing.T) {
	got := haServiceEndpoint("light group", "turn/on")
	want := "/api/services/light%20group/turn%2Fon"
	if got != want {
		t.Fatalf("haServiceEndpoint() = %q, want %q", got, want)
	}
}

func TestHomeAssistantDefaultHTTPClientAllowsConfiguredPrivateHosts(t *testing.T) {
	t.Parallel()

	transport, ok := haHTTPClient.Transport.(*http.Transport)
	if ok && (transport.DialContext != nil || transport.DialTLSContext != nil) {
		t.Fatal("Home Assistant uses a configured integration URL and must not use the generic SSRF-protected dialer")
	}
}

func TestHACallServiceRespectsDirectReadOnly(t *testing.T) {
	cfg := HAConfig{ReadOnly: true}

	result := HACallService(cfg, "light", "turn_on", "light.office", nil)
	parsed := parseHAToolJSON(t, result)
	if parsed["status"] != "error" {
		t.Fatalf("status = %q, want error; result=%s", parsed["status"], result)
	}
	if !strings.Contains(parsed["message"], "read-only mode") {
		t.Fatalf("message = %q, want read-only guidance", parsed["message"])
	}
}

func TestHACallServiceRespectsDirectAllowAndBlockLists(t *testing.T) {
	cases := []struct {
		name     string
		cfg      HAConfig
		domain   string
		service  string
		wantText string
	}{
		{
			name:     "empty allowlist denies",
			cfg:      HAConfig{},
			domain:   "light",
			service:  "turn_on",
			wantText: "allowed_services is empty",
		},
		{
			name:     "missing allowlist entry denies",
			cfg:      HAConfig{AllowedServices: []string{"switch.turn_on"}},
			domain:   "light",
			service:  "turn_on",
			wantText: "not allowed by home_assistant.allowed_services",
		},
		{
			name:     "blocklist wins",
			cfg:      HAConfig{AllowedServices: []string{"lock.unlock"}, BlockedServices: []string{"LOCK.UNLOCK"}},
			domain:   "lock",
			service:  "unlock",
			wantText: "blocked by home_assistant.blocked_services",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := HACallService(tc.cfg, tc.domain, tc.service, "entity.test", nil)
			parsed := parseHAToolJSON(t, result)
			if parsed["status"] != "error" {
				t.Fatalf("status = %q, want error; result=%s", parsed["status"], result)
			}
			if !strings.Contains(parsed["message"], tc.wantText) {
				t.Fatalf("message = %q, want %q", parsed["message"], tc.wantText)
			}
		})
	}
}

func parseHAToolJSON(t *testing.T, result string) map[string]string {
	t.Helper()
	var parsed map[string]string
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v; result=%s", err, result)
	}
	return parsed
}
