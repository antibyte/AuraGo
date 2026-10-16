package videostudio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// parityCase is one case of a shared math fixture (testdata/parity/<topic>.json, plan index §4.13). Go runs a topic in
// TestParity<Topic>; the JS side runs every file in TestVideoStudioEffectsParity (ui). Numbers agree within 1e-6, pixel
// geometry exactly. Later topics (transitions, easing, keyframes, …) reuse this loader.
type parityCase struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	Want  json.RawMessage `json:"want"`
}

// readParityCases loads the cases of one topic and fails when the file is missing or has no cases.
func readParityCases(t *testing.T, topic string) []parityCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "parity", topic+".json"))
	if err != nil {
		t.Fatalf("parity fixture %s: %v", topic, err)
	}
	var fixture struct {
		Cases []parityCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parity fixture %s: %v", topic, err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatalf("parity fixture %s has no cases", topic)
	}
	return fixture.Cases
}
