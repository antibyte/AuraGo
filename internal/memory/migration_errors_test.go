package memory

import (
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateFileIndexReportsSchemaInspectionError(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "closed.db"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	err = migrateFileIndexToCollectionAware(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil || !strings.Contains(err.Error(), "check file_indices schema") {
		t.Fatalf("migrateFileIndexToCollectionAware error = %v, want the failed schema inspection to be returned", err)
	}
}

func TestKGInitReportsFTSMarkerWriteFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := filepath.Join(t.TempDir(), "kg.db")
	kg, err := NewKnowledgeGraph(path, "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	if err := kg.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(`
		DELETE FROM kg_meta WHERE key = 'fts_schema_version';
		CREATE TRIGGER block_kg_meta_insert BEFORE INSERT ON kg_meta
		BEGIN
			SELECT RAISE(ABORT, 'kg_meta writes blocked');
		END;
	`); err != nil {
		_ = db.Close()
		t.Fatalf("prepare kg_meta: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	reopened, err := NewKnowledgeGraph(path, "", logger)
	if err == nil {
		_ = reopened.Close()
		t.Fatal("NewKnowledgeGraph succeeded although the FTS marker could not be written")
	}
	if !strings.Contains(err.Error(), "fts_schema_version") {
		t.Fatalf("error = %v, want the FTS marker write to be named", err)
	}
}
