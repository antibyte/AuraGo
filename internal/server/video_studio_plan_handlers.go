package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/uid"
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

// videoStudioPlanAgentBody is the body of POST .../plan/agent.
type videoStudioPlanAgentBody struct {
	Mode          string `json:"mode"`
	Idea          string `json:"idea"`
	TargetSeconds int    `json:"target_seconds"`
	Language      string `json:"language"`
	SceneID       string `json:"scene_id"`
	Note          string `json:"note"`
	Replace       bool   `json:"replace"`
}

// handleVideoStudioPlanAgent answers POST .../plan/agent. It checks the request, prepares the plan (a create stores a
// new empty plan) and starts the planning job. When the start is refused, a create's plan write is undone.
func handleVideoStudioPlanAgent(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeVideoStudioMethodError(w)
		return
	}
	svc, manager, cfg, ok := videoStudioMutationAccess(s, w, r, desktopScopeWrite)
	if !ok {
		return
	}
	var body videoStudioPlanAgentBody
	if err := decodeVideoStudioJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
		writeVideoStudioError(w, http.StatusBadRequest, "invalid_request", "Planning request is invalid or too large.")
		return
	}
	req, ok := videoStudioPlanAgentRequestFrom(w, id, body)
	if !ok {
		return
	}
	if !cfg.VideoStudio.Plan.Enabled {
		writeVideoStudioError(w, http.StatusConflict, "plan_disabled", "Plan mode is disabled in configuration.")
		return
	}
	profile := videoStudioGenerationProfile(cfg, "")
	if !profile.PlanSupported {
		writeVideoStudioErrorFields(w, http.StatusConflict, "plan_unsupported_provider", "The configured video generation cannot be used in plan mode.", map[string]any{"reason": profile.Reason})
		return
	}
	status, ok := videoStudioPlanAgentPrepare(w, r, s, svc, manager, cfg, &req, profile, body.Replace)
	if !ok {
		return
	}
	writeVideoStudioJSON(w, http.StatusAccepted, map[string]any{"agent": status})
}

// videoStudioPlanAgentRequestFrom checks the body of a start request and returns the job request. On a refusal it
// has answered.
func videoStudioPlanAgentRequestFrom(w http.ResponseWriter, projectID string, body videoStudioPlanAgentBody) (videoStudioPlanAgentRequest, bool) {
	req := videoStudioPlanAgentRequest{ProjectID: projectID, Mode: strings.TrimSpace(body.Mode)}
	switch req.Mode {
	case agent.VideoStudioPlanModeCreate:
		req.Idea = strings.TrimSpace(body.Idea)
		if req.Idea == "" || utf8.RuneCountInString(req.Idea) > int(storyplan.MaxIdeaChars) {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_idea", "The idea must be 1 to 4000 characters.")
			return req, false
		}
		if body.TargetSeconds < int(storyplan.MinTargetSeconds) || body.TargetSeconds > int(storyplan.MaxTotalSeconds) {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_target_seconds", "The target length must be 5 to 120 seconds.")
			return req, false
		}
		req.TargetSeconds = body.TargetSeconds
		language, ok := videoStudioPlanLanguage(body.Language)
		if !ok {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_language", "The language must be a short language tag such as de or en.")
			return req, false
		}
		req.Language = language
	case agent.VideoStudioPlanModeRevise:
		req.SceneID = strings.TrimSpace(body.SceneID)
		if req.SceneID == "" {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_scene_id", "Name the scene to revise.")
			return req, false
		}
		req.Note = strings.TrimSpace(body.Note)
		if req.Note == "" || utf8.RuneCountInString(req.Note) > int(storyplan.MaxNoteChars) {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_note", "The note must be 1 to 1000 characters.")
			return req, false
		}
	default:
		writeVideoStudioError(w, http.StatusBadRequest, "invalid_mode", "The mode must be create or revise.")
		return req, false
	}
	return req, true
}

// videoStudioPlanLanguage returns the plan language of a start request: a lower-case tag of 2 to 16 letters, digits,
// hyphens or underscores. Empty means en.
func videoStudioPlanLanguage(raw string) (string, bool) {
	language := strings.ToLower(strings.TrimSpace(raw))
	if language == "" {
		return "en", true
	}
	if len(language) < 2 || len(language) > 16 {
		return "", false
	}
	for _, r := range language {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return "", false
		}
	}
	return language, true
}

// videoStudioPlanAgentPrepare runs the plan checks of a start under the project lock: no job may run, a create stores
// its new plan, a revise needs the plan and its scene. It then starts the job still under the lock, so two starts
// cannot both pass the checks, and a refused start of a create is undone under the same lock. Lock order: project lock,
// then the agents mutex (Start never waits for a project lock; the job takes it later). On a refusal it has answered.
func videoStudioPlanAgentPrepare(w http.ResponseWriter, r *http.Request, s *Server, svc *desktop.Service, manager *videoStudioManager, cfg *config.Config, req *videoStudioPlanAgentRequest, profile storyplan.Profile, replace bool) (videoStudioPlanAgentStatus, bool) {
	lock := manager.projectLock(req.ProjectID)
	lock.Lock()
	defer lock.Unlock()
	if s.videoStudioPlanAgents().Running(req.ProjectID) {
		writeVideoStudioError(w, http.StatusConflict, "plan_agent_running", "The planning agent is already working on this project.")
		return videoStudioPlanAgentStatus{}, false
	}
	var undo func()
	if req.Mode == agent.VideoStudioPlanModeRevise {
		if !videoStudioPlanAgentCheckRevise(w, r, svc, req) {
			return videoStudioPlanAgentStatus{}, false
		}
	} else {
		var ok bool
		if undo, ok = videoStudioPlanAgentCreatePlan(w, r, svc, manager, cfg, *req, profile, replace); !ok {
			return videoStudioPlanAgentStatus{}, false
		}
	}
	status, err := s.videoStudioPlanAgents().Start(*req)
	if err != nil {
		if undo != nil {
			undo()
		}
		writeVideoStudioPlanAgentStartError(w, err)
		return videoStudioPlanAgentStatus{}, false
	}
	return status, true
}

