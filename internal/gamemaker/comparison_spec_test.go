package gamemaker

import (
	"encoding/json"
	"os"
	"testing"
)

// The weak-model benchmark is a product decision, not a test fixture: keep the
// documented spec executable and stable so the runner cannot drift silently.
func TestWeakModelComparisonSpecStaysExecutable(t *testing.T) {
	const specPath = "../../documentation/game-maker-comparison.json"
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s: %v", specPath, err)
	}
	var spec struct {
		SchemaVersion int      `json:"schema_version"`
		Policy        string   `json:"policy"`
		RecordFields  []string `json:"record_fields"`
		Cases         []struct {
			ID       string   `json:"id"`
			Prompt   string   `json:"prompt"`
			BaseCase string   `json:"base_case"`
			Observe  []string `json:"observe"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse %s: %v", specPath, err)
	}
	if spec.SchemaVersion < 1 || spec.Policy == "" {
		t.Fatalf("spec header incomplete: schema_version=%d policy=%q", spec.SchemaVersion, spec.Policy)
	}
	if len(spec.Cases) != 8 {
		t.Fatalf("benchmark cases = %d, want the documented 8", len(spec.Cases))
	}
	seen := map[string]bool{}
	for _, entry := range spec.Cases {
		if entry.ID == "" || seen[entry.ID] {
			t.Fatalf("benchmark case id %q is empty or duplicated", entry.ID)
		}
		seen[entry.ID] = true
		if entry.Prompt == "" || len(entry.Observe) < 2 {
			t.Fatalf("case %q needs a prompt and at least two observations", entry.ID)
		}
		if entry.BaseCase != "" && !seen[entry.BaseCase] {
			t.Fatalf("case %q references unknown base case %q", entry.ID, entry.BaseCase)
		}
	}
	if _, err := os.Stat("../../scripts/game-maker-benchmark.mjs"); err != nil {
		t.Fatalf("benchmark runner missing: %v", err)
	}
}
