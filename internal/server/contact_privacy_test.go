package server

import (
	"aurago/internal/contacts"
	"aurago/internal/memory"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestContactDeleteKeepsKnowledgeUnlessExplicitlySelected(t *testing.T) {
	for _, optin := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "optin"}[optin], func(t *testing.T) {
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
			if err = kg.SyncContact(t.Context(), "contact_"+id, "Private Contact", map[string]string{"email": "private@example.invalid"}); err != nil {
				t.Fatal(err)
			}
			url := "/api/contacts/" + id
			if optin {
				url += "?delete_memories=true"
			}
			recorder := httptest.NewRecorder()
			handleContactByID(&Server{ContactsDB: db, KG: kg, ShortTermMem: stm, Logger: slog.Default()})(recorder, httptest.NewRequest(http.MethodDelete, url, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("delete status %d: %s", recorder.Code, recorder.Body.String())
			}
			var response struct {
				ContactDeleted bool `json:"contact_deleted"`
				Deleted        int  `json:"deleted_graph_items"`
				Retained       int  `json:"retained_graph_items"`
			}
			if err = json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if !response.ContactDeleted || optin && response.Deleted != 1 || !optin && (response.Deleted != 0 || response.Retained != 1) {
				t.Fatalf("scope ignored: %+v", response)
			}
			if _, err = contacts.GetByID(db, id); err == nil {
				t.Fatal("contact retained")
			}
			if err = kg.SyncContact(t.Context(), "contact_"+id, "Late Contact", nil); err == nil {
				t.Fatal("delayed contact resurrected")
			}
		})
	}
}
