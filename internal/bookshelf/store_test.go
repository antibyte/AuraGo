package bookshelf

import (
	"context"
	"path/filepath"
	"testing"

	"aurago/internal/dbutil"
)

func TestMigrationBacksUpCatalogBeforeUpgrade(t *testing.T) {
	root := t.TempDir()
	dbPath := createVersionFiveCatalog(t, root, false)

	svc, err := New(context.Background(), Options{Root: root})
	if err != nil {
		t.Fatalf("New() migration: %v", err)
	}
	_ = svc.Close()

	backups, err := filepath.Glob(dbPath + ".v5.*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("migration backups = %v, %v; want exactly one", backups, err)
	}
	backup, err := dbutil.Open(backups[0])
	if err != nil {
		t.Fatalf("open migration backup: %v", err)
	}
	defer backup.Close()
	var version int
	if err := backup.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 5 {
		t.Fatalf("backup schema version = %d, %v; want 5", version, err)
	}
	var sourceName string
	if err := backup.QueryRow(`SELECT name FROM sources WHERE id='legacy-source'`).Scan(&sourceName); err != nil || sourceName != "Legacy" {
		t.Fatalf("backup did not preserve source row: %q, %v", sourceName, err)
	}
	live, err := dbutil.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen migrated catalog: %v", err)
	}
	defer live.Close()
	var migratedVersion int
	if err := live.QueryRow(`PRAGMA user_version`).Scan(&migratedVersion); err != nil || migratedVersion != schemaVersion {
		t.Fatalf("live catalog schema version = %d, %v; want %d", migratedVersion, err, schemaVersion)
	}
}

func TestMigrationFailureLeavesCatalogAndBackupRestorable(t *testing.T) {
	root := t.TempDir()
	dbPath := createVersionFiveCatalog(t, root, true)
	if _, err := New(context.Background(), Options{Root: root}); err == nil {
		t.Fatal("New() unexpectedly migrated a malformed legacy catalog")
	}

	backups, err := filepath.Glob(dbPath + ".v5.*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("failed-migration backups = %v, %v; want exactly one", backups, err)
	}
	for _, path := range []string{dbPath, backups[0]} {
		db, err := dbutil.Open(path)
		if err != nil {
			t.Fatalf("open catalog %q after failed migration: %v", path, err)
		}
		var version int
		if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 5 {
			_ = db.Close()
			t.Fatalf("catalog %q version = %d, %v; want 5", path, version, err)
		}
		var sourceName string
		if err := db.QueryRow(`SELECT name FROM sources WHERE id='legacy-source'`).Scan(&sourceName); err != nil || sourceName != "Legacy" {
			_ = db.Close()
			t.Fatalf("catalog %q lost source row: %q, %v", path, sourceName, err)
		}
		rows, err := db.Query(`PRAGMA table_info(renditions)`)
		if err != nil {
			_ = db.Close()
			t.Fatal(err)
		}
		converterColumn := false
		for rows.Next() {
			var cid, notNull, primaryKey int
			var name, dataType string
			var defaultValue any
			if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
				_ = rows.Close()
				_ = db.Close()
				t.Fatal(err)
			}
			if name == "converter_version" {
				converterColumn = true
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			_ = db.Close()
			t.Fatal(err)
		}
		_ = rows.Close()
		if converterColumn {
			_ = db.Close()
			t.Fatalf("catalog %q retained a partial migration column", path)
		}
		_ = db.Close()
	}
}

func createVersionFiveCatalog(t *testing.T, root string, malformedRenditions bool) string {
	t.Helper()
	dbPath := filepath.Join(root, "catalog.sqlite")
	db, err := dbutil.Open(dbPath)
	if err != nil {
		t.Fatalf("create legacy catalog: %v", err)
	}
	statements := []string{
		`CREATE TABLE sources (id TEXT PRIMARY KEY, name TEXT NOT NULL)`,
		`INSERT INTO sources(id,name) VALUES('legacy-source','Legacy')`,
	}
	if malformedRenditions {
		statements = append(statements, `CREATE TABLE renditions (format TEXT)`)
	} else {
		statements = append(statements, `CREATE TABLE renditions (book_id TEXT NOT NULL, format TEXT NOT NULL, base_revision TEXT NOT NULL, revision TEXT NOT NULL, size INTEGER NOT NULL, path TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY(book_id,format,base_revision))`)
	}
	statements = append(statements, `PRAGMA user_version=5`)
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			_ = db.Close()
			t.Fatalf("prepare v5 catalog: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy catalog: %v", err)
	}
	return dbPath
}
