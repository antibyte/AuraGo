package warnings

import (
	"strings"
	"testing"
	"time"

	"aurago/internal/llm"
	"aurago/internal/security"
)

func TestLLMProviderMonitorEmitsWarningAfterTransientThreshold(t *testing.T) {
	reg := NewRegistry()
	monitor := NewLLMProviderMonitor(reg, 3, 10*time.Minute)
	base := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)

	event := llm.HealthEvent{
		Operation:     "chat_completion",
		Provider:      "openrouter",
		Model:         "primary-model",
		ErrorCategory: llm.ErrCategoryTemporaryTransport,
		ErrorSummary:  "connection timeout",
		Retryable:     true,
		Timestamp:     base,
	}

	monitor.ReportLLMHealthEvent(event)
	monitor.ReportLLMHealthEvent(event.WithTimestamp(base.Add(time.Minute)))
	if total, _ := reg.Count(); total != 0 {
		t.Fatalf("warnings before threshold = %d, want 0", total)
	}

	monitor.ReportLLMHealthEvent(event.WithTimestamp(base.Add(2 * time.Minute)))
	monitor.ReportLLMHealthEvent(event.WithTimestamp(base.Add(3 * time.Minute)))

	all := reg.Warnings()
	if len(all) != 1 {
		t.Fatalf("warnings len = %d, want 1: %+v", len(all), all)
	}
	w := all[0]
	if w.Severity != SeverityWarning {
		t.Fatalf("severity = %q, want %q", w.Severity, SeverityWarning)
	}
	if w.Category != CategorySystem {
		t.Fatalf("category = %q, want %q", w.Category, CategorySystem)
	}
	if !strings.Contains(w.Title, "Repeated") {
		t.Fatalf("title %q does not mention repeated errors", w.Title)
	}
	if !strings.Contains(w.Description, "openrouter") || !strings.Contains(w.Description, "primary-model") {
		t.Fatalf("description does not include provider/model: %q", w.Description)
	}
}

func TestLLMProviderMonitorEmitsCriticalWarningForNonRetryableProblem(t *testing.T) {
	reg := NewRegistry()
	monitor := NewLLMProviderMonitor(reg, 3, 10*time.Minute)

	monitor.ReportLLMHealthEvent(llm.HealthEvent{
		Operation:     "chat_completion",
		Provider:      "openai",
		Model:         "gpt-4o",
		ErrorCategory: llm.ErrCategoryAuthError,
		ErrorSummary:  "invalid api key",
		Retryable:     false,
	})

	all := reg.Warnings()
	if len(all) != 1 {
		t.Fatalf("warnings len = %d, want 1", len(all))
	}
	if all[0].Severity != SeverityCritical {
		t.Fatalf("severity = %q, want %q", all[0].Severity, SeverityCritical)
	}
	if !strings.Contains(all[0].Title, "Configuration") {
		t.Fatalf("title %q does not mention configuration", all[0].Title)
	}
}

func TestLLMProviderMonitorSuccessResetsTransientStreak(t *testing.T) {
	reg := NewRegistry()
	monitor := NewLLMProviderMonitor(reg, 3, 10*time.Minute)
	base := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)

	event := llm.HealthEvent{
		Operation:     "chat_completion",
		Provider:      "openrouter",
		Model:         "primary-model",
		ErrorCategory: llm.ErrCategoryTemporaryTransport,
		ErrorSummary:  "connection timeout",
		Retryable:     true,
		Timestamp:     base,
	}

	monitor.ReportLLMHealthEvent(event)
	monitor.ReportLLMHealthEvent(event.WithTimestamp(base.Add(time.Minute)))
	monitor.ReportLLMHealthSuccess(llm.HealthSuccess{Provider: "openrouter", Model: "primary-model"})
	monitor.ReportLLMHealthEvent(event.WithTimestamp(base.Add(2 * time.Minute)))

	if total, _ := reg.Count(); total != 0 {
		t.Fatalf("warnings after reset + one failure = %d, want 0", total)
	}
}

func TestLLMProviderMonitorScrubsAndTruncatesWarningText(t *testing.T) {
	security.RegisterSensitive("sk-secret-health-test")
	reg := NewRegistry()
	monitor := NewLLMProviderMonitor(reg, 3, 10*time.Minute)
	longSummary := strings.Repeat("provider failed with sk-secret-health-test and verbose payload ", 40)

	event := llm.HealthEvent{
		Operation:     "stream_chat_completion",
		Provider:      "openrouter",
		Model:         "primary-model",
		ErrorCategory: llm.ErrCategoryTemporaryTransport,
		ErrorSummary:  longSummary,
		Retryable:     true,
	}

	monitor.ReportLLMHealthEvent(event)
	monitor.ReportLLMHealthEvent(event)
	monitor.ReportLLMHealthEvent(event)

	all := reg.Warnings()
	if len(all) != 1 {
		t.Fatalf("warnings len = %d, want 1", len(all))
	}
	desc := all[0].Description
	if strings.Contains(desc, "sk-secret-health-test") {
		t.Fatalf("description leaked registered secret: %q", desc)
	}
	if len(desc) > 900 {
		t.Fatalf("description length = %d, want <= 900", len(desc))
	}
}
