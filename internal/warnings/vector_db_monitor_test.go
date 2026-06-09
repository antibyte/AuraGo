package warnings

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/config"
)

type fakeVectorDBHealth struct {
	ready    bool
	disabled bool
}

func (f *fakeVectorDBHealth) IsReady() bool    { return f.ready }
func (f *fakeVectorDBHealth) IsDisabled() bool { return f.disabled }

func TestVectorDBMonitor_EmitsWarningWhenValidationFails(t *testing.T) {
	reg := NewRegistry()
	cfg := &config.Config{}
	cfg.Embeddings.Provider = "openai"
	vdb := &fakeVectorDBHealth{ready: true, disabled: true}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	NewVectorDBMonitor(reg, cfg, vdb, logger).Start()
	time.Sleep(50 * time.Millisecond)

	total, _ := reg.Count()
	if total != 1 {
		t.Fatalf("expected 1 warning, got %d", total)
	}
	w := reg.Warnings()[0]
	if w.ID != "vectordb_validation_failed" {
		t.Fatalf("warning id = %q, want vectordb_validation_failed", w.ID)
	}
	if w.Severity != SeverityWarning {
		t.Fatalf("severity = %q, want %q", w.Severity, SeverityWarning)
	}
}

func TestVectorDBMonitor_SkipsWhenEmbeddingsHealthy(t *testing.T) {
	reg := NewRegistry()
	cfg := &config.Config{}
	cfg.Embeddings.Provider = "openai"
	vdb := &fakeVectorDBHealth{ready: true, disabled: false}

	NewVectorDBMonitor(reg, cfg, vdb, nil).Start()
	time.Sleep(50 * time.Millisecond)

	total, _ := reg.Count()
	if total != 0 {
		t.Fatalf("expected 0 warnings, got %d", total)
	}
}

func TestVectorDBMonitor_SkipsWhenEmbeddingsDisabledInConfig(t *testing.T) {
	reg := NewRegistry()
	cfg := &config.Config{}
	cfg.Embeddings.Provider = "disabled"
	vdb := &fakeVectorDBHealth{ready: true, disabled: true}

	NewVectorDBMonitor(reg, cfg, vdb, nil).Start()
	time.Sleep(50 * time.Millisecond)

	total, _ := reg.Count()
	if total != 0 {
		t.Fatalf("expected monitor to skip config-disabled embeddings, got %d warnings", total)
	}
}

func TestWatchVectorDBRecovery_ClearsWarningOnSuccess(t *testing.T) {
	reg := NewRegistry()
	reg.Add(Warning{
		ID:       "vectordb_validation_failed",
		Severity: SeverityWarning,
		Title:    "stale",
	})
	cfg := &config.Config{}
	cfg.Embeddings.Provider = "openai"

	vdb := &fakeVectorDBHealth{ready: false, disabled: false}
	WatchVectorDBRecovery(reg, cfg, vdb, nil)

	time.Sleep(50 * time.Millisecond)
	vdb.ready = true

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		found := false
		for _, w := range reg.Warnings() {
			if w.ID == "vectordb_validation_failed" {
				found = true
				break
			}
		}
		if !found {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, w := range reg.Warnings() {
		if w.ID == "vectordb_validation_failed" {
			t.Fatal("expected vectordb_validation_failed to be cleared after successful recovery")
		}
	}
}

func TestWatchVectorDBRecovery_KeepsWarningWhenValidationFails(t *testing.T) {
	reg := NewRegistry()
	cfg := &config.Config{}
	cfg.Embeddings.Provider = "openai"
	vdb := &fakeVectorDBHealth{ready: true, disabled: true}

	WatchVectorDBRecovery(reg, cfg, vdb, nil)
	time.Sleep(100 * time.Millisecond)

	found := false
	for _, w := range reg.Warnings() {
		if w.ID == "vectordb_validation_failed" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected vectordb_validation_failed when recovery validation fails")
	}
}

func TestCheckVectorDBDisabled_ConfigDisabled(t *testing.T) {
	reg := NewRegistry()
	cfg := &config.Config{}
	cfg.Embeddings.Provider = "disabled"

	checkVectorDBDisabled(reg, cfg, nil)

	total, _ := reg.Count()
	if total != 1 {
		t.Fatalf("expected 1 warning, got %d", total)
	}
	if reg.Warnings()[0].ID != "vectordb_disabled" {
		t.Fatalf("id = %q, want vectordb_disabled", reg.Warnings()[0].ID)
	}
}