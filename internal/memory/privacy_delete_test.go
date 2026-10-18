package memory

import (
	"context"
	"encoding/json"
	"errors"
	chromem "github.com/philippgille/chromem-go"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type privacyDeleteVectors struct {
	VectorDB
	deleted []string
}

func (v *privacyDeleteVectors) IsDisabled() bool { return false }
func (v *privacyDeleteVectors) DeleteDocument(id string) error {
	v.deleted = append(v.deleted, id)
	return nil
}

func TestPrivacyPurgeFencesDelayedWritersAndSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.InitPlanTables(); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{`INSERT INTO chat_sessions(id) VALUES('private')`, `INSERT INTO plans(id,session_id,title) VALUES('p','private','secret')`, `INSERT INTO plan_tasks(id,plan_id,title,task_order) VALUES('t','p','secret',0)`, `INSERT INTO plan_events(plan_id,event_type,message) VALUES('p','created','secret')`} {
		if _, err = s.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.PurgeChatSession("private"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"plans", "plan_tasks", "plan_events"} {
		var count int
		if err = s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s: count=%d err=%v", table, count, err)
		}
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, statement := range []string{`INSERT INTO chat_sessions(id) VALUES('private')`, `INSERT INTO messages(session_id,role,content) VALUES('private','assistant','late')`, `INSERT INTO archived_messages(session_id,role,content) VALUES('private','assistant','late')`, `INSERT INTO plans(id,session_id,title) VALUES('late','private','secret')`} {
		if _, err = s.db.Exec(statement); err == nil {
			t.Fatalf("late writer accepted: %s", statement)
		}
	}
	if deleted, err := s.IsSessionDeleted("private"); err != nil || !deleted {
		t.Fatalf("fence missing: %v %v", deleted, err)
	}
}

func TestPrivacyDeleteRetainsSharedUnknownAndProtectedMemories(t *testing.T) {
	s := newTestSTM(t)
	for _, id := range []string{"exclusive", "shared", "unknown", "protected"} {
		if err := s.UpsertMemoryMetaWithDetails(id, MemoryMetaUpdate{SourceType: "memory_analysis"}); err != nil {
			t.Fatal(err)
		}
		if err := s.RecordMemoryExtractionSource(id, "memory_analysis", "private"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RecordMemoryExtractionSource("shared", "memory_analysis", "other"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE memory_meta SET source_type='unknown' WHERE doc_id='unknown'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE memory_meta SET protected=1 WHERE doc_id='protected'`); err != nil {
		t.Fatal(err)
	}
	v := &privacyDeleteVectors{}
	report, err := s.DeleteChatSessionWithMemories(context.Background(), "private", v, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.DeletedMemories != 1 || report.RetainedMemories != 3 || len(v.deleted) != 1 || v.deleted[0] != "exclusive" {
		t.Fatalf("unsafe deletion: %+v ids=%v", report, v.deleted)
	}
	sources, err := s.GetMemoryExtractionSources("shared")
	if err != nil || len(sources) != 2 {
		t.Fatalf("lost shared provenance: %+v %v", sources, err)
	}
}

func TestNoteProtectionRecheckedDuringCurationAndRetention(t *testing.T) {
	s := newTestNotesDB(t)
	id, err := s.AddNote("general", "private", "content", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`UPDATE notes SET updated_at='2020-01-01T00:00:00Z' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	plan, err := s.BuildNotesCurationPlan(NotesCurationOptions{Now: time.Now()})
	if err != nil || len(plan.AutoArchive) != 1 {
		t.Fatalf("plan %+v %v", plan, err)
	}
	if err = s.SetNoteProtection(id, true, true); err != nil {
		t.Fatal(err)
	}
	if err = s.ApplyNoteCurationAction(plan.AutoArchive[0], "test", false); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteNote(id); err == nil {
		t.Fatal("deleted protected note")
	}
	if err = s.UpdateNote(id, "changed", "", "", 0, ""); err == nil {
		t.Fatal("changed protected note")
	}
	if _, err = s.db.Exec(`UPDATE notes SET done=1,updated_at='2020-01-01T00:00:00Z' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if count, err := s.DeleteOldDoneNotes(30); err != nil || count != 0 {
		t.Fatalf("retention removed protected note: %d %v", count, err)
	}
	var archived bool
	if err = s.db.QueryRow(`SELECT archived FROM notes WHERE id=?`, id).Scan(&archived); err != nil || archived {
		t.Fatalf("curation ignored protection %v %v", archived, err)
	}
	if err = s.SetNoteProtection(id, false, false); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteNote(id); err != nil {
		t.Fatal(err)
	}
}

func TestPrivacyGraphRetainsProtectedSharedAndMergedEvidence(t *testing.T) {
	kg := contactSyncTestGraph(t)
	for _, statement := range []string{
		`INSERT INTO kg_nodes(id,label,properties) VALUES('owned','private','{"source":"memory_analysis","session_id":"private","type":"person"}')`,
		`INSERT INTO kg_nodes(id,label,properties) VALUES('merged','private','{"source":"memory_analysis","session_id":"private","type":"person","foreign_note":"keep"}')`,
		`INSERT INTO kg_nodes(id,label,properties) VALUES('other','other','{}')`,
		`INSERT INTO kg_edges(source,target,relation,properties) VALUES('owned','other','knows','{"session_id":"private","protected":"true"}')`,
		`INSERT INTO kg_edges(source,target,relation,properties) VALUES('merged','other','knows','{"session_id":"private"}')`,
		`INSERT INTO kg_claims(id,subject_id,predicate,object_id,session_id) VALUES('foreign','merged','knows','other','other-session')`,
	} {
		if _, err := kg.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	deleted, retained, err := kg.deleteExclusiveSessionGraph(t.Context(), "private")
	if err != nil || deleted != 0 || retained < 4 {
		t.Fatalf("unsafe graph deletion %d %d %v", deleted, retained, err)
	}
	var nodes, edges int
	if err = kg.db.QueryRow(`SELECT COUNT(*) FROM kg_nodes`).Scan(&nodes); err != nil {
		t.Fatal(err)
	}
	if err = kg.db.QueryRow(`SELECT COUNT(*) FROM kg_edges`).Scan(&edges); err != nil || nodes != 3 || edges != 2 {
		t.Fatalf("shared graph lost %d %d %v", nodes, edges, err)
	}
}

func TestPrivacyContactDeletePreservesForeignFieldsAndRejectsDelayedSync(t *testing.T) {
	kg := contactSyncTestGraph(t)
	if err := kg.SyncContact(t.Context(), "contact_private", "Private Contact", map[string]string{"email": "private@example.invalid"}); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_nodes SET properties=json_set(properties,'$.foreign_note','keep') WHERE id='contact_private'`); err != nil {
		t.Fatal(err)
	}
	deleted, retained, err := kg.DeleteContactSource(t.Context(), "contact_private")
	if err != nil || deleted != 0 || retained != 1 {
		t.Fatalf("foreign contact data was deleted: %d %d %v", deleted, retained, err)
	}
	if err = kg.SyncContact(t.Context(), "contact_private", "Late Contact", nil); err == nil {
		t.Fatal("deleted contact was restored by delayed sync")
	}
	var value string
	if err = kg.db.QueryRow(`SELECT json_extract(properties,'$.foreign_note') FROM kg_nodes WHERE id='contact_private'`).Scan(&value); err != nil || value != "keep" {
		t.Fatalf("lost foreign field: %q %v", value, err)
	}
}

func TestChatDeleteScrubsRawGraphTranscriptWithoutRemovingKnowledge(t *testing.T) {
	s := newTestSTM(t)
	kg := contactSyncTestGraph(t)
	for _, sql := range []string{`INSERT INTO kg_nodes(id,label,properties) VALUES('person','Derived fact','{}')`, `INSERT INTO kg_evidence(id,session_id,raw_text,source_message_id,source_uri,actor) VALUES('e','private','raw private conversation','message-1','private-uri','private actor')`, `INSERT INTO kg_claims(id,subject_id,predicate,object_literal,session_id,source_message_id,evidence_id) VALUES('c','person','likes','Go','private','message-1','e')`} {
		if _, err := kg.db.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	report, err := s.DeleteChatSessionWithMemories(t.Context(), "private", nil, kg, false)
	if err != nil || !report.TranscriptDeleted {
		t.Fatalf("delete failed: %+v %v", report, err)
	}
	var raw, session, message, uri, actor string
	if err = kg.db.QueryRow(`SELECT raw_text,session_id,source_message_id,source_uri,actor FROM kg_evidence WHERE id='e'`).Scan(&raw, &session, &message, &uri, &actor); err != nil || raw+session+message+uri+actor != "" {
		t.Fatalf("transcript copy retained: %v", err)
	}
	var fact string
	if err = kg.db.QueryRow(`SELECT object_literal FROM kg_claims WHERE id='c'`).Scan(&fact); err != nil || fact != "Go" {
		t.Fatalf("derived fact lost: %q %v", fact, err)
	}
}

func TestPrivacySessionDeleteRetainsContactProvenance(t *testing.T) {
	kg := contactSyncTestGraph(t)
	if _, err := kg.db.Exec(`INSERT INTO kg_nodes(id,label,properties) VALUES('mixed','Mixed source','{"source":"memory_analysis","session_id":"private","contact_sync_id":"contact_other","type":"person","email":"kept@example.invalid"}')`); err != nil {
		t.Fatal(err)
	}
	deleted, retained, err := kg.deleteExclusiveSessionGraph(t.Context(), "private")
	if err != nil || deleted != 0 || retained != 1 {
		t.Fatalf("contact provenance lost: %d %d %v", deleted, retained, err)
	}
}

func TestPrivacyPurgeRemovesQueuedSourceEvenWithoutRetryService(t *testing.T) {
	s := setupLearnedRulesTest(t)
	details := &MemoryMetaUpdate{SourceSessionID: "private", SourceType: "memory_analysis"}
	if err := s.EnqueuePendingMemoryWrite(PendingMemoryWrite{Concept: "private fact", Content: "private transcript", Metadata: details}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.EnqueuePendingMemoryWrite(PendingMemoryWrite{Concept: "unknown fact", Content: "retained unknown provenance"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.PurgeChatSession("private"); err != nil {
		t.Fatal(err)
	}
	var own, unknown int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_writes WHERE content='private transcript'`).Scan(&own); err != nil || own != 0 {
		t.Fatalf("queued private source retained: %d %v", own, err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_writes WHERE content='retained unknown provenance'`).Scan(&unknown); err != nil || unknown != 1 {
		t.Fatalf("unknown provenance lost: %d %v", unknown, err)
	}
	if err := s.EnqueuePendingMemoryWrite(PendingMemoryWrite{Concept: "late fact", Content: "private transcript", Metadata: details}, nil); !errors.Is(err, ErrSessionDeleted) {
		t.Fatalf("late enqueue was accepted: %v", err)
	}
}

func TestPrivacyPurgeRecognizesLegacyQueueEnvelopeAndFencesLateEnqueue(t *testing.T) {
	s := setupLearnedRulesTest(t)
	for _, content := range []string{"source:memory_analysis session:private role:unknown", "source:memory_analysis session:other", "source:memory_analysis session:private\nquoted factual paragraph"} {
		if _, err := s.db.Exec(`INSERT INTO pending_memory_writes(concept,content) VALUES('legacy fact',?)`, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.PurgeChatSession("private"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_writes`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("legacy purge scope: %d %v", count, err)
	}
	if err := s.EnqueuePendingMemoryWrite(PendingMemoryWrite{Concept: "late fact", Content: "source:memory_analysis session:private"}, nil); !errors.Is(err, ErrSessionDeleted) {
		t.Fatalf("legacy late enqueue accepted: %v", err)
	}
	if err := s.InitPendingMemoryWritesTable(); err != nil {
		t.Fatal(err)
	}
	var payload string
	if err := s.db.QueryRow(`SELECT metadata_json FROM pending_memory_writes WHERE content='source:memory_analysis session:other'`).Scan(&payload); err != nil || !strings.Contains(payload, `"SourceSessionID":"other"`) {
		t.Fatalf("legacy source not normalized: %s %v", payload, err)
	}
}

func TestGraphTranscriptPurgeAnonymizesExactPropertiesAndSemanticIndex(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.enableSemanticSearchWithCollection(chromem.NewDB(), func(context.Context, string) ([]float32, error) { return []float32{1, 0}, nil }, nil); err != nil {
		t.Fatal(err)
	}
	waitForSemanticReindexIdle(t, kg)
	for _, query := range []string{
		`INSERT INTO kg_nodes(id,label,properties,protected) VALUES('private-node','Preserved fact','{"session_id":"private-session","source_sessions":"[\"private-session\",\"other-session\"]","foreign_note":"preserve","source":"auto_extraction"}',1)`,
		`INSERT INTO kg_nodes(id,label,properties) VALUES('other-node','Other fact','{"session_id":"other-session"}')`,
		`INSERT INTO kg_edges(source,target,relation,properties) VALUES('private-node','other-node','knows','{"session_id":"private-session","source_sessions":["other-session","private-session"],"protected":"true","fact":"preserve"}')`,
	} {
		if _, err := kg.db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if err := kg.ScrubSessionTranscriptEvidence(t.Context(), "private-session"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"kg_nodes", "kg_edges"} {
		var payload string
		if err := kg.db.QueryRow(`SELECT properties FROM ` + table + ` WHERE instr(properties,'source_sessions')>0`).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload, "private-session") || !strings.Contains(payload, "other-session") || !strings.Contains(payload, "deleted:") || !strings.Contains(payload, "preserve") {
			t.Fatalf("unsafe source scrub: %s", payload)
		}
		var properties map[string]string
		if err := json.Unmarshal([]byte(payload), &properties); err != nil {
			t.Fatal(err)
		}
		var sessions []string
		if err := json.Unmarshal([]byte(properties["source_sessions"]), &sessions); err != nil || len(sessions) != 2 {
			t.Fatalf("shared provenance lost: %v %v", sessions, err)
		}
	}
	var protected bool
	if err := kg.db.QueryRow(`SELECT protected FROM kg_nodes WHERE id='private-node'`).Scan(&protected); err != nil || !protected {
		t.Fatalf("protection changed: %v %v", protected, err)
	}
	for _, id := range []string{"private-node", "edge://private-node\x00other-node\x00knows"} {
		if _, err := kg.semanticIndex().Collection.GetByID(t.Context(), id); err == nil {
			t.Fatalf("stale semantic document %q survived the privacy scrub", id)
		}
	}
	if err := kg.RunSemanticReindexContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"private-node", "edge://private-node\x00other-node\x00knows"} {
		doc, err := kg.semanticIndex().Collection.GetByID(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(doc.Content, "private-session") || strings.Contains(doc.Content, "source_sessions") {
			t.Fatalf("semantic provenance retained: %s", doc.Content)
		}
	}
	deleted, _, err := kg.deleteExclusiveSessionGraph(t.Context(), "other-session")
	if err != nil || deleted != 0 {
		t.Fatalf("anonymization made shared data exclusive: %d %v", deleted, err)
	}
}
