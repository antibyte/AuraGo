package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"aurago/internal/config"
	"aurago/internal/videostudio/storyplan"
)

// videoStudioPlanResponse is the body of GET and PUT .../projects/{id}/plan. Agent is the project's planning job
// (live, or the last one finished on this server), null when there is none.
type videoStudioPlanResponse struct {
	Plan         storyplan.Plan              `json:"plan"`
	ETag         string                      `json:"etag"`
	Issues       []storyplan.Issue           `json:"issues"`
	Profile      storyplan.Profile           `json:"profile"`
	Availability videoStudioPlanAvailability `json:"availability"`
	Agent        *videoStudioPlanAgentStatus `json:"agent"`
}

// handleVideoStudioPlan answers GET (read access) and PUT (write access) .../projects/{id}/plan.
func handleVideoStudioPlan(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		handleVideoStudioPlanGet(s, w, r, id)
	case http.MethodPut:
		handleVideoStudioPlanPut(s, w, r, id)
	default:
		writeVideoStudioMethodError(w)
	}
}

func handleVideoStudioPlanGet(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	svc, manager, cfg, ok := videoStudioReadAccess(s, w, r)
	if !ok {
		return
	}
	plan, _, version, err := readVideoStudioPlan(r.Context(), svc, id)
	if err != nil {
		writeVideoStudioPlanReadError(w, err)
		return
	}
	// 304 only for a settled plan. A job's state (live or finished) is not part of the file version, so a project with
	// a known job always gets the full answer. A stored planning status is reported as review, so it never gets 304.
	_, agentKnown := s.videoStudioPlanAgents().Status(id)
	if strings.TrimSpace(r.Header.Get("If-None-Match")) == version && !agentKnown && plan.Status != storyplan.StatusPlanning {
		w.Header().Set("ETag", version)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeVideoStudioPlanReply(r.Context(), s, w, cfg, manager, id, plan, version)
}

// handleVideoStudioPlanPut saves the user's edits. The stored plan keeps its server-owned fields
// (mergeVideoStudioPlanServerFields), issues never block the save, and no edit is taken while a job plans.
func handleVideoStudioPlanPut(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	svc, manager, cfg, ok := videoStudioMutationAccess(s, w, r, desktopScopeWrite)
	if !ok {
		return
	}
	ifMatch := strings.TrimSpace(r.Header.Get("If-Match"))
	if ifMatch == "" {
		writeVideoStudioError(w, http.StatusPreconditionRequired, "precondition_required", "Save requires the current plan ETag in If-Match.")
		return
	}
	if _, err := desktopFilePrecondition(r); err != nil {
		writeVideoStudioFileError(w, err)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(storyplan.MaxPlanBytes)))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "plan_too_large", "Plan document exceeds 256 KiB.")
			return
		}
		writeVideoStudioError(w, http.StatusBadRequest, "invalid_plan", "Plan document could not be read.")
		return
	}
	incoming, err := storyplan.Decode(data)
	if err != nil {
		writeVideoStudioError(w, http.StatusBadRequest, "invalid_plan", "Plan document is invalid.")
		return
	}
	lock := manager.projectLock(id)
	lock.Lock()
	defer lock.Unlock()
	if s.videoStudioPlanAgents().Running(id) {
		writeVideoStudioError(w, http.StatusConflict, "plan_agent_running", "The planning agent is working on this plan.")
		return
	}
	stored, _, version, err := readVideoStudioPlan(r.Context(), svc, id)
	if err != nil {
		writeVideoStudioPlanReadError(w, err)
		return
	}
	if version != ifMatch {
		writeVideoStudioFileError(w, &desktopFileConflict{Code: "file_conflict", Path: videoStudioPlanPath(id), Version: version, Status: http.StatusPreconditionFailed})
		return
	}
	merged := mergeVideoStudioPlanServerFields(stored, incoming)
	// No job runs now, so a stored planning status is an interrupted job: the save stores the review GET reports.
	merged.Status = videoStudioPlanReportedStatus(merged.Status, false)
	if _, err := writeVideoStudioPlan(r.Context(), svc, manager, cfg, id, merged, ifMatch); err != nil {
		writeVideoStudioPlanWriteError(w, r, err)
		return
	}
	saved, _, savedVersion, err := readVideoStudioPlan(r.Context(), svc, id)
	if err != nil {
		writeVideoStudioPlanReadError(w, err)
		return
	}
	writeVideoStudioPlanReply(r.Context(), s, w, cfg, manager, id, saved, savedVersion)
}

// writeVideoStudioPlanReply answers GET and PUT: the plan with its reported status, its version (also as ETag), the
// validator issues (always an array), the profile, the availability and the agent status.
func writeVideoStudioPlanReply(ctx context.Context, s *Server, w http.ResponseWriter, cfg *config.Config, manager *videoStudioManager, projectID string, plan storyplan.Plan, version string) {
	env := videoStudioPlanEnvFor(ctx, cfg, manager, projectID, plan.Profile.Resolution)
	issues := storyplan.Validate(plan, env)
	if issues == nil {
		issues = []storyplan.Issue{}
	}
	agents := s.videoStudioPlanAgents()
	plan.Status = videoStudioPlanReportedStatus(plan.Status, agents.Running(projectID))
	reply := videoStudioPlanResponse{Plan: plan, ETag: version, Issues: issues, Profile: env.Profile, Availability: videoStudioPlanAvailabilityFor(cfg, env.Profile)}
	if status, ok := agents.Status(projectID); ok {
		reply.Agent = &status
	}
	w.Header().Set("ETag", version)
	w.Header().Set("Cache-Control", "no-store")
	writeVideoStudioJSON(w, http.StatusOK, reply)
}

// videoStudioPlanReportedStatus is the plan status a reader sees: planning while a planning job runs for the project,
// and review for a stored planning status without a job (the job was interrupted, for example by a restart).
func videoStudioPlanReportedStatus(stored string, running bool) string {
	switch {
	case running:
		return storyplan.StatusPlanning
	case stored == storyplan.StatusPlanning:
		return storyplan.StatusReview
	default:
		return stored
	}
}

// writeVideoStudioPlanReadError answers a failed readVideoStudioPlan.
func writeVideoStudioPlanReadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errVideoStudioPlanNotFound):
		writeVideoStudioError(w, http.StatusNotFound, "plan_not_found", "This project has no plan yet.")
	case errors.Is(err, errVideoStudioPlanInvalid):
		writeVideoStudioError(w, http.StatusConflict, "plan_invalid", "The stored plan is damaged. Plan again to replace it.")
	default:
		writeVideoStudioError(w, http.StatusInternalServerError, "plan_read_failed", "Plan could not be read.")
	}
}

// writeVideoStudioPlanWriteError answers a failed writeVideoStudioPlan.
func writeVideoStudioPlanWriteError(w http.ResponseWriter, r *http.Request, err error) {
	var conflict *desktopFileConflict
	switch {
	case errors.As(err, &conflict):
		writeVideoStudioFileError(w, err)
	case errors.Is(err, errVideoStudioProjectSizeLimit):
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "project_size_limit", "Project storage exceeds the configured project limit.")
	case errors.Is(err, errVideoStudioPlanTooLarge):
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "plan_too_large", "Plan document exceeds 256 KiB.")
	case errors.Is(err, os.ErrNotExist):
		writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
	case videoStudioPublicationRevoked(r, err):
		writeVideoStudioError(w, http.StatusConflict, "configuration_changed", "Video Studio configuration changed before the plan was saved.")
	default:
		writeVideoStudioError(w, http.StatusInternalServerError, "plan_save_failed", "Plan could not be saved.")
	}
}
