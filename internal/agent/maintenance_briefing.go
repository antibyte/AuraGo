package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/tools"
	"aurago/internal/virtualcomputers"
)

func completeMaintenanceRun(cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory, plannerDB *sql.DB, startedAt time.Time, ledger *maintenanceRunLedger) {
	finishedAt := time.Now()
	if ledger != nil && ledger.currentPhase != "" {
		ledger.finishPhase(ledger.currentPhase, false)
	}
	results := ledger.results()
	results.IntegrationChecks = runMaintenanceIntegrationChecks(cfg, logger, finishedAt)
	ledger.phaseResults = results
	reconcileMaintenancePhaseIssues(plannerDB, results.Phases, finishedAt, logger)
	if stm == nil {
		return
	}
	status := ledger.status()
	if err := stm.InsertMaintenanceRun(startedAt, finishedAt, status, results); err != nil {
		if logger != nil {
			logger.Warn("[Maintenance] Failed to persist maintenance run ledger", "error_code", "maintenance_ledger_persist")
		}
		return
	}
	openIssues := 0
	if plannerDB != nil {
		if page, err := planner.ListOperationalIssues(plannerDB, planner.OperationalIssueListFilter{Status: "active", Limit: 1}); err == nil {
			openIssues = page.Total
		}
	}
	notification := memory.SystemNotification{
		Type:     "morning_briefing",
		Title:    "Maintenance " + finishedAt.Local().Format("2006-01-02 15:04"),
		Message:  formatMorningBriefing(cfg, finishedAt, status, results, openIssues),
		SourceID: "maintenance:" + startedAt.UTC().Format(time.RFC3339Nano),
		Data: map[string]interface{}{
			"maintenance_status": status,
			"processed":          results.Processed,
			"deferred":           results.Deferred,
			"open_issues":        openIssues,
			"finished_at":        finishedAt.UTC().Format(time.RFC3339),
		},
	}
	if _, _, err := stm.AddSystemNotification(notification); err != nil && logger != nil {
		logger.Warn("[Maintenance] Failed to store morning briefing", "error_code", "morning_briefing_persist")
	}
}

func runMaintenanceIntegrationChecks(cfg *config.Config, logger *slog.Logger, now time.Time) []memory.IntegrationCheckResult {
	checks := []memory.IntegrationCheckResult{
		runSandboxIntegrationCheck(cfg, now),
		runVirtualComputersIntegrationCheck(cfg, now),
		runTelegramIntegrationCheck(now),
		runModelLimitsIntegrationCheck(cfg, now),
	}
	return checks
}

func runSandboxIntegrationCheck(cfg *config.Config, now time.Time) memory.IntegrationCheckResult {
	result := memory.IntegrationCheckResult{ID: "sandbox", CheckedAt: now.UTC().Format(time.RFC3339)}
	if cfg == nil || !cfg.Sandbox.Enabled {
		result.Status, result.Code, result.Detail = "skipped", "disabled", "sandbox is disabled"
		return result
	}
	manager := tools.GetSandboxManager()
	if manager == nil {
		result.Status, result.Code, result.Detail = "failed", "not_initialized", "sandbox manager is not initialized"
		return result
	}
	status := manager.Status()
	result.Data = map[string]interface{}{"ready": status.Ready, "backend": status.Backend}
	if !status.Ready {
		result.Status, result.Code, result.Detail = "failed", "not_ready", "sandbox is not ready"
		return result
	}
	output, err := manager.ExecuteCode("print('AURAGO_SANDBOX_SMOKE_OK')", "python", nil, 10)
	if err != nil || !strings.Contains(output, "AURAGO_SANDBOX_SMOKE_OK") {
		result.Status, result.Code, result.Detail = "failed", "smoke_failed", "isolated print smoke failed"
		return result
	}
	result.Status, result.Detail = "passed", "isolated print smoke passed"
	return result
}

