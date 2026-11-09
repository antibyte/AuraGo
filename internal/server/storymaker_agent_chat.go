package server

import (
	"context"
	"errors"
	"io/fs"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/videostudio"
)

const (
	storymakerFacadeMaxText       = 2000
	storymakerFacadeMaxAudience   = 120
	storymakerFacadeDefaultLength = 60
	storymakerFacadeMinLength     = 15
	storymakerFacadeMaxRows       = 20
	storymakerFacadeMaxChecks     = 8
	storymakerFacadePollSeconds   = 30
	storymakerFacadeNoJobState    = "none"
)

// storymakerAgentFacade serves the chat tool. Each write call passes the same gates as the routes:
// disabled, read-only, the daily limit, and render and agent excluding each other.
type storymakerAgentFacade struct{ s *Server }

var _ storymaker.AgentFacade = (*storymakerAgentFacade)(nil)

// publishStorymakerAgentFacade hands this server's facade to the chat tool.
func publishStorymakerAgentFacade(s *Server) {
	storymaker.SetAgentFacade(&storymakerAgentFacade{s: s})
}

// withdrawStorymakerAgentFacade clears the published facade only while it belongs to s.
func withdrawStorymakerAgentFacade(s *Server) {
	if current, ok := storymaker.CurrentAgentFacade().(*storymakerAgentFacade); ok && current.s == s {
		storymaker.SetAgentFacade(nil)
	}
}

func storymakerFacadeRefusal(code, message, next string) error {
	return &storymaker.AgentError{Code: code, Message: message, NextAction: next}
}

// Available is the gate of the tool schema. Read-only stays available: status and list still work.
func (f *storymakerAgentFacade) Available() bool {
	cfg := f.s.ConfigSnapshot()
	return cfg != nil && cfg.Storymaker.Enabled && cfg.Storymaker.Agent.Enabled && cfg.VirtualDesktop.Enabled
}

func (f *storymakerAgentFacade) access(ctx context.Context, write bool) (*storymakerAccess, error) {
	cfg, err := normalizedStorymakerConfig(f.s)
	if err != nil {
		return nil, storymakerFacadeRefusal("configuration_invalid", "The Storymaker configuration is invalid.", "Tell the user to check the Storymaker settings.")
	}
	switch {
	case !cfg.Storymaker.Enabled:
		return nil, storymakerFacadeRefusal("storymaker_disabled", "Storymaker is disabled in the configuration.", "Tell the user to enable Storymaker in the settings.")
	case !cfg.Storymaker.Agent.Enabled:
		return nil, storymakerFacadeRefusal("agent_disabled", "The Storymaker agent is disabled in the configuration.", "Tell the user to enable the Storymaker agent in the settings.")
	case !cfg.VirtualDesktop.Enabled:
		return nil, storymakerFacadeRefusal("desktop_disabled", "The virtual desktop is disabled in the configuration.", "Tell the user to enable the virtual desktop.")
	case write && cfg.VirtualDesktop.ReadOnly:
		return nil, storymakerFacadeRefusal("desktop_readonly", "The desktop is read-only.", "Tell the user the desktop is read-only, so nothing can be changed.")
	}
	rt, err := f.s.storymakerRuntime()
	if err != nil {
		return nil, storymakerFacadeRefusal("storage_unavailable", "Storymaker storage is unavailable.", "Try once more shortly; if it fails again, tell the user.")
	}
	svc, _, err := f.s.getDesktopService(ctx)
	if err != nil {
		return nil, storymakerFacadeRefusal("desktop_unavailable", "The virtual desktop storage is unavailable.", "Try once more shortly; if it fails again, tell the user.")
	}
	rt.store.use(svc)
	return &storymakerAccess{cfg: cfg, rt: rt, svc: svc}, nil
}

func storymakerFacadeStoreError(err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, storymaker.ErrInvalidID):
		return storymakerFacadeRefusal("project_not_found", "Project not found.", "Call list and use a returned project_id.")
	case errors.Is(err, storymaker.ErrInvalidArgument):
		return storymakerFacadeRefusal("invalid_argument", err.Error(), "Fix the argument and try again.")
	case errors.Is(err, storymaker.ErrConflict):
		return storymakerFacadeRefusal("conflict", "The story changed while the request was prepared.", "Call status, then try again.")
	}
	return storymakerFacadeRefusal("storage_failed", "The project storage failed.", "Try once more; if it fails again, tell the user.")
}

