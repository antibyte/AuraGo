package agent

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/memory"
)

func TestExplicitMemoryQueriesReportMetadataErrors(t *testing.T) {
	stm, db := newMemorySafetyStore(t)
	if _, err := db.Exec("DROP TABLE memory_meta"); err != nil {
		t.Fatal(err)
	}
	vdb := &fakeVectorDB{}
	tc := ToolCall{Query: "editor", Content: "editor", Sources: []string{"ltm"}}
	for _, query := range []func() (string, error){
		func() (string, error) { return executeQueryMemory(tc, "", stm, vdb, nil, nil, nil) },
		func() (string, error) { return executeContextMemoryQuery(tc, "", stm, vdb, nil, nil, nil) },
	} {
		out, err := query()
		if err != nil || !strings.Contains(out, `"errors":`) || strings.Contains(out, "Krankenkasse PDF hit") {
			t.Fatalf("metadata error hidden or results leaked: %s (%v)", out, err)
		}
	}
}

func newMemorySafetyStore(t *testing.T) (*memory.SQLiteMemory, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "memory.db")
	stm, err := memory.NewSQLiteMemory(path, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return stm, db
}

type memorySafetyVector struct {
	archiveFilterVectorDB
	stored  map[string]string
	reads   map[string]int
	readErr error
}

func (v *memorySafetyVector) SearchSimilarScored(query string, topK int, _ ...string) ([]memory.SearchResult, error) {
	return v.SearchMemoriesOnlyScored(query, topK)
}

func (v *memorySafetyVector) GetByID(id string) (string, error) {
	if v.reads != nil {
		v.reads[id]++
	}
	return v.stored[id], v.readErr
}

func TestMemoryRetrievalDoesNotFailOpen(t *testing.T) {
	for _, scenario := range []string{"database-error", "beyond-50000", "archive-timestamp"} {
		t.Run(scenario, func(t *testing.T) {
			stm, db := newMemorySafetyStore(t)
			if err := stm.UpsertMemoryMeta("z-retired"); err != nil {
				t.Fatal(err)
			}
			if err := stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "z-retired", Action: memory.MemoryCurationActionArchive}, "user", false); err != nil {
				t.Fatal(err)
			}
			var err error
			switch scenario {
			case "database-error":
				_, err = db.Exec(`DROP TABLE memory_meta`)
			case "beyond-50000":
				_, err = db.Exec(`WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n < 50000) INSERT INTO memory_meta(doc_id) SELECT printf('a-%05d', n) FROM seq`)
			case "archive-timestamp":
				_, err = db.Exec(`UPDATE memory_meta SET verification_status='unverified' WHERE doc_id='z-retired'`)
			}
			if err != nil {
				t.Fatal(err)
			}
			vdb := &memorySafetyVector{archiveFilterVectorDB: archiveFilterVectorDB{byQuery: map[string][]memory.SearchResult{"project": {{DocID: "z-retired", Text: "retired fact", Similarity: .99}}}}}
			ranked, err := searchRankedMemoriesOnly(context.Background(), vdb, stm, "project", 1, nil, time.Now())
			if len(ranked) != 0 || (scenario == "database-error" && err == nil) || (scenario != "database-error" && err != nil) {
				t.Fatalf("scenario=%s ranked=%v err=%v", scenario, ranked, err)
			}
			filtered, filterErr := filterArchivedMemoryResults([]string{"retired fact"}, []string{"z-retired"}, stm)
			if len(filtered) != 0 || (scenario == "database-error" && filterErr == nil) {
				t.Fatalf("filtered=%v err=%v", filtered, filterErr)
			}
		})
	}
}

func TestMemoryCandidateMetadataIsCurrentAndStoreScoped(t *testing.T) {
	stm, _ := newMemorySafetyStore(t)
	other, _ := newMemorySafetyStore(t)
	if err := stm.UpsertMemoryMeta("doc"); err != nil {
		t.Fatal(err)
	}
	first, err := loadMemoryMetaMap(stm, []string{"doc", "doc", "legacy"})
	if err != nil || len(first) != 1 {
		t.Fatalf("first=%v err=%v", first, err)
	}
	if err := stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "doc", Action: memory.MemoryCurationActionArchive}, "user", false); err != nil {
		t.Fatal(err)
	}
	second, err := loadMemoryMetaMap(stm, []string{"doc"})
	if err != nil || !memory.IsMemoryArchived(second["doc"]) {
		t.Fatalf("stale metadata=%v err=%v", second, err)
	}
	third, err := loadMemoryMetaMap(other, []string{"doc"})
	if err != nil || len(third) != 0 {
		t.Fatalf("cross-store metadata=%v err=%v", third, err)
	}
}

func TestMemoryRetrievalRejectsUnavailableMetadataButAllowsLegacyRows(t *testing.T) {
	stm, _ := newMemorySafetyStore(t)
	for _, store := range []*memory.SQLiteMemory{nil, stm} {
		ranked, err := rankMemoryCandidates([]string{"legacy fact"}, []string{"legacy"}, store, nil, time.Now())
		if store == nil {
			if err == nil || len(ranked) != 0 {
				t.Fatalf("unavailable store leaked results: %v %v", ranked, err)
			}
		} else if err != nil || len(ranked) != 1 {
			t.Fatalf("legacy result=%v err=%v", ranked, err)
		}
	}
	for _, ids := range [][]string{nil, {""}} {
		if ranked, err := rankMemoryCandidates([]string{"fact"}, ids, stm, nil, time.Now()); err == nil || len(ranked) != 0 {
			t.Fatalf("missing identity: %v %v", ranked, err)
		}
	}
	_ = stm.Close()
	if _, err := loadMemoryMetaMap(stm, []string{"legacy"}); err == nil || errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("closed store error=%v", err)
	}
}
