package agent

import (
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/contacts"
	"aurago/internal/memory"
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
	stm, err := memory.NewSQLiteMemory(filepath.Join(t.TempDir(), "stm.db"), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
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
	out, handled := dispatchComm(t.Context(), call, &DispatchContext{Cfg: cfg, Logger: slog.Default(), ContactsDB: db, KG: kg, ShortTermMem: stm})
	if !handled {
		t.Fatalf("native delete failed: %s", out)
	}
	var result struct {
		Status         string `json:"status"`
		DeletionJobID  string `json:"deletion_job_id"`
		DeletionStatus string `json:"deletion_status"`
		ContactDeleted bool   `json:"contact_deleted"`
		RetainedCount  int    `json:"retained_count"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &result); err != nil {
		t.Fatalf("decode native delete result %q: %v", out, err)
	}
	if result.Status != "success" || result.DeletionJobID == "" || result.DeletionStatus != memory.PrivacyDeletionRetainedReview || !result.ContactDeleted || result.RetainedCount != 1 {
		t.Fatalf("native deletion response lacks completed job state: %+v", result)
	}
	count, err := kg.CountContactSource(t.Context(), "contact_"+id)
	if err != nil || count != 1 {
		t.Fatalf("native delete removed knowledge: %d %v", count, err)
	}
}

func TestNativeContactDeleteFailureReleasesOwnedFenceAndKeepsContact(t *testing.T) {
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
	stm, err := memory.NewSQLiteMemory(filepath.Join(t.TempDir(), "stm.db"), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	id, err := contacts.Create(db, contacts.Contact{Name: "Private Contact"})
	if err != nil {
		t.Fatal(err)
	}
	if err := kg.SyncContact(t.Context(), "contact_"+id, "Private Contact", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER reject_native_contact_delete BEFORE DELETE ON contacts BEGIN SELECT RAISE(ABORT,'synthetic contact delete failure'); END`); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Tools.Contacts.Enabled = true
	call := ToolCall{Action: "address_book", Params: map[string]interface{}{"operation": "delete", "id": id}}
	out, handled := dispatchComm(t.Context(), call, &DispatchContext{Cfg: cfg, Logger: slog.Default(), ContactsDB: db, KG: kg, ShortTermMem: stm})
	if !handled {
		t.Fatalf("native delete was not handled: %s", out)
	}
	var result struct {
		Status         string `json:"status"`
		ErrorCode      string `json:"error_code"`
		DeletionJobID  string `json:"deletion_job_id"`
		DeletionStatus string `json:"deletion_status"`
		ContactDeleted bool   `json:"contact_deleted"`
		OutcomeKnown   bool   `json:"outcome_known"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &result); err != nil {
		t.Fatalf("decode native delete failure result %q: %v", out, err)
	}
	if result.Status != "error" || result.ErrorCode != "contact_delete_failed" || result.DeletionJobID == "" || result.DeletionStatus != memory.PrivacyDeletionAborted || result.ContactDeleted || !result.OutcomeKnown {
		t.Fatalf("native deletion failure has unsafe/missing status: %+v", result)
	}
	if strings.Contains(out, "synthetic contact delete failure") {
		t.Fatalf("native response leaked database error: %s", out)
	}
	if _, err := contacts.GetByIDContext(t.Context(), db, id); err != nil {
		t.Fatalf("failed native delete removed contact: %v", err)
	}
	fenced, err := kg.IsContactSourceDeleted(t.Context(), "contact_"+id)
	if err != nil || fenced {
		t.Fatalf("failed native delete retained its own fence: fenced=%v err=%v", fenced, err)
	}
	if _, err := db.Exec(`DROP TRIGGER reject_native_contact_delete`); err != nil {
		t.Fatal(err)
	}
	contact, err := contacts.GetByIDContext(t.Context(), db, id)
	if err != nil {
		t.Fatal(err)
	}
	contact.Name = "Updated Contact"
	if err := contacts.UpdateContext(t.Context(), db, *contact); err != nil {
		t.Fatalf("contact update remained blocked after failed native delete: %v", err)
	}
	if err := kg.SyncContact(t.Context(), "contact_"+id, contact.Name, nil); err != nil {
		t.Fatalf("contact sync remained blocked after failed native delete: %v", err)
	}
}

func TestNativeContactDeleteRequiresPrivacyStore(t *testing.T) {
	db, err := contacts.InitDB(filepath.Join(t.TempDir(), "contacts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, err := contacts.Create(db, contacts.Contact{Name: "Private Contact"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Tools.Contacts.Enabled = true
	call := ToolCall{Action: "address_book", Params: map[string]interface{}{"operation": "delete", "id": id}}
	out, handled := dispatchComm(t.Context(), call, &DispatchContext{Cfg: cfg, Logger: slog.Default(), ContactsDB: db})
	if !handled {
		t.Fatalf("native delete without privacy storage was not handled: %s", out)
	}
	var result struct {
		Status    string `json:"status"`
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &result); err != nil {
		t.Fatalf("decode missing-store result %q: %v", out, err)
	}
	if result.Status != "error" || result.ErrorCode != "privacy_deletion_unavailable" {
		t.Fatalf("delete without privacy store was not rejected safely: %+v", result)
	}
	if _, err := contacts.GetByIDContext(t.Context(), db, id); err != nil {
		t.Fatalf("privacy-store rejection deleted contact: %v", err)
	}
}

func TestNativeContactDeleteRejectsInitialContactsDatabaseFailure(t *testing.T) {
	db, err := contacts.InitDB(filepath.Join(t.TempDir(), "contacts.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := contacts.Create(db, contacts.Contact{Name: "Private Contact"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	stm, err := memory.NewSQLiteMemory(filepath.Join(t.TempDir(), "stm.db"), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	cfg := &config.Config{}
	cfg.Tools.Contacts.Enabled = true
	call := ToolCall{Action: "address_book", Params: map[string]interface{}{"operation": "delete", "id": "synthetic-contact"}}
	out, handled := dispatchComm(t.Context(), call, &DispatchContext{Cfg: cfg, Logger: slog.Default(), ContactsDB: db, ShortTermMem: stm})
	if !handled {
		t.Fatalf("native delete with unreadable contacts database was not handled: %s", out)
	}
	var result struct {
		Status         string `json:"status"`
		ErrorCode      string `json:"error_code"`
		DeletionJobID  string `json:"deletion_job_id"`
		DeletionStatus string `json:"deletion_status"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &result); err != nil {
		t.Fatalf("decode failed-read result %q: %v", out, err)
	}
	if result.Status != "error" || result.ErrorCode != "contact_delete_not_accepted" || result.DeletionJobID != "" || result.DeletionStatus != "" {
		t.Fatalf("unaccepted native delete was reported as an accepted reconciliation: %+v", result)
	}
	if strings.Contains(out, "database is closed") {
		t.Fatalf("native result leaked database error: %s", out)
	}
	j, err := stm.ListPrivacyDeletionJobs(t.Context(), 10)
	if err != nil || len(j) != 0 {
		t.Fatalf("failed initial contact read created a privacy job: %+v err=%v", j, err)
	}
}
