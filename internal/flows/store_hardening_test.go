package flows

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// versionStats returns the number of stored versions of flowID and the lowest
// and highest stored revision.
func versionStats(t *testing.T, s *Store, flowID string) (count, lowest, highest int) {
	t.Helper()
	err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(MIN(revision), 0), COALESCE(MAX(revision), 0) FROM flow_versions WHERE flow_id = ?`, flowID).
		Scan(&count, &lowest, &highest)
	if err != nil {
		t.Fatalf("version stats: %v", err)
	}
	return count, lowest, highest
}

func TestStoreKeepsExactlyFiftyVersionRows(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := sampleFlow("flow_aaaaaaaaba")
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
	if count, lowest, highest := versionStats(t, s, f.ID); count != 50 || lowest != 6 || highest != 55 {
		t.Fatalf("stored versions = %d (revisions %d..%d), want 50 (6..55)", count, lowest, highest)
	}
}

func TestStorePublishOfAnAlreadyLiveDraftIsANoOp(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := sampleFlow("flow_aaaaaaaabb")
	if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
		t.Fatal(err)
	}
	first, err := s.Publish(ctx, f.ID, 1, storeNow)
	if err != nil || first.LiveRevision != 1 {
		t.Fatalf("first Publish = %+v, %v", first, err)
	}
	again, err := s.Publish(ctx, f.ID, 1, storeNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("publishing the live draft again = %v", err)
	}
	if again.LiveRevision != 1 || !again.PublishedAt.Equal(first.PublishedAt) || !again.UpdatedAt.Equal(first.UpdatedAt) ||
		again.DraftRevision != 1 || again.HasUnpublishedChanges() {
		t.Fatalf("a repeated publish changed the record: %+v, first %+v", again, first)
	}
	if count, _, _ := versionStats(t, s, f.ID); count != 1 {
		t.Fatalf("versions after a repeated publish = %d, want 1", count)
	}
	if _, err := s.Publish(ctx, f.ID, 0, storeNow); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("a stale base revision after the no-op = %v, want ErrRevisionConflict", err)
	}
	if _, err := s.Publish(ctx, "flow_missing00", 1, storeNow); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Publish of a missing flow = %v, want ErrNotFound", err)
	}

	// A new draft revision publishes again.
	f.Name = "Second"
	rev, err := s.SaveDraft(ctx, f.ID, f, 1, storeNow.Add(2*time.Hour))
	if err != nil || rev != 2 {
		t.Fatalf("SaveDraft = %d, %v", rev, err)
	}
	second, err := s.Publish(ctx, f.ID, 2, storeNow.Add(3*time.Hour))
	if err != nil || second.LiveRevision != 2 || second.Live.Name != "Second" {
		t.Fatalf("second Publish = %+v, %v", second, err)
	}
	if count, _, _ := versionStats(t, s, f.ID); count != 2 {
		t.Fatalf("versions = %d, want 2", count)
	}
}

// Publish used to read first and write second; SQLite does not run the busy
// handler when a read transaction upgrades to a write, so concurrent publishes
// failed with a raw "database is locked". Every outcome must now be nil or a
// revision conflict.
func TestStoreConcurrentPublishesNeverFailWithLockErrors(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const rounds, workers = 40, 8
	for r := 0; r < rounds; r++ {
		f := sampleFlow(fmt.Sprintf("flow_race%06d", r))
		if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
			t.Fatal(err)
		}
		errs := make([]error, workers)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[w] = s.Publish(ctx, f.ID, 1, storeNow)
			}()
		}
		close(start)
		wg.Wait()
		for w, err := range errs {
			if err != nil && !errors.Is(err, ErrRevisionConflict) {
				t.Fatalf("round %d, worker %d: Publish = %v, want nil or ErrRevisionConflict", r, w, err)
			}
		}
		rec, err := s.GetFlow(ctx, f.ID)
		if err != nil {
			t.Fatal(err)
		}
		if count, _, _ := versionStats(t, s, f.ID); rec.LiveRevision != 1 || count != 1 {
			t.Fatalf("round %d: live revision %d with %d versions, want one of each", r, rec.LiveRevision, count)
		}
	}
}

// The test has two phases against an unthrottled autosave loop.
//
// The racy phase fires 80 GetFlow+Publish pairs while the loop saves as fast as
// it can. Every Publish must return nil or ErrRevisionConflict, as must every
// SaveDraft. How many of these publishes win is up to the scheduler: the loop can
// bump the draft revision between every GetFlow and Publish, so the racy phase
// alone cannot promise a success, and a real editor saves debounced anyway.
//
// The handshake round makes the success deterministic, without sleeps. The main
// goroutine asks the loop to park (pause); the loop answers with one last save,
// so the draft is fresh and was never published, signals that it is parked and
// waits for resume. While it is parked, but still alive and mid-loop, the main
// goroutine runs one GetFlow+Publish pair, which no writer can interleave with.
// That Publish must succeed. Afterwards the loop is released and stopped, and the
// version bookkeeping must match the live revision.
func TestStorePublishRunsAgainstAnAutosaveLoop(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := sampleFlow("flow_aaaaaaaabc")
	if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
		t.Fatal(err)
	}
	var autosaveErr error // written by the goroutine before it closes done, read only after <-done
	stop := make(chan struct{})
	done := make(chan struct{})
	pause := make(chan struct{})  // main -> loop: park after a fresh save
	parked := make(chan struct{}) // loop -> main: the fresh save is in, the loop is waiting
	resume := make(chan struct{}) // main -> loop: carry on
	var stopOnce sync.Once
	stopLoop := func() {
		stopOnce.Do(func() { close(stop) })
		<-done
	}
	// Also stops the loop when a Fatal below ends the test early. Registered after
	// openTestStore, so it runs before the store is closed.
	t.Cleanup(stopLoop)
	autosaveFailed := func() { t.Fatalf("SaveDraft = %v, want nil or ErrRevisionConflict", autosaveErr) }

	go func() {
		defer close(done)
		rev := 1
		save := func() bool {
			cur, err := s.SaveDraft(ctx, f.ID, f, rev, storeNow)
			switch {
			case err == nil, errors.Is(err, ErrRevisionConflict):
				rev = cur
				return true
			default:
				autosaveErr = err
				return false
			}
		}
		for {
			select {
			case <-stop:
				return
			default:
			}
			if !save() {
				return
			}
			select {
			case <-pause:
				if !save() {
					return
				}
				select {
				case parked <- struct{}{}:
				case <-stop:
					return
				}
				select {
				case <-resume:
				case <-stop:
					return
				}
			default:
			}
		}
	}()

	// Racy phase: properties only, no promise that a publish wins.
	published := 0
	for i := 0; i < 80; i++ {
		rec, err := s.GetFlow(ctx, f.ID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Publish(ctx, f.ID, rec.DraftRevision, storeNow)
		if err == nil {
			published++
		} else if !errors.Is(err, ErrRevisionConflict) {
			t.Errorf("attempt %d: Publish = %v, want nil or ErrRevisionConflict", i, err)
			break
		}
	}
	t.Logf("racy phase: %d of 80 publishes won against the autosave loop", published)

	// Handshake round: one Publish next to a parked, still running autosave loop.
	select {
	case pause <- struct{}{}:
	case <-done:
		autosaveFailed()
	}
	select {
	case <-parked:
	case <-done:
		autosaveFailed()
	}
	before, err := s.GetFlow(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !before.HasUnpublishedChanges() {
		t.Fatalf("the parked loop's last save left no unpublished draft: %+v", before)
	}
	got, publishErr := s.Publish(ctx, f.ID, before.DraftRevision, storeNow)
	close(resume)
	if publishErr != nil {
		t.Fatalf("Publish while the autosave loop is parked = %v, want nil", publishErr)
	}
	if got.LiveRevision != before.LiveRevision+1 || got.PublishedDraftRevision != before.DraftRevision {
		t.Fatalf("handshake Publish: live revision %d (was %d), published draft revision %d, want %d and %d",
			got.LiveRevision, before.LiveRevision, got.PublishedDraftRevision, before.LiveRevision+1, before.DraftRevision)
	}

	stopLoop()
	if autosaveErr != nil {
		autosaveFailed()
	}
	rec, err := s.GetFlow(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	count, _, highest := versionStats(t, s, f.ID)
	want := rec.LiveRevision
	if want > maxStoredVersions {
		want = maxStoredVersions
	}
	if rec.LiveRevision < got.LiveRevision || count != want || highest != rec.LiveRevision {
		t.Fatalf("handshake published revision %d; live revision %d, versions %d (highest %d)", got.LiveRevision, rec.LiveRevision, count, highest)
	}
}

func TestStoreOpenFailsLoudlyOnAnUnreadableDatabase(t *testing.T) {
	assertLeftInPlace := func(t *testing.T, dir, path string, want []byte) {
		t.Helper()
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("the unreadable database was changed or removed (read: %d bytes, %v)", len(got), err)
		}
		if backups, _ := filepath.Glob(filepath.Join(dir, "*.bak")); len(backups) != 0 {
			t.Fatalf("OpenStore must not move the database aside, found %v", backups)
		}
	}

	t.Run("not a database", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "flows.db")
		garbage := bytes.Repeat([]byte("this is not a sqlite database. "), 400)
		if err := os.WriteFile(path, garbage, 0o600); err != nil {
			t.Fatal(err)
		}
		s, err := OpenStore(path, discardLogger())
		if err == nil {
			_ = s.Close()
			t.Fatal("OpenStore must fail on a file that is not a database")
		}
		t.Logf("OpenStore error: %v", err)
		assertLeftInPlace(t, dir, path, garbage)
	})

	t.Run("damaged page", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "flows.db")
		s, err := OpenStore(path, discardLogger())
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 20; i++ {
			f := sampleFlow(fmt.Sprintf("flow_dmg%07d", i))
			f.Description = strings.Repeat("z", 50000)
			if _, err := s.CreateFlow(context.Background(), f, "", storeNow); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) < 3*4096 {
			t.Fatalf("database file: %d bytes, %v", len(data), err)
		}
		// Overwrite pages 2 and 3, right after the header page, with 0xFF.
		for i := 4096; i < 4096*3; i++ {
			data[i] = 0xFF
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		s, err = OpenStore(path, discardLogger())
		if err == nil {
			_ = s.Close()
			t.Fatal("OpenStore must fail on a damaged database")
		}
		t.Logf("OpenStore error: %v", err)
		assertLeftInPlace(t, dir, path, data)
	})
}

func TestStoreOpenCreatesTheParentDirectory(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "data", "flows", "flows.db")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatalf("OpenStore in a missing directory: %v", err)
	}
	if _, err := s.CreateFlow(context.Background(), sampleFlow("flow_aaaaaaaabd"), "", storeNow); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	if info, err := os.Stat(filepath.Dir(path)); err != nil || !info.IsDir() {
		t.Fatalf("parent directory: %v, %v", info, err)
	}

	// A parent that is a file cannot be created: the error is reported, not swallowed.
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenStore(filepath.Join(blocker, "flows.db"), discardLogger()); err == nil {
		_ = s.Close()
		t.Fatal("OpenStore under a regular file must fail")
	}
}

func TestStoreOpenLeavesSpecialDSNsAlone(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadDir(cwd)
	s, err := OpenStore(":memory:", discardLogger())
	if err != nil {
		t.Fatalf("OpenStore(:memory:): %v", err)
	}
	defer s.Close()
	if _, err := s.CreateFlow(context.Background(), sampleFlow("flow_aaaaaaaabe"), "", storeNow); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadDir(cwd); len(after) != len(before) {
		t.Fatalf("OpenStore(:memory:) created files or directories in %s", cwd)
	}
	for _, dsn := range []string{":memory:", "file::memory:?cache=shared", "file:flows.db?mode=memory", ""} {
		if !skipsDirectoryCreation(dsn) {
			t.Errorf("skipsDirectoryCreation(%q) = false", dsn)
		}
	}
	if skipsDirectoryCreation(filepath.Join("data", "flows.db")) {
		t.Error("an ordinary path must get its directory created")
	}
}

func TestStoreRejectsAnUnsupportedSchemaBeforeWriting(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	for _, schema := range []int{0, -1, SchemaVersion + 1} {
		f := sampleFlow("flow_aaaaaaaabf")
		f.Schema = schema
		_, err := s.CreateFlow(ctx, f, "", storeNow)
		if !errors.Is(err, ErrUnsupportedSchema) {
			t.Fatalf("CreateFlow(schema %d) = %v, want ErrUnsupportedSchema", schema, err)
		}
		if _, err := s.GetFlow(ctx, f.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("CreateFlow(schema %d) left a row behind: %v", schema, err)
		}
	}

	good := sampleFlow("flow_aaaaaaaabg")
	if _, err := s.CreateFlow(ctx, good, "", storeNow); err != nil {
		t.Fatal(err)
	}
	bad := sampleFlow(good.ID)
	bad.Schema = 0
	bad.Name = "Schema-less"
	rev, err := s.SaveDraft(ctx, good.ID, bad, 1, storeNow.Add(time.Minute))
	if !errors.Is(err, ErrUnsupportedSchema) || rev != 0 {
		t.Fatalf("SaveDraft(schema 0) = %d, %v, want 0 and ErrUnsupportedSchema", rev, err)
	}
	rec, err := s.GetFlow(ctx, good.ID)
	if err != nil {
		t.Fatalf("the flow must stay readable after the rejected save: %v", err)
	}
	if rec.DraftRevision != 1 || rec.Name != "Sample" || !rec.UpdatedAt.Equal(storeNow) || rec.Draft.Schema != SchemaVersion {
		t.Fatalf("the rejected save changed the flow: %+v", rec)
	}
	if _, err := s.Publish(ctx, good.ID, 1, storeNow); err != nil {
		t.Fatalf("Publish after the rejected save: %v", err)
	}
}

func TestStoreKindColumnFollowsTheDocument(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// An empty or unknown kind is stored the way Normalize reads it back.
	for i, kind := range []Kind{"", "bogus", KindFlow, KindBlock} {
		f := sampleFlow(fmt.Sprintf("flow_kind%06d", i))
		f.Kind = kind
		probe, err := f.Clone()
		if err != nil {
			t.Fatal(err)
		}
		probe.Normalize()
		rec, err := s.CreateFlow(ctx, f, "", storeNow)
		if err != nil {
			t.Fatal(err)
		}
		if rec.Kind != probe.Kind || rec.Draft.Kind != probe.Kind {
			t.Fatalf("CreateFlow(kind %q): record kind %q, document kind %q, Normalize gives %q", kind, rec.Kind, rec.Draft.Kind, probe.Kind)
		}
		if f.Kind != kind {
			t.Fatalf("CreateFlow changed the caller's kind from %q to %q", kind, f.Kind)
		}
	}
	if list, _ := s.ListFlows(ctx, KindFlow); len(list) != 3 {
		t.Fatalf("ListFlows(flow) = %d, want 3 (empty, unknown and flow kinds)", len(list))
	}
	if list, _ := s.ListFlows(ctx, KindBlock); len(list) != 1 {
		t.Fatalf("ListFlows(block) = %d, want 1", len(list))
	}

	// SaveDraft moves the column together with the document.
	f := sampleFlow("flow_aaaaaaaabh")
	if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
		t.Fatal(err)
	}
	f.Kind = KindBlock
	rev, err := s.SaveDraft(ctx, f.ID, f, 1, storeNow)
	if err != nil || rev != 2 {
		t.Fatalf("SaveDraft(block) = %d, %v", rev, err)
	}
	rec, _ := s.GetFlow(ctx, f.ID)
	if rec.Kind != KindBlock || rec.Draft.Kind != KindBlock {
		t.Fatalf("after SaveDraft(block): column %q, document %q", rec.Kind, rec.Draft.Kind)
	}
	if list, _ := s.ListFlows(ctx, KindBlock); len(list) != 2 {
		t.Fatalf("ListFlows(block) after the change = %d, want 2", len(list))
	}
	f.Kind = ""
	if rev, err = s.SaveDraft(ctx, f.ID, f, rev, storeNow); err != nil || rev != 3 {
		t.Fatalf("SaveDraft(empty kind) = %d, %v", rev, err)
	}
	if rec, _ = s.GetFlow(ctx, f.ID); rec.Kind != KindFlow {
		t.Fatalf("an empty kind must be stored as %q, got %q", KindFlow, rec.Kind)
	}
	if f.Kind != "" {
		t.Fatalf("SaveDraft changed the caller's kind to %q", f.Kind)
	}
}

func TestStoreIndexesMissionID(t *testing.T) {
	s := openTestStore(t)
	rows, err := s.db.Query(`PRAGMA index_list(flows)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found, unique := false, 0
	for rows.Next() {
		var seq, isUnique, partial int
		var name, origin string
		if err := rows.Scan(&seq, &name, &isUnique, &origin, &partial); err != nil {
			t.Fatal(err)
		}
		if name == "idx_flows_mission" {
			found, unique = true, isUnique
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !found || unique != 0 {
		t.Fatalf("idx_flows_mission: found=%v unique=%d, want a plain index", found, unique)
	}

	var plan string
	if err := s.db.QueryRow(`EXPLAIN QUERY PLAN SELECT id FROM flows WHERE mission_id = ?`, "mission_1").Scan(new(int), new(int), new(int), &plan); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan, "idx_flows_mission") {
		t.Fatalf("a lookup by mission id does not use the index: %s", plan)
	}

	// Flows without a mission share the empty id, so the index must not be unique.
	ctx := context.Background()
	for _, id := range []string{"flow_aaaaaaaabi", "flow_aaaaaaaabj"} {
		if _, err := s.CreateFlow(ctx, sampleFlow(id), "", storeNow); err != nil {
			t.Fatalf("two flows without a mission must coexist: %v", err)
		}
	}
}

// A context that ends right after Publish committed does not hide the committed record:
// the publishing path and the already-live path both return it, not the context's error.
func TestStorePublishReturnsTheRecordAfterCommit(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := sampleFlow("flow_aaaaaaaapc")
	if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	for _, path := range []string{"publish", "already live"} {
		cctx, cancel := context.WithCancel(ctx)
		s.afterPublishCommit = cancel
		rec, err := s.Publish(cctx, f.ID, 1, storeNow)
		s.afterPublishCommit = nil
		if err != nil || rec == nil || rec.LiveRevision != 1 || cctx.Err() == nil {
			t.Fatalf("%s: Publish cancelled after the commit = %+v, %v (ctx %v)", path, rec, err, cctx.Err())
		}
	}
}
