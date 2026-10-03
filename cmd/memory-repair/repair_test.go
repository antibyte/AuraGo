package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"aurago/internal/memory"
	"github.com/gofrs/flock"
	chromem "github.com/philippgille/chromem-go"
)

type repairFixture struct {
	root, path, vectors string
	stm                 *memory.SQLiteMemory
	db                  *sql.DB
}

func newRepairFixture(t *testing.T, ids ...string) *repairFixture {
	t.Helper()
	if len(ids) == 0 {
		ids = []string{"a", "b", "c"}
	}
	f := &repairFixture{root: t.TempDir()}
	f.path = filepath.Join(f.root, "memory.db")
	f.vectors = filepath.Join(f.root, "vectors")
	if err := os.WriteFile(filepath.Join(f.root, "config.yaml"), []byte("directories:\n  vectordb_dir: vectors\nsqlite:\n  short_term_path: memory.db\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "aurago.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var err error
	f.stm, err = memory.NewSQLiteMemory(f.path, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.stm.Close() })
	if err := f.stm.InitJournalTables(); err != nil {
		t.Fatal(err)
	}
	vectors, err := chromem.NewPersistentDB(f.vectors, false)
	if err != nil {
		t.Fatal(err)
	}
	collection, err := vectors.GetOrCreateCollection("aurago_memories", nil, func(context.Context, string) ([]float32, error) {
		t.Error("offline fixture requested an embedding")
		return nil, errors.New("offline")
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if err := collection.AddDocument(t.Context(), chromem.Document{ID: id, Content: "[preference:editor] User prefers Vim\n\nsource:memory_analysis session:" + id, Metadata: map[string]string{"domain": "work"}, Embedding: []float32{1, 0, 0}}); err != nil {
			t.Fatal(err)
		}
		if err := f.stm.EnsureMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{SourceType: "memory_analysis", VerificationStatus: "unverified"}); err != nil {
			t.Fatal(err)
		}
		if err := f.stm.RecordMemoryExtractionSource(id, "memory_analysis", id); err != nil {
			t.Fatal(err)
		}
		if err := f.stm.RecordMemoryExtractionSource(id, "memory_analysis", "shared"); err != nil {
			t.Fatal(err)
		}
	}
	f.db, err = openSQLite(f.path, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.db.Close() })
	return f
}

func TestApplyRequiresConfiguredLockedDataPaths(t *testing.T) {
	f := newRepairFixture(t, "a", "b")
	other := newRepairFixture(t, "c", "d")
	args := []string{"--db", other.path, "--vector-db", f.vectors, "--install-dir", f.root, "--plan", filepath.Join(f.root, "reports", "preview.json"), "--apply"}
	if err := run(t.Context(), args); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched DB apply error = %v", err)
	}
	args[1], args[3] = f.path, other.vectors
	if err := run(t.Context(), args); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched vector apply error = %v", err)
	}
}
func (f *repairFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := f.db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}
func (f *repairFixture) plan(t *testing.T) mergePlan {
	t.Helper()
	vectors, err := openVectors(f.vectors)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := buildPlan(t.Context(), f.db, vectors)
	if err != nil {
		t.Fatal(err)
	}
	plan.Database, plan.Vectors, plan.InstallDir = f.path, f.vectors, f.root
	return plan
}
func (f *repairFixture) count(t *testing.T) int {
	t.Helper()
	vectors, err := openVectors(f.vectors)
	if err != nil {
		t.Fatal(err)
	}
	return vectors.Count()
}
func (f *repairFixture) curate(t *testing.T, id, action string) {
	t.Helper()
	if err := f.stm.UpsertMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{SourceType: "user", SourceReliability: .98, ExtractionConfidence: .99, VerificationStatus: "unverified"}); err != nil {
		t.Fatal(err)
	}
	if err := f.stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: id, Action: action, Reason: "human review"}, "user", false); err != nil {
		t.Fatal(err)
	}
}

