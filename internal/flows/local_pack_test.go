package flows

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/security"
)

func localPackPlainFlow(t *testing.T, s *Service) *FlowRecord {
	t.Helper()
	const secret = "local-pack-private-value-1234"
	security.RegisterSensitive(secret)
	b := newFlow("Local recipe")
	start := b.node("start", TypeTriggerManual, map[string]any{"data": map[string]any{"name": "sample"}})
	result := b.node("result", TypeSet, map[string]any{"keep_input": false, "fields": []any{
		map[string]any{"name": "greeting", "type": "text", "value": "Hello {{trigger.data.name}}"},
	}})
	b.edge(start, PortOut, result)
	doc := b.build()
	doc.Nodes[1].Position.X = 12.5
	doc.Description = "Private note: " + secret
	rec, err := s.CreateFlow(context.Background(), CreateRequest{Import: doc})
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

type localPackPanicClock struct {
	mu       sync.Mutex
	calls    int
	panicAt  int
	panicVal any
}

func (c *localPackPanicClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.calls == c.panicAt {
		panic(c.panicVal)
	}
	return time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)
}

func (*localPackPanicClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

func localPackDBCounts(t *testing.T, db *sql.DB) [5]int {
	t.Helper()
	var counts [5]int
	queries := []string{
		`SELECT count(*) FROM flows`,
		`SELECT count(*) FROM flow_versions`,
		`SELECT count(*) FROM flow_block_version_pins`,
		`SELECT count(*) FROM flow_block_run_pins`,
		`SELECT count(*) FROM flow_runs`,
	}
	for i, query := range queries {
		if err := db.QueryRow(query).Scan(&counts[i]); err != nil {
			t.Fatal(err)
		}
	}
	return counts
}

func localPackMissionCount(bridge *fakeBridge) int {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	return len(bridge.missions)
}

func TestLocalPackRoundTripWithoutBlocks(t *testing.T) {
	ctx := context.Background()
	s, bridge := newServiceFixture(t, nil)
	source := localPackPlainFlow(t, s)
	before, err := s.Simulate(ctx, source.ID, SimulationRequest{BaseRevision: source.DraftRevision, TriggerData: map[string]any{"name": "Ada"}})
	if err != nil || before.Status != RunSuccess {
		t.Fatalf("source simulation: %+v, %v", before, err)
	}
	storedBefore, err := s.GetFlow(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	storedBeforeBytes, err := storedBefore.Draft.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	countsBefore := localPackDBCounts(t, s.store.db)
	missionsBefore := localPackMissionCount(bridge)

	pack, err := s.ExportLocalPack(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Format != LocalPackFormat || pack.Version != LocalPackVersion || pack.Blocks == nil || len(pack.Blocks) != 0 {
		t.Fatalf("unexpected simple pack envelope: %+v", pack)
	}
	if !manualSamplesCleared(pack.Parent.Flow) {
		t.Fatal("export retained manual trigger sample data")
	}
	if strings.Contains(pack.Parent.Flow.Description, "local-pack-private-value-1234") {
		t.Fatal("export retained a registered secret")
	}
	if got, hashErr := localPackHash(pack.Parent.Flow); hashErr != nil || got != pack.Parent.Hash {
		t.Fatalf("sanitized parent hash does not match document: %q %v", got, hashErr)
	}
	storedAfter, err := s.GetFlow(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	storedAfterBytes, err := storedAfter.Draft.Marshal()
	if err != nil || string(storedBeforeBytes) != string(storedAfterBytes) {
		t.Fatalf("export mutated source draft: %v", err)
	}

	preview, err := s.PreviewLocalPack(ctx, pack)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Proposal.Blocks == nil || preview.Review.Blocks == nil || !preview.Review.SampleDataCleared || !preview.Review.LocalOnly {
		t.Fatalf("preview did not return explicit empty lists/review: %+v", preview)
	}
	if got := localPackDBCounts(t, s.store.db); got != countsBefore {
		t.Fatalf("preview wrote SQL state: before=%v after=%v", countsBefore, got)
	}
	if got := localPackMissionCount(bridge); got != missionsBefore {
		t.Fatalf("preview created a mission: before=%d after=%d", missionsBefore, got)
	}

	// A normal JSON parse/re-encode reorders object keys and converts JSON numbers
	// through float64. Semantic Flow hashes must remain stable across that trip.
	encoded, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	var generic any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		t.Fatal(err)
	}
	reordered, err := json.Marshal(generic)
	if err != nil {
		t.Fatal(err)
	}
	var roundTripped LocalPack
	if err := json.Unmarshal(reordered, &roundTripped); err != nil {
		t.Fatal(err)
	}
	preview, err = s.PreviewLocalPack(ctx, roundTripped)
	if err != nil {
		t.Fatalf("JSON-reencoded pack preview: %v", err)
	}
	if preview.Proposal.PackDigest == "" {
		t.Fatal("preview omitted pack digest")
	}
	result, err := s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: roundTripped, Proposal: preview.Proposal})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "applied" || result.Flow == nil || result.Flow.ID == source.ID || result.Flow.LiveRevision != 0 || result.Flow.DraftRevision != 1 || result.Blocks == nil || len(result.Blocks) != 0 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if bridge.FlowMissionState(result.Flow.MissionID, result.Flow.ID) != MissionStateDisabled {
		t.Fatal("imported mission was not left disabled")
	}
	after, err := s.Simulate(ctx, result.Flow.ID, SimulationRequest{BaseRevision: result.Flow.DraftRevision, TriggerData: map[string]any{"name": "Ada"}})
	if err != nil || after.Status != RunSuccess {
		t.Fatalf("import simulation: %+v, %v", after, err)
	}
	if got := after.Steps[len(after.Steps)-1].Output["greeting"]; got != before.Steps[len(before.Steps)-1].Output["greeting"] {
		t.Fatalf("import changed behavior: before=%v after=%v", before.Steps, after.Steps)
	}
	confirmed, err := s.ReconcileLocalPack(ctx, LocalPackReconcileRequest{Proposal: preview.Proposal})
	if err != nil || confirmed.Status != "applied_state_confirmed" || confirmed.Flow == nil || confirmed.Blocks == nil || len(confirmed.Blocks) != 0 {
		t.Fatalf("reconcile: %+v %v", confirmed, err)
	}
	reconcileJSON, err := json.Marshal(confirmed)
	if err != nil {
		t.Fatal(err)
	}
	var reconcileWire map[string]json.RawMessage
	if err := json.Unmarshal(reconcileJSON, &reconcileWire); err != nil || string(reconcileWire["blocks"]) != "[]" {
		t.Fatalf("confirmed empty closure must serialize blocks as [], got %s (%v)", reconcileJSON, err)
	}
	countsAfter, err := s.store.db.QueryContext(ctx, `SELECT count(*) FROM flow_runs`)
	if err != nil {
		t.Fatal(err)
	}
	defer countsAfter.Close()
	var runs int
	if !countsAfter.Next() || countsAfter.Scan(&runs) != nil || runs != 0 {
		t.Fatalf("local pack simulation persisted runtime runs: %d", runs)
	}
}

func TestLocalPackExportScrubsRegisteredSecretFromUnknownParamKey(t *testing.T) {
	const secret = "local-pack-param-key-secret-5678"
	security.RegisterSensitive(secret)
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	source := localPackPlainFlow(t, s)
	draft, err := source.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	key := "api_key=" + secret
	draft.Nodes[1].Params[key] = "opaque"
	revision, _, err := s.SaveDraft(ctx, source.ID, draft, source.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	storedBefore, err := s.GetFlow(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	before, err := storedBefore.Draft.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	pack, err := s.ExportLocalPack(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, hashErr := localPackHash(pack.Parent.Flow); hashErr != nil || got != pack.Parent.Hash {
		t.Fatalf("scrubbed document hash mismatch: %q %v", got, hashErr)
	}
	wire, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), secret) {
		t.Fatal("export leaked a registered secret from a top-level parameter key")
	}
	cleanKey := security.ScrubSecrets(key)
	if cleanKey == key || pack.Parent.Flow.Nodes[1].Params[cleanKey] != "opaque" {
		t.Fatalf("export did not scrub the parameter key: %#v", pack.Parent.Flow.Nodes[1].Params)
	}
	if _, err := s.PreviewLocalPack(ctx, pack); err != nil {
		t.Fatalf("preview rejected a redacted pack: %v", err)
	}
	storedAfter, err := s.GetFlow(ctx, source.ID)
	if err != nil || storedAfter.DraftRevision != revision {
		t.Fatalf("source revision changed after export: %+v %v", storedAfter, err)
	}
	after, err := storedAfter.Draft.Marshal()
	if err != nil || string(before) != string(after) {
		t.Fatalf("export mutated stored param key or draft: %v", err)
	}
}

func localPackNestedRevisionSource(t *testing.T, s *Service) (*FlowRecord, *FlowRecord, *FlowRecord, *FlowRecord) {
	t.Helper()
	ctx := context.Background()
	leaf1 := publishedGreetingBlock(t, s)
	leafDraft, err := leaf1.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	leafDraft.Nodes[1].Params["values"] = map[string]any{"greeting": "Bonjour {{block_input.data.name}}"}
	rev, issues, err := s.SaveBlockDraft(ctx, leaf1.ID, leafDraft, leaf1.DraftRevision)
	if err != nil {
		t.Fatalf("save leaf revision 2: %v %+v", err, issues)
	}
	leaf2, issues, err := s.PublishBlock(ctx, leaf1.ID, rev)
	if err != nil {
		t.Fatalf("publish leaf revision 2: %v %+v", err, issues)
	}

	middle1 := nestedGreeting(t, s, leaf1)
	middleDraft, err := middle1.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	for i := range middleDraft.Nodes {
		if middleDraft.Nodes[i].Type == blockCallType {
			middleDraft.Nodes[i].Params["revision"] = float64(leaf2.LiveRevision)
		}
	}
	middleRev, issues, err := s.SaveBlockDraft(ctx, middle1.ID, middleDraft, middle1.DraftRevision)
	if err != nil {
		t.Fatalf("save middle revision 2: %v %+v", err, issues)
	}
	middle2, issues, err := s.PublishBlock(ctx, middle1.ID, middleRev)
	if err != nil {
		t.Fatalf("publish middle revision 2: %v %+v", err, issues)
	}
	return leaf1, leaf2, middle1, middle2
}

func TestLocalPackNestedTwoSourceRevisionsImportAndSimulate(t *testing.T) {
	ctx := context.Background()
	source, _ := newServiceFixture(t, nil)
	target, targetBridge := newServiceFixture(t, nil)
	leaf1, leaf2, middle1, middle2 := localPackNestedRevisionSource(t, source)
	b := newFlow("Nested revision recipe")
	start := b.node("start", TypeTriggerManual, nil)
	first := b.node("v1", blockCallType, map[string]any{"flow_id": middle1.ID, "revision": float64(middle1.LiveRevision), "inputs": map[string]any{"name": "Ada"}})
	second := b.node("v2", blockCallType, map[string]any{"flow_id": middle2.ID, "revision": float64(middle2.LiveRevision), "inputs": map[string]any{"name": "Ada"}})
	literal := b.node("literal", TypeSet, map[string]any{"keep_input": false, "fields": []any{
		map[string]any{"name": "source_id", "type": "text", "value": leaf1.ID},
		map[string]any{"name": "expression", "type": "text", "value": "{{trigger.data.name}}"},
	}})
	b.edge(start, PortOut, first)
	b.edge(first, PortOut, second)
	b.edge(second, PortOut, literal)
	doc := b.build()
	groups := []VisualGroup{{ID: "g_abcdefgh", Title: "Retain group", Color: "blue", Members: []string{first}}}
	doc.Groups = &groups
	parent, err := source.CreateFlow(ctx, CreateRequest{Import: doc})
	if err != nil {
		t.Fatal(err)
	}
	sourceSim, err := source.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision, TriggerData: map[string]any{"name": "Ada"}})
	if err != nil || sourceSim.Status != RunSuccess {
		t.Fatalf("source nested simulation: %+v %v", sourceSim, err)
	}
	pack, err := source.ExportLocalPack(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Blocks) != 4 {
		t.Fatalf("wanted leaf+middle revisions in closure, got %d blocks: %+v", len(pack.Blocks), pack.Blocks)
	}
	seenSource := make(map[blockRef]bool)
	for _, block := range pack.Blocks {
		seenSource[blockRef{FlowID: block.SourceID, Revision: block.SourceRevision}] = true
	}
	for _, ref := range []blockRef{{FlowID: leaf1.ID, Revision: leaf1.LiveRevision}, {FlowID: leaf2.ID, Revision: leaf2.LiveRevision}, {FlowID: middle1.ID, Revision: middle1.LiveRevision}, {FlowID: middle2.ID, Revision: middle2.LiveRevision}} {
		if !seenSource[ref] {
			t.Fatalf("export omitted source revision %+v", ref)
		}
	}
	if got := localPackDBCounts(t, target.store.db); got != ([5]int{}) || localPackMissionCount(targetBridge) != 0 {
		t.Fatalf("target store was not empty before transfer: rows=%v missions=%d", got, localPackMissionCount(targetBridge))
	}
	preview, err := target.PreviewLocalPack(ctx, pack)
	if err != nil {
		t.Fatal(err)
	}
	result, err := target.ImportLocalPack(ctx, LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Blocks) != 4 || result.Blocks == nil || len(result.Flow.Pins) != 0 {
		t.Fatalf("unexpected imported records: %+v", result)
	}
	sourceIDs := map[string]bool{parent.ID: true, leaf1.ID: true, middle1.ID: true}
	for _, block := range pack.Blocks {
		sourceIDs[block.SourceID] = true
	}
	localBySource := make(map[blockRef]string)
	localIDs := make(map[string]bool)
	for _, target := range preview.Proposal.Blocks {
		if sourceIDs[target.LocalID] || target.LocalID == parent.ID || localIDs[target.LocalID] {
			t.Fatalf("local identifier reused a source/target identity: %+v", target)
		}
		localIDs[target.LocalID] = true
		localBySource[blockRef{FlowID: target.SourceID, Revision: target.SourceRevision}] = target.LocalID
	}
	if localBySource[blockRef{FlowID: middle1.ID, Revision: middle1.LiveRevision}] == localBySource[blockRef{FlowID: middle2.ID, Revision: middle2.LiveRevision}] {
		t.Fatal("two revisions of one source block collapsed to one local block")
	}
	for _, block := range result.Blocks {
		if block.Kind != KindBlock || block.LiveRevision != 1 || block.DraftRevision != 1 {
			t.Fatalf("imported block is not an immutable v1: %+v", block)
		}
	}
	var simInvoked bool
	definition, _ := target.reg.Lookup(blockCallType)
	if definition != nil {
		fake := *definition
		fake.Execute = func(context.Context, ExecInput) (ExecResult, error) { simInvoked = true; return ExecResult{}, nil }
		target.reg.Replace(&fake)
	}
	importedSim, err := target.Simulate(ctx, result.Flow.ID, SimulationRequest{BaseRevision: result.Flow.DraftRevision, TriggerData: map[string]any{"name": "Ada"}})
	if err != nil || importedSim.Status != RunSuccess {
		t.Fatalf("imported nested simulation: %+v %v", importedSim, err)
	}
	outputs := map[string]any{}
	for _, step := range importedSim.Steps {
		outputs[step.NodeID] = step.Output["greeting"]
	}
	if outputs[first] != "Hello Ada" || outputs[second] != "Bonjour Ada" {
		t.Fatalf("nested behavior changed: source=%v imported=%v", sourceSim.Steps, importedSim.Steps)
	}
	importedParent, err := target.GetFlow(ctx, result.Flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if importedParent.Draft.NodeByID(literal) == nil || importedParent.Draft.NodeByID(literal).Params["fields"].([]any)[0].(map[string]any)["value"] != leaf1.ID ||
		importedParent.Draft.NodeByID(literal).Params["fields"].([]any)[1].(map[string]any)["value"] != "{{trigger.data.name}}" {
		t.Fatal("import rewrote a literal source ID or ordinary expression")
	}
	if importedParent.Draft.Groups == nil || len(*importedParent.Draft.Groups) != 1 || (*importedParent.Draft.Groups)[0].ID != "g_abcdefgh" || (*importedParent.Draft.Groups)[0].Members[0] != first {
		t.Fatalf("import rewrote frame-local group identity: %+v", importedParent.Draft.Groups)
	}
	importedMiddle, err := target.GetBlock(ctx, localBySource[blockRef{FlowID: middle1.ID, Revision: middle1.LiveRevision}])
	if err != nil {
		t.Fatal(err)
	}
	var remappedCall bool
	for _, node := range importedMiddle.Draft.Nodes {
		if node.Type == blockCallType {
			remappedCall = node.Params["flow_id"] == localBySource[blockRef{FlowID: leaf1.ID, Revision: leaf1.LiveRevision}] && node.Params["revision"] == float64(1)
		}
	}
	if !remappedCall {
		t.Fatal("typed block call reference was not remapped to the imported revision 1")
	}
	if simInvoked {
		t.Fatal("pure block simulation invoked the block node registry")
	}
	if targetBridge.FlowMissionState(result.Flow.MissionID, result.Flow.ID) != MissionStateDisabled || len(targetBridge.startedRuns()) != 0 {
		t.Fatal("import activated or ran the mission")
	}
	confirmed, err := target.ReconcileLocalPack(ctx, LocalPackReconcileRequest{Proposal: preview.Proposal})
	if err != nil || confirmed.Status != "applied_state_confirmed" || len(confirmed.Blocks) != 4 {
		t.Fatalf("nested reconciliation: %+v %v", confirmed, err)
	}
	pinCount := 0
	for _, block := range confirmed.Blocks {
		pinCount += len(block.Pins)
	}
	if pinCount != 2 {
		t.Fatalf("reconciliation returned %d exact nested pins, want 2: %+v", pinCount, confirmed.Blocks)
	}
	var runs int
	if err := target.store.db.QueryRow(`SELECT count(*) FROM flow_runs`).Scan(&runs); err != nil || runs != 0 {
		t.Fatalf("simulation persisted a live run: %d %v", runs, err)
	}
}

func TestLocalPackRejectsTamperedFreshIdentifiersBeforeMissionCreate(t *testing.T) {
	ctx := context.Background()
	s, bridge := newServiceFixture(t, nil)
	leaf1, _, middle1, _ := localPackNestedRevisionSource(t, s)
	parent, _ := blockParent(t, s, middle1, "Source parent")
	pack, err := s.ExportLocalPack(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewLocalPack(ctx, pack)
	if err != nil {
		t.Fatal(err)
	}
	baselineMissions := localPackMissionCount(bridge)
	for name, mutate := range map[string]func(*LocalPackProposal){
		"parent reuses source parent":   func(p *LocalPackProposal) { p.ParentID = parent.ID },
		"parent reuses source child":    func(p *LocalPackProposal) { p.ParentID = leaf1.ID },
		"block reuses different source": func(p *LocalPackProposal) { p.Blocks[0].LocalID = middle1.ID },
		"duplicate local block":         func(p *LocalPackProposal) { p.Blocks[1].LocalID = p.Blocks[0].LocalID },
	} {
		t.Run(name, func(t *testing.T) {
			proposal := preview.Proposal
			proposal.Blocks = append([]LocalPackBlockTarget(nil), preview.Proposal.Blocks...)
			mutate(&proposal)
			proposal.Digest, err = localPackProposalDigest(proposal)
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: pack, Proposal: proposal})
			if !errors.Is(err, ErrLocalPackInvalid) {
				t.Fatalf("tampered identifier accepted: %v", err)
			}
			if got := localPackMissionCount(bridge); got != baselineMissions {
				t.Fatalf("invalid proposal created mission: %d != %d", got, baselineMissions)
			}
		})
	}
}

