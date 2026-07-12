package agent

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestNormalizeRememberCategorySupportsAliases(t *testing.T) {
	tests := map[string]string{
		"core_memory":     "fact",
		"journal_entry":   "event",
		"notes":           "task",
		"knowledge_graph": "relationship",
	}

	for input, want := range tests {
		if got := normalizeRememberCategory(input); got != want {
			t.Fatalf("normalizeRememberCategory(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestClassifyMemoryTargetPrefersStructuredRelationship(t *testing.T) {
	tc := ToolCall{Source: "server-a", Target: "portainer", Relation: "uses"}
	if got := classifyMemoryTarget(tc, "Portainer is used on server-a"); got != "relationship" {
		t.Fatalf("classifyMemoryTarget() = %q, want relationship", got)
	}
}

func TestClassifyMemoryTargetTreatsInstallationPreferencesAsFact(t *testing.T) {
	content := "Server installation preferences: use Docker Compose and keep volumes under /srv/data"
	if got := classifyMemoryTarget(ToolCall{}, content); got != "fact" {
		t.Fatalf("classifyMemoryTarget() = %q, want fact", got)
	}
}

func TestClassifyMemoryTargetTreatsStableUserLocationAsFact(t *testing.T) {
	for _, content := range []string{
		"Andi wohnt in Pforzheim.",
		"User location is Pforzheim.",
		"Der Wohnort des Nutzers ist Pforzheim.",
	} {
		if got := classifyMemoryTarget(ToolCall{}, content); got != "fact" {
			t.Fatalf("classifyMemoryTarget(%q) = %q, want fact", content, got)
		}
	}
}

func TestClassifyMemoryTargetTreatsDebuggingTipAsEvent(t *testing.T) {
	content := "Python debugging tip: run pytest -k failing_test before touching the implementation"
	if got := classifyMemoryTarget(ToolCall{}, content); got != "event" {
		t.Fatalf("classifyMemoryTarget() = %q, want event", got)
	}
}

func TestClassifyMemoryTargetDetectsTasksConservatively(t *testing.T) {
	content := "Reminder: check the backup retention policy tomorrow"
	if got := classifyMemoryTarget(ToolCall{}, content); got != "task" {
		t.Fatalf("classifyMemoryTarget() = %q, want task", got)
	}
}

func TestClassifyMemoryTargetDetectsEventsWithTemporalSignals(t *testing.T) {
	content := "Completed Docker migration successfully yesterday"
	if got := classifyMemoryTarget(ToolCall{}, content); got != "event" {
		t.Fatalf("classifyMemoryTarget() = %q, want event", got)
	}
}

func TestClassifyMemoryTargetDefaultsAmbiguousContentToEvent(t *testing.T) {
	content := "Koofr upload returned a zero-byte image during mission test run"
	if got := classifyMemoryTarget(ToolCall{}, content); got != "event" {
		t.Fatalf("classifyMemoryTarget() = %q, want event", got)
	}
}

func TestNormalizeHeuristicTextPreservesNonWesternLettersAndDigits(t *testing.T) {
	got := normalizeHeuristicText("服务器 运行 Docker 版本二 ٢")
	for _, want := range []string{"服务器", "运行", "版本二", "٢"} {
		if !strings.Contains(got, want) {
			t.Fatalf("normalizeHeuristicText = %q, want to preserve %q", got, want)
		}
	}
}

func TestRememberAmbiguousContentUsesJournal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	out := handleRemember(
		ToolCall{Content: "Koofr upload returned a zero-byte image during mission test run"},
		nil,
		logger,
		stm,
		nil,
		"session-test",
	)

	if !strings.Contains(out, `"stored_as":"journal"`) {
		t.Fatalf("handleRemember output = %q, want journal storage", out)
	}
}

func TestRememberStableUserLocationUsesCoreMemory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	out := handleRemember(
		ToolCall{Content: "Andi wohnt in Pforzheim."},
		&config.Config{},
		logger,
		stm,
		nil,
		"session-test",
	)

	if !strings.Contains(out, `"stored_as":"core_memory"`) {
		t.Fatalf("handleRemember output = %q, want core memory storage", out)
	}
	if !stm.CoreMemoryFactExists("Andi wohnt in Pforzheim.") {
		t.Fatal("expected stable location fact in core memory")
	}
}

func TestRememberExplicitCoreRejectsTransientFacts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	out := handleRemember(
		ToolCall{
			Content:  `2026-05-08: Created "Chaos Symphony XIII", uploaded to Koofr /aurago/music. Media Registry ID: 2320.`,
			Category: "core",
		},
		&config.Config{},
		logger,
		stm,
		nil,
		"session-test",
	)

	if !strings.Contains(out, `"status":"error"`) {
		t.Fatalf("handleRemember output = %q, want core-memory rejection", out)
	}
	count, err := stm.GetCoreMemoryCount()
	if err != nil {
		t.Fatalf("GetCoreMemoryCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("core memory count = %d, want 0", count)
	}
}

func TestRememberTaskUsesExplicitTitleForNote(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	out := handleRemember(
		ToolCall{Content: "Check the backup retention policy tomorrow", Title: "Review backup retention", Category: "task"},
		nil,
		logger,
		stm,
		nil,
		"session-test",
	)

	if !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("handleRemember output = %q, want success", out)
	}
	notes, err := stm.SearchNotes("Review backup retention", 5)
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}
	if len(notes) == 0 {
		t.Fatal("expected note to be created with explicit title")
	}
	if notes[0].Title != "Review backup retention" {
		t.Fatalf("note title = %q, want explicit title", notes[0].Title)
	}
	if notes[0].Content != "Check the backup retention policy tomorrow" {
		t.Fatalf("note content = %q, want original remember content", notes[0].Content)
	}
}