func TestMergeTransfersEveryReferenceAndPreservesCuration(t *testing.T) {
	f := newRepairFixture(t)
	f.curate(t, "b", "confirm")
	f.exec(t, `UPDATE memory_meta SET access_count=2,useful_count=3,useless_count=1,last_accessed='2026-08-01 00:00:00' WHERE doc_id='a'`)
	f.exec(t, `UPDATE memory_meta SET access_count=4,last_accessed='2026-09-01T00:00:00Z' WHERE doc_id='b'`)
	f.exec(t, `UPDATE memory_meta SET last_accessed='2026-08-01 00:00:00' WHERE doc_id='c'`)
	f.exec(t, `INSERT INTO memory_usage_log(memory_id,memory_type,session_id) VALUES ('a','ltm','chat-A'),('c','ltm','chat-C')`)
	f.exec(t, `INSERT INTO episodic_memories(event_date,title,summary,related_doc_ids) VALUES ('2026-09-01','episode','summary','["a","b","other","c"]')`)
	f.exec(t, `INSERT INTO memory_conflicts(doc_id_left,doc_id_right,conflict_key,left_value,right_value,status,winning_doc_id,superseded_doc_id)
		VALUES ('a','aa','editor','vim','emacs','resolved','a','aa')`)
	f.exec(t, `INSERT INTO memory_maintenance_failures(action,target_id,failure_count) VALUES ('canonical_repair','a',2),('canonical_repair','b',3)`)
	f.exec(t, `INSERT INTO memory_maintenance_meta VALUES ('memory_conflict_scan.cursor','c'),('canonical_repair.v1.cursor','a')`)
	f.exec(t, `UPDATE memory_extraction_sources SET first_seen_at='2026-09-01T01:00:00Z',last_seen_at='2026-09-10T02:00:00Z' WHERE doc_id='a' AND session_id='shared'`)
	f.exec(t, `UPDATE memory_extraction_sources SET first_seen_at='2026-09-01 02:00:00',last_seen_at='2026-09-10 03:00:00' WHERE doc_id='b' AND session_id='shared'`)
	f.exec(t, `UPDATE memory_extraction_sources SET first_seen_at='2026-09-01T00:00:00Z',last_seen_at='2026-09-10T01:00:00Z' WHERE doc_id='c' AND session_id='shared'`)
	plan := f.plan(t)
	if len(plan.Groups) != 1 || plan.Groups[0].Canonical != "b" {
		t.Fatalf("plan=%+v", plan)
	}
	completed, failed := applyGroups(t.Context(), f.db, f.vectors, plan, nil)
	if completed != 1 || len(failed) != 0 || f.count(t) != 1 {
		t.Fatalf("completed=%d failures=%+v count=%d", completed, failed, f.count(t))
	}
	meta, err := f.stm.GetMemoryMeta("b")
	if err != nil || meta.VerificationStatus != "confirmed" || meta.SourceType != "user" || meta.ExtractionConfidence != .99 || meta.AccessCount != 6 || meta.UsefulCount != 3 || meta.UselessCount != 1 || meta.LastAccessed != "2026-09-01T00:00:00Z" {
		t.Fatalf("canonical=%+v err=%v", meta, err)
	}
	for _, table := range []string{"memory_meta", "memory_curation_events", "memory_extraction_sources"} {
		rows, err := rowsFor(t.Context(), f.db, `SELECT * FROM `+table+` WHERE doc_id IN ('a','c')`)
		if err != nil || len(rows) != 0 {
			t.Fatalf("old references in %s: %+v %v", table, rows, err)
		}
	}
	var uses, failures int
	var episode, left, right, leftValue, rightValue, winner, cursor string
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM memory_usage_log WHERE memory_id='b'`).Scan(&uses); err != nil || uses != 2 {
		t.Fatalf("usage=%d %v", uses, err)
	}
	if err := f.db.QueryRow(`SELECT related_doc_ids FROM episodic_memories`).Scan(&episode); err != nil || episode != `["b","other"]` {
		t.Fatalf("episode=%s %v", episode, err)
	}
	if err := f.db.QueryRow(`SELECT doc_id_left,doc_id_right,left_value,right_value,winning_doc_id FROM memory_conflicts`).Scan(&left, &right, &leftValue, &rightValue, &winner); err != nil || left != "aa" || right != "b" || leftValue != "emacs" || rightValue != "vim" || winner != "b" {
		t.Fatalf("conflict=%s/%s %s/%s %s %v", left, right, leftValue, rightValue, winner, err)
	}
	if err := f.db.QueryRow(`SELECT failure_count FROM memory_maintenance_failures WHERE target_id='b'`).Scan(&failures); err != nil || failures != 5 {
		t.Fatalf("failures=%d %v", failures, err)
	}
	if err := f.db.QueryRow(`SELECT value FROM memory_maintenance_meta WHERE key='canonical_repair.v1.cursor'`).Scan(&cursor); err != nil || cursor != "b" {
		t.Fatalf("cursor=%s %v", cursor, err)
	}
	sources, err := f.stm.GetMemoryExtractionSources("b")
	if err != nil || len(sources) != 4 {
		t.Fatalf("sources=%+v %v", sources, err)
	}
	for _, source := range sources {
		if source.SessionID == "shared" && (source.FirstSeenAt != "2026-09-01T00:00:00.000000000Z" || source.LastSeenAt != "2026-09-10T03:00:00.000000000Z") {
			t.Fatalf("source range changed: %+v", source)
		}
	}
	if completed, failed := applyGroups(t.Context(), f.db, f.vectors, plan, nil); completed != 0 || len(failed) != 0 {
		t.Fatalf("repeat=%d %+v", completed, failed)
	}
	again, _ := f.stm.GetMemoryMeta("b")
	if again != meta {
		t.Fatal("repeated merge changed metadata")
	}
}

func TestMergeArchiveProtectionAndAmbiguity(t *testing.T) {
	for _, mode := range []string{"archive", "protected", "permanent", "multiple", "curation", "index", "unknown", "conflicts", "pending_repair", "domain", "category", "chunk", "null_counter", "fractional_counter", "invalid_protection", "missing_activity", "invalid_quality", "unknown_source", "incomplete_failure"} {
		t.Run(mode, func(t *testing.T) {
			f := newRepairFixture(t, "a", "b")
			want := 1
			canonical := "a"
			switch mode {
			case "archive":
				f.curate(t, "b", "archive")
				canonical = "b"
			case "protected":
				f.exec(t, `UPDATE memory_meta SET protected=1 WHERE doc_id='b'`)
				canonical = "b"
			case "permanent":
				f.exec(t, `UPDATE memory_meta SET keep_forever=1 WHERE doc_id='b'`)
				canonical = "b"
			case "multiple":
				f.exec(t, `UPDATE memory_meta SET protected=1`)
				want = 0
			case "curation":
				f.curate(t, "a", "confirm")
				f.curate(t, "b", "archive")
				want = 0
			case "index":
				f.exec(t, `INSERT INTO file_embedding_docs(file_path,collection,doc_id) VALUES ('note','file_index','b')`)
				want = 0
			case "unknown":
				f.exec(t, `CREATE TABLE other_refs(doc_id TEXT)`)
				f.exec(t, `INSERT INTO other_refs VALUES ('b')`)
				want = 0
			case "conflicts":
				f.exec(t, `INSERT INTO memory_conflicts(doc_id_left,doc_id_right,conflict_key) VALUES ('a','z','editor'),('b','z','editor')`)
				want = 0
			case "pending_repair":
				f.curate(t, "b", "confirm")
				f.exec(t, `UPDATE memory_meta SET source_type='memory_analysis' WHERE doc_id='b'`)
				want = 0
			case "unknown_source":
				f.exec(t, `UPDATE memory_extraction_sources SET source_type='' WHERE doc_id='b'`)
				want = 0
			case "incomplete_failure":
				f.exec(t, `INSERT INTO memory_maintenance_failures(action,target_id,failure_count) VALUES ('canonical_repair','b',NULL)`)
				want = 0
			case "null_counter", "fractional_counter", "invalid_protection", "missing_activity", "invalid_quality":
				updates := map[string]string{"null_counter": "access_count=NULL", "fractional_counter": "access_count=1.5", "invalid_protection": "protected=2", "missing_activity": "last_accessed=NULL", "invalid_quality": "source_reliability='unknown'"}
				f.exec(t, `UPDATE memory_meta SET `+updates[mode]+` WHERE doc_id='b'`)
				want = 0
			default:
				vectors, err := openVectors(f.vectors)
				if err != nil {
					t.Fatal(err)
				}
				doc, err := vectors.GetByID(t.Context(), "b")
				if err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "domain":
					doc.Metadata["domain"] = "home"
				case "category":
					doc.Content = strings.Replace(doc.Content, "preference:editor", "correction:editor", 1)
				case "chunk":
					doc.Metadata["chunk_index"] = "0"
				}
				if err := vectors.AddDocument(t.Context(), doc); err != nil {
					t.Fatal(err)
				}
				want = 0
			}
			plan := f.plan(t)
			if len(plan.Groups) != want {
				t.Fatalf("plan=%+v", plan)
			}
			if want == 0 {
				if f.count(t) != 2 {
					t.Fatal("preview deleted vectors")
				}
				return
			}
			if plan.Groups[0].Canonical != canonical {
				t.Fatalf("canonical=%s", plan.Groups[0].Canonical)
			}
			completed, failed := applyGroups(t.Context(), f.db, f.vectors, plan, nil)
			if completed != 1 || len(failed) > 0 {
				t.Fatalf("merge=%d %+v", completed, failed)
			}
			meta, err := f.stm.GetMemoryMeta(canonical)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "archive" && !memory.IsMemoryArchived(meta) {
				t.Fatal("archive was revived")
			}
			if mode == "protected" && !meta.Protected {
				t.Fatal("protection lost")
			}
			if mode == "permanent" && !meta.KeepForever {
				t.Fatal("permanence lost")
			}
		})
	}
}

func appendRepairAudit(t *testing.T, f *repairFixture, changes record, remaining []string, at string) {
	t.Helper()
	rows, err := sqlSnapshot(t.Context(), f.db, []string{"b"})
	if err != nil {
		t.Fatal(err)
	}
	before := rows["memory_meta"][0]
	events := rows["memory_curation_events"]
	sort.Slice(events, func(i, j int) bool { return number(events[i], "id") < number(events[j], "id") })
	proof, err := json.Marshal(record{"events": events, "conflicts": rows["memory_conflicts"]})
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(proof)
	after := record{}
	for key, value := range before {
		after[key] = value
	}
	for key, value := range changes {
		after[key] = value
		f.exec(t, `UPDATE memory_meta SET `+key+`=? WHERE doc_id='b'`, value)
	}
	after["last_event_at"] = at
	f.exec(t, `UPDATE memory_meta SET last_event_at=? WHERE doc_id='b'`, at)
	audit, err := json.Marshal(record{"version": 2, "before": before, "after": after, "changes": changes, "remaining": remaining, "evidence": string(proof), "evidence_hash": hex.EncodeToString(hash[:])})
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO memory_curation_events(doc_id,action,actor,previous_status,new_status,reason,timestamp) VALUES ('b','metadata_repair','operator_repair',?,?,?,?)`, before["verification_status"], after["verification_status"], string(audit), at)
}

