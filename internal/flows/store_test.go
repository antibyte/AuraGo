package flows

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var storeNow = time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "flows.db"), discardLogger())
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func sampleFlow(id string) *Flow {
	b := newFlow("Sample")
	tr := b.node("start", "test.trigger", nil)
	e := b.node("echo", "test.echo", map[string]any{"value": "x"})
	b.edge(tr, PortOut, e)
	f := b.build()
	f.ID = id
	return f
}

func TestStoreFlowLifecycle(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := sampleFlow("flow_aaaaaaaaaa")
	rec, err := s.CreateFlow(ctx, f, "mission_1", storeNow)
	if err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	if rec.DraftRevision != 1 || rec.Live != nil || !rec.HasUnpublishedChanges() || rec.MissionID != "mission_1" ||
		len(rec.Draft.Nodes) != 2 || !rec.CreatedAt.Equal(storeNow) || rec.Kind != KindFlow {
		t.Fatalf("created record = %+v", rec)
	}
	if _, err := s.CreateFlow(ctx, f, "", storeNow); !errors.Is(err, ErrFlowExists) {
		t.Fatalf("creating the same id twice = %v, want ErrFlowExists", err)
	}

	f.Name = "Renamed"
	rev, err := s.SaveDraft(ctx, f.ID, f, 1, storeNow.Add(time.Minute))
	if err != nil || rev != 2 {
		t.Fatalf("SaveDraft = %d, %v", rev, err)
	}
	current, err := s.SaveDraft(ctx, f.ID, f, 1, storeNow)
	if !errors.Is(err, ErrRevisionConflict) || current != 2 {
		t.Fatalf("stale SaveDraft = %d, %v", current, err)
	}
	// The flow must carry the id it is saved under, so a missing row is probed with a matching flow.
	if _, err := s.SaveDraft(ctx, "flow_missing00", sampleFlow("flow_missing00"), 1, storeNow); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SaveDraft on a missing flow = %v", err)
	}

	pub, err := s.Publish(ctx, f.ID, 2, storeNow.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if pub.LiveRevision != 1 || pub.Live == nil || pub.Live.Name != "Renamed" || pub.HasUnpublishedChanges() ||
		!pub.PublishedAt.Equal(storeNow.Add(2*time.Minute)) {
		t.Fatalf("published record = %+v", pub)
	}
	if _, err := s.Publish(ctx, f.ID, 1, storeNow); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale Publish = %v", err)
	}
	if v, err := s.GetVersion(ctx, f.ID, 1); err != nil || v.Name != "Renamed" {
		t.Fatalf("GetVersion(1) = %+v, %v", v, err)
	}
	if _, err := s.GetVersion(ctx, f.ID, 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetVersion(9) = %v", err)
	}

	f.Name = "Edited"
	if _, err := s.SaveDraft(ctx, f.ID, f, 2, storeNow.Add(3*time.Minute)); err != nil {
		t.Fatalf("SaveDraft after publish: %v", err)
	}
	rec, _ = s.GetFlow(ctx, f.ID)
	if !rec.HasUnpublishedChanges() || rec.Live.Name != "Renamed" || rec.Name != "Edited" {
		t.Fatalf("after edit = %+v", rec)
	}

	if list, err := s.ListFlows(ctx, ""); err != nil || len(list) != 1 {
		t.Fatalf("ListFlows = %d, %v", len(list), err)
	}
	if list, _ := s.ListFlows(ctx, KindBlock); len(list) != 0 {
		t.Fatalf("ListFlows(block) = %d", len(list))
	}
	if err := s.SetMissionID(ctx, f.ID, "mission_2"); err != nil {
		t.Fatalf("SetMissionID: %v", err)
	}
	if rec, _ = s.GetFlow(ctx, f.ID); rec.MissionID != "mission_2" {
		t.Fatalf("mission id = %q", rec.MissionID)
	}
	if err := s.DeleteFlow(ctx, f.ID); err != nil {
		t.Fatalf("DeleteFlow: %v", err)
	}
	if _, err := s.GetFlow(ctx, f.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetFlow after delete = %v", err)
	}
	if err := s.DeleteFlow(ctx, f.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second DeleteFlow = %v", err)
	}
}

func TestStoreKeepsFiftyVersions(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := sampleFlow("flow_aaaaaaaaab")
	if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
		t.Fatal(err)
	}
	rev := 1
	for i := 0; i < 55; i++ {
		var err error
		if rev, err = s.SaveDraft(ctx, f.ID, f, rev, storeNow); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Publish(ctx, f.ID, rev, storeNow); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.GetVersion(ctx, f.ID, 5); !errors.Is(err, ErrNotFound) {
		t.Fatalf("version 5 must be pruned, got %v", err)
	}
	if _, err := s.GetVersion(ctx, f.ID, 6); err != nil {
		t.Fatalf("version 6 must remain: %v", err)
	}
}

func TestStoreReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flows.db")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateFlow(context.Background(), sampleFlow("flow_aaaaaaaaac"), "", storeNow); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	s, err = OpenStore(path, discardLogger())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s.Close()
	if _, err := s.GetFlow(context.Background(), "flow_aaaaaaaaac"); err != nil {
		t.Fatalf("GetFlow after reopen: %v", err)
	}
}

func TestStoreCreateDuplicateKeepsTheOriginal(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	original := sampleFlow("flow_aaaaaaaaad")
	if _, err := s.CreateFlow(ctx, original, "mission_1", storeNow); err != nil {
		t.Fatal(err)
	}
	other := sampleFlow("flow_aaaaaaaaad")
	other.Name = "Impostor"
	_, err := s.CreateFlow(ctx, other, "mission_2", storeNow.Add(time.Hour))
	if !errors.Is(err, ErrFlowExists) {
		t.Fatalf("duplicate CreateFlow = %v, want ErrFlowExists", err)
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("ErrFlowExists must be distinguishable from the other store errors: %v", err)
	}
	rec, err := s.GetFlow(ctx, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Name != "Sample" || rec.MissionID != "mission_1" || rec.DraftRevision != 1 || !rec.CreatedAt.Equal(storeNow) {
		t.Fatalf("the original flow was changed by the failed create: %+v", rec)
	}
	// Other failures stay plain errors, not ErrFlowExists.
	if _, err := s.CreateFlow(ctx, nil, "", storeNow); err == nil || errors.Is(err, ErrFlowExists) {
		t.Fatalf("CreateFlow(nil) = %v", err)
	}
	if _, err := s.CreateFlow(ctx, &Flow{Schema: SchemaVersion}, "", storeNow); err == nil || errors.Is(err, ErrFlowExists) {
		t.Fatalf("CreateFlow without an id = %v", err)
	}
}

func TestStoreSaveDraftRejectsAMismatchedID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	target := sampleFlow("flow_aaaaaaaaae")
	if _, err := s.CreateFlow(ctx, target, "", storeNow); err != nil {
		t.Fatal(err)
	}
	stranger := sampleFlow("flow_aaaaaaaaaf")
	stranger.Name = "Stranger"
	_, err := s.SaveDraft(ctx, target.ID, stranger, 1, storeNow.Add(time.Minute))
	if err == nil {
		t.Fatal("a draft claiming another flow id must be rejected")
	}
	if errors.Is(err, ErrRevisionConflict) || errors.Is(err, ErrNotFound) {
		t.Fatalf("an id mismatch is neither a conflict nor a missing flow: %v", err)
	}
	if stranger.ID != "flow_aaaaaaaaaf" || stranger.Name != "Stranger" {
		t.Fatalf("the caller's flow was modified: id=%q name=%q", stranger.ID, stranger.Name)
	}
	rec, err := s.GetFlow(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.DraftRevision != 1 || rec.Name != "Sample" || rec.Draft.ID != target.ID || !rec.UpdatedAt.Equal(storeNow) {
		t.Fatalf("the rejected save changed the stored draft: %+v", rec)
	}
	// The row of the flow whose id the draft carried is not touched either.
	if _, err := s.GetFlow(ctx, stranger.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetFlow(stranger) = %v", err)
	}

	if _, err := s.SaveDraft(ctx, target.ID, nil, 1, storeNow); err == nil {
		t.Fatal("a nil flow must be rejected, not panic")
	}
	// A matching id still saves.
	target.Name = "Matching"
	if rev, err := s.SaveDraft(ctx, target.ID, target, 1, storeNow.Add(2*time.Minute)); err != nil || rev != 2 {
		t.Fatalf("SaveDraft with a matching id = %d, %v", rev, err)
	}
}

func TestStoreDoesNotMutateTheCallersFlow(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	// A document that was never normalized: nil slices and zero settings stay
	// exactly as they are in the caller's copy.
	f := &Flow{Schema: SchemaVersion, ID: "flow_aaaaaaaaag", Kind: KindFlow, Name: "Raw"}
	before, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	assertUntouched := func(step string) {
		t.Helper()
		after, err := f.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) || f.Nodes != nil || f.Edges != nil {
			t.Fatalf("%s changed the caller's flow:\nbefore %s\nafter  %s", step, before, after)
		}
	}
	rec, err := s.CreateFlow(ctx, f, "", storeNow)
	if err != nil {
		t.Fatal(err)
	}
	assertUntouched("CreateFlow")
	if rec.Draft == f {
		t.Fatal("the record must not alias the caller's flow")
	}
	if _, err := s.SaveDraft(ctx, f.ID, f, 1, storeNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertUntouched("SaveDraft")
}

func TestStoreRejectsOversizedDocuments(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	big := sampleFlow("flow_aaaaaaaaah")
	big.Description = strings.Repeat("a", MaxDocumentBytes+1)
	if _, err := s.CreateFlow(ctx, big, "", storeNow); !errors.Is(err, ErrDocumentTooLarge) {
		t.Fatalf("oversized CreateFlow = %v", err)
	}
	if _, err := s.GetFlow(ctx, big.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the oversized flow must not be stored: %v", err)
	}
	small := sampleFlow(big.ID)
	if _, err := s.CreateFlow(ctx, small, "", storeNow); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveDraft(ctx, big.ID, big, 1, storeNow); !errors.Is(err, ErrDocumentTooLarge) {
		t.Fatalf("oversized SaveDraft = %v", err)
	}
	if rec, err := s.GetFlow(ctx, big.ID); err != nil || rec.DraftRevision != 1 || rec.Draft.Description != "" {
		t.Fatalf("the rejected draft changed the flow: %+v, %v", rec, err)
	}
}

// insertDependents adds one row to every table that references flowID. Versions
// go through Publish; runs, steps, timers and test data arrive with the later
// store tasks, so raw SQL stands in for them here.
func insertDependents(t *testing.T, s *Store, flowID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := s.Publish(ctx, flowID, 1, storeNow); err != nil {
		t.Fatalf("Publish %s: %v", flowID, err)
	}
	runID := "run_" + flowID
	stmts := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO flow_runs (id, flow_id, mode, status, started_at) VALUES (?, ?, 'live', 'success', ?)`,
			[]any{runID, flowID, formatTime(storeNow)}},
		{`INSERT INTO flow_run_steps (run_id, seq, node_id, status) VALUES (?, 1, 'n_aaaaaaaa', 'success')`,
			[]any{runID}},
		{`INSERT INTO flow_timers (flow_id, node_id, fire_at) VALUES (?, 'n_aaaaaaaa', ?)`,
			[]any{flowID, formatTime(storeNow)}},
		{`INSERT INTO flow_test_data (flow_id, node_id, kind, json, updated_at) VALUES (?, 'n_aaaaaaaa', 'trigger', '{}', ?)`,
			[]any{flowID, formatTime(storeNow)}},
	}
	for _, st := range stmts {
		if _, err := s.db.Exec(st.query, st.args...); err != nil {
			t.Fatalf("%s: %v", st.query, err)
		}
	}
}

// dependentRows counts the rows of flowID in versions, runs, steps, timers and test data.
func dependentRows(t *testing.T, s *Store, flowID string) [5]int {
	t.Helper()
	queries := []struct {
		query string
		arg   string
	}{
		{`SELECT COUNT(*) FROM flow_versions WHERE flow_id = ?`, flowID},
		{`SELECT COUNT(*) FROM flow_runs WHERE flow_id = ?`, flowID},
		// By the run id itself: counting through flow_runs would hide orphaned steps.
		{`SELECT COUNT(*) FROM flow_run_steps WHERE run_id = ?`, "run_" + flowID},
		{`SELECT COUNT(*) FROM flow_timers WHERE flow_id = ?`, flowID},
		{`SELECT COUNT(*) FROM flow_test_data WHERE flow_id = ?`, flowID},
	}
	var counts [5]int
	for i, q := range queries {
		if err := s.db.QueryRow(q.query, q.arg).Scan(&counts[i]); err != nil {
			t.Fatalf("%s: %v", q.query, err)
		}
	}
	return counts
}

func TestStoreDeleteCascadesToDependents(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	doomed, keep := sampleFlow("flow_aaaaaaaaai"), sampleFlow("flow_aaaaaaaaaj")
	for _, f := range []*Flow{doomed, keep} {
		if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
			t.Fatal(err)
		}
		insertDependents(t, s, f.ID)
	}
	want := [5]int{1, 1, 1, 1, 1}
	if got := dependentRows(t, s, doomed.ID); got != want {
		t.Fatalf("dependents before delete = %v, want %v", got, want)
	}
	if err := s.DeleteFlow(ctx, doomed.ID); err != nil {
		t.Fatalf("DeleteFlow: %v", err)
	}
	if got := dependentRows(t, s, doomed.ID); got != [5]int{} {
		t.Fatalf("dependents left behind by DeleteFlow (versions, runs, steps, timers, test data) = %v", got)
	}
	if got := dependentRows(t, s, keep.ID); got != want {
		t.Fatalf("DeleteFlow removed another flow's dependents: %v, want %v", got, want)
	}
}

func TestStoreEnforcesForeignKeysOnEveryConnection(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	n := s.db.Stats().MaxOpenConnections
	if n < 2 {
		t.Skipf("pool holds %d connection(s); nothing to compare", n)
	}
	// Hold every connection of the pool at once so each one is checked.
	conns := make([]*sql.Conn, 0, n)
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()
	for i := 0; i < n; i++ {
		c, err := s.db.Conn(ctx)
		if err != nil {
			t.Fatalf("connection %d: %v", i, err)
		}
		conns = append(conns, c)
		var on int
		if err := c.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&on); err != nil || on != 1 {
			t.Fatalf("connection %d: foreign_keys = %d, %v", i, on, err)
		}
		_, err = c.ExecContext(ctx, `INSERT INTO flow_runs (id, flow_id, mode, status, started_at) VALUES (?, 'flow_nowhere000', 'live', 'success', '')`,
			fmt.Sprintf("run_orphan%d", i))
		if err == nil {
			t.Fatalf("connection %d accepted a run for a flow that does not exist", i)
		}
	}
}

func TestStoreRunStepsHaveTruncationColumns(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := sampleFlow("flow_aaaaaaaaak")
	if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO flow_runs (id, flow_id, mode, status, started_at) VALUES ('run_aaaaaaaaaaaa', ?, 'test', 'success', ?)`,
		f.ID, formatTime(storeNow)); err != nil {
		t.Fatal(err)
	}
	// Step 1 relies on the column defaults, step 2 sets both flags.
	if _, err := s.db.Exec(`INSERT INTO flow_run_steps (run_id, seq, node_id, status) VALUES ('run_aaaaaaaaaaaa', 1, 'n_aaaaaaaa', 'success')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO flow_run_steps (run_id, seq, node_id, status, params_truncated, output_truncated)
		VALUES ('run_aaaaaaaaaaaa', 2, 'n_aaaaaaaa', 'success', 1, 1)`); err != nil {
		t.Fatalf("flow_run_steps needs params_truncated and output_truncated: %v", err)
	}
	rows, err := s.db.Query(`SELECT seq, params_truncated, output_truncated FROM flow_run_steps ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := [][3]int{{1, 0, 0}, {2, 1, 1}}
	var got [][3]int
	for rows.Next() {
		var r [3]int
		if err := rows.Scan(&r[0], &r[1], &r[2]); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("step flags = %v, want %v", got, want)
	}
}

func TestStoreErrorsDoNotEchoLongValues(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	long := "flow_" + strings.Repeat("x", 5000)
	assertBounded := func(label string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: expected an error", label)
		}
		if len(err.Error()) > 400 || strings.Contains(err.Error(), strings.Repeat("x", 100)) {
			t.Fatalf("%s: the error echoes a long value (%d bytes): %.120s", label, len(err.Error()), err.Error())
		}
	}

	f := sampleFlow(long)
	if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
		t.Fatalf("CreateFlow with a long id: %v", err)
	}
	_, err := s.CreateFlow(ctx, f, "", storeNow)
	if !errors.Is(err, ErrFlowExists) {
		t.Fatalf("duplicate CreateFlow = %v", err)
	}
	assertBounded("duplicate CreateFlow", err)
	_, err = s.SaveDraft(ctx, "flow_aaaaaaaaal", f, 1, storeNow)
	assertBounded("SaveDraft (flow id too long)", err)
	_, err = s.SaveDraft(ctx, long, sampleFlow("flow_aaaaaaaaal"), 1, storeNow)
	assertBounded("SaveDraft (row id too long)", err)

	// Rows that cannot be decoded: the error names the flow, bounded.
	broken := []struct{ id, draft, live string }{
		{"flow_" + strings.Repeat("d", 5000), `{"schema": "not a number"`, ""},
		{"flow_" + strings.Repeat("l", 5000), `{"schema":1,"id":"x","name":"x"}`, `{"schema": 7}`},
	}
	for _, b := range broken {
		if _, err := s.db.Exec(`INSERT INTO flows (id, name, draft_json, live_json, created_at, updated_at) VALUES (?, 'broken', ?, ?, '', '')`,
			b.id, b.draft, b.live); err != nil {
			t.Fatal(err)
		}
		_, err := s.GetFlow(ctx, b.id)
		assertBounded("GetFlow of an undecodable row", err)
		if errors.Is(err, ErrNotFound) {
			t.Fatalf("an undecodable row is not a missing flow: %v", err)
		}
	}
	list, err := s.ListFlows(ctx, "")
	if err != nil || len(list) != 1 || list[0].ID != long {
		t.Fatalf("ListFlows must skip undecodable rows, got %d rows, %v", len(list), err)
	}
}
