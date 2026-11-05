package flows

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type kindProbeKeys struct{ calls int }

func (k *kindProbeKeys) ScenarioKey(context.Context, bool) ([]byte, error) {
	k.calls++
	return nil, errors.New("a wrong-kind operation must not request a key")
}

func kindStoreCounts(t *testing.T, s *Service) map[string]int {
	t.Helper()
	counts := make(map[string]int)
	for _, table := range []string{"flows", "flow_versions", "flow_runs", "flow_run_steps", "flow_timers", "flow_test_data", "flow_test_scenarios", "flow_block_version_pins", "flow_block_run_pins"} {
		var count int
		if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		counts[table] = count
	}
	return counts
}

func TestFlowServiceRejectsBlockIDsBeforeEffects(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Reusable boundary")
	if err != nil {
		t.Fatal(err)
	}
	block, _, err = s.PublishBlock(ctx, block.ID, block.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	forgedFlow := simpleFlow("Must not replace block")
	forgedFlow.ID = block.ID
	keys := &kindProbeKeys{}
	scenarioID := "sc_000000000000000000000001"
	before := kindStoreCounts(t, s)
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"import block", func() error { _, err := s.CreateFlow(ctx, CreateRequest{Import: block.Draft}); return err }},
		{"save forged flow", func() error { _, _, err := s.SaveDraft(ctx, block.ID, forgedFlow, block.DraftRevision); return err }},
		{"publish", func() error { _, _, err := s.Publish(ctx, block.ID, block.DraftRevision); return err }},
		{"enable", func() error { return s.SetEnabled(ctx, block.ID, true) }},
		{"disable", func() error { return s.SetEnabled(ctx, block.ID, false) }},
		{"delete", func() error { return s.DeleteFlow(ctx, block.ID) }},
		{"versions", func() error { _, err := s.ListVersions(ctx, block.ID); return err }},
		{"restore version", func() error { _, _, err := s.RestoreVersion(ctx, block.ID, 1, block.DraftRevision); return err }},
		{"real test with remembered data", func() error {
			_, err := s.StartTestRun(ctx, block.ID, TestRunRequest{BaseRevision: block.DraftRevision, TriggerNode: block.Draft.Nodes[0].ID, TriggerData: map[string]any{"sample": "must not persist"}, RememberData: true})
			return err
		}},
		{"live run", func() error { _, err := s.RunNow(ctx, block.ID); return err }},
		{"runs", func() error { _, err := s.Runs(ctx, block.ID, RunFilter{}); return err }},
		{"trigger sample read", func() error { _, err := s.TriggerSampleData(ctx, block.ID, block.Draft.Nodes[0].ID); return err }},
		{"trigger sample write", func() error {
			return s.SaveTriggerSample(ctx, block.ID, block.Draft.Nodes[0].ID, map[string]any{"sample": true})
		}},
		{"repair mission", func() error { return s.RepairMission(ctx, block.ID, block.DraftRevision, block.LiveRevision) }},
		{"scenario list", func() error { _, err := s.ListScenarios(ctx, block.ID, keys); return err }},
		{"scenario read", func() error { _, err := s.GetScenario(ctx, block.ID, scenarioID, keys); return err }},
		{"scenario save", func() error {
			_, err := s.SaveScenario(ctx, block.ID, block.DraftRevision, 0, Scenario{Name: "Private sample", TriggerNode: block.Draft.Nodes[0].ID, At: storeNow}, keys)
			return err
		}},
		{"scenario simulate", func() error {
			_, err := s.SimulateScenario(ctx, block.ID, scenarioID, block.DraftRevision, 1, keys)
			return err
		}},
		{"scenario delete", func() error { return s.DeleteScenario(ctx, block.ID, scenarioID, 1, keys) }},
		{"scenario rename", func() error { _, err := s.RenameScenario(ctx, block.ID, scenarioID, 1, "Renamed", keys); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, ErrWrongFlowKind) {
				t.Fatalf("wrong-kind operation = %v", err)
			}
		})
	}
	if keys.calls != 0 {
		t.Fatalf("wrong-kind operations requested Vault keys %d times", keys.calls)
	}
	if after := kindStoreCounts(t, s); !reflect.DeepEqual(before, after) {
		t.Fatalf("wrong-kind operation changed persisted rows: before=%v after=%v", before, after)
	}
	after, err := s.GetBlock(ctx, block.ID)
	if err != nil || !reflect.DeepEqual(block, after) {
		t.Fatalf("block record changed: %v", err)
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.next != 0 || len(bridge.missions) != 0 || len(bridge.started) != 0 || len(bridge.finished) != 0 {
		t.Fatal("wrong-kind operations reached Mission Control")
	}
}

func TestBlockServiceRejectsForgedFlowTargets(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	flow, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Regular flow")})
	if err != nil {
		t.Fatal(err)
	}
	flow, _, err = s.Publish(ctx, flow.ID, flow.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	block, err := s.CreateBlock(ctx, "Other document kind")
	if err != nil {
		t.Fatal(err)
	}
	forged, err := block.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	forged.ID = flow.ID
	before := kindStoreCounts(t, s)
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"save correct block shape to flow row", func() error { _, _, err := s.SaveBlockDraft(ctx, flow.ID, forged, flow.DraftRevision); return err }},
		{"preview", func() error { _, err := s.BlockPublishPreview(ctx, flow.ID); return err }},
		{"publish", func() error { _, _, err := s.PublishBlock(ctx, flow.ID, flow.DraftRevision); return err }},
		{"versions", func() error { _, err := s.BlockVersions(ctx, flow.ID, 0, 10); return err }},
		{"published version", func() error { _, err := s.GetBlockVersion(ctx, flow.ID, flow.LiveRevision); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, ErrWrongFlowKind) {
				t.Fatalf("wrong-kind operation = %v", err)
			}
		})
	}
	if after := kindStoreCounts(t, s); !reflect.DeepEqual(before, after) {
		t.Fatalf("wrong-kind operation changed persisted rows: before=%v after=%v", before, after)
	}
	after, err := s.GetFlow(ctx, flow.ID)
	if err != nil || !reflect.DeepEqual(flow, after) {
		t.Fatalf("flow record changed: %v", err)
	}
}

