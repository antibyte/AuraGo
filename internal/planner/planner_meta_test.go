package planner

import (
	"testing"
)

func TestPlannerMetaRoundTrip(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	if err := SetPlannerMeta(db, "kg_dropped_access_hits_last_recorded", "42"); err != nil {
		t.Fatalf("SetPlannerMeta: %v", err)
	}
	value, err := GetPlannerMeta(db, "kg_dropped_access_hits_last_recorded")
	if err != nil {
		t.Fatalf("GetPlannerMeta: %v", err)
	}
	if value != "42" {
		t.Fatalf("value = %q, want 42", value)
	}
}