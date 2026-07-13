package manus

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestLedgerPersistsOnlyTrackedTaskMetadataAcrossRestart(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "manus.db")
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger() error = %v", err)
	}
	record := TaskRecord{
		TaskID:       "task-1",
		Title:        "Research",
		TaskURL:      "https://manus.im/app/task-1",
		Status:       "running",
		AgentProfile: "manus-1.6",
		CreditUsage:  7,
		LastCursor:   "cursor-1",
	}
	if err := ledger.Upsert(context.Background(), record); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}

	ledger, err = OpenLedger(path)
	if err != nil {
		t.Fatalf("reopen ledger: %v", err)
	}
	defer ledger.Close()
	got, ok, err := ledger.Get(context.Background(), "task-1")
	if err != nil || !ok {
		t.Fatalf("Get() = %#v, %t, %v", got, ok, err)
	}
	if got.Title != record.Title || got.LastCursor != record.LastCursor || got.CreditUsage != record.CreditUsage {
		t.Fatalf("Get() = %#v, want %#v", got, record)
	}
	if _, ok, err := ledger.Get(context.Background(), "foreign-task"); err != nil || ok {
		t.Fatalf("foreign Get() ok=%t err=%v", ok, err)
	}
}

func TestLedgerSchemaDoesNotPersistMessagesOrAttachments(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "manus.db")
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`PRAGMA table_info(manus_tasks)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		if name == "messages" || name == "attachments" || name == "raw_json" {
			t.Fatalf("sensitive content column %q must not exist", name)
		}
	}
}