func TestRememberRelationshipStoresProvenanceClaim(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })

	content := "Server A uses Portainer for container management."
	out := handleRemember(
		ToolCall{
			Content:  content,
			Category: "relationship",
			Source:   "server-a",
			Target:   "portainer",
			Relation: "uses",
		},
		nil,
		logger,
		nil,
		kg,
		"session-test",
	)

	if !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("handleRemember output = %q, want success", out)
	}
	claims, err := kg.GetClaimsForEdge("server-a", "portainer", "uses", true, 10)
	if err != nil {
		t.Fatalf("GetClaimsForEdge: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("claims len = %d, want 1", len(claims))
	}
	claim := claims[0]
	if claim.SourceKind != "user" || claim.SessionID != "session-test" || claim.ConfidenceLabel != "user_provided" {
		t.Fatalf("unexpected claim provenance: %+v", claim)
	}
	if claim.Evidence == nil {
		t.Fatalf("expected evidence for remember claim: %+v", claim)
	}
	if claim.Evidence.RawText != content || claim.Evidence.EvidenceType != "remember" || claim.Evidence.Channel != "agent" {
		t.Fatalf("unexpected evidence: %+v", claim.Evidence)
	}
}

func TestRememberTaskRespectsDisabledNotesConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	cfg := &config.Config{}
	cfg.Tools.Notes.Enabled = false

	out := handleRemember(
		ToolCall{Content: "Check the backup retention policy tomorrow", Title: "Review backup retention", Category: "task"},
		cfg,
		logger,
		stm,
		nil,
		"session-test",
	)

	if !strings.Contains(out, "Notes are disabled") {
		t.Fatalf("handleRemember output = %q, want disabled notes error", out)
	}
	notes, err := stm.SearchNotes("Review backup retention", 5)
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("remember created %d notes despite disabled notes config", len(notes))
	}
}

func TestRememberTaskRespectsReadOnlyNotesConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	cfg := &config.Config{}
	cfg.Tools.Notes.Enabled = true
	cfg.Tools.Notes.ReadOnly = true

	out := handleRemember(
		ToolCall{Content: "Check the backup retention policy tomorrow", Title: "Review backup retention", Category: "task"},
		cfg,
		logger,
		stm,
		nil,
		"session-test",
	)

	if !strings.Contains(out, "Notes are in read-only mode") {
		t.Fatalf("handleRemember output = %q, want read-only notes error", out)
	}
	notes, err := stm.SearchNotes("Review backup retention", 5)
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("remember created %d notes despite read-only notes config", len(notes))
	}
}
