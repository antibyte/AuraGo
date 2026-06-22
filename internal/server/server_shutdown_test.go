package server

import (
	"database/sql"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"

	"aurago/internal/dbutil"
	"aurago/internal/tools"

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

	dataDir := filepath.Join(root, "runtime")
	cronMgr := tools.NewCronManager(dataDir)
	bgMgr := tools.NewBackgroundTaskManager(dataDir, slog.Default())
	s.CronManager = cronMgr
	s.BackgroundTasks = bgMgr

	s.closeRuntimeResources()

	if s.CronManager != nil || s.BackgroundTasks != nil {
		t.Fatalf("expected task managers to be cleared after shutdown")
	}
	if s.SkillsDB != nil || s.MissionHistoryDB != nil || s.PreparedMissionsDB != nil {
		t.Fatalf("expected server-owned DB handles to be cleared, got skills=%v history=%v prepared=%v",
			s.SkillsDB, s.MissionHistoryDB, s.PreparedMissionsDB)
	}
}

func TestCloseGalaxaDBResetsSingleton(t *testing.T) {
	root := t.TempDir()
	if _, err := getGalaxaDB(root); err != nil {
		t.Fatalf("getGalaxaDB: %v", err)
	}
	closeGalaxaDB(slog.Default())

	if galaxaDBInst != nil {
		t.Fatal("expected galaxa singleton to be cleared")
	}
}

func TestGalaxaDBSingletonOpenCloseIsConcurrencySafe(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup

	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if _, err := getGalaxaDB(root); err != nil {
					t.Errorf("getGalaxaDB: %v", err)
					return
				}
				closeGalaxaDB(slog.Default())
			}
		}()
	}

	wg.Wait()
	closeGalaxaDB(slog.Default())
}
