package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aurago/internal/sqlconnections"
)

func TestHandleSQLConnectionsListAfterUpdate(t *testing.T) {
	t.Parallel()

	metaDB, err := sqlconnections.InitDB(filepath.Join(t.TempDir(), "sqlconnections.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer metaDB.Close()

	// Create initial connection
	id, err := sqlconnections.Create(
		metaDB,
		"Test Conn",
		"postgres",
		"localhost",
		5432,
		"testdb",
		"desc",
		true,
		false,
		false,
		false,
		"vault-key",
		"disable",
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	s := &Server{
		SQLConnectionsDB: metaDB,
	}

	// Update the connection
	rec, err := sqlconnections.GetByID(metaDB, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	err = sqlconnections.Update(metaDB, id, "Updated Name", rec.Driver, rec.Host, rec.Port, rec.DatabaseName, rec.Description,
		rec.AllowRead, true, true, true, rec.VaultSecretID, rec.SSLMode)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// List via API
	req := httptest.NewRequest(http.MethodGet, "/api/sql-connections", nil)
	rec2 := httptest.NewRecorder()
	handleSQLConnections(s)(rec2, req)

	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec2.Code, http.StatusOK, rec2.Body.String())
	}

	var list []map[string]interface{}
	if err := json.Unmarshal(rec2.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(list))
	}

	item := list[0]
	if item["name"] != "Updated Name" {
		t.Errorf("name = %v, want %v", item["name"], "Updated Name")
	}
	if item["allow_write"] != true {
		t.Errorf("allow_write = %v, want true", item["allow_write"])
	}
	if item["allow_change"] != true {
		t.Errorf("allow_change = %v, want true", item["allow_change"])
	}
	if item["allow_delete"] != true {
		t.Errorf("allow_delete = %v, want true", item["allow_delete"])
	}
	if item["vault_secret_id"] != "vault-key" {
		t.Errorf("vault_secret_id = %v, want vault-key", item["vault_secret_id"])
	}
}
