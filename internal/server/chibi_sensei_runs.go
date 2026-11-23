package server

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"

	"aurago/internal/chibisensei"

	"github.com/google/uuid"
)

func chibiRunNotFound(w http.ResponseWriter) {
	writeChibiError(w, http.StatusNotFound, "run_not_found", "Run not found.", nil)
}

func chibiArtifactNotFound(w http.ResponseWriter) {
	writeChibiError(w, http.StatusNotFound, "artifact_not_found", "Artifact not found.", nil)
}

func chibiPlanExpired(w http.ResponseWriter) {
	writeChibiError(w, http.StatusConflict, "plan_expired", "The plan changed or the confirmation expired. Prepare the start again.", nil)
}

// chibiPreflightRefusal answers 409 with the failing block items in "missing": run_active when another run is active,
// license_consent_required when the license is the only gap, preflight_failed otherwise.
func chibiPreflightRefusal(w http.ResponseWriter, pf chibisensei.Preflight) {
	missing := []string{}
	for _, it := range pf.Items {
		if it.Level == chibisensei.PreflightBlock && !it.OK {
			missing = append(missing, it.ID)
		}
	}
	code, msg := "preflight_failed", "A requirement for training is not met."
	switch {
	case slices.Contains(missing, "run_active"):
		code, msg = "run_active", "A training run is active."
	case len(missing) == 1 && missing[0] == "license_consent":
		code, msg = "license_consent_required", "Accept the license of the base model first."
	}
	writeChibiJSON(w, http.StatusConflict, map[string]any{"error": code, "code": code, "message": msg, "missing": missing})
}

// chibiSnapshotBytes is the size of the canonical JSONL of the train, val and eval rows (what the snapshot will hold).
func chibiSnapshotBytes(ctx context.Context, st *chibisensei.Store, projectID string, split chibisensei.SplitResult) (int64, error) {
	var n int64
	err := st.EachRow(ctx, projectID, func(r chibisensei.Row) error {
		switch split.Assign[r.ID] {
		case "train", "val", "eval":
			n += int64(len(chibisensei.ExportLine(r)))
		}
		return nil
	})
	return n, err
}

// handleChibiRunPrepare answers POST /projects/{id}/runs/prepare {"preset"}: a fresh preflight, the confirmation data and
// a plan token bound to run ID, preset and plan hash. Nothing leaves the machine here.
func handleChibiRunPrepare(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	var req struct {
		Preset string `json:"preset"`
	}
	if !chibiDecode(w, r, &req, chibiJSONLimit) || !chibiValidPreset(w, req.Preset) {
		return
	}
	ctx := r.Context()
	tr, err := s.chibiTrainingState(ctx, r, acc, id, req.Preset, true)
	if err != nil {
		chibiTrainingError(w, err)
		return
	}
	if !tr.pf.Ready {
		chibiPreflightRefusal(w, tr.pf)
		return
	}
	dataBytes, err := chibiSnapshotBytes(ctx, acc.rt.store, id, tr.split)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	runID := uuid.NewString()
	token, ttl := acc.rt.plans.Issue(chibisensei.PlanTicket{ProjectID: id, RunID: runID, Preset: tr.plan.Preset, Hash: tr.planHash(acc.cfg)})
	owner, lic, tc := tr.in.Account, tr.prof.License, acc.cfg.ChibiSensei.Training
	var quota any
	if tr.in.QuotaKnown {
		quota = math.Round(tr.in.QuotaHours*10) / 10
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{
		"plan_token": token, "expires_in_s": int(ttl / time.Second), "run_id": runID, "plan": tr.plan,
		"destinations": map[string]any{"owner": owner,
			"dataset":       chibisensei.KaggleRef(owner, chibisensei.DatasetSlug(runID)),
			"notebook":      chibisensei.KaggleRef(owner, chibisensei.NotebookSlug(id, runID)),
			"machine_shape": tc.MachineShape, "private": true},
		"data": map[string]any{"rows": tr.plan.Split, "bytes": dataBytes},
		"license": map[string]any{"id": lic.ID, "name": lic.Name, "url": lic.URL, "version": lic.Version,
			"requires_consent": lic.RequiresConsent, "accepted": !lic.RequiresConsent || tr.in.ConsentRecorded},
		"time": map[string]any{"estimate_minutes": tr.plan.EstimateMinutes,
			"session_timeout_minutes": tr.plan.SessionTimeoutMinutes, "quota_hours_remaining": quota},
		"cleanup": map[string]any{"delete_remote_after_run": tc.DeleteRemoteAfterRun, "allow_delete": tr.in.AllowDelete},
	})
}