func TestBlockMissionCorruptionCannotStartCancelOrDelete(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Corrupt mission fixture")
	if err != nil {
		t.Fatal(err)
	}
	const missionID = "mission_foreign_block"
	if _, err := s.store.db.Exec(`UPDATE flows SET mission_id=? WHERE id=?`, missionID, block.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.Exec(`INSERT INTO flow_timers(flow_id,node_id,fire_at) VALUES(?,?,?)`, block.ID, block.Draft.Nodes[0].ID, formatTime(storeNow.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	const runID = "run_wrong_kind_fixture"
	if _, err := s.store.db.Exec(`INSERT INTO flow_runs(id,flow_id,mode,status,started_at) VALUES(?,?,?,?,?)`, runID, block.ID, "test", "success", formatTime(storeNow)); err != nil {
		t.Fatal(err)
	}
	before := kindStoreCounts(t, s)
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"run detail", func() error { _, err := s.Run(ctx, runID, true); return err }},
		{"run header", func() error { _, err := s.RunHeader(ctx, runID); return err }},
		{"store mission lookup", func() error { _, err := s.store.GetFlowByMission(ctx, missionID); return err }},
		{"mission start", func() error { _, err := s.TriggerFromMission(missionID, "", "manual", nil); return err }},
		{"mission cancel", func() error { _, err := s.CancelMissionRuns(ctx, missionID); return err }},
		{"mission enable callback", func() error { return s.MissionEnabledChanged(ctx, missionID) }},
		{"mission delete callback", func() error { return s.DeleteFlowForMission(ctx, missionID) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, ErrWrongFlowKind) {
				t.Fatalf("corrupt block mission accepted: %v", err)
			}
		})
	}
	if at, ok := s.NextTimer(ctx, missionID); ok || !at.IsZero() {
		t.Fatalf("block advertised a mission timer: %s", at)
	}
	if after := kindStoreCounts(t, s); !reflect.DeepEqual(before, after) {
		t.Fatalf("rejected mission hooks changed state: before=%v after=%v", before, after)
	}
	if len(bridge.startedRuns()) != 0 {
		t.Fatal("corrupt block started a mission")
	}
	regular, err := s.CreateFlow(ctx, CreateRequest{Name: "Legitimate owner"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.Exec(`UPDATE flows SET mission_id=? WHERE id=?`, regular.MissionID, block.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.GetFlowByMission(ctx, regular.MissionID); !errors.Is(err, ErrMissionAmbiguous) {
		t.Fatalf("corrupt second owner was hidden from mission lookup: %v", err)
	}
	if _, err := s.store.flowIDByMission(ctx, regular.MissionID); !errors.Is(err, ErrMissionAmbiguous) {
		t.Fatalf("corrupt second owner was hidden from timer lookup: %v", err)
	}
}

func TestKindMismatchCannotBeSilentlyOverwrittenBySaving(t *testing.T) {
	for _, kind := range []Kind{KindFlow, KindBlock} {
		for _, column := range []string{"draft_json", "live_json"} {
			t.Run(string(kind)+"/"+column, func(t *testing.T) {
				s, _ := newServiceFixture(t, nil)
				ctx := context.Background()
				var rec *FlowRecord
				var err error
				if kind == KindFlow {
					rec, err = s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Original flow")})
				} else {
					rec, err = s.CreateBlock(ctx, "Original block")
				}
				if err != nil {
					t.Fatal(err)
				}
				if kind == KindFlow {
					rec, _, err = s.Publish(ctx, rec.ID, rec.DraftRevision)
				} else {
					rec, _, err = s.PublishBlock(ctx, rec.ID, rec.DraftRevision)
				}
				if err != nil {
					t.Fatal(err)
				}
				corrupt, err := rec.Draft.Clone()
				if err != nil {
					t.Fatal(err)
				}
				corrupt.Kind = KindBlock
				if kind == KindBlock {
					corrupt.Kind = KindFlow
				}
				badJSON, err := corrupt.Marshal()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.store.db.Exec(`UPDATE flows SET `+column+`=? WHERE id=?`, string(badJSON), rec.ID); err != nil {
					t.Fatal(err)
				}
				if kind == KindFlow {
					_, _, err = s.SaveDraft(ctx, rec.ID, rec.Draft, rec.DraftRevision)
				} else {
					_, _, err = s.SaveBlockDraft(ctx, rec.ID, rec.Draft, rec.DraftRevision)
				}
				if !errors.Is(err, ErrWrongFlowKind) {
					t.Fatalf("save silently accepted inconsistent stored kind: %v", err)
				}
				var draftRevision int
				var unchanged string
				if err := s.store.db.QueryRow(`SELECT draft_revision,`+column+` FROM flows WHERE id=?`, rec.ID).Scan(&draftRevision, &unchanged); err != nil {
					t.Fatal(err)
				}
				if draftRevision != rec.DraftRevision || unchanged != string(badJSON) {
					t.Fatal("failed save changed the original inconsistent record")
				}
			})
		}
	}
}
