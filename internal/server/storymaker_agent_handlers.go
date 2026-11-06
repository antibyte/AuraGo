package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
)

const (
	storymakerStartTopicRunes       = 2000
	storymakerStartInstructionRunes = 2000
	storymakerStartAudienceRunes    = 120
	storymakerStartTitleRunes       = 120
	storymakerStartDefaultSeconds   = 60
	storymakerStartMinSeconds       = 15
)

// storymakerAgentRefusal is why an agent job was not started: the HTTP status, the stable code and,
// for busy, the job in the way.
type storymakerAgentRefusal struct {
	Status  int
	Code    string
	Message string
	JobID   string
}

func (e *storymakerAgentRefusal) Error() string { return e.Code + ": " + e.Message }

// writeStorymakerAgentStartError answers a refusal, or a project store error.
func writeStorymakerAgentStartError(w http.ResponseWriter, err error) {
	var refusal *storymakerAgentRefusal
	if errors.As(err, &refusal) {
		extra := map[string]any{}
		if refusal.JobID != "" {
			extra["job_id"] = refusal.JobID
		}
		writeStorymakerErrorWith(w, refusal.Status, refusal.Code, refusal.Message, extra)
		return
	}
	mapStorymakerError(w, err, "project_not_found")
}

// storymakerAgentModelTooSmall reports whether the pinned or main model has too little context for a job.
func storymakerAgentModelTooSmall(cfg *config.Config) bool {
	_, blocked := gameMakerModelLimitBlocked(cfg, strings.TrimSpace(cfg.Storymaker.Agent.Provider), strings.TrimSpace(cfg.Storymaker.Agent.Model), false)
	return blocked
}

// storymakerAgentStatus is the agent block of GET /status. The route is the one a job would use.
func (s *Server) storymakerAgentStatus(cfg *config.Config) map[string]any {
	provider := strings.TrimSpace(cfg.Storymaker.Agent.Provider)
	if provider == "" {
		provider = cfg.LLM.Provider
	}
	route, err := s.storymakerAgentResolveRoute(cfg.Storymaker.Agent.Provider, cfg.Storymaker.Agent.Model)
	status := map[string]any{
		"enabled":            cfg.Storymaker.Agent.Enabled,
		"available":          cfg.Storymaker.Agent.Enabled && err == nil && !storymakerAgentModelTooSmall(cfg),
		"provider":           provider,
		"model":              "",
		"compact":            false,
		"text_mode":          false,
		"max_length_seconds": cfg.Storymaker.Agent.MaxLengthSeconds,
		"max_daily_jobs":     cfg.Storymaker.Agent.MaxDailyJobs,
	}
	if err == nil {
		status["model"] = route.cfg.LLM.Model
		status["compact"] = route.compact
		status["text_mode"] = route.text
	}
	return status
}

// storymakerAgentReady refuses when the agent is off, no usable model route exists or the model is too small.
func (s *Server) storymakerAgentReady(acc *storymakerAccess) error {
	if !acc.cfg.Storymaker.Agent.Enabled {
		return &storymakerAgentRefusal{Status: http.StatusForbidden, Code: "agent_disabled", Message: "The Storymaker agent is disabled in the configuration."}
	}
	if _, err := s.storymakerAgentResolveRoute(acc.cfg.Storymaker.Agent.Provider, acc.cfg.Storymaker.Agent.Model); err != nil {
		return &storymakerAgentRefusal{Status: http.StatusConflict, Code: "llm_unavailable", Message: "No usable language model is configured for the Storymaker agent."}
	}
	if storymakerAgentModelTooSmall(acc.cfg) {
		return &storymakerAgentRefusal{Status: http.StatusUnprocessableEntity, Code: "model_too_small", Message: "The selected model has too little context for the Storymaker agent. Choose a larger model."}
	}
	return nil
}