// storymakerFacadeStartError maps a refusal of storymakerStartAgentJob to the chat tool and passes store errors on.
func storymakerFacadeStartError(err error) error {
	var refused *storymakerAgentRefusal
	if errors.As(err, &refused) {
		return storymakerFacadeRefusal(refused.Code, refused.Message, storymakerFacadeStartNextAction(refused.Code))
	}
	return storymakerFacadeStoreError(err)
}

// storymakerFacadeStartNextAction is the next step the model gets after a refused job start.
func storymakerFacadeStartNextAction(code string) string {
	switch code {
	case "agent_disabled":
		return "Tell the user to enable the Storymaker agent in the settings."
	case "daily_limit_reached":
		return "Tell the user to try again tomorrow."
	case render.CodeBusy:
		return "Call status and try again when the job has finished."
	case "llm_unavailable":
		return "Tell the user that no language model is set up for the Storymaker agent."
	case "model_too_small":
		return "Tell the user to choose a larger model for the Storymaker agent."
	case "story_not_empty":
		return "Call revise to change the story instead."
	}
	return "Tell the user the code; the Storymaker app shows the details."
}

func storymakerFacadeRequireProject(ctx context.Context, acc *storymakerAccess, id string) error {
	if _, err := acc.rt.projects.Get(ctx, id); err != nil {
		return storymakerFacadeStoreError(err)
	}
	return nil
}

func storymakerFacadeHasActiveJob(id string, managers ...*render.Manager) bool {
	for _, m := range managers {
		for _, job := range m.List(id) {
			if !job.Terminal() {
				return true
			}
		}
	}
	return false
}

// reserveDaily counts one job. max_daily_jobs fails closed: an unreadable count refuses too.
func (f *storymakerAgentFacade) reserveDaily(acc *storymakerAccess) error {
	if err := acc.rt.agentUsage.reserve(acc.cfg.Storymaker.Agent.MaxDailyJobs); err != nil {
		return storymakerFacadeRefusal("daily_limit_reached", "The daily limit of Storymaker agent jobs is reached, or the daily count could not be read.", "Tell the user to try again tomorrow.")
	}
	return nil
}

func (f *storymakerAgentFacade) releaseDaily(acc *storymakerAccess) {
	acc.rt.agentUsage.release(acc.cfg.Storymaker.Agent.MaxDailyJobs)
}

// storymakerFacadeLength applies decision 13: default 60, clamped to lower..upper, where upper is the
// smaller of the two limits and lower is 15 unless upper is smaller.
func storymakerFacadeLength(cfg *config.Config, requested int) int {
	if requested <= 0 {
		requested = storymakerFacadeDefaultLength
	}
	upper := min(cfg.Storymaker.Agent.MaxLengthSeconds, cfg.Storymaker.Render.MaxDurationSeconds)
	lower := min(storymakerFacadeMinLength, upper)
	return max(min(requested, upper), lower)
}

// storymakerFacadeETA is 60 + 25 x suggested scenes, with scenes = clamp(round(target x 2.3 / 19), 2, 24).
func storymakerFacadeETA(target int) int {
	scenes := int(math.Round(float64(target) * 2.3 / 19))
	return 60 + 25*max(min(scenes, 24), 2)
}

func storymakerFacadeClip(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return strings.TrimSpace(string([]rune(text)[:limit]))
}

func (f *storymakerAgentFacade) Create(ctx context.Context, req storymaker.AgentCreateRequest) (storymaker.AgentJobRef, error) {
	acc, err := f.access(ctx, true)
	if err != nil {
		return storymaker.AgentJobRef{}, err
	}
	topic := strings.TrimSpace(req.Topic)
	if n := utf8.RuneCountInString(topic); n == 0 || n > storymakerFacadeMaxText {
		return storymaker.AgentJobRef{}, storymakerFacadeRefusal("invalid_topic", "topic must have 1 to 2000 characters.", "Send a shorter topic.")
	}
	if err := f.reserveDaily(acc); err != nil {
		return storymaker.AgentJobRef{}, err
	}
	projectID, err := storymakerAgentCreateProject(ctx, acc, topic, req.Aspect)
	if err != nil {
		f.releaseDaily(acc)
		return storymaker.AgentJobRef{}, storymakerFacadeStoreError(err)
	}
	length := storymakerFacadeLength(acc.cfg, req.LengthSeconds)
	job, err := f.s.storymakerStartAgentJob(ctx, acc, storymakerAgentPayload{
		Kind:          storymaker.AgentKindCreate,
		ProjectID:     projectID,
		Topic:         topic,
		Audience:      storymakerFacadeClip(strings.TrimSpace(req.Audience), storymakerFacadeMaxAudience),
		Language:      strings.TrimSpace(req.Language),
		Provider:      acc.cfg.Storymaker.Agent.Provider,
		Model:         acc.cfg.Storymaker.Agent.Model,
		LengthSeconds: length,
		Voice:         req.Voice,
	})
	if err != nil {
		f.releaseDaily(acc)
		_ = acc.rt.projects.Delete(ctx, projectID)
		return storymaker.AgentJobRef{}, storymakerFacadeStartError(err)
	}
	return storymaker.AgentJobRef{ProjectID: projectID, JobID: job.ID, State: job.Status, ETAHintSeconds: storymakerFacadeETA(length)}, nil
}

