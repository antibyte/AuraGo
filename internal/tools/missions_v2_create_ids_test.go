package tools

import (
	"fmt"
	"testing"
)

// A coarse clock (Windows) hands out the same UnixNano to back-to-back calls; a reused id used to
// replace the earlier mission silently.
func TestCreateNeverReusesAGeneratedMissionID(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	const n = 300
	seen := make(map[string]string, 2*n)
	for i := 0; i < n; i++ {
		// Flow missions draw from the same id space.
		flowID, err := mm.CreateFlowMission(fmt.Sprintf("flow_ids_%03d", i), "Flow")
		if err != nil {
			t.Fatalf("CreateFlowMission %d: %v", i, err)
		}
		name := fmt.Sprintf("m%03d", i)
		m := &MissionV2{Name: name, Prompt: "p", ExecutionType: ExecutionManual}
		if err := mm.Create(m); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		for _, id := range []string{flowID, m.ID} {
			if id == "" {
				t.Fatalf("round %d: an empty id", i)
			}
			if prev, dup := seen[id]; dup {
				t.Fatalf("round %d: id %s was handed out twice (%s)", i, id, prev)
			}
		}
		seen[flowID], seen[m.ID] = "flow", name
	}
	if got := len(mm.List()); got != 2*n {
		t.Fatalf("%d creates stored %d missions, want %d", 2*n, got, 2*n)
	}
	for id, name := range seen {
		got, ok := mm.Get(id)
		if !ok {
			t.Fatalf("mission %s is missing", id)
		}
		if name != "flow" && got.Name != name {
			t.Fatalf("mission %s holds %q, want %q", id, got.Name, name)
		}
	}
}

func TestCreateKeepsAnExplicitMissionID(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	m := &MissionV2{ID: "mission_explicit", Name: "x", Prompt: "p", ExecutionType: ExecutionManual}
	if err := mm.Create(m); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got, ok := mm.Get("mission_explicit"); !ok || got.Name != "x" || m.ID != "mission_explicit" {
		t.Fatalf("an explicit id must be kept: %+v", got)
	}
}
