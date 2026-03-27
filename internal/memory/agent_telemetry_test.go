package memory

import (
	"io"
	"log/slog"
	"testing"
)

func TestAgentTelemetryPersistence(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	if err := stm.UpsertAgentTelemetry("parse_source", "native"); err != nil {
		t.Fatalf("UpsertAgentTelemetry parse_source: %v", err)
	}
	if err := stm.UpsertAgentTelemetry("parse_source", "native"); err != nil {
		t.Fatalf("UpsertAgentTelemetry parse_source second: %v", err)
	}
	if err := stm.UpsertAgentTelemetry("recovery_event", "provider_422_recovered"); err != nil {
		t.Fatalf("UpsertAgentTelemetry recovery_event: %v", err)
	}

	rows, err := stm.LoadAgentTelemetry()
	if err != nil {
		t.Fatalf("LoadAgentTelemetry: %v", err)
	}

	got := map[string]int{}
	for _, row := range rows {
		got[row.EventType+":"+row.EventName] = row.Count
	}

	if got["parse_source:native"] != 2 {
		t.Fatalf("parse_source:native = %d, want 2", got["parse_source:native"])
	}
	if got["recovery_event:provider_422_recovered"] != 1 {
		t.Fatalf("recovery_event count = %d, want 1", got["recovery_event:provider_422_recovered"])
	}

	if err := stm.UpsertScopedAgentTelemetry("openrouter", "deepseek-chat", "parse_source", "native"); err != nil {
		t.Fatalf("UpsertScopedAgentTelemetry parse_source: %v", err)
	}
	if err := stm.UpsertScopedAgentTelemetry("openrouter", "deepseek-chat", "parse_source", "native"); err != nil {
		t.Fatalf("UpsertScopedAgentTelemetry parse_source second: %v", err)
	}

	scopedRows, err := stm.LoadScopedAgentTelemetry()
	if err != nil {
		t.Fatalf("LoadScopedAgentTelemetry: %v", err)
	}
	if len(scopedRows) != 1 {
		t.Fatalf("expected 1 scoped row, got %d", len(scopedRows))
	}
	if scopedRows[0].ProviderType != "openrouter" || scopedRows[0].Model != "deepseek-chat" {
		t.Fatalf("unexpected scoped row: %+v", scopedRows[0])
	}
	if scopedRows[0].Count != 2 {
		t.Fatalf("scoped count = %d, want 2", scopedRows[0].Count)
	}

	if err := stm.UpsertScopedAgentTelemetry("openrouter", "deepseek-chat", "tool_result", "success"); err != nil {
		t.Fatalf("UpsertScopedAgentTelemetry tool_result success: %v", err)
	}
	if err := stm.UpsertScopedAgentTelemetry("openrouter", "deepseek-chat", "tool_result", "failure"); err != nil {
		t.Fatalf("UpsertScopedAgentTelemetry tool_result failure: %v", err)
	}

	scopedRows, err = stm.LoadScopedAgentTelemetry()
	if err != nil {
		t.Fatalf("LoadScopedAgentTelemetry after tool results: %v", err)
	}
	var successCount, failureCount int
	for _, row := range scopedRows {
		if row.EventType == "tool_result" && row.EventName == "success" {
			successCount = row.Count
		}
		if row.EventType == "tool_result" && row.EventName == "failure" {
			failureCount = row.Count
		}
	}
	if successCount != 1 || failureCount != 1 {
		t.Fatalf("unexpected tool_result rows: success=%d failure=%d", successCount, failureCount)
	}
}
