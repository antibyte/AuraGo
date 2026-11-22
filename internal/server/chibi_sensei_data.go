package server

import (
	"net/http"
	"path"
	"strings"
	"time"

	"aurago/internal/chibisensei"
)

// chibiSplitOf computes the split of a project from its current rows.
func chibiSplitOf(rows []chibisensei.SplitRow, p chibisensei.Project) chibisensei.SplitResult {
	return chibisensei.ComputeSplit(rows, p.SplitSeed, p.TaskKind == "classify")
}

// handleChibiChecksRun runs the dataset checks and stores the result with the row flags.
func handleChibiChecksRun(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok {
		return
	}
	ctx := r.Context()
	rows, proj, err := acc.rt.store.LoadCheckRows(ctx, id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	prof, known := chibiProfileOf(ctx, acc, proj)
	if !known {
		chibiProfileUnknown(w)
		return
	}
	srows := make([]chibisensei.SplitRow, 0, len(rows))
	for _, cr := range rows {
		srows = append(srows, chibisensei.SplitRow{ID: cr.ID, Split: cr.Split, Label: cr.Label, EstTokens: cr.EstTokens})
	}
	split := chibiSplitOf(srows, proj)
	plan, err := chibisensei.ComputePlan(chibisensei.PlanInput{Project: proj, Profile: prof, Rows: srows, Split: split,
		RemoteTimeoutMinutes: acc.cfg.Kaggle.RemoteTimeoutMinutes, NeedBaseGGUF: true})
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	res, flags := chibisensei.RunChecks(chibisensei.CheckInput{Project: proj, Profile: prof, Rows: rows, Split: split,
		MaxSeqLen: plan.Hyper.MaxSeqLen}, time.Now().UTC())
	if err := acc.rt.store.SaveChecks(ctx, res, flags); err != nil {
		chibiDomainError(w, err)
		return
	}
	acc.rt.hub.Publish(id, chibisensei.Event{Name: "checks", Data: map[string]any{"stale": false}})
	writeChibiJSON(w, http.StatusOK, res)
}

func handleChibiChecks(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	res, stale, err := acc.rt.store.GetChecks(r.Context(), id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"result": res, "stale": stale})
}

func handleChibiSplit(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	rows, proj, err := acc.rt.store.LoadSplitRows(r.Context(), id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, chibiSplitOf(rows, proj))
}

func handleChibiSplitReseed(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok {
		return
	}
	if _, err := acc.rt.store.Reseed(r.Context(), id); err != nil {
		chibiDomainError(w, err)
		return
	}
	rows, proj, err := acc.rt.store.LoadSplitRows(r.Context(), id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, chibiSplitOf(rows, proj))
}

// handleChibiPlan answers the read-only training plan of a preset.
func handleChibiPlan(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	preset := r.URL.Query().Get("preset")
	switch preset {
	case "", "quick", "balanced", "thorough":
	default:
		chibiValidation(w, "preset", "project.preset", "Unknown preset.")
		return
	}
	ctx := r.Context()
	rows, proj, err := acc.rt.store.LoadSplitRows(ctx, id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	prof, known := chibiProfileOf(ctx, acc, proj)
	if !known {
		chibiProfileUnknown(w)
		return
	}
	plan, err := chibisensei.ComputePlan(chibisensei.PlanInput{Project: proj, Profile: prof, Rows: rows,
		Split: chibiSplitOf(rows, proj), Preset: preset,
		RemoteTimeoutMinutes: acc.cfg.Kaggle.RemoteTimeoutMinutes, NeedBaseGGUF: true})
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, plan)
}

func handleChibiDocuments(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
		if !ok {
			return
		}
		if _, err := acc.rt.store.GetProject(r.Context(), id); err != nil {
			chibiDomainError(w, err)
			return
		}
		docs, err := acc.rt.store.ListDocuments(r.Context(), id)
		if err != nil {
			chibiDomainError(w, err)
			return
		}
		writeChibiJSON(w, http.StatusOK, map[string]any{"documents": docs})
	case http.MethodPost:
		acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
		if !ok {
			return
		}
		var req struct {
			WorkspacePath string `json:"workspace_path"`
		}
		if !chibiDecode(w, r, &req, chibiJSONLimit) {
			return
		}
		req.WorkspacePath = strings.TrimSpace(req.WorkspacePath)
		if req.WorkspacePath == "" {
			chibiValidation(w, "workspace_path", "document.no_path", "Name a file in the workspace.")
			return
		}
		switch strings.ToLower(path.Ext(req.WorkspacePath)) {
		case ".txt", ".md", ".markdown":
		default:
			chibiValidation(w, "workspace_path", "document.unsupported_file", "Only TXT and Markdown files are supported.")
			return
		}
		ctx := r.Context()
		if _, err := acc.rt.store.GetProject(ctx, id); err != nil {
			chibiDomainError(w, err)
			return
		}
		releaseRead, ok := chibiAcquireRead(ctx)
		if !ok {
			return // the client left while waiting for a read slot
		}
		defer releaseRead()
		// The read stops at the document limit (not the import file limit), so a big file is never loaded whole.
		docLimit := int64(acc.cfg.ChibiSensei.Import.MaxFileMB) << 20
		if docLimit > chibisensei.MaxDocumentBytes {
			docLimit = chibisensei.MaxDocumentBytes
		}
		data, err := chibiWorkspace{acc.svc}.ReadFile(ctx, req.WorkspacePath, docLimit)
		if err != nil {
			chibiWorkspaceError(w, err)
			return
		}
		doc, err := acc.rt.store.AddDocument(ctx, id, req.WorkspacePath, data)
		if err != nil {
			chibiDomainError(w, err)
			return
		}
		writeChibiJSON(w, http.StatusCreated, map[string]any{"document": doc})
	default:
		writeChibiMethodError(w, "GET, POST")
	}
}

func handleChibiDocument(s *Server, w http.ResponseWriter, r *http.Request, id, docID string) {
	if r.Method != http.MethodDelete {
		writeChibiMethodError(w, http.MethodDelete)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok {
		return
	}
	if err := acc.rt.store.DeleteDocument(r.Context(), id, docID); err != nil {
		chibiDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
