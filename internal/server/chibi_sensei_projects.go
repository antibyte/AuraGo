package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"aurago/internal/chibisensei"
)

// chibiSuggestion computes the profile suggestion of a project. The p95 of the row sizes only matters
// for a specialist and the language only for an assistant, so each is read only then.
func chibiSuggestion(ctx context.Context, acc *chibiSenseiAccess, p chibisensei.Project) *chibisensei.Suggestion {
	p95, lang := 0, ""
	if p.Recipe == chibisensei.RecipeSpecialist && p.Rows > 0 {
		if rows, _, err := acc.rt.store.LoadSplitRows(ctx, p.ID); err == nil {
			est := make([]int, 0, len(rows))
			for _, r := range rows {
				est = append(est, r.EstTokens)
			}
			p95 = chibisensei.Percentile(est, 0.95)
		}
	}
	if p.Recipe == chibisensei.RecipeAssistant {
		var rows []chibisensei.Row
		if page, err := acc.rt.store.ListRows(ctx, p.ID, chibisensei.RowQuery{Limit: 200}); err == nil {
			rows = page.Rows
		}
		lang = chibisensei.DetectLanguage(chibisensei.LanguageSample(p.GoalText, rows))
	}
	sug := chibisensei.SuggestProfile(p.Recipe, p.Rows, p95, lang)
	return &sug
}

// chibiProjectAnswer fills the suggestion of a project.
func chibiProjectAnswer(ctx context.Context, acc *chibiSenseiAccess, p chibisensei.Project) chibisensei.Project {
	p.SuggestedProfile = chibiSuggestion(ctx, acc, p)
	return p
}

// chibiProfileOf returns the catalog profile a project plans with: its own, or the suggestion.
func chibiProfileOf(ctx context.Context, acc *chibiSenseiAccess, p chibisensei.Project) (*chibisensei.Profile, bool) {
	id := p.ProfileID
	if id == "" {
		id = chibiSuggestion(ctx, acc, p).ID
	}
	return acc.rt.catalog.Get(id)
}

func handleChibiProjects(s *Server, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
		if !ok {
			return
		}
		list, err := acc.rt.store.ListProjects(r.Context(), r.URL.Query().Get("archived") == "1")
		if err != nil {
			chibiDomainError(w, err)
			return
		}
		if list == nil {
			list = []chibisensei.ProjectSummary{}
		}
		writeChibiJSON(w, http.StatusOK, map[string]any{"projects": list})
	case http.MethodPost:
		acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
		if !ok {
			return
		}
		var in chibisensei.ProjectInput
		if !chibiDecode(w, r, &in, chibiJSONLimit) {
			return
		}
		in.ProfileID = strings.TrimSpace(in.ProfileID)
		if in.ProfileID != "" {
			if _, known := acc.rt.catalog.Get(in.ProfileID); !known {
				chibiProfileUnknown(w)
				return
			}
		} else {
			lang := chibisensei.DetectLanguage(chibisensei.LanguageSample(in.GoalText, nil))
			in.ProfileID = chibisensei.SuggestProfile(in.Recipe, 0, 0, lang).ID
		}
		p, err := acc.rt.store.CreateProject(r.Context(), in)
		if err != nil {
			chibiDomainError(w, err)
			return
		}
		w.Header().Set("Location", chibiSenseiAPIBase+"/projects/"+p.ID)
		writeChibiJSON(w, http.StatusCreated, map[string]any{"project": chibiProjectAnswer(r.Context(), acc, p)})
	default:
		writeChibiMethodError(w, "GET, POST")
	}
}

func handleChibiProject(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
		if !ok {
			return
		}
		p, err := acc.rt.store.GetProject(r.Context(), id)
		if err != nil {
			chibiDomainError(w, err)
			return
		}
		writeChibiJSON(w, http.StatusOK, map[string]any{"project": chibiProjectAnswer(r.Context(), acc, p)})
	case http.MethodPatch:
		acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
		if !ok {
			return
		}
		rev, ok := chibiIfMatch(w, r)
		if !ok {
			return
		}
		var patch chibisensei.ProjectPatch
		if !chibiDecode(w, r, &patch, chibiJSONLimit) {
			return
		}
		ctx := r.Context()
		cur, err := acc.rt.store.GetProject(ctx, id)
		if err != nil {
			chibiDomainError(w, err)
			return
		}
		if patch.ProfileID != nil {
			trimmed := strings.TrimSpace(*patch.ProfileID)
			patch.ProfileID = &trimmed
			if trimmed != "" {
				if _, known := acc.rt.catalog.Get(trimmed); !known {
					chibiProfileUnknown(w)
					return
				}
			}
		}
		if patch.ProfileID != nil || patch.Overrides != nil {
			next := cur
			if patch.ProfileID != nil {
				next.ProfileID = *patch.ProfileID
			}
			if patch.Recipe != nil {
				next.Recipe = *patch.Recipe
			}
			prof, known := chibiProfileOf(ctx, acc, next)
			if !known {
				chibiProfileUnknown(w)
				return
			}
			overrides := cur.Overrides
			if patch.Overrides != nil {
				overrides = *patch.Overrides
			}
			if issues := chibisensei.ValidateOverrides(overrides, prof); len(issues) > 0 {
				chibiDomainError(w, &chibisensei.ValidationError{Issues: issues})
				return
			}
		}
		p, err := acc.rt.store.UpdateProject(ctx, id, rev, patch)
		if err != nil {
			chibiDomainError(w, err)
			return
		}
		writeChibiJSON(w, http.StatusOK, map[string]any{"project": chibiProjectAnswer(ctx, acc, p)})
	default:
		writeChibiMethodError(w, "GET, PATCH")
	}
}

func handleChibiProjectArchive(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok {
		return
	}
	var req struct {
		Archived *bool `json:"archived"`
	}
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	if req.Archived == nil {
		writeChibiError(w, http.StatusBadRequest, "invalid_request", "The request body must be {\"archived\": true|false}.", nil)
		return
	}
	p, err := acc.rt.store.SetArchived(r.Context(), id, *req.Archived)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"project": chibiProjectAnswer(r.Context(), acc, p)})
}

// handleChibiProjectDelete deletes an archived project. The checks (archive, name, no active run) run
// before the jobs are cancelled, so a refused delete leaves the running jobs alone; after the store delete the files of
// every run go.
func handleChibiProjectDelete(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok {
		return
	}
	var req struct {
		ConfirmName string `json:"confirm_name"`
	}
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	p, err := acc.rt.store.GetProject(r.Context(), id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	if !p.Archived {
		chibiDomainError(w, chibisensei.ErrNotArchived)
		return
	}
	if req.ConfirmName != p.Name {
		chibiValidation(w, "confirm_name", "project.confirm_name", "The confirmation name does not match.")
		return
	}
	ctx := r.Context()
	active, err := acc.rt.store.ActiveRuns(ctx)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	for _, run := range active {
		if run.ProjectID == id {
			chibiDomainError(w, chibisensei.ErrRunActive)
			return
		}
	}
	runIDs, err := acc.rt.store.ListRunIDs(ctx, id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	acc.rt.jobs.CancelProject(id)
	if err := acc.rt.store.DeleteProject(ctx, id, req.ConfirmName); err != nil {
		chibiDomainError(w, err)
		return
	}
	for _, runID := range runIDs {
		if err := chibisensei.RemoveRunFiles(acc.rt.dataDir, runID); err != nil {
			slog.Warn("[ChibiSensei] Run files of a deleted project could not be removed", "run", runID, "error", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
