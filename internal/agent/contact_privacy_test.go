package agent

import (
	"aurago/internal/config"
	"aurago/internal/contacts"
	"aurago/internal/memory"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeContactDeleteRetainsDerivedKnowledge(t *testing.T) {
	db, err := contacts.InitDB(filepath.Join(t.TempDir(), "contacts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	kg, err := memory.NewKnowledgeGraph(":memory:", "", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer kg.Close()
	id, err := contacts.Create(db, contacts.Contact{Name: "Private Contact"})
	if err != nil {
		t.Fatal(err)
	}
	if err = kg.SyncContact(t.Context(), "contact_"+id, "Private Contact", nil); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Tools.Contacts.Enabled = true
	call := ToolCall{Action: "address_book", Params: map[string]interface{}{"operation": "delete", "id": id}}
	out, handled := dispatchComm(context.Background(), call, &DispatchContext{Cfg: cfg, Logger: slog.Default(), ContactsDB: db, KG: kg})
	if !handled || !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("native delete failed: %s", out)
	}
	count, err := kg.CountContactSource(t.Context(), "contact_"+id)
	if err != nil || count != 1 {
		t.Fatalf("native delete removed knowledge: %d %v", count, err)
	}
}