func (f *storymakerAgentFacade) Revise(ctx context.Context, projectID, instruction string) (storymaker.AgentJobRef, error) {
	acc, err := f.access(ctx, true)
	if err != nil {
		return storymaker.AgentJobRef{}, err
	}
	instruction = strings.TrimSpace(instruction)
	if n := utf8.RuneCountInString(instruction); n == 0 || n > storymakerFacadeMaxText {
		return storymaker.AgentJobRef{}, storymakerFacadeRefusal("invalid_instruction", "instruction must have 1 to 2000 characters.", "Send a shorter instruction.")
	}
	data, _, err := acc.rt.projects.LoadStory(ctx, projectID)
	if err != nil {
		return storymaker.AgentJobRef{}, storymakerFacadeStoreError(err)
	}
	story, _, err := storymaker.DecodeStory(data, storymakerLimits(acc.cfg))
	if err != nil {
		return storymaker.AgentJobRef{}, storymakerFacadeRefusal(render.CodeInvalidStory, "The story file cannot be read.", "Tell the user to open the project in the Storymaker app.")
	}
	if storymaker.StoryIsBlank(&story) {
		return storymaker.AgentJobRef{}, storymakerFacadeRefusal(render.CodeInvalidStory, "The story is empty, so there is nothing to revise.", "Call create to write a story from a topic.")
	}
	if storymakerFacadeHasActiveJob(projectID, acc.rt.agent, acc.rt.jobs, acc.rt.narrate) {
		return storymaker.AgentJobRef{}, storymakerFacadeRefusal(render.CodeBusy, "A job of this project is queued or running.", "Call status and try again when it has finished.")
	}
	if err := f.reserveDaily(acc); err != nil {
		return storymaker.AgentJobRef{}, err
	}
	job, err := f.s.storymakerStartAgentJob(ctx, acc, storymakerAgentPayload{
		Kind:        storymaker.AgentKindRevise,
		ProjectID:   projectID,
		Instruction: instruction,
		Provider:    acc.cfg.Storymaker.Agent.Provider,
		Model:       acc.cfg.Storymaker.Agent.Model,
	})
	if err != nil {
		f.releaseDaily(acc)
		return storymaker.AgentJobRef{}, storymakerFacadeStartError(err)
	}
	return storymaker.AgentJobRef{ProjectID: projectID, JobID: job.ID, State: job.Status}, nil
}

func storymakerFacadeRenderNextAction(code string) string {
	switch code {
	case render.CodeNarrationIncomplete:
		return "Generate the narration in the Storymaker app, or call render again with silent:true, which removes the speech for this export."
	case render.CodeMusicIncomplete:
		return "Generate or remove the music in the Storymaker app, then call render again."
	case render.CodeBrowserUnavailable:
		return "Tell the user that the export needs Chrome or Chromium on the server."
	case render.CodeFFmpegUnavailable:
		return "Tell the user that the export needs FFmpeg on the server."
	case render.CodeInvalidStory:
		return "Call revise to fix the story, then call render again."
	case render.CodeBusy:
		return "The export queue is full. Try again when an export has finished."
	}
	return "Tell the user the code; the Storymaker app shows the details."
}

