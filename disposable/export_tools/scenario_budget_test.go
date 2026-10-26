package main

import "testing"

func TestScenarioBudgetPreservesBilingualCoverageAndPackSize(t *testing.T) {
	for _, operations := range []int{1125, 1127, 1375, 1380, 1388, 1390, 1396, 1400, 1402, 1405} {
		direct, multi, err := scenarioOperationBudget(operations)
		if err != nil || direct < operations*2 || multi < 500 || direct+multi+1750 != targetScenarioCount {
			t.Fatalf("operations=%d: direct=%d multi=%d err=%v", operations, direct, multi, err)
		}
	}
	if _, _, err := scenarioOperationBudget(1406); err == nil {
		t.Fatal("oversized catalogs require an explicit review of the scenario mix")
	}
}
