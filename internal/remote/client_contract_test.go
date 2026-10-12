package remote

import (
	"encoding/json"
	"testing"
)

func TestTelemetryDistinguishesZeroFromUnknown(t *testing.T) {
	data, err := json.Marshal(HeartbeatPayload{MetricsAvailable: []string{"cpu_percent"}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["cpu_percent"] != float64(0) || got["mem_percent"] != nil {
		t.Fatalf("availability lost: %s", data)
	}
}