// handleChibiRunStart answers POST /projects/{id}/runs {"plan_token"}: consume the token (always), recompute plan and
// preflight with fresh Kaggle reads, compare the hash, submit. 409 plan_expired for an unknown, used, expired or
// mismatched token.
func handleChibiRunStart(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	var req struct {
		PlanToken string `json:"plan_token"`
	}
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	ticket, ok := acc.rt.plans.Consume(strings.TrimSpace(req.PlanToken), id)
	if !ok {
		chibiPlanExpired(w)
		return
	}
	ctx := r.Context()
	tr, err := s.chibiTrainingState(ctx, r, acc, id, ticket.Preset, true)
	if err != nil {
		chibiTrainingError(w, err)
		return
	}
	if !tr.in.ConnectionOK { // the plan hash needs the account; the preflight names the cause
		chibiPreflightRefusal(w, tr.pf)
		return
	}
	if tr.planHash(acc.cfg) != ticket.Hash {
		chibiPlanExpired(w)
		return
	}
	if !tr.pf.Ready {
		chibiPreflightRefusal(w, tr.pf)
		return
	}
	if lic := tr.prof.License; !lic.RequiresConsent {
		if _, err := acc.rt.store.RecordConsent(ctx, tr.prof.ID, lic.ID, lic.Version); err != nil {
			slog.Warn("[ChibiSensei] The license note could not be recorded", "profile", tr.prof.ID, "error", err)
		}
	}
	run, err := acc.rt.orch.Submit(ctx, chibisensei.SubmitInput{RunID: ticket.RunID, Project: tr.proj, Profile: tr.prof,
		ProfileSHA: tr.profSHA, Plan: tr.plan, Owner: tr.in.Account})
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	s.chibiStartOrchestrator(acc.rt)
	writeChibiJSON(w, http.StatusAccepted, map[string]any{"run": run})
}

// handleChibiProjectRuns: GET lists the runs of a project, POST starts one.
func handleChibiProjectRuns(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		handleChibiRunList(s, w, r, id)
	case http.MethodPost:
		handleChibiRunStart(s, w, r, id)
	default:
		writeChibiMethodError(w, "GET, POST")
	}
}

// handleChibiRunCancel is a stop operation: it works on a read-only desktop. The orchestrator only marks the run; the
// next step tries the remote cancel once.
func handleChibiRunCancel(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopStop, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	run, err := acc.rt.orch.Cancel(r.Context(), runID)
	if errors.Is(err, chibisensei.ErrNotFound) {
		chibiRunNotFound(w)
		return
	}
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	if !chibisensei.RunTerminal(run.State) {
		s.chibiStartOrchestrator(acc.rt)
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"run": run})
}

// handleChibiRunDelete deletes a finished run's rows, then its local files (RunDir, ModelDir; base GGUFs stay). Kaggle is
// not touched.
func handleChibiRunDelete(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	err := acc.rt.store.DeleteRun(r.Context(), runID)
	if errors.Is(err, chibisensei.ErrNotFound) {
		chibiRunNotFound(w)
		return
	}
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	if err := chibisensei.RemoveRunFiles(acc.rt.dataDir, runID); err != nil {
		slog.Warn("[ChibiSensei] Run files could not be removed", "run", runID, "error", err)
	}
	w.WriteHeader(http.StatusNoContent)
}
