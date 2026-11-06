package server

import (
	"time"

	"aurago/internal/desktop"
	"aurago/internal/i18n"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
)

// storymakerAgentNotifyWhenDone waits for the end of one agent job and posts its one notification.
// Every job reaches a terminal state, also when the manager closes, so this goroutine always ends.
func (s *Server) storymakerAgentNotifyWhenDone(manager *render.Manager, job render.Job) {
	changed, unsubscribe := manager.Subscribe(job.ProjectID)
	defer unsubscribe()
	for {
		current, ok := manager.Get(job.ID)
		if !ok {
			return
		}
		if current.Terminal() {
			s.notifyStorymakerAgentJob(current)
			return
		}
		<-changed
	}
}

// notifyStorymakerAgentJob is the Desktop sink for a finished agent job. It stays silent without
// Virtual Desktop.
func (s *Server) notifyStorymakerAgentJob(job render.Job) {
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		return
	}
	kind, title, message, ok := storymakerAgentNotification(desktopUILanguage(s), job)
	if !ok {
		return
	}
	s.DesktopMu.Lock()
	hub := s.DesktopHub
	s.DesktopMu.Unlock()
	broadcastDesktopEvent(s, hub, desktop.Event{Type: "notification", Payload: map[string]any{
		"title":   title,
		"message": message,
		"type":    kind,
		"appId":   "storymaker",
		"context": map[string]any{"project_id": job.ProjectID},
	}, CreatedAt: time.Now().UTC()})
}

// storymakerAgentNotification picks the text of the outcome. A cancelled or interrupted job has none.
func storymakerAgentNotification(lang string, job render.Job) (kind, title, message string, ok bool) {
	if job.Status == render.StatusCancelled || job.Status == render.StatusInterrupted {
		return "", "", "", false
	}
	outcome := ""
	if job.Agent != nil {
		outcome = job.Agent.Outcome
	}
	if job.Status == render.StatusFailed {
		outcome = storymaker.OutcomeFailed
	}
	switch outcome {
	case storymaker.OutcomeReady, storymaker.OutcomeReadyUnverified:
		return "info", i18n.T(lang, "backend.storymaker_agent_ready_title"), i18n.T(lang, "backend.storymaker_agent_ready_message"), true
	case storymaker.OutcomeIncomplete:
		return "info", i18n.T(lang, "backend.storymaker_agent_incomplete_title"),
			i18n.T(lang, "backend.storymaker_agent_incomplete_message", map[string]any{"n": storymakerAgentScenesToFinish(job.Agent)}), true
	case storymaker.OutcomeFailed:
		return "error", i18n.T(lang, "backend.storymaker_agent_failed_title"), storymakerAgentFailureMessage(lang, job), true
	}
	return "", "", "", false
}

// storymakerAgentScenesToFinish counts the scenes named by open checks, at least one.
func storymakerAgentScenesToFinish(info *storymaker.AgentJobInfo) int {
	scenes := map[string]bool{}
	if info != nil {
		for _, check := range info.OpenChecks {
			if check.SceneID != "" {
				scenes[check.SceneID] = true
			}
		}
	}
	if len(scenes) == 0 {
		return 1
	}
	return len(scenes)
}

// storymakerAgentFailureMessage is the localised job_error text of the app, which i18n merges into
// the same store as the backend texts.
func storymakerAgentFailureMessage(lang string, job render.Job) string {
	if job.Error != nil {
		key := "storymaker.job_error." + job.Error.Code
		if text := i18n.T(lang, key); text != key {
			return text
		}
	}
	return i18n.T(lang, "storymaker.job_error.unknown")
}
