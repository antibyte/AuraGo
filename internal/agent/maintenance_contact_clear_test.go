package agent

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/contacts"
	"aurago/internal/memory"
)

func TestAuditContactsSyncClearsRemovedRelationshipAndFields(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := contacts.InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO contacts(id,name,email,phone,mobile,address,relationship,notes,birthday,reminder,created_at,updated_at)
	 VALUES('synthetic','Synthetic Contact','synthetic@example.invalid','','','','Old Org','','','',datetime('now'),datetime('now'))`); err != nil {
		t.Fatal(err)
	}
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer kg.Close()
	if err := SyncContactsToKnowledgeGraph(context.Background(), db, kg, logger); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE contacts SET email='',relationship='' WHERE id='synthetic'`); err != nil {
		t.Fatal(err)
	}
	if err := SyncContactsToKnowledgeGraph(context.Background(), db, kg, logger); err != nil {
		t.Fatal(err)
	}
	node, err := kg.GetNode("contact_synthetic")
	if err != nil {
		t.Fatal(err)
	}
	edges, err := kg.GetImportantEdges(20, []string{"contact_synthetic"})
	if err != nil || node.Properties["email"] != "" || node.Properties["relationship"] != "" || len(edges) != 0 {
		t.Fatalf("stale contact data: node=%+v edges=%v err=%v", node, edges, err)
	}
	if _, err := db.Exec(`DELETE FROM contacts WHERE id='synthetic'`); err != nil {
		t.Fatal(err)
	}
	if err := SyncContactsToKnowledgeGraph(context.Background(), db, kg, logger); err != nil {
		t.Fatal(err)
	}
	if node, err := kg.GetNode("contact_synthetic"); err != nil || node == nil {
		t.Fatalf("deleted contact triggered unreviewed graph deletion: %+v err=%v", node, err)
	}
}
