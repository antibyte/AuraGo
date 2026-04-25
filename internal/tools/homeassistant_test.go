package tools

import (
	"net/http"
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