// storymakerAgentReserveDaily counts one job against storymaker.agent.max_daily_jobs. The handler calls it
// before storymakerStartAgentJob. A full day, or a count that cannot be read, is a 429 refusal.
func storymakerAgentReserveDaily(acc *storymakerAccess) error {
	if err := acc.rt.agentUsage.reserve(acc.cfg.Storymaker.Agent.MaxDailyJobs); err != nil {
		return &storymakerAgentRefusal{Status: http.StatusTooManyRequests, Code: storymakerCodeDailyLimit, Message: "The daily limit of Storymaker agent jobs is reached."}
	}
	return nil
}

// storymakerAgentReleaseDaily gives the count back for a job that was not queued.
func storymakerAgentReleaseDaily(acc *storymakerAccess) {
	acc.rt.agentUsage.release(acc.cfg.Storymaker.Agent.MaxDailyJobs)
}

// storymakerProjectActiveJob finds a queued or running job of the project in any queue.
func storymakerProjectActiveJob(rt *storymakerRuntime, projectID string) (render.Job, bool) {
	for _, job := range rt.listJobs(projectID) {
		if !job.Terminal() {
			return job, true
		}
	}
	return render.Job{}, false
}

// storymakerAgentBusy is the refusal for a project that already has a job.
func storymakerAgentBusy(jobID string) *storymakerAgentRefusal {
	return &storymakerAgentRefusal{Status: http.StatusConflict, Code: render.CodeBusy, Message: "This project already has a job. Wait until it has finished.", JobID: jobID}
}

// refuseStorymakerWhileAgentRuns answers 409 agent_job_active while an agent job of the project is
// queued or running, and reports whether it answered.
func refuseStorymakerWhileAgentRuns(w http.ResponseWriter, rt *storymakerRuntime, projectID string) bool {
	for _, job := range rt.agent.List(projectID) {
		if !job.Terminal() {
			writeStorymakerErrorWith(w, http.StatusConflict, "agent_job_active", "The agent is working on this story. Wait until it has finished or cancel it.", map[string]any{"job_id": job.ID})
			return true
		}
	}
	return false
}

// storymakerStartSeconds applies the default and clamps instead of refusing, so a chat call does not fail.
func storymakerStartSeconds(cfg *config.Config, requested int) int {
	if requested <= 0 {
		requested = storymakerStartDefaultSeconds
	}
	limit := cfg.Storymaker.Agent.MaxLengthSeconds
	if renderLimit := cfg.Storymaker.Render.MaxDurationSeconds; renderLimit > 0 && (limit <= 0 || renderLimit < limit) {
		limit = renderLimit
	}
	if requested < storymakerStartMinSeconds {
		requested = storymakerStartMinSeconds
	}
	if limit > 0 && requested > limit {
		requested = limit
	}
	return requested
}

// storymakerStartAgentJob runs the shared gates and queues the job. The project must exist. A refusal
// is a *storymakerAgentRefusal; any other error comes from the project store. It never counts the job
// against the daily limit: the caller reserves that count and releases it when this function refuses.
func (s *Server) storymakerStartAgentJob(ctx context.Context, acc *storymakerAccess, p storymakerAgentPayload) (render.Job, error) {
	if err := s.storymakerAgentReady(acc); err != nil {
		return render.Job{}, err
	}
	if _, err := acc.rt.projects.Get(ctx, p.ProjectID); err != nil {
		return render.Job{}, err
	}
	if active, busy := storymakerProjectActiveJob(acc.rt, p.ProjectID); busy {
		return render.Job{}, storymakerAgentBusy(active.ID)
	}
	if p.Kind == storymaker.AgentKindRevise {
		data, _, err := acc.rt.projects.LoadStory(ctx, p.ProjectID)
		if err != nil {
			return render.Job{}, err
		}
		story, _, err := storymaker.DecodeStory(data, storymakerLimits(acc.cfg))
		if err != nil {
			return render.Job{}, err
		}
		if storymaker.StoryIsBlank(&story) {
			return render.Job{}, &storymakerAgentRefusal{Status: http.StatusUnprocessableEntity, Code: "invalid_story", Message: "This story is empty. Start a new film instead."}
		}
	}
	p.Provider = strings.TrimSpace(acc.cfg.Storymaker.Agent.Provider)
	p.Model = strings.TrimSpace(acc.cfg.Storymaker.Agent.Model)
	p.LengthSeconds = storymakerStartSeconds(acc.cfg, p.LengthSeconds)
	p.Language = strings.TrimSpace(p.Language)
	if p.Language == "" {
		p.Language = i18n.NormalizeLang(acc.cfg.Server.UILanguage)
	}
	job, replayed, err := acc.rt.agent.Enqueue(render.EnqueueRequest{ProjectID: p.ProjectID, Kind: render.KindAgent, Coalesce: true, Payload: p})
	switch {
	case errors.Is(err, render.ErrBusy):
		return render.Job{}, &storymakerAgentRefusal{Status: http.StatusTooManyRequests, Code: render.CodeBusy, Message: "Eight agent jobs are already waiting. Try again when one has finished."}
	case err != nil:
		return render.Job{}, &storymakerAgentRefusal{Status: http.StatusInternalServerError, Code: "job_failed", Message: "The agent job could not be queued."}
	case replayed:
		return render.Job{}, storymakerAgentBusy(job.ID)
	}
	return job, nil
}

