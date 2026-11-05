package server

import (
	"context"

	"aurago/internal/storymaker/render"
)

// storymakerNarratePayload is the payload of a narrate job.
type storymakerNarratePayload struct{ ProjectID string }

// storymakerAudioRun runs a job of the narrate queue. It gets rt directly, because
// s.storymakerRuntime() holds storymakerMu while a replaced runtime waits for its running job.
func (s *Server) storymakerAudioRun(ctx context.Context, rt *storymakerRuntime, job render.Job, payload any, h *render.Handle) (render.Output, error) {
	return render.Output{}, render.Fail(render.CodeNarrationUnavailable, "Narration is not available yet.")
}