// videoStudioPlanAgentCheckRevise checks that the plan and the scene of a revise job exist, and copies the plan's
// idea, target and language into req. On a refusal it has answered.
func videoStudioPlanAgentCheckRevise(w http.ResponseWriter, r *http.Request, svc *desktop.Service, req *videoStudioPlanAgentRequest) bool {
	plan, _, _, err := readVideoStudioPlan(r.Context(), svc, req.ProjectID)
	if err != nil {
		writeVideoStudioPlanReadError(w, err)
		return false
	}
	for _, scene := range plan.Scenes {
		if scene.ID == req.SceneID {
			req.Idea, req.TargetSeconds, req.Language = plan.Idea, plan.TargetSeconds, plan.Language
			return true
		}
	}
	writeVideoStudioError(w, http.StatusBadRequest, "scene_not_found", "The plan has no scene with this id.")
	return false
}

// videoStudioPlanAgentCreatePlan stores the empty plan of a create job with the current profile. An existing (or
// damaged) plan is replaced only with replace. It returns how to put the previous file back when the job does not
// start. The caller holds the project lock (the undo runs under it too); on a refusal it has answered.
func videoStudioPlanAgentCreatePlan(w http.ResponseWriter, r *http.Request, svc *desktop.Service, manager *videoStudioManager, cfg *config.Config, req videoStudioPlanAgentRequest, profile storyplan.Profile, replace bool) (func(), bool) {
	_, previous, version, err := readVideoStudioPlan(r.Context(), svc, req.ProjectID)
	expect := ""
	switch {
	case err == nil:
		expect = version
	case errors.Is(err, errVideoStudioPlanNotFound):
	case errors.Is(err, errVideoStudioPlanInvalid):
		expect = "*" // a damaged plan is replaced whatever it holds; the project lock keeps other writers out
	default:
		writeVideoStudioPlanReadError(w, err)
		return nil, false
	}
	if expect != "" && !replace {
		writeVideoStudioError(w, http.StatusConflict, "plan_exists", "This project already has a plan. Confirm to replace it.")
		return nil, false
	}
	ref := storyplan.ProfileRef{Provider: profile.Provider, Model: profile.Model, Resolution: profile.Resolution}
	plan := storyplan.NewPlan("plan_"+uid.New(), req.Idea, req.TargetSeconds, req.Language, ref)
	written, err := writeVideoStudioPlan(r.Context(), svc, manager, cfg, req.ProjectID, plan, expect)
	if err != nil {
		writeVideoStudioPlanWriteError(w, r, err)
		return nil, false
	}
	undo := func() { // the caller still holds the project lock
		_ = restoreVideoStudioPlan(r.Context(), svc, req.ProjectID, previous, written)
	}
	return undo, true
}

// restoreVideoStudioPlan undoes the plan write of a create whose job did not start: previous goes back, or plan.json
// is removed when there was none. Nothing changes when the file is no longer the written version. The caller holds
// manager.projectLock(projectID).
func restoreVideoStudioPlan(ctx context.Context, svc *desktop.Service, projectID string, previous []byte, written string) error {
	current, err := readVideoStudioPlanBytes(ctx, svc, projectID)
	if err != nil {
		return err
	}
	if desktop.NoteVersion(current) != written {
		return nil
	}
	path := videoStudioPlanPath(projectID)
	if previous == nil {
		return svc.DeletePath(ctx, path, desktop.SourceUser)
	}
	_, err = svc.WriteFileBytesConditional(ctx, path, previous, desktop.SourceUser, videoStudioPlanPrecondition(path, written))
	return err
}

// writeVideoStudioPlanAgentStartError answers a refused videoStudioPlanAgents.Start.
func writeVideoStudioPlanAgentStartError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errVideoStudioPlanAgentBusy):
		writeVideoStudioError(w, http.StatusConflict, "plan_agent_running", "The planning agent is already working on this project.")
	case errors.Is(err, errVideoStudioPlanAgentDailyLimit):
		writeVideoStudioError(w, http.StatusTooManyRequests, "plan_daily_limit", "The daily limit of planning jobs is reached.")
	case errors.Is(err, errVideoStudioPlanAgentUnavailable):
		writeVideoStudioError(w, http.StatusServiceUnavailable, "plan_agent_unavailable", "No language model is available for the planning agent.")
	default:
		writeVideoStudioError(w, http.StatusInternalServerError, "plan_agent_start_failed", "The planning agent could not start.")
	}
}

// handleVideoStudioPlanAgentStop answers POST .../plan/agent/stop with whether a running job was stopped. Like job
// cancel, stopping stays allowed on a read-only desktop or Video Studio, so a running job can always be ended.
func handleVideoStudioPlanAgentStop(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeVideoStudioMethodError(w)
		return
	}
	if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopStop) {
		return
	}
	writeVideoStudioJSON(w, http.StatusOK, map[string]bool{"stopped": s.videoStudioPlanAgents().Stop(id)})
}