func localPackFailTrigger(t *testing.T, db *sql.DB, table, column, value string) {
	t.Helper()
	name := "fail_local_pack_" + strings.ToLower(strings.ReplaceAll(table+column, "_", ""))
	query := fmt.Sprintf("CREATE TRIGGER %s BEFORE INSERT ON %s WHEN NEW.%s = '%s' BEGIN SELECT RAISE(ABORT, 'injected local pack failure'); END", name, table, column, value)
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
}

func TestLocalPackImportRollsBackEveryInsertBoundaryAndCleansExactMission(t *testing.T) {
	for _, tc := range []struct {
		name, table, column string
		value               func(LocalPackProposal) string
	}{
		{name: "parent", table: "flows", column: "id", value: func(p LocalPackProposal) string { return p.ParentID }},
		{name: "block", table: "flows", column: "id", value: func(p LocalPackProposal) string { return p.Blocks[0].LocalID }},
		{name: "version", table: "flow_versions", column: "flow_id", value: func(p LocalPackProposal) string { return p.Blocks[0].LocalID }},
		{name: "pin", table: "flow_block_version_pins", column: "parent_flow_id", value: func(p LocalPackProposal) string { return p.Blocks[len(p.Blocks)-1].LocalID }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s, bridge := newServiceFixture(t, nil)
			leaf1, _, middle1, _ := localPackNestedRevisionSource(t, s)
			parent, _ := blockParent(t, s, middle1, "Rollback parent")
			pack, err := s.ExportLocalPack(ctx, parent.ID)
			if err != nil {
				t.Fatal(err)
			}
			preview, err := s.PreviewLocalPack(ctx, pack)
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "pin" {
				// The last dependency-first target is a parent block with descendants.
				target := preview.Proposal.Blocks[len(preview.Proposal.Blocks)-1]
				if target.SourceID == leaf1.ID {
					t.Fatalf("selected pin target is not a nested block: %+v", target)
				}
			}
			countsBefore := localPackDBCounts(t, s.store.db)
			missionsBefore := localPackMissionCount(bridge)
			localPackFailTrigger(t, s.store.db, tc.table, tc.column, tc.value(preview.Proposal))
			_, err = s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal})
			if err == nil {
				t.Fatal("injected SQL failure unexpectedly imported the pack")
			}
			if got := localPackDBCounts(t, s.store.db); got != countsBefore {
				t.Fatalf("transaction left partial records: before=%v after=%v err=%v", countsBefore, got, err)
			}
			if got := localPackMissionCount(bridge); got != missionsBefore {
				t.Fatalf("exact-owner cleanup did not remove created mission: %d != %d", got, missionsBefore)
			}
		})
	}
}