// storymakerExportPreflight runs the FFmpeg and browser gates that storymakerEnqueueRender runs first. A silent
// export calls it before the speech is removed, so a refused export never touches the story. The enqueue runs the
// same gates again.
func storymakerExportPreflight(ctx context.Context, acc *storymakerAccess) error {
	if ff := videostudio.CheckRuntime(ctx, acc.cfg.VideoStudio.FFmpegPath); !ff.Ready {
		return storymakerFacadeRefusal(render.CodeFFmpegUnavailable, "FFmpeg is not ready: "+ff.Error, storymakerFacadeRenderNextAction(render.CodeFFmpegUnavailable))
	}
	if b := acc.rt.browsers.Status(ctx, acc.cfg.Storymaker.Render.BrowserPath); !b.Ready && !b.CanDownload {
		return storymakerFacadeRefusal(render.CodeBrowserUnavailable, b.Issue, storymakerFacadeRenderNextAction(render.CodeBrowserUnavailable))
	}
	return nil
}

// storymakerRestoreSilent puts back the speech that a silent export removed. It saves the kept copy with the normal
// story save, against the etag of the silent save, and then removes the copy.
func storymakerRestoreSilent(ctx context.Context, acc *storymakerAccess, projectID, ifMatch string) error {
	backup, err := acc.rt.projects.LoadSilentBackup(ctx, projectID)
	if err != nil {
		return err
	}
	if _, err := acc.rt.projects.SaveStory(ctx, projectID, backup, ifMatch); err != nil {
		return err
	}
	return acc.rt.projects.RemoveSilentBackup(ctx, projectID)
}

func (f *storymakerAgentFacade) render(ctx context.Context, projectID string, silent bool) (string, int, error) {
	acc, err := f.access(ctx, true)
	if err != nil {
		return "", 0, err
	}
	if err := storymakerFacadeRequireProject(ctx, acc, projectID); err != nil {
		return "", 0, err
	}
	if storymakerFacadeHasActiveJob(projectID, acc.rt.agent) {
		return "", 0, storymakerFacadeRefusal("agent_job_active", "An agent job of this project is queued or running.", "Call status and try again when it has finished.")
	}
	if err := f.reserveDaily(acc); err != nil {
		return "", 0, err
	}
	if err := storymakerExportPreflight(ctx, acc); err != nil {
		f.releaseDaily(acc)
		return "", 0, err
	}
	opts, err := render.ParseRenderOptions(nil)
	if err != nil {
		f.releaseDaily(acc)
		return "", 0, storymakerFacadeRefusal("invalid_request", "The export options are invalid.", "Tell the user.")
	}
	_, etag, err := acc.rt.projects.LoadStory(ctx, projectID)
	if err != nil {
		f.releaseDaily(acc)
		return "", 0, storymakerFacadeStoreError(err)
	}
	changed := 0
	if silent {
		etag, _, changed, err = storymakerMakeSilent(ctx, acc, projectID, etag)
		if err != nil {
			f.releaseDaily(acc)
			return "", 0, storymakerFacadeStoreError(err)
		}
	}
	job, _, refusal := f.s.storymakerEnqueueRender(ctx, acc, projectID, etag, "", opts)
	if refusal != nil {
		f.releaseDaily(acc)
		if changed > 0 {
			// The export was refused after the speech was removed, so the speech goes back. If the restore fails,
			// the kept copy stays for the Restore speech action.
			_ = storymakerRestoreSilent(ctx, acc, projectID, etag)
		}
		return "", 0, storymakerFacadeRefusal(refusal.Code, refusal.Message, storymakerFacadeRenderNextAction(refusal.Code))
	}
	return job.ID, changed, nil
}

func (f *storymakerAgentFacade) Render(ctx context.Context, projectID string) (string, error) {
	id, _, err := f.render(ctx, projectID, false)
	return id, err
}

func (f *storymakerAgentFacade) RenderSilent(ctx context.Context, projectID string) (string, int, error) {
	return f.render(ctx, projectID, true)
}

func storymakerFacadeLatestAgentJob(rt *storymakerRuntime, id string) (render.Job, bool) {
	jobs := rt.agent.List(id) // newest first
	if len(jobs) == 0 {
		return render.Job{}, false
	}
	return jobs[0], true
}

// storymakerFacadeJobState reads a job as the chat states: cancelled and interrupted read as failed
// with their own error code, and a succeeded job reads as its outcome.
func storymakerFacadeJobState(job render.Job) (string, string) {
	code := ""
	if job.Error != nil {
		code = job.Error.Code
	}
	switch job.Status {
	case render.StatusSucceeded:
		if job.Agent != nil && job.Agent.Outcome != "" {
			return job.Agent.Outcome, ""
		}
		return storymaker.OutcomeIncomplete, ""
	case render.StatusFailed:
		return storymaker.OutcomeFailed, code
	case render.StatusCancelled, render.StatusInterrupted:
		if code == "" {
			code = job.Status
		}
		return storymaker.OutcomeFailed, code
	}
	return job.Status, ""
}

