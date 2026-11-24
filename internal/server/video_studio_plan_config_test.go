package server

import (
	"testing"

	"aurago/internal/config"
)

// Plan mode settings only steer the planning agent. Saving them must not count as a Video Studio root change, which
// cancels running FFmpeg jobs; every other Video Studio setting still counts.
func TestVideoStudioPlanSettingsKeepVideoStudioJobs(t *testing.T) {
	previous := &config.Config{}
	previous.VideoStudio.Enabled = true
	previous.VideoStudio.Plan = config.VideoStudioPlanConfig{Enabled: true, MaxDailyJobs: 20, JobTimeoutSeconds: 900}
	next := *previous
	next.VideoStudio.Plan = config.VideoStudioPlanConfig{AgentProvider: "p1", AgentModel: "m1", MaxDailyJobs: 5, JobTimeoutSeconds: 120}
	if videoStudioConfigRootsChanged(previous, &next) {
		t.Fatal("a plan settings save counts as a Video Studio root change")
	}
	next.VideoStudio.RenderTimeoutSeconds = 600
	if !videoStudioConfigRootsChanged(previous, &next) {
		t.Fatal("a render timeout change must still count as a Video Studio root change")
	}
}