func TestLocalPackFailureAndLostReplyReconcile(t *testing.T) {
	t.Run("mission create failure", func(t *testing.T) {
		ctx := context.Background()
		s, bridge := newServiceFixture(t, nil)
		source := localPackPlainFlow(t, s)
		pack, err := s.ExportLocalPack(ctx, source.ID)
		if err != nil {
			t.Fatal(err)
		}
		preview, err := s.PreviewLocalPack(ctx, pack)
		if err != nil {
			t.Fatal(err)
		}
		countsBefore := localPackDBCounts(t, s.store.db)
		missionsBefore := localPackMissionCount(bridge)
		bridge.failCreate = errors.New("manager unavailable")
		if _, err := s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal}); err == nil {
			t.Fatal("mission create failure ignored")
		}
		if got := localPackDBCounts(t, s.store.db); got != countsBefore || localPackMissionCount(bridge) != missionsBefore {
			t.Fatalf("mission create failure mutated state: db=%v missions=%d", got, localPackMissionCount(bridge))
		}
	})

	t.Run("cleanup refusal leaves unknown disabled orphan", func(t *testing.T) {
		ctx := context.Background()
		s, bridge := newServiceFixture(t, nil)
		source := localPackPlainFlow(t, s)
		pack, err := s.ExportLocalPack(ctx, source.ID)
		if err != nil {
			t.Fatal(err)
		}
		preview, err := s.PreviewLocalPack(ctx, pack)
		if err != nil {
			t.Fatal(err)
		}
		localPackFailTrigger(t, s.store.db, "flows", "id", preview.Proposal.ParentID)
		bridge.failCleanup = errors.New("cleanup persistence failed")
		if _, err := s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal}); err == nil || !strings.Contains(err.Error(), "cleanup persistence failed") {
			t.Fatalf("cleanup failure not reported: %v", err)
		}
		if bridge.FlowMissionState("mission_2", preview.Proposal.ParentID) != MissionStateDisabled {
			t.Fatal("expected retained disabled mission orphan")
		}
		unknown, err := s.ReconcileLocalPack(ctx, LocalPackReconcileRequest{Proposal: preview.Proposal})
		if err != nil || unknown.Status != "unknown" || unknown.Flow != nil || len(unknown.Blocks) != 0 {
			t.Fatalf("orphan was incorrectly reconciled as imported: %+v %v", unknown, err)
		}
	})

	t.Run("deferred foreign key commit failure retains disabled mission", func(t *testing.T) {
		ctx := context.Background()
		s, bridge := newServiceFixture(t, nil)
		source := localPackPlainFlow(t, s)
		pack, err := s.ExportLocalPack(ctx, source.ID)
		if err != nil {
			t.Fatal(err)
		}
		preview, err := s.PreviewLocalPack(ctx, pack)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.store.db.Exec(`CREATE TABLE local_pack_commit_fault (child_id TEXT NOT NULL REFERENCES flows(id) DEFERRABLE INITIALLY DEFERRED)`); err != nil {
			t.Fatal(err)
		}
		trigger := fmt.Sprintf(`CREATE TRIGGER defer_local_pack_failure AFTER INSERT ON flows WHEN NEW.id='%s' BEGIN INSERT INTO local_pack_commit_fault(child_id) VALUES('flow_missing_commit_target'); END`, preview.Proposal.ParentID)
		if _, err := s.store.db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		countsBefore := localPackDBCounts(t, s.store.db)
		missionCountBefore := localPackMissionCount(bridge)
		_, err = s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal})
		if err == nil {
			t.Fatal("deferred foreign-key violation did not fail transaction commit")
		}
		if got := localPackDBCounts(t, s.store.db); got != countsBefore {
			t.Fatalf("failed commit left partial pack rows: before=%v after=%v err=%v", countsBefore, got, err)
		}
		if got := localPackMissionCount(bridge); got != missionCountBefore+1 {
			t.Fatalf("commit uncertainty did not retain exactly one disabled mission: before=%d after=%d", missionCountBefore, got)
		}
		bridge.mu.Lock()
		missionID := ""
		for id, mission := range bridge.missions {
			if mission.flowID == preview.Proposal.ParentID {
				missionID = id
				break
			}
		}
		bridge.mu.Unlock()
		if missionID == "" || bridge.FlowMissionState(missionID, preview.Proposal.ParentID) != MissionStateDisabled {
			t.Fatal("commit failure did not retain the exact disabled mission owner")
		}
		unknown, err := s.ReconcileLocalPack(context.Background(), LocalPackReconcileRequest{Proposal: preview.Proposal})
		if err != nil || unknown.Status != "unknown" || unknown.Flow != nil || len(unknown.Blocks) != 0 {
			t.Fatalf("failed commit was not conservatively unknown: %+v %v", unknown, err)
		}
	})

	t.Run("lost reply after committed import", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s, bridge := newServiceFixture(t, nil)
		source := localPackPlainFlow(t, s)
		pack, err := s.ExportLocalPack(context.Background(), source.ID)
		if err != nil {
			t.Fatal(err)
		}
		preview, err := s.PreviewLocalPack(context.Background(), pack)
		if err != nil {
			t.Fatal(err)
		}
		s.store.afterLocalPackCommit = cancel
		defer func() { s.store.afterLocalPackCommit = nil }()
		if _, err := s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal}); err == nil {
			t.Fatal("cancelled post-commit readback did not report uncertainty")
		}
		confirmed, err := s.ReconcileLocalPack(context.Background(), LocalPackReconcileRequest{Proposal: preview.Proposal})
		if err != nil || confirmed.Status != "applied_state_confirmed" || confirmed.Flow == nil {
			t.Fatalf("read-only reconciliation failed to confirm commit: %+v %v", confirmed, err)
		}
		if bridge.FlowMissionState(confirmed.Flow.MissionID, confirmed.Flow.ID) != MissionStateDisabled {
			t.Fatal("uncertain import was enabled")
		}
	})

	t.Run("panic after committed import", func(t *testing.T) {
		ctx := context.Background()
		s, bridge := newServiceFixture(t, nil)
		source := localPackPlainFlow(t, s)
		pack, err := s.ExportLocalPack(ctx, source.ID)
		if err != nil {
			t.Fatal(err)
		}
		preview, err := s.PreviewLocalPack(ctx, pack)
		if err != nil {
			t.Fatal(err)
		}
		missionsBefore := localPackMissionCount(bridge)
		panicValue := errors.New("post-commit panic")
		s.store.afterLocalPackCommit = func() { panic(panicValue) }
		defer func() { s.store.afterLocalPackCommit = nil }()
		func() {
			defer func() {
				if got := recover(); got != panicValue {
					t.Fatalf("panic = %v, want original %v", got, panicValue)
				}
			}()
			_, _ = s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal})
		}()
		if got := localPackMissionCount(bridge); got != missionsBefore+1 {
			t.Fatalf("post-commit panic did not retain exactly one mission owner: before=%d after=%d", missionsBefore, got)
		}
		confirmed, err := s.ReconcileLocalPack(ctx, LocalPackReconcileRequest{Proposal: preview.Proposal})
		if err != nil || confirmed.Status != "applied_state_confirmed" || confirmed.Flow == nil {
			t.Fatalf("read-only reconciliation failed to confirm committed import: %+v %v", confirmed, err)
		}
		if confirmed.Flow.ID != preview.Proposal.ParentID || confirmed.Flow.MissionID == source.MissionID {
			t.Fatalf("reconciliation returned the wrong imported owner: %+v", confirmed.Flow)
		}
		if bridge.FlowMissionState(confirmed.Flow.MissionID, confirmed.Flow.ID) != MissionStateDisabled {
			t.Fatal("post-commit panic deleted or enabled the exact imported mission owner")
		}
	})
}

