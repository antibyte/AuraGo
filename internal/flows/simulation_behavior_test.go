package flows

import "testing"

// SimulationBehaviorOf is the one rule for what Simulation does with a step. The manuals
// are generated from it, so the whole table is pinned, not only the types that
// TestSimulationIsIsolated happens to use.
func TestSimulationBehaviorOf(t *testing.T) {
	cases := []struct {
		typ     string
		trigger bool
		want    SimulationBehavior
	}{
		{TypeIf, false, SimulationRunsLocally},
		{TypeSwitch, false, SimulationRunsLocally},
		{TypeMerge, false, SimulationRunsLocally},
		{TypeSet, false, SimulationRunsLocally},
		{TypeStop, false, SimulationRunsLocally},
		{blockCallType, false, SimulationRunsLocally},
		{blockEachType, false, SimulationRunsLocally},
		{TypeWait, false, SimulationNeedsSample},
		{TypeHTTPRequest, false, SimulationNeedsSample},
		{TypeAIStep, false, SimulationNeedsSample},
		{TypeTelegram, false, SimulationNeedsSample},
		{TypeFileWrite, false, SimulationNeedsSample},
		{durableWaitType, false, SimulationUnsupported},
		{durableApprovalType, false, SimulationUnsupported},
		{GenericTypePrefix + "docker", false, SimulationUnsupported},
		{TypeTriggerManual, true, SimulationSuppliesData},
		{TypeTriggerSchedule, true, SimulationSuppliesData},
	}
	for _, c := range cases {
		if got := SimulationBehaviorOf(c.typ, c.trigger); got != c.want {
			t.Errorf("%s (trigger %v): got %q, want %q", c.typ, c.trigger, got, c.want)
		}
	}

	// The rule and the catalog flag agree on what Simulate accepts.
	for _, info := range DescribeNodeTypes(catalogRegistry(t, fullEnv()), nil) {
		if info.Trigger {
			continue
		}
		if unsupported := SimulationBehaviorOf(info.Type, false) == SimulationUnsupported; unsupported == info.Simulatable {
			t.Errorf("%s: behaviour unsupported=%v but the catalog says simulatable=%v", info.Type, unsupported, info.Simulatable)
		}
	}
}