func runVirtualComputersIntegrationCheck(cfg *config.Config, now time.Time) memory.IntegrationCheckResult {
	result := memory.IntegrationCheckResult{ID: "virtual_computers", CheckedAt: now.UTC().Format(time.RFC3339)}
	if cfg == nil || !cfg.VirtualComputers.Enabled {
		result.Status, result.Code, result.Detail = "skipped", "disabled", "virtual computers are disabled"
		return result
	}
	toolCfg := virtualcomputers.FromAuraConfig(cfg)
	client, err := virtualcomputers.NewClient(virtualcomputers.ClientConfig{
		BaseURL: toolCfg.BoringdURL, Token: toolCfg.BoringToken, Timeout: 10 * time.Second,
	})
	if err != nil {
		result.Status, result.Code, result.Detail = "failed", "client_config", "virtual computers client is not configured"
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := client.Status(ctx); err != nil {
		result.Status, result.Code, result.Detail = "failed", "status_failed", "virtual computers status check failed"
		return result
	}
	machines, err := client.ListMachines(ctx)
	if err != nil {
		result.Status, result.Code, result.Detail = "failed", "list_failed", "virtual computers list check failed"
		return result
	}
	result.Status, result.Detail = "passed", "status and machine list passed"
	result.Data = map[string]interface{}{"machines": len(machines)}
	return result
}

func runTelegramIntegrationCheck(now time.Time) memory.IntegrationCheckResult {
	status := integrationstatus.TelegramStatus()
	result := memory.IntegrationCheckResult{
		ID: "telegram", CheckedAt: now.UTC().Format(time.RFC3339),
		Data: map[string]interface{}{
			"state": status.State, "polling": status.Polling,
			"consecutive_poll_errors": status.ConsecutivePollErrors,
		},
	}
	if !status.Enabled || !status.Configured {
		result.Status, result.Code, result.Detail = "skipped", status.State, "Telegram is disabled or not fully configured"
		return result
	}
	if status.State == "healthy" {
		result.Status, result.Detail = "passed", "passive polling runtime is healthy"
		return result
	}
	result.Status, result.Code, result.Detail = "failed", status.LastErrorCode, "passive polling runtime is not healthy"
	if result.Code == "" {
		result.Code = status.State
	}
	return result
}

func runModelLimitsIntegrationCheck(cfg *config.Config, now time.Time) memory.IntegrationCheckResult {
	result := memory.IntegrationCheckResult{ID: "model_limits", CheckedAt: now.UTC().Format(time.RFC3339)}
	if cfg == nil {
		result.Status, result.Code, result.Detail = "failed", "config_unavailable", "model configuration is unavailable"
		return result
	}
	route := llm.ModelRoute{
		ProviderID: cfg.LLM.Provider, ProviderType: cfg.LLM.ProviderType,
		BaseURL: cfg.LLM.BaseURL, APIKey: cfg.LLM.APIKey, Model: cfg.LLM.Model, Primary: true,
	}
	if provider := cfg.FindProvider(cfg.LLM.Provider); provider != nil {
		route.ContextWindowOverride = provider.ContextWindow
		route.MaxOutputTokensOverride = provider.MaxOutputTokens
		if route.Model == "" {
			route.Model = provider.Model
		}
	}
	limits := llm.ResolveModelLimitsCached(route, cfg.Agent.ContextWindow)
	result.Data = map[string]interface{}{
		"provider_id": route.ProviderID, "model": route.Model,
		"context_window": limits.ContextWindow, "max_output_tokens": limits.MaxOutputTokens,
		"context_source": limits.ContextSource, "output_source": limits.OutputSource,
		"metadata_source": limits.MetadataSource, "context_cap_applied": limits.ContextCapApplied,
	}
	if limits.Unknown {
		result.Status, result.Code, result.Detail = "failed", "unresolved_limits", "model limits use unresolved conservative values"
		return result
	}
	result.Status, result.Detail = "passed", "model limits resolved"
	if limits.ContextCapApplied {
		result.Code, result.Detail = "context_cap_applied", "model limits resolved with configured context cap"
	}
	return result
}

func reconcileMaintenancePhaseIssues(db *sql.DB, phases []memory.MaintenancePhaseResult, now time.Time, logger *slog.Logger) {
	if db == nil {
		return
	}
	for _, phase := range phases {
		fingerprint := "maintenance|phase|" + phase.Name
		if phase.Status == "completed" {
			_, _ = planner.ResolveOperationalIssue(db, fingerprint, "The maintenance phase completed successfully.", now)
			continue
		}
		if phase.Status != "partial" && phase.Status != "failed" {
			continue
		}
		detail := fmt.Sprintf("status=%s deferred=%d error_codes=%s", phase.Status, phase.Deferred, strings.Join(phase.ErrorCodes, ","))
		severity := "warning"
		if phase.Status == "failed" {
			severity = "error"
		}
		if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{
			Source: "maintenance", Context: phase.Name, Title: "Maintenance phase did not complete",
			Detail: detail, Severity: severity, Kind: planner.OperationalIssueKindRuntimeFailure,
			Reference: phase.Name, Fingerprint: fingerprint, OccurredAt: now,
		}); err != nil && logger != nil {
			logger.Warn("[Maintenance] Failed to record phase issue", "phase", phase.Name, "error_code", "operational_issue_persist")
		}
	}
}

func formatMorningBriefing(cfg *config.Config, finishedAt time.Time, status string, results memory.MaintenancePhaseResults, openIssues int) string {
	german := cfg != nil && strings.EqualFold(strings.TrimSpace(cfg.Server.UILanguage), "de")
	var b strings.Builder
	if german {
		fmt.Fprintf(&b, "Stand: %s\nWartung: %s; verarbeitet: %d; aufgeschoben: %d.\n", finishedAt.Local().Format(time.RFC3339), status, results.Processed, results.Deferred)
	} else {
		fmt.Fprintf(&b, "As of: %s\nMaintenance: %s; processed: %d; deferred: %d.\n", finishedAt.Local().Format(time.RFC3339), status, results.Processed, results.Deferred)
	}
	for _, check := range results.IntegrationChecks {
		fmt.Fprintf(&b, "- %s: %s", check.ID, check.Status)
		if check.Code != "" {
			fmt.Fprintf(&b, " (%s)", check.Code)
		}
		b.WriteByte('\n')
	}
	if german {
		fmt.Fprintf(&b, "Offene operative Probleme: %d.", openIssues)
	} else {
		fmt.Fprintf(&b, "Open operational issues: %d.", openIssues)
	}
	return strings.TrimSpace(b.String())
}