func TestLocalPackPrecommitPanicCleansExactMissionAndRepanics(t *testing.T) {
	ctx := context.Background()
	s, bridge := newServiceFixture(t, nil)
	source := localPackPlainFlow(t, s)
	pack, err := s.ExportLocalPack(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewLocalPack(ctx, pack)
	if err != nil {
		t.Fatal(err)
	}
	countsBefore := localPackDBCounts(t, s.store.db)
	panicValue := errors.New("precommit panic")
	s.services.Clock = &localPackPanicClock{panicAt: 2, panicVal: panicValue}
	func() {
		defer func() {
			if got := recover(); got != panicValue {
				t.Fatalf("panic = %v, want original %v", got, panicValue)
			}
		}()
		_, _ = s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal})
	}()
	if got := localPackDBCounts(t, s.store.db); got != countsBefore {
		t.Fatalf("panic left partial pack records: before=%v after=%v", countsBefore, got)
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.missions[source.MissionID] == nil {
		t.Fatal("panic cleanup deleted the existing mission")
	}
	for _, mission := range bridge.missions {
		if mission.flowID == preview.Proposal.ParentID {
			t.Fatal("panic cleanup retained the created mission")
		}
	}
}

func TestLocalPackReconcileReturnsUnknownAfterLaterEdit(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	source := localPackPlainFlow(t, s)
	pack, err := s.ExportLocalPack(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewLocalPack(ctx, pack)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal})
	if err != nil {
		t.Fatal(err)
	}
	current, err := s.GetFlow(ctx, imported.Flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := current.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	changed.Name = "Edited after confirmation"
	if _, _, err := s.SaveDraft(ctx, current.ID, changed, current.DraftRevision); err != nil {
		t.Fatal(err)
	}
	unknown, err := s.ReconcileLocalPack(ctx, LocalPackReconcileRequest{Proposal: preview.Proposal})
	if err != nil || unknown.Status != "unknown" || unknown.Flow != nil || len(unknown.Blocks) != 0 {
		t.Fatalf("later edit retained stale confirmation: %+v %v", unknown, err)
	}
}