func TestMergeChecksCompletedPartialRepairEvidence(t *testing.T) {
	for _, mode := range []string{"complete", "partial", "bad_evidence", "later_curation", "later_conflict"} {
		t.Run(mode, func(t *testing.T) {
			f := newRepairFixture(t, "a", "b")
			f.curate(t, "b", "confirm")
			f.exec(t, `UPDATE memory_meta SET verification_status='unverified',source_type='memory_analysis',source_reliability=.7,extraction_confidence=.75 WHERE doc_id='b'`)
			appendRepairAudit(t, f, record{"verification_status": "confirmed"}, []string{"provenance needs backup"}, "2026-09-30T19:00:00Z")
			if mode != "partial" {
				appendRepairAudit(t, f, record{"source_type": "user", "source_reliability": .98, "extraction_confidence": .99}, nil, "2026-09-30T19:00:01Z")
			}
			switch mode {
			case "bad_evidence":
				f.exec(t, `UPDATE memory_curation_events SET reason=json_set(reason,'$.evidence_hash','invalid') WHERE id=(SELECT MAX(id) FROM memory_curation_events)`)
			case "later_curation":
				if err := f.stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "b", Action: "protect", Reason: "later decision"}, "user", false); err != nil {
					t.Fatal(err)
				}
			case "later_conflict":
				f.exec(t, `INSERT INTO memory_conflicts(doc_id_left,doc_id_right,conflict_key) VALUES ('b','other','editor')`)
			}
			plan := f.plan(t)
			if mode != "complete" {
				if len(plan.Groups) != 0 || len(plan.Review) == 0 {
					t.Fatalf("unproven history accepted: %+v", plan)
				}
				return
			}
			if len(plan.Groups) != 1 || plan.Groups[0].Canonical != "b" {
				t.Fatalf("completed partial repair rejected: %+v", plan)
			}
			if complete, failed := applyGroups(t.Context(), f.db, f.vectors, plan, nil); complete != 1 || len(failed) != 0 {
				t.Fatalf("merge=%d %+v", complete, failed)
			}
		})
	}
}

