package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"
)

type fakeVectorDB struct {
	searchSimilarCalled      bool
	searchMemoriesOnlyCalled bool
}

func (f *fakeVectorDB) StoreDocument(concept, content string) ([]string, error) {
	return nil, nil
}

func (f *fakeVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	return "", nil
}

func (f *fakeVectorDB) StoreBatch(items []memory.ArchiveItem) ([]string, error) {
	return nil, nil
}

func (f *fakeVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	f.searchSimilarCalled = true
	return []string{"[tool_guides] wrong hit"}, []string{"tool-1"}, nil
}

func (f *fakeVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	f.searchMemoriesOnlyCalled = true
	return []string{"Vincenzo memory hit"}, []string{"mem-1"}, nil
}

func (f *fakeVectorDB) GetByID(id string) (string, error)                           { return "", nil }
func (f *fakeVectorDB) GetByIDFromCollection(id, collection string) (string, error) { return "", nil }
func (f *fakeVectorDB) DeleteDocument(id string) error                              { return nil }
func (f *fakeVectorDB) DeleteDocumentFromCollection(id, collection string) error    { return nil }
func (f *fakeVectorDB) Count() int                                                  { return 1 }
func (f *fakeVectorDB) IsDisabled() bool                                            { return false }
func (f *fakeVectorDB) Close() error                                                { return nil }
func (f *fakeVectorDB) StoreDocumentInCollection(concept, content, collection string) ([]string, error) {
	return nil, nil
}
func (f *fakeVectorDB) StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error) {
	return "", nil
}
func (f *fakeVectorDB) StoreCheatsheet(id, name, content string, attachments ...string) error {
	return nil
}
func (f *fakeVectorDB) DeleteCheatsheet(id string) error { return nil }

func TestDispatchExecQueryMemoryUsesMemoriesOnlyForVectorDB(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Memory.Enabled = true
	logger := slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelError}))
	vdb := &fakeVectorDB{}

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "query_memory", Query: "Vincenzo", Sources: []string{"vector_db"}},
		&DispatchContext{Cfg: cfg, Logger: logger, LongTermMem: vdb},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle query_memory")
	}

	if !vdb.searchMemoriesOnlyCalled {
		t.Fatal("expected SearchMemoriesOnly to be called")
	}
	if vdb.searchSimilarCalled {
		t.Fatal("did not expect SearchSimilar to be called")
	}
	if !strings.Contains(out, "Vincenzo memory hit") {
		t.Fatalf("output = %q, want memory hit", out)
	}
	if strings.Contains(out, "tool_guides") {
		t.Fatalf("output = %q, did not expect tool guide hit", out)
	}
}

func TestDispatchExecQueryMemoryUnderstandsTemporalJournalQueries(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Memory.Enabled = true
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	oldDate := time.Now().AddDate(0, 0, -14).Format("2006-01-02")

	if _, err := stm.InsertJournalEntry(memory.JournalEntry{
		EntryType: "task_completed",
		Title:     "Docker issue",
		Content:   "Fixed docker deployment",
		Date:      yesterday,
	}); err != nil {
		t.Fatalf("InsertJournalEntry recent: %v", err)
	}
	if _, err := stm.InsertJournalEntry(memory.JournalEntry{
		EntryType: "task_completed",
		Title:     "Docker old issue",
		Content:   "Old docker deployment",
		Date:      oldDate,
	}); err != nil {
		t.Fatalf("InsertJournalEntry old: %v", err)
	}

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "query_memory", Query: "docker gestern", Sources: []string{"journal"}},
		&DispatchContext{Cfg: cfg, Logger: logger, ShortTermMem: stm},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle query_memory")
	}

	if !strings.Contains(out, `"temporal_range"`) {
		t.Fatalf("output = %q, want temporal_range metadata", out)
	}
	if !strings.Contains(out, "Docker issue") {
		t.Fatalf("output = %q, want recent docker issue", out)
	}
	if strings.Contains(out, "Docker old issue") {
		t.Fatalf("output = %q, did not expect out-of-range journal hit", out)
	}

	raw := strings.TrimPrefix(out, "Tool Output: ")
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("json.Unmarshal output: %v", err)
	}
	if _, ok := parsed["temporal_range"]; !ok {
		t.Fatal("expected temporal_range field in parsed response")
	}
}

func TestDispatchExecQueryMemoryIncludesActivitySource(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Memory.Enabled = true
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if _, err := stm.InsertActivityTurn(memory.ActivityTurn{
		Date:            time.Now().Format("2006-01-02"),
		SessionID:       "default",
		Channel:         "web_chat",
		UserRelevant:    true,
		Intent:          "Fix docker deployment",
		UserRequest:     "Please fix the docker deployment",
		UserGoal:        "Fix docker deployment",
		ActionsTaken:    []string{"execute_shell"},
		Outcomes:        []string{"Docker deployment fixed"},
		ImportantPoints: []string{"The compose file had a bad path"},
		ToolNames:       []string{"execute_shell"},
	}); err != nil {
		t.Fatalf("InsertActivityTurn: %v", err)
	}

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "query_memory", Query: "docker deployment", Sources: []string{"activity"}},
		&DispatchContext{Cfg: cfg, Logger: logger, ShortTermMem: stm},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle query_memory")
	}

	if !strings.Contains(out, `"source":"activity"`) {
		t.Fatalf("output = %q, want activity source", out)
	}
	if !strings.Contains(out, "Fix docker deployment") {
		t.Fatalf("output = %q, want activity hit", out)
	}
}

