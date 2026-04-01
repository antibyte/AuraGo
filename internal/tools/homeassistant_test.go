package tools

import "testing"

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