func TestMergeCancellationRetainsPendingWork(t *testing.T) {
	f := newRepairFixture(t, "a", "b")
	plan := f.plan(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if complete, failed := applyGroups(ctx, f.db, f.vectors, plan, func(stage string) error {
		if stage == "after_sql_commit" {
			cancel()
		}
		return nil
	}); complete != 0 || len(failed) != 1 || f.count(t) != 2 {
		t.Fatalf("cancellation=%d %+v", complete, failed)
	}
	preview := f.plan(t)
	if len(preview.Review) == 0 {
		t.Fatal("pending merge hidden from preview")
	}
	if complete, failed := applyGroups(t.Context(), f.db, f.vectors, plan, nil); complete != 1 || len(failed) != 0 {
		t.Fatalf("resume=%d %+v", complete, failed)
	}
}

func TestMergeHumanCurationSurvivesNewerAutomaticReview(t *testing.T) {
	for _, protected := range []bool{false, true} {
		t.Run(fmt.Sprint(protected), func(t *testing.T) {
			f := newRepairFixture(t, "a", "b")
			f.curate(t, "b", "confirm")
			if err := f.stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "a", Action: "confirm", Reason: "automatic review"}, "auto_curation", false); err != nil {
				t.Fatal(err)
			}
			f.exec(t, `UPDATE memory_meta SET last_reviewed_at='2099-01-01 00:00:00',last_event_at='2099-01-01 00:00:00' WHERE doc_id='a'`)
			f.exec(t, `UPDATE memory_curation_events SET timestamp='2099-01-01 00:00:00' WHERE doc_id='a'`)
			if protected {
				f.exec(t, `UPDATE memory_meta SET protected=1 WHERE doc_id='a'`)
			}
			plan := f.plan(t)
			if protected {
				if len(plan.Groups) != 0 || len(plan.Review) == 0 {
					t.Fatalf("protected automatic copy superseded human curation: %+v", plan)
				}
			} else if len(plan.Groups) != 1 || plan.Groups[0].Canonical != "b" {
				t.Fatalf("newer automatic review superseded human: %+v", plan)
			}
		})
	}
}