// storymakerAgentCreateProject makes the project of a new job, titled after the topic.
func storymakerAgentCreateProject(ctx context.Context, acc *storymakerAccess, topic, aspect string) (string, error) {
	title := strings.Join(strings.Fields(topic), " ")
	if utf8.RuneCountInString(title) > storymakerStartTitleRunes {
		title = strings.TrimSpace(string([]rune(title)[:storymakerStartTitleRunes]))
	}
	project, _, _, err := acc.rt.projects.CreateWithVoice(ctx, title, aspect, 0, nil)
	if err != nil {
		return "", err
	}
	return project.ID, nil
}

// decodeStorymakerAgentBody decodes a small strict JSON body; on a failure it has answered.
func decodeStorymakerAgentBody(w http.ResponseWriter, r *http.Request, v any, shape string) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, storymakerJSONLimit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The request body must be "+shape+".", nil)
		return false
	}
	return true
}

type storymakerAgentStartRequest struct {
	Topic         string            `json:"topic"`
	Audience      string            `json:"audience"`
	LengthSeconds int               `json:"length_seconds"`
	Aspect        string            `json:"aspect"`
	Language      string            `json:"language"`
	Voice         *storymaker.Voice `json:"voice"`
	ProjectID     string            `json:"project_id"`
}

// handleStorymakerAgentJobs answers POST /agent/jobs. Without project_id it creates the project and
// removes it again when the start is refused. With project_id the story must still be blank.
func handleStorymakerAgentJobs(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	var req storymakerAgentStartRequest
	if !decodeStorymakerAgentBody(w, r, &req, `{"topic": ..., "audience"?: ..., "length_seconds"?: ..., "aspect"?: ..., "language"?: ..., "voice"?: ..., "project_id"?: ...}`) {
		return
	}
	topic := strings.TrimSpace(req.Topic)
	if topic == "" || utf8.RuneCountInString(topic) > storymakerStartTopicRunes {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_topic", "The topic must be 1 to 2000 characters.", nil)
		return
	}
	if err := s.storymakerAgentReady(acc); err != nil {
		writeStorymakerAgentStartError(w, err)
		return
	}
	ctx := r.Context()
	projectID := strings.TrimSpace(req.ProjectID)
	created := false
	if projectID == "" {
		id, err := storymakerAgentCreateProject(ctx, acc, topic, req.Aspect)
		if err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		projectID, created = id, true
	} else if !storymakerAgentProjectAcceptsCreate(ctx, w, acc, projectID) {
		return
	}
	audience := strings.TrimSpace(req.Audience)
	if utf8.RuneCountInString(audience) > storymakerStartAudienceRunes {
		audience = strings.TrimSpace(string([]rune(audience)[:storymakerStartAudienceRunes]))
	}
	payload := storymakerAgentPayload{
		Kind: storymaker.AgentKindCreate, ProjectID: projectID, Topic: topic, Audience: audience,
		Language: req.Language, LengthSeconds: req.LengthSeconds, Voice: req.Voice,
	}
	// The handler counts the job before the start and gives the count back when the start refuses.
	if err := storymakerAgentReserveDaily(acc); err != nil {
		if created {
			_ = acc.rt.projects.Delete(ctx, projectID)
		}
		writeStorymakerAgentStartError(w, err)
		return
	}
	job, err := s.storymakerStartAgentJob(ctx, acc, payload)
	if err != nil {
		storymakerAgentReleaseDaily(acc)
		if created {
			_ = acc.rt.projects.Delete(ctx, projectID)
		}
		writeStorymakerAgentStartError(w, err)
		return
	}
	writeStorymakerJSON(w, http.StatusAccepted, map[string]any{"project_id": projectID, "job": job})
}