func TestDispatchExecContextMemoryReturnsCombinedResults(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Memory.Enabled = true
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	if _, err := stm.AddNote("todo", "Check backup retention", "", 3, ""); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if _, err := stm.InsertJournalEntry(memory.JournalEntry{
		EntryType: "milestone",
		Title:     "Backup issue resolved",
		Content:   "Resolved the retention issue",
		Date:      time.Now().Format("2006-01-02"),
	}); err != nil {
		t.Fatalf("InsertJournalEntry: %v", err)
	}
	if _, err := stm.InsertActivityTurn(memory.ActivityTurn{
		Date:            time.Now().Format("2006-01-02"),
		SessionID:       "default",
		Channel:         "web_chat",
		UserRelevant:    true,
		Intent:          "Review backup status",
		UserRequest:     "What did we do on backups?",
		UserGoal:        "Review backup status",
		ActionsTaken:    []string{"query_memory"},
		Outcomes:        []string{"Found the retention misconfiguration"},
		ImportantPoints: []string{"Retention policy needed a fix"},
		ToolNames:       []string{"query_memory"},
	}); err != nil {
		t.Fatalf("InsertActivityTurn: %v", err)
	}

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "context_memory", Query: "backup", Sources: []string{"activity", "journal", "notes"}, TimeRange: "last_week", ContextDepth: "deep"},
		&DispatchContext{Cfg: cfg, Logger: logger, ShortTermMem: stm},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle context_memory")
	}

	if !strings.Contains(out, `"combined_results"`) {
		t.Fatalf("output = %q, want combined results", out)
	}
	if !strings.Contains(out, `"source":"activity"`) {
		t.Fatalf("output = %q, want activity result", out)
	}
	if !strings.Contains(out, `"source":"journal"`) {
		t.Fatalf("output = %q, want journal result", out)
	}
}

func TestDispatchExecContextMemorySupportsVectorAliasSources(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Memory.Enabled = true
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	vdb := &fakeVectorDB{}

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "context_memory", Query: "Vincenzo", Sources: []string{"vector_db"}},
		&DispatchContext{Cfg: cfg, Logger: logger, LongTermMem: vdb, ShortTermMem: nil},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle context_memory")
	}

	if !vdb.searchMemoriesOnlyCalled {
		t.Fatal("expected context_memory to use SearchMemoriesOnly via vector_db alias")
	}
	if !strings.Contains(out, `"source":"ltm"`) {
		t.Fatalf("output = %q, want ltm source", out)
	}
	if !strings.Contains(out, "Vincenzo memory hit") {
		t.Fatalf("output = %q, want vector memory hit", out)
	}
}

func TestDispatchExecQueryMemoryIncludesPlannerSource(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Memory.Enabled = true
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	plannerDB := newPlannerTestDB(t)
	defer plannerDB.Close()

	if _, err := planner.CreateTodo(plannerDB, planner.Todo{
		Title:       "Server backup audit",
		Description: "Review the backup schedule",
		Priority:    "high",
		Status:      "open",
		DueDate:     "2026-04-18T00:00:00Z",
	}); err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "query_memory", Query: "what is open", Sources: []string{"planner"}},
		&DispatchContext{Cfg: cfg, Logger: logger, PlannerDB: plannerDB},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle query_memory")
	}
	if !strings.Contains(out, `"source":"planner"`) {
		t.Fatalf("output = %q, want planner source", out)
	}
	if !strings.Contains(out, "Server backup audit") {
		t.Fatalf("output = %q, want planner todo hit", out)
	}
}

func TestDispatchExecContextMemoryIncludesPlannerSummary(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Memory.Enabled = true
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	plannerDB := newPlannerTestDB(t)
	defer plannerDB.Close()

	now := time.Now()

	if _, err := planner.CreateTodo(plannerDB, planner.Todo{
		Title:    "Prepare release notes",
		Priority: "high",
		Status:   "open",
		DueDate:  now.AddDate(0, 0, -1).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}
	todayAtTen := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location())
	if _, err := planner.CreateAppointment(plannerDB, planner.Appointment{
		Title:    "Release sync",
		DateTime: todayAtTen.Format(time.RFC3339),
		Status:   "upcoming",
	}); err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "context_memory", Query: "what is on today", Sources: []string{"planner"}, TimeRange: "today"},
		&DispatchContext{Cfg: cfg, Logger: logger, PlannerDB: plannerDB},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle context_memory")
	}
	if !strings.Contains(out, `"source":"planner"`) {
		t.Fatalf("output = %q, want planner source", out)
	}
	if !strings.Contains(out, `"type":"planner_summary"`) {
		t.Fatalf("output = %q, want planner summary result", out)
	}
	if !strings.Contains(out, "Release sync") {
		t.Fatalf("output = %q, want appointment hit", out)
	}
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}
