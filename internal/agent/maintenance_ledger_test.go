package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestMaintenanceLedgerDeferredWorkIsPartialNotFailed(t *testing.T) {
	ledger := newMaintenanceRunLedger()
	ledger.beginPhase("consolidation")
	ledger.addProcessed("consolidation", 12)
	ledger.addDeferred("consolidation", 4)
	ledger.finishPhase("consolidation", true)

	if got := ledger.status(); got != "partial" {
		t.Fatalf("status = %q, want partial", got)
	}
	results := ledger.results()
	if results.Processed != 12 || results.Deferred != 4 || len(results.Phases) != 1 {
		t.Fatalf("results = %#v", results)
	}
	if results.Phases[0].Status != "partial" {
		t.Fatalf("phase = %#v", results.Phases[0])
	}
	ledger.markFailed()
	if got := ledger.status(); got != "failed" {
		t.Fatalf("critical failure status = %q, want failed", got)
	}
}

func TestMaintenanceErrorCodesAreSanitized(t *testing.T) {
	if got := sanitizeMaintenanceErrorCode("KG extraction failed: token=do-not-store"); got != "kg_extraction_failed" {
		t.Fatalf("error code = %q", got)
	}
}

func TestMorningBriefingUsesOnlyCurrentStructuredResults(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.UILanguage = "de"
	results := memory.MaintenancePhaseResults{
		Processed: 7,
		Deferred:  2,
		IntegrationChecks: []memory.IntegrationCheckResult{
			{ID: "sandbox", Status: "passed"},
			{ID: "telegram", Status: "skipped", Code: "disabled"},
		},
	}
	got := formatMorningBriefing(cfg, time.Date(2026, 8, 29, 4, 5, 0, 0, time.UTC), "partial", results, 3)
	for _, want := range []string{"Wartung: partial", "verarbeitet: 7", "aufgeschoben: 2", "sandbox: passed", "telegram: skipped (disabled)", "Offene operative Probleme: 3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("briefing missing %q: %s", want, got)
		}
	}
	if strings.Contains(strings.ToLower(got), "system healthy overall") || strings.Contains(got, "4 FAIL") {
		t.Fatalf("briefing contains stale health prose: %s", got)
	}
}

func TestMaintenanceContextClaimGateRequiresSeventyFiveSeconds(t *testing.T) {
	longCtx, longCancel := context.WithTimeout(context.Background(), 76*time.Second)
	defer longCancel()
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 74*time.Second)
	defer shortCancel()
	if !maintenanceContextHasAtLeast(longCtx, 75*time.Second) {
		t.Fatal("76-second deadline should allow another claim")
	}
	if maintenanceContextHasAtLeast(shortCtx, 75*time.Second) {
		t.Fatal("74-second deadline should defer the next claim")
	}
}
