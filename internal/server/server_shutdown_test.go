package server

import (
	"database/sql"
	"log/slog"
	"path/filepath"
	"testing"

	"aurago/internal/dbutil"

	_ "modernc.org/sqlite"
)

func TestCloseRuntimeResourcesClosesServerOwnedSQLiteHandles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	openDB := func(name string) *sql.DB {
		path := filepath.Join(root, name+".db")
		db, err := dbutil.Open(path)
		if err != nil {
			t.Fatalf("Open(%s): %v", name, err)
		}
		return db
	}

	s := &Server{
		Logger:             slog.Default(),
		SkillsDB:           openDB("skills"),
		MissionHistoryDB:   openDB("history"),
		PreparedMissionsDB: openDB("prepared"),
	}

	s.closeRuntimeResources()

	if s.SkillsDB != nil || s.MissionHistoryDB != nil || s.PreparedMissionsDB != nil {
		t.Fatalf("expected server-owned DB handles to be cleared, got skills=%v history=%v prepared=%v",
			s.SkillsDB, s.MissionHistoryDB, s.PreparedMissionsDB)
	}
}

func TestCloseGalaxaDBResetsSingleton(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := getGalaxaDB(root); err != nil {
		t.Fatalf("getGalaxaDB: %v", err)
	}
	closeGalaxaDB(slog.Default())

	if galaxaDBInst != nil {
		t.Fatal("expected galaxa singleton to be cleared")
	}
}