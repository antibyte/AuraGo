package agent

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"aurago/internal/contacts"
	"aurago/internal/memory"
)

func TestSyncContactsToKnowledgeGraphPrunesStaleBelongsToEdges(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	contactsDB, err := contacts.InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer contactsDB.Close()

	if _, err := contactsDB.Exec(`
		INSERT INTO contacts (id, name, email, phone, mobile, address, relationship, notes, birthday, reminder, created_at, updated_at)
		VALUES ('1', 'Alice Example', '', '', '', '', 'Old Corp', '', '', '', datetime('now'), datetime('now'))
	`); err != nil {
		t.Fatalf("insert contact: %v", err)
	}

	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	defer kg.Close()

	if err := SyncContactsToKnowledgeGraph(context.Background(), contactsDB, kg, logger); err != nil {
		t.Fatalf("initial contact sync: %v", err)
	}
	if _, err := contactsDB.Exec(`UPDATE contacts SET relationship='New Corp' WHERE id='1'`); err != nil {
		t.Fatal(err)
	}
	if err := SyncContactsToKnowledgeGraph(context.Background(), contactsDB, kg, logger); err != nil {
		t.Fatal(err)
	}

	if _, err := kg.GetNode("org_new_corp"); err != nil {
		t.Fatalf("GetNode org_new_corp: %v", err)
	}

	edges, err := kg.GetImportantEdges(20, []string{"contact_1"})
	if err != nil {
		t.Fatalf("GetImportantEdges: %v", err)
	}
	var hasNew, hasOld bool
	for _, edge := range edges {
		if edge.Relation != "belongs_to" {
			continue
		}
		switch edge.Target {
		case "org_new_corp":
			hasNew = true
		case "org_old_corp":
			hasOld = true
		}
	}
	if !hasNew {
		t.Fatal("expected contact to belong to org_new_corp after relationship change")
	}
	if hasOld {
		t.Fatal("expected stale belongs_to edge to org_old_corp to be pruned")
	}
}