// storymakerAgentProjectAcceptsCreate checks a named project in the order 404, busy, story_not_empty;
// on a refusal it has answered.
func storymakerAgentProjectAcceptsCreate(ctx context.Context, w http.ResponseWriter, acc *storymakerAccess, projectID string) bool {
	if !storymaker.ValidProjectID(projectID) {
		writeStorymakerError(w, http.StatusNotFound, "project_not_found", "Project not found.", nil)
		return false
	}
	if _, err := acc.rt.projects.Get(ctx, projectID); err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return false
	}
	if active, busy := storymakerProjectActiveJob(acc.rt, projectID); busy {
		writeStorymakerAgentStartError(w, storymakerAgentBusy(active.ID))
		return false
	}
	data, _, err := acc.rt.projects.LoadStory(ctx, projectID)
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return false
	}
	story, _, err := storymaker.DecodeStory(data, storymakerLimits(acc.cfg))
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return false
	}
	if !storymaker.StoryIsBlank(&story) {
		writeStorymakerError(w, http.StatusConflict, "story_not_empty", "This project already has a story. Use revise instead.", nil)
		return false
	}
	return true
}

// handleStorymakerAgentRevise answers POST /projects/{id}/agent/revise.
func handleStorymakerAgentRevise(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	var req struct {
		Instruction string `json:"instruction"`
	}
	if !decodeStorymakerAgentBody(w, r, &req, `{"instruction": ...}`) {
		return
	}
	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" || utf8.RuneCountInString(instruction) > storymakerStartInstructionRunes {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_instruction", "The instruction must be 1 to 2000 characters.", nil)
		return
	}
	if err := storymakerAgentReserveDaily(acc); err != nil {
		writeStorymakerAgentStartError(w, err)
		return
	}
	job, err := s.storymakerStartAgentJob(r.Context(), acc, storymakerAgentPayload{Kind: storymaker.AgentKindRevise, ProjectID: id, Instruction: instruction})
	if err != nil {
		storymakerAgentReleaseDaily(acc)
		writeStorymakerAgentStartError(w, err)
		return
	}
	writeStorymakerJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

// storymakerMakeSilent turns the speech of a story into on-screen text (decision 11). Before it saves, it keeps the
// story as the speech backup, which restore-speech puts back. ifMatch is the etag the caller read. It returns the new
// etag, the story and the number of scenes changed. Nothing is written when no scene has speech, so an older backup
// stays. The make-silent route and chat render with silent both call it.
func storymakerMakeSilent(ctx context.Context, acc *storymakerAccess, projectID, ifMatch string) (string, storymaker.Story, int, error) {
	data, etag, err := acc.rt.projects.LoadStory(ctx, projectID)
	if err != nil {
		return "", storymaker.Story{}, 0, err
	}
	if etag != ifMatch {
		return "", storymaker.Story{}, 0, storymaker.ErrConflict
	}
	story, _, err := storymaker.DecodeStory(data, storymakerLimits(acc.cfg))
	if err != nil {
		return "", storymaker.Story{}, 0, err
	}
	storymaker.ApplyDefaults(&story)
	changed := storymaker.MakeSilent(&story)
	if changed == 0 {
		return etag, story, 0, nil
	}
	canonical, err := storymaker.CanonicalJSON(story)
	if err != nil {
		return "", storymaker.Story{}, 0, err
	}
	if err := acc.rt.projects.SaveSilentBackup(ctx, projectID, data); err != nil {
		return "", storymaker.Story{}, 0, err
	}
	saved, err := acc.rt.projects.SaveStory(ctx, projectID, canonical, ifMatch)
	if err != nil {
		_ = acc.rt.projects.RemoveSilentBackup(ctx, projectID)
		return "", storymaker.Story{}, 0, err
	}
	return saved, story, changed, nil
}