// storymakerFacadeRenderState looks at the newest export, but not one older than the agent job: that
// film no longer shows the story.
func storymakerFacadeRenderState(rt *storymakerRuntime, id string, since time.Time) (bool, string) {
	jobs := rt.jobs.List(id)
	if len(jobs) == 0 || jobs[0].CreatedAt.Before(since) {
		return false, ""
	}
	if !jobs[0].Terminal() {
		return true, ""
	}
	if jobs[0].Status == render.StatusSucceeded && jobs[0].Output != nil {
		return false, jobs[0].Output.Path
	}
	return false, ""
}

func storymakerFacadeNextAction(state string, checks []storymaker.AgentCheck, rendering bool) string {
	switch {
	case state == render.StatusQueued || state == render.StatusRunning:
		return "The job runs in the background and a desktop notification follows. Do not call status before poll_after_seconds."
	case rendering:
		return "An export is running. Do not call status before poll_after_seconds."
	case state == storymaker.OutcomeFailed:
		return "Tell the user the error_code. Call revise if the story has scenes, otherwise create again."
	case state == storymaker.OutcomeIncomplete:
		return "Call revise to finish the open scenes, or call open to show the project."
	case state == storymaker.OutcomeReady:
		return "The film was checked; call render to export it."
	}
	for _, check := range checks {
		if strings.HasPrefix(check.Code, "narration_") {
			return "Narration is not ready. The user can fix the voice and generate it in the app, or call render with silent:true to export without speech."
		}
	}
	return "Ask the user to watch the film once (open), then call render to export it."
}

func (f *storymakerAgentFacade) Status(ctx context.Context, projectID string) (storymaker.AgentStatus, error) {
	acc, err := f.access(ctx, false)
	if err != nil {
		return storymaker.AgentStatus{}, err
	}
	if err := storymakerFacadeRequireProject(ctx, acc, projectID); err != nil {
		return storymaker.AgentStatus{}, err
	}
	job, found := storymakerFacadeLatestAgentJob(acc.rt, projectID)
	if !found {
		return storymaker.AgentStatus{State: storymakerFacadeNoJobState, NextAction: "This project has no agent job. Call revise to change it, or open to show it."}, nil
	}
	state, code := storymakerFacadeJobState(job)
	status := storymaker.AgentStatus{State: state, ErrorCode: code}
	if job.Agent != nil {
		status.Phase = job.Agent.Phase
		status.ScenesDone = job.Agent.ScenesDone
		status.ScenesTotal = job.Agent.ScenesTotal
		status.OpenChecks = job.Agent.OpenChecks
		status.ChangedScenes = job.Agent.ChangedScenes
		status.UnchangedScenes = job.Agent.UnchangedScenes
		status.RemovedScenes = job.Agent.RemovedScenes
		if len(status.OpenChecks) > storymakerFacadeMaxChecks {
			status.OpenChecks = status.OpenChecks[:storymakerFacadeMaxChecks]
		}
	}
	rendering, output := storymakerFacadeRenderState(acc.rt, projectID, job.CreatedAt)
	status.OutputPath = output
	if !job.Terminal() || rendering {
		status.PollAfterSeconds = storymakerFacadePollSeconds
	}
	status.NextAction = storymakerFacadeNextAction(state, status.OpenChecks, rendering)
	return status, nil
}

func (f *storymakerAgentFacade) List(ctx context.Context) ([]storymaker.AgentProjectRow, error) {
	acc, err := f.access(ctx, false)
	if err != nil {
		return nil, err
	}
	projects, err := acc.rt.projects.List(ctx)
	if err != nil {
		return nil, storymakerFacadeStoreError(err)
	}
	if len(projects) > storymakerFacadeMaxRows {
		projects = projects[:storymakerFacadeMaxRows]
	}
	rows := make([]storymaker.AgentProjectRow, 0, len(projects))
	for _, project := range projects {
		row := storymaker.AgentProjectRow{
			ProjectID: project.ID,
			Title:     project.Title,
			UpdatedAt: project.UpdatedAt.UTC().Format(time.RFC3339),
			State:     storymakerFacadeNoJobState,
		}
		if job, found := storymakerFacadeLatestAgentJob(acc.rt, project.ID); found {
			row.State, _ = storymakerFacadeJobState(job)
			if job.Agent != nil {
				row.DurationSeconds = job.Agent.DurationSeconds
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}