func TestMergeResumesInterruptedStagesWithoutCountingTwice(t *testing.T) {
	for _, stage := range []string{"before_sql_commit", "after_sql_commit", "before_vector_delete:b", "after_vector_delete:b", "before_cleanup_commit"} {
		t.Run(stage, func(t *testing.T) {
			f := newRepairFixture(t)
			f.exec(t, `UPDATE memory_meta SET access_count=2`)
			plan := f.plan(t)
			if len(plan.Groups) != 1 {
				t.Fatalf("plan=%+v", plan)
			}
			hook := func(current string) error {
				if current == stage {
					return errors.New("synthetic interruption")
				}
				return nil
			}
			if completed, failed := applyGroups(t.Context(), f.db, f.vectors, plan, hook); completed != 0 || len(failed) != 1 {
				t.Fatalf("interruption=%d %+v", completed, failed)
			}
			if stage != "before_sql_commit" {
				for _, id := range []string{"b", "c"} {
					meta, err := f.stm.GetMemoryMeta(id)
					if err != nil || !memory.IsMemoryArchived(meta) {
						t.Fatalf("missing tombstone %s: %+v %v", id, meta, err)
					}
				}
			}
			// Reopen both stores to prove recovery survives process state.
			f.db.Close()
			var err error
			f.db, err = openSQLite(f.path, true)
			if err != nil {
				t.Fatal(err)
			}
			completed, failed := applyGroups(t.Context(), f.db, f.vectors, plan, nil)
			if completed != 1 || len(failed) != 0 || f.count(t) != 1 {
				t.Fatalf("resume=%d %+v", completed, failed)
			}
			meta, err := f.stm.GetMemoryMeta("a")
			if err != nil || meta.AccessCount != 6 {
				t.Fatalf("double counted: %+v %v", meta, err)
			}
			if completed, failed := applyGroups(t.Context(), f.db, f.vectors, plan, nil); completed != 0 || len(failed) > 0 {
				t.Fatalf("repeat=%d %+v", completed, failed)
			}
		})
	}
}

