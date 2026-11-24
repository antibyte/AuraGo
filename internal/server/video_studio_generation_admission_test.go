package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// The generate admission decides durations and image modes from the capability profile. These answers are the ones
// manual generation gave before the profile existed.
func TestVideoStudioGenerationProfileDrivesGenerateAdmission(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	adminToken, _, err := s.TokenManager.Create("video studio admin fixture", []string{desktopScopeAdmin, desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	project, _ := videoStudioAPICreate(t, s, writeToken)
	s.Cfg.VideoGeneration.Enabled = true
	s.Cfg.VideoGeneration.APIKey = "fixture-only"
	s.Cfg.VideoGeneration.ResolvedModel = ""
	s.Cfg.VideoGeneration.DefaultAspectRatio = "16:9"
	for i, tc := range []struct {
		name, provider, model, resolution, fields string
		status                                    int
		code, message                             string
	}{
		{"minimax refuses a last frame", "minimax", "MiniMax-Hailuo-02", "768P", `"duration_seconds":6,"last_frame_asset_id":"img"`, http.StatusBadRequest, "unsupported_image_mode", "first-frame"},
		{"minimax refuses references", "minimax", "MiniMax-Hailuo-2.3", "768P", `"duration_seconds":6,"reference_asset_ids":["img"]`, http.StatusBadRequest, "unsupported_image_mode", "first-frame"},
		{"minimax first frame passes the mode check", "minimax", "MiniMax-Hailuo-2.3", "768P", `"duration_seconds":6,"first_frame_asset_id":"img"`, http.StatusBadRequest, "invalid_image_asset", ""},
		{"veo references need 8 s", "google_veo", "", "720p", `"duration_seconds":6,"reference_asset_ids":["img"]`, http.StatusBadRequest, "unsupported_duration", ""},
		{"minimax has no 10 s at 1080P", "minimax", "MiniMax-Hailuo-2.3", "1080P", `"duration_seconds":10`, http.StatusBadRequest, "unsupported_duration", ""},
		{"agnes is text only", "agnes", "agnes-3.0-flash", "", `"duration_seconds":6,"first_frame_asset_id":"img"`, http.StatusBadRequest, "unsupported_image_mode", "text-only"},
		{"unknown provider has no durations", "acme", "", "", `"duration_seconds":6`, http.StatusBadRequest, "unsupported_duration", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s.Cfg.VideoGeneration.ProviderType = tc.provider
			s.Cfg.VideoGeneration.Model = tc.model
			s.Cfg.VideoGeneration.DefaultResolution = tc.resolution
			body := `{"kind":"generate","prompt":"fixture",` + tc.fields + `}`
			response := videoStudioAPIRequest(s, adminToken, http.MethodPost, "/projects/"+project.ID+"/jobs", strings.NewReader(body), map[string]string{"Idempotency-Key": fmt.Sprintf("admission-%d", i)})
			if response.Code != tc.status || !strings.Contains(response.Body.String(), tc.code) || !strings.Contains(response.Body.String(), tc.message) {
				t.Fatalf("%d %s, want %d %s %q", response.Code, response.Body.String(), tc.status, tc.code, tc.message)
			}
		})
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	if jobs := manager.jobsForProject(project.ID); len(jobs) != 0 {
		t.Fatalf("refused generation requests created jobs: %+v", jobs)
	}
}
