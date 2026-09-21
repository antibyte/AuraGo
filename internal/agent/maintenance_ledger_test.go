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
		Processed:             7,
		Deferred:              2,
		ConsolidationBacklog:  11,
		ConsolidationExcluded: 13,
		IntegrationChecks: []memory.IntegrationCheckResult{
			{ID: "sandbox", Status: "passed"},
			{ID: "telegram", Status: "skipped", Code: "disabled"},
		},
	}
	got := formatMorningBriefing(cfg, time.Date(2026, 8, 29, 4, 5, 0, 0, time.UTC), "partial", results, 3)
	for _, want := range []string{"Wartung: partial", "verarbeitet: 7", "aufgeschoben: 2", "Konsolidierung: Rückstand 11", "interne Einträge in diesem Lauf ausgeschlossen: 13", "sandbox: passed", "telegram: skipped (disabled)", "Offene operative Probleme: 3"} {
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

func TestMaintenanceContextWithReserveProtectsTailBudget(t *testing.T) {
	parent, cancelParent := context.WithTimeout(context.Background(), time.Second)
	defer cancelParent()
	child, cancelChild, available := maintenanceContextWithReserve(parent, 200*time.Millisecond)
	defer cancelChild()
	if !available {
		t.Fatal("one-second parent should provide a reserved child context")
	}
	parentDeadline, _ := parent.Deadline()
	childDeadline, ok := child.Deadline()
	if !ok {
		t.Fatal("reserved child context has no deadline")
	}
	reserved := parentDeadline.Sub(childDeadline)
	if reserved < 190*time.Millisecond || reserved > 210*time.Millisecond {
		t.Fatalf("reserved tail = %s, want about 200ms", reserved)
	}

	shortParent, cancelShort := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelShort()
	_, cancelUnavailable, available := maintenanceContextWithReserve(shortParent, 100*time.Millisecond)
	cancelUnavailable()
	if available {
		t.Fatal("short parent should defer work that would consume the protected tail")
	}
}

func TestMaintenanceLedgerSeparatesConsolidationAndMemoryOptimization(t *testing.T) {
	ledger := newMaintenanceRunLedger()
	ledger.beginPhase("consolidation")
	ledger.addProcessed("consolidation", 4)
	ledger.finishPhase("consolidation", false)
	ledger.beginPhase("memory_optimization")
	ledger.addDeferred("memory_optimization", 1)
	ledger.finishPhase("memory_optimization", true)

	results := ledger.results()
	if len(results.Phases) != 2 {
		t.Fatalf("phases = %#v, want consolidation and memory_optimization", results.Phases)
	}
	if results.Phases[0].Name != "consolidation" || results.Phases[0].Status != "completed" {
		t.Fatalf("consolidation phase = %#v", results.Phases[0])
	}
	if results.Phases[1].Name != "memory_optimization" || results.Phases[1].Status != "partial" || results.Phases[1].Deferred != 1 {
		t.Fatalf("memory optimization phase = %#v", results.Phases[1])
	}
}

func TestMaintenanceLedgerAddsFallbackCodeForDeferredPhase(t *testing.T) {
	ledger := newMaintenanceRunLedger()
	ledger.beginPhase("memory_optimization")
	ledger.addDeferred("memory_optimization", 1)
	ledger.finishPhase("memory_optimization", true)

	phase := ledger.results().Phases[0]
	if len(phase.ErrorCodes) != 1 || phase.ErrorCodes[0] != "deferred_work" {
		t.Fatalf("phase error codes = %#v, want deferred_work", phase.ErrorCodes)
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