// storymakerStoryFindings validates a story the way PUT story does.
func storymakerStoryFindings(ctx context.Context, acc *storymakerAccess, projectID string, story *storymaker.Story) []storymaker.Issue {
	issues := storymaker.Validate(story, acc.rt.catalog, storymakerLimits(acc.cfg))
	if status, err := acc.rt.projects.CodeSceneStatus(ctx, projectID); err == nil {
		issues = append(issues, storymaker.CodeSceneIssues(story, status)...)
	}
	return issues
}

// writeStorymakerStoryConflict answers a failed save and names the current version on a conflict.
func writeStorymakerStoryConflict(ctx context.Context, w http.ResponseWriter, acc *storymakerAccess, projectID string, err error) {
	if errors.Is(err, storymaker.ErrConflict) {
		if _, current, readErr := acc.rt.projects.LoadStory(ctx, projectID); readErr == nil {
			w.Header().Set("ETag", current)
		}
	}
	mapStorymakerError(w, err, "project_not_found")
}

// handleStorymakerMakeSilent answers POST /projects/{id}/make-silent. A story with nothing to change
// keeps its older backup, so a second press cannot overwrite the speech with a silent copy.
func handleStorymakerMakeSilent(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	match, ok := requireStorymakerMatch(w, r)
	if !ok {
		return
	}
	if refuseStorymakerWhileAgentRuns(w, acc.rt, id) {
		return
	}
	ctx := r.Context()
	etag, story, changed, err := storymakerMakeSilent(ctx, acc, id, match)
	if err != nil {
		writeStorymakerStoryConflict(ctx, w, acc, id, err)
		return
	}
	w.Header().Set("ETag", etag)
	writeStorymakerJSON(w, http.StatusOK, map[string]any{
		"etag": etag, "issues": issuesOrEmpty(storymakerStoryFindings(ctx, acc, id, &story)), "story": story, "changed": changed,
	})
}

// handleStorymakerRestoreSpeech answers POST /projects/{id}/restore-speech: it puts the backup of
// make-silent back and deletes it.
func handleStorymakerRestoreSpeech(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	match, ok := requireStorymakerMatch(w, r)
	if !ok {
		return
	}
	if refuseStorymakerWhileAgentRuns(w, acc.rt, id) {
		return
	}
	ctx := r.Context()
	if _, err := acc.rt.projects.Get(ctx, id); err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	backup, err := acc.rt.projects.LoadSilentBackup(ctx, id)
	if errors.Is(err, fs.ErrNotExist) {
		writeStorymakerError(w, http.StatusNotFound, "no_silent_backup", "There is no copy with speech to restore.", nil)
		return
	}
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	story, _, err := storymaker.DecodeStory(backup, storymakerLimits(acc.cfg))
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	etag, err := acc.rt.projects.SaveStory(ctx, id, backup, match)
	if err != nil {
		writeStorymakerStoryConflict(ctx, w, acc, id, err)
		return
	}
	_ = acc.rt.projects.RemoveSilentBackup(ctx, id)
	storymaker.ApplyDefaults(&story)
	w.Header().Set("ETag", etag)
	writeStorymakerJSON(w, http.StatusOK, map[string]any{
		"etag": etag, "issues": issuesOrEmpty(storymakerStoryFindings(ctx, acc, id, &story)), "story": story,
	})
}
