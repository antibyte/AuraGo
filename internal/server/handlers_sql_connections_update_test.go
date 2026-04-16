package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aurago/internal/sqlconnections"
)

func TestHandleSQLConnectionUpdate(t *testing.T) {
	t.Parallel()

	metaDB, err := sqlconnections.InitDB(filepath.Join(t.TempDir(), "sqlconnections.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer metaDB.Close()

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
		"",
		"disable",
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	s := &Server{
		SQLConnectionsDB: metaDB,
	}

	payload := map[string]interface{}{
		"name":          "Updated Conn",
		"driver":        "mysql",
		"host":          "newhost",
		"port":          3306,
		"database_name": "newdb",
		"description":   "updated",
		"ssl_mode":      "require",
		"allow_read":    true,
		"allow_write":   true,
		"allow_change":  true,
		"allow_delete":  true,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/sql-connections/"+id, bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handleSQLConnectionByID(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	rec2, err := sqlconnections.GetByID(metaDB, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if rec2.Name != "Updated Conn" {
		t.Errorf("name = %q, want %q", rec2.Name, "Updated Conn")
	}
	if rec2.Driver != "mysql" {
		t.Errorf("driver = %q, want %q", rec2.Driver, "mysql")
	}
	if rec2.Host != "newhost" {
		t.Errorf("host = %q, want %q", rec2.Host, "newhost")
	}
	if rec2.Port != 3306 {
		t.Errorf("port = %d, want %d", rec2.Port, 3306)
	}
	if rec2.DatabaseName != "newdb" {
		t.Errorf("database = %q, want %q", rec2.DatabaseName, "newdb")
	}
	if rec2.SSLMode != "require" {
		t.Errorf("ssl_mode = %q, want %q", rec2.SSLMode, "require")
	}
	if !rec2.AllowRead || !rec2.AllowWrite || !rec2.AllowChange || !rec2.AllowDelete {
		t.Errorf("permissions not updated correctly: R=%v W=%v C=%v D=%v", rec2.AllowRead, rec2.AllowWrite, rec2.AllowChange, rec2.AllowDelete)
	}
}

func TestHandleSQLConnectionUpdatePermissionsToFalse(t *testing.T) {
	t.Parallel()

	metaDB, err := sqlconnections.InitDB(filepath.Join(t.TempDir(), "sqlconnections.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer metaDB.Close()

	id, err := sqlconnections.Create(
		metaDB,
		"Perm Test",
		"postgres",
		"localhost",
		5432,
		"testdb",
		"desc",
		true,
		true,
		true,
		true,
		"",
		"disable",
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	s := &Server{
		SQLConnectionsDB: metaDB,
	}

	payload := map[string]interface{}{
		"name":          "Perm Test",
		"driver":        "postgres",
		"host":          "localhost",
		"port":          5432,
		"database_name": "testdb",
		"description":   "desc",
		"ssl_mode":      "disable",
		"allow_read":    false,
		"allow_write":   false,
		"allow_change":  false,
		"allow_delete":  false,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/sql-connections/"+id, bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handleSQLConnectionByID(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	rec2, err := sqlconnections.GetByID(metaDB, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if rec2.AllowRead || rec2.AllowWrite || rec2.AllowChange || rec2.AllowDelete {
		t.Errorf("permissions should all be false: R=%v W=%v C=%v D=%v", rec2.AllowRead, rec2.AllowWrite, rec2.AllowChange, rec2.AllowDelete)
	}
}

func TestHandleSQLConnectionCreate(t *testing.T) {
	t.Parallel()

	metaDB, err := sqlconnections.InitDB(filepath.Join(t.TempDir(), "sqlconnections.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer metaDB.Close()

	s := &Server{
		SQLConnectionsDB: metaDB,
	}

	payload := map[string]interface{}{
		"name":          "New Conn",
		"driver":        "sqlite",
		"host":          "",
		"port":          0,
		"database_name": "/tmp/test.db",
		"description":   "sqlite conn",
		"ssl_mode":      "",
		"allow_read":    true,
		"allow_write":   false,
		"allow_change":  false,
		"allow_delete":  false,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/sql-connections", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handleSQLConnections(s)(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var result map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	id := result["id"]
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	rec2, err := sqlconnections.GetByID(metaDB, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if rec2.Name != "New Conn" {
		t.Errorf("name = %q, want %q", rec2.Name, "New Conn")
	}
	if rec2.Driver != "sqlite" {
		t.Errorf("driver = %q, want %q", rec2.Driver, "sqlite")
	}
	if rec2.DatabaseName != "/tmp/test.db" {
		t.Errorf("database = %q, want %q", rec2.DatabaseName, "/tmp/test.db")
	}
}

func TestHandleSQLConnectionCreateUsesPermissionDefaults(t *testing.T) {
	t.Parallel()

	metaDB, err := sqlconnections.InitDB(filepath.Join(t.TempDir(), "sqlconnections.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer metaDB.Close()

	s := &Server{
		SQLConnectionsDB: metaDB,
	}

	payload := map[string]interface{}{
		"name":          "Defaulted Conn",
		"driver":        "sqlite",
		"host":          "",
		"port":          0,
		"database_name": "/tmp/defaulted.db",
		"description":   "sqlite conn",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/sql-connections", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handleSQLConnections(s)(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var result map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	stored, err := sqlconnections.GetByID(metaDB, result["id"])
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if !stored.AllowRead {
		t.Fatalf("allow_read = %v, want true", stored.AllowRead)
	}
	if stored.AllowWrite || stored.AllowChange || stored.AllowDelete {
		t.Fatalf("unexpected defaults: write=%v change=%v delete=%v", stored.AllowWrite, stored.AllowChange, stored.AllowDelete)
	}
}

func TestHandleSQLConnectionUpdatePreservesPermissionsWhenOmitted(t *testing.T) {
	t.Parallel()

	metaDB, err := sqlconnections.InitDB(filepath.Join(t.TempDir(), "sqlconnections.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer metaDB.Close()

	id, err := sqlconnections.Create(
		metaDB,
		"Keep Perms",
		"postgres",
		"localhost",
		5432,
		"testdb",
		"desc",
		true,
		true,
		false,
		true,
		"",
		"disable",
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	s := &Server{
		SQLConnectionsDB: metaDB,
	}

	payload := map[string]interface{}{
		"name":          "Keep Perms",
		"driver":        "postgres",
		"host":          "db.internal",
		"port":          5432,
		"database_name": "testdb",
		"description":   "updated desc",
		"ssl_mode":      "disable",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/sql-connections/"+id, bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handleSQLConnectionByID(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	stored, err := sqlconnections.GetByID(metaDB, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if !stored.AllowRead || !stored.AllowWrite || stored.AllowChange || !stored.AllowDelete {
		t.Fatalf("permissions changed unexpectedly: R=%v W=%v C=%v D=%v", stored.AllowRead, stored.AllowWrite, stored.AllowChange, stored.AllowDelete)
	}
}

func TestHandleSQLConnectionUpdatePreservesNameWhenOmitted(t *testing.T) {
	t.Parallel()

	metaDB, err := sqlconnections.InitDB(filepath.Join(t.TempDir(), "sqlconnections.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer metaDB.Close()

	id, err := sqlconnections.Create(
		metaDB,
		"Stable Name",
		"postgres",
		"localhost",
		5432,
		"testdb",
		"desc",
		true,
		false,
		false,
		false,
		"",
		"disable",
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	s := &Server{SQLConnectionsDB: metaDB}

	payload := map[string]interface{}{
		"host":        "db.internal",
		"description": "updated desc",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/sql-connections/"+id, bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handleSQLConnectionByID(s)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	stored, err := sqlconnections.GetByID(metaDB, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if stored.Name != "Stable Name" {
		t.Fatalf("name = %q, want %q", stored.Name, "Stable Name")
	}
	if stored.Host != "db.internal" {
		t.Fatalf("host = %q, want %q", stored.Host, "db.internal")
	}
}