func TestLocalPackReconcileRequiresDisabledExactMissionOwner(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*fakeBridge, string)
	}{
		{name: "missing", mutate: func(bridge *fakeBridge, id string) { delete(bridge.missions, id) }},
		{name: "foreign owner", mutate: func(bridge *fakeBridge, id string) { bridge.missions[id].flowID = "flow_foreign_owner" }},
		{name: "enabled", mutate: func(bridge *fakeBridge, id string) { bridge.missions[id].enabled = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s, bridge := newServiceFixture(t, nil)
			source := localPackPlainFlow(t, s)
			pack, err := s.ExportLocalPack(ctx, source.ID)
			if err != nil {
				t.Fatal(err)
			}
			preview, err := s.PreviewLocalPack(ctx, pack)
			if err != nil {
				t.Fatal(err)
			}
			imported, err := s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: pack, Proposal: preview.Proposal})
			if err != nil {
				t.Fatal(err)
			}
			bridge.mu.Lock()
			tc.mutate(bridge, imported.Flow.MissionID)
			bridge.mu.Unlock()
			result, err := s.ReconcileLocalPack(ctx, LocalPackReconcileRequest{Proposal: preview.Proposal})
			if err != nil || result.Status != "unknown" || result.Flow != nil || len(result.Blocks) != 0 {
				t.Fatalf("invalid owner was confirmed: %+v %v", result, err)
			}
		})
	}
}
