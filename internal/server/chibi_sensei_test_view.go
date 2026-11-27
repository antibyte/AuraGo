package server

import (
	"errors"
	"net/http"
	"slices"

	"aurago/internal/chibisensei"
)

// handleChibiRunTest answers GET /runs/{id}/test: everything the Test step shows before the first chat. The model
// keys and file paths stay on the server.
func handleChibiRunTest(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	acc, run, ok := chibiRunRead(s, w, r, runID)
	if !ok {
		return
	}
	if run.State != chibisensei.RunSucceeded {
		chibiDomainError(w, chibisensei.ErrRunNotSucceeded)
		return
	}
	prof, ok := acc.rt.catalog.Get(run.ProfileID)
	if !ok {
		chibiProfileUnknown(w)
		return
	}
	report, found, err := acc.rt.store.GetReport(r.Context(), runID, "auto")
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	var reportBody any
	if found {
		reportBody = report
	}
	recipe, taskKind, labels, terr := chibisensei.RunTask(run)
	if terr != nil {
		recipe, taskKind, labels = chibisensei.RecipeAssistant, "", nil
	}
	total := 0
	page, err := chibisensei.ReadSamples(acc.rt.dataDir, run, recipe, taskKind, labels, 0, 1, "all")
	switch {
	case err == nil:
		total = page.Total
	case !errors.Is(err, chibisensei.ErrSamplesMissing):
		chibiDomainError(w, err)
		return
	}
	tuned, base, err := chibisensei.RunModels(r.Context(), acc.rt.store, acc.rt.dataDir, run)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{
		"run": run.Summary(), "report": reportBody, "report_seen": run.Remote.ReportSeen, "samples_total": total,
		"models": map[string]any{"tuned": tuned, "base": base},
		"chat": map[string]any{"system_role": prof.Chat.SystemRole, "sampling": prof.Sampling,
			"max_tokens": chibiChatMaxTokens(prof, run)},
	})
}

// handleChibiRunSamples answers GET /runs/{id}/samples?offset=&limit=&filter=.
func handleChibiRunSamples(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	offset, ok := chibiQueryInt(w, r, "offset", 0)
	if !ok {
		return
	}
	limit, ok := chibiQueryInt(w, r, "limit", 20)
	if !ok {
		return
	}
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}
	switch {
	case offset < 0:
		chibiValidation(w, "offset", "chat.invalid", "The offset must not be negative.")
		return
	case limit < 1 || limit > chibisensei.SamplePageMax:
		chibiValidation(w, "limit", "chat.invalid", "The limit must be between 1 and 50.")
		return
	case !slices.Contains([]string{"all", "different", "wrong"}, filter):
		chibiValidation(w, "filter", "chat.invalid", "The filter must be all, different or wrong.")
		return
	}
	run, ok := chibiLoadRun(w, r, acc, runID)
	if !ok {
		return
	}
	if run.State != chibisensei.RunSucceeded {
		chibiDomainError(w, chibisensei.ErrRunNotSucceeded)
		return
	}
	recipe, taskKind, labels, terr := chibisensei.RunTask(run)
	if terr != nil {
		recipe, taskKind, labels = chibisensei.RecipeAssistant, "", nil
	}
	page, err := chibisensei.ReadSamples(acc.rt.dataDir, run, recipe, taskKind, labels, offset, limit, filter)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, page)
}

// handleChibiReportSeen answers POST /runs/{id}/report/seen: the report card was opened. It keeps the default
// write classification, so a read-only desktop refuses it.
func handleChibiReportSeen(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	if err := acc.rt.store.MarkReportSeen(r.Context(), runID); err != nil {
		if errors.Is(err, chibisensei.ErrNotFound) {
			chibiRunNotFound(w)
			return
		}
		chibiDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
