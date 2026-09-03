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

func TestMaintenanceFinishPhaseMismatchKeepsActivePhase(t *testing.T) {
	ledger := newMaintenanceRunLedger()
	ledger.beginPhase("consolidation")
	ledger.finishPhase("memory_maintenance", false)

	if ledger.currentPhase != "consolidation" {
		t.Fatalf("currentPhase = %q, want consolidation", ledger.currentPhase)
	}
	if got := ledger.results().Phases[0].Status; got != "running" {
		t.Fatalf("phase status = %q, want running", got)
	}
}

func TestMaintenanceContextTimeoutClosesActiveLedgerPhase(t *testing.T) {
	for _, tt := range []struct {
		phase     string
		operation string
	}{
		{phase: "consolidation", operation: "memory_maintenance"},
		{phase: "agent_loop", operation: "skill_quality"},
	} {
		t.Run(tt.operation, func(t *testing.T) {
			ledger := newMaintenanceRunLedger()
			ledger.beginPhase(tt.phase)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			if !maintenanceContextDone(ctx, ledger, nil, tt.operation) {
				t.Fatal("maintenanceContextDone = false, want true")
			}
			results := ledger.results()
			if ledger.currentPhase != "" || ledger.status() != "partial" {
				t.Fatalf("currentPhase=%q status=%q", ledger.currentPhase, ledger.status())
			}
			if results.Deferred != 1 || len(results.Phases) != 1 || results.Phases[0].Deferred != 1 || results.Phases[0].Status != "partial" {
				t.Fatalf("results = %#v", results)
			}
			if len(results.Phases[0].ErrorCodes) != 1 || results.Phases[0].ErrorCodes[0] != tt.operation {
				t.Fatalf("phase error codes = %#v, want %q", results.Phases[0].ErrorCodes, tt.operation)
			}
		})
	}
}
