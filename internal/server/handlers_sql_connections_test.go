package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/sqlconnections"
)

func TestHandleSQLConnectionTestMasksDriverError(t *testing.T) {
	t.Parallel()

	metaDB, err := sqlconnections.InitDB(filepath.Join(t.TempDir(), "sqlconnections.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer metaDB.Close()

	id, err := sqlconnections.Create(
		metaDB,
		"Broken SQLite",
		"sqlite",
		"",
		0,
		filepath.Join(t.TempDir(), "missing", "db.sqlite"),
		"test",
		true,
		false,
		false,
		false,
		"",
		"",
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	s := &Server{
		SQLConnectionsDB:  metaDB,
		SQLConnectionPool: sqlconnections.NewConnectionPool(metaDB, nil, 2, 1, nil),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sql-connections/"+id+"/test", nil)
	rec := httptest.NewRecorder()

	handleSQLConnectionTest(s)(rec, req)

	// A failed connection test should return 400 Bad Request with sanitized error
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Connection test failed") || strings.Contains(strings.ToLower(body), "unable to open database file") {
		t.Fatalf("expected generic connection test failure, got %q", body)
	}
}
