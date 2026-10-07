package flows

import (
	"context"
	"errors"
	"testing"
)

func TestGetFlowByMission(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.CreateFlow(ctx, sampleFlow("flow_aaaaaaaaea"), "mission_7", storeNow); err != nil {
		t.Fatal(err)
	}
	rec, err := s.GetFlowByMission(ctx, "mission_7")
	if err != nil || rec.ID != "flow_aaaaaaaaea" {
		t.Fatalf("GetFlowByMission = %+v, %v", rec, err)
	}
	for _, id := range []string{"", "mission_8"} {
		if _, err := s.GetFlowByMission(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetFlowByMission(%q) = %v", id, err)
		}
	}
}