func TestMergeStopsInterveningCurationAndVectorChanges(t *testing.T) {
	for _, mode := range []string{"before", "after_sql", "vector"} {
		t.Run(mode, func(t *testing.T) {
			f := newRepairFixture(t, "a", "b")
			plan := f.plan(t)
			var hook checkpoint
			if mode == "before" {
				f.exec(t, `UPDATE memory_meta SET protected=1 WHERE doc_id='b'`)
			} else {
				hook = func(stage string) error {
					if stage != "after_sql_commit" {
						return nil
					}
					if mode == "after_sql" {
						f.exec(t, `UPDATE memory_meta SET protected=1 WHERE doc_id='b'`)
					} else {
						vectors, err := openVectors(f.vectors)
						if err != nil {
							return err
						}
						doc, err := vectors.GetByID(t.Context(), "b")
						if err != nil {
							return err
						}
						doc.Content = "different fact"
						return vectors.AddDocument(t.Context(), doc)
					}
					return nil
				}
			}
			if completed, failed := applyGroups(t.Context(), f.db, f.vectors, plan, hook); completed != 0 || len(failed) != 1 || f.count(t) != 2 {
				t.Fatalf("unsafe merge=%d %+v", completed, failed)
			}
		})
	}
}

func TestOperatorCLIBacksUpRehearsesAndUsesActualLock(t *testing.T) {
	f := newRepairFixture(t, "a", "b")
	if err := f.stm.EnsureMemoryMetaWithDetails("missing", memory.MemoryMetaUpdate{SourceType: "memory_analysis"}); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(f.root, "reports", "preview.json")
	args := []string{"--db", f.path, "--vector-db", f.vectors, "--install-dir", f.root, "--plan", planPath}
	if err := run(t.Context(), args); err != nil {
		t.Fatal(err)
	}
	if f.count(t) != 2 {
		t.Fatal("preview mutated vectors")
	}
	lock := flock.New(filepath.Join(f.root, "aurago.lock"))
	held, err := lock.TryLock()
	if err != nil || !held {
		t.Fatal(err)
	}
	if err := run(t.Context(), append(args, "--apply")); err == nil {
		t.Fatal("application lock bypassed")
	}
	lock.Unlock()
	if err := run(t.Context(), append(args, "--apply")); err != nil {
		t.Fatal(err)
	}
	if f.count(t) != 1 {
		t.Fatal("live copy was not merged")
	}
	if err := run(t.Context(), append(args, "--apply")); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(f.root, "reports", "memory-merge-*", "backup", "memory.sqlite"))
	if err != nil || len(backups) != 2 {
		t.Fatalf("backups=%v %v", backups, err)
	}
	backup, err := openSQLite(backups[0], false)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var count int
	if err := backup.QueryRow(`SELECT COUNT(*) FROM memory_meta`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 && count != 2 {
		t.Fatalf("invalid backup count=%d", count)
	}
	if _, err := artifactPath(filepath.Join(f.root, "reports"), filepath.Join(f.root, "outside.json")); err == nil {
		t.Fatal("artifact escaped reports")
	}
	var plan mergePlan
	raw, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Groups) != 1 {
		t.Fatal("missing saved proof")
	}
	results, err := filepath.Glob(filepath.Join(f.root, "reports", "memory-merge-*", "result.json"))
	if err != nil || len(results) != 2 {
		t.Fatalf("results=%v %v", results, err)
	}
	for _, path := range results {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Review []reviewItem `json:"review_required"`
		}
		if err := json.Unmarshal(raw, &result); err != nil || len(result.Review) != 1 || !contains(result.Review[0].IDs, "missing") {
			t.Fatalf("unresolved preview evidence hidden after apply: %+v %v", result, err)
		}
	}
}
