package gamemaker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestControlsCheckFailsOnlyOnSwappedDirection(t *testing.T) {
	t.Parallel()
	scenario, ok := controlsScenario("platformer", "")
	if !ok {
		t.Fatal("2D bases must receive the controls check")
	}
	observe := func(beforeRight [2]float64, dx, dy float64, withBasis bool) CheckResult {
		before := map[string]float64{"player_x": 10, "player_y": 20}
		if withBasis {
			before["view_right_x"], before["view_right_y"] = beforeRight[0], beforeRight[1]
		}
		after := map[string]float64{"player_x": 10 + dx, "player_y": 20 + dy}
		return compareGameObservations([]GameScenario{scenario}, []GameObservation{{ID: scenario.ID, Before: before, After: after}})[0]
	}
	for name, tc := range map[string]struct {
		right  [2]float64
		dx, dy float64
		basis  bool
		status string
	}{
		"moves right":                 {right: [2]float64{1, 0}, dx: 96, status: "passed"},
		"no horizontal movement":      {right: [2]float64{1, 0}, dy: -40, status: "passed"},
		"swapped":                     {right: [2]float64{1, 0}, dx: -96, status: "failed"},
		"3d camera right is world -x": {right: [2]float64{-1, 0}, dx: -2, status: "passed"},
		"3d swapped world +x":         {right: [2]float64{-1, 0}, dx: 2, status: "failed"},
		"missing basis":               {dx: 96, status: "unavailable"},
	} {
		tc.basis = name != "missing basis"
		got := observe(tc.right, tc.dx, tc.dy, tc.basis)
		if got.Status != tc.status {
			t.Errorf("%s: status %q, want %q (%s)", name, got.Status, tc.status, got.Observed)
		}
		if tc.status == "failed" && !strings.Contains(got.Observed, "camera's right vector") {
			t.Errorf("%s: failure lacks the repair hint: %s", name, got.Observed)
		}
	}
}

func TestControlsCheckRequiresCapableHelper(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		template, version, key string
		ok                     bool
	}{
		{"topdown", "", "RIGHT", true},
		{"board", "phaser-2", "RIGHT", true},
		{"exploration", "three-4", "D", true},
		{"fps", "three-5", "D", true},
		{"exploration", "three-3", "", false},
		{"transport", "", "", false},
		{"three", "three-4", "", false},
	} {
		scenario, ok := controlsScenario(tc.template, tc.version)
		if ok != tc.ok || ok && (scenario.Steps[0].Key != tc.key || scenario.Compare != "not_decreased") {
			t.Errorf("%s/%s: ok=%v scenario=%+v", tc.template, tc.version, ok, scenario)
		}
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o750); err != nil {
		t.Fatal(err)
	}
	common, err := gameTemplates.ReadFile("templates/three-common.ts")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "common.ts"), common, 0o640); err != nil {
		t.Fatal(err)
	}
	if got := installedRuntimeVersion(dir); !threeRuntimeAtLeast(got, 4) {
		t.Fatalf("bundled 3D helper reports %q; camera-relative controls need three-4 or newer", got)
	}
}

func TestControlsCheckStaysBeforePlanScenarios(t *testing.T) {
	t.Parallel()
	controls, _ := controlsScenario("shooter", "")
	got := withRequiredScenario([]GameScenario{{ID: "required_input"}, {ID: "required_end"}, {ID: "laser_hits"}}, controls)
	ids := []string{}
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	if strings.Join(ids, ",") != "required_input,required_end,required_controls,laser_hits" {
		t.Fatalf("order = %v", ids)
	}
}
