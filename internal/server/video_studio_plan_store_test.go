package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/videostudio"
	"aurago/internal/videostudio/storyplan"
)

// videoStudioPlanFixture is a Video Studio test server with plan mode on, a configured MiniMax provider, no FFmpeg
// and one project. It returns the server, the read and write tokens and the project id.
func videoStudioPlanFixture(t *testing.T) (*Server, string, string, string) {
	t.Helper()
	s, readToken, writeToken := videoStudioAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = "video-studio-test-missing-ffmpeg"
	s.Cfg.VideoStudio.Plan.Enabled = true
	s.Cfg.VideoGeneration.Enabled = true
	s.Cfg.VideoGeneration.APIKey = "fixture-key"
	s.Cfg.VideoGeneration.ProviderType = "minimax"
	s.Cfg.VideoGeneration.Model = "hailuo-02"
	project, _ := videoStudioAPICreate(t, s, writeToken)
	return s, readToken, writeToken, project.ID
}

// videoStudioPlanFixturePlan is a one-scene plan in review for the fixture's MiniMax profile.
func videoStudioPlanFixturePlan() storyplan.Plan {
	plan := storyplan.NewPlan("plan_fixture", "A fisherman heads out at dawn", 30, "de", storyplan.ProfileRef{Provider: "minimax", Model: "MiniMax-Hailuo-02", Resolution: "768P"})
	plan.Status = storyplan.StatusReview
	plan.Scenes = []storyplan.Scene{{
		ID:              "s1",
		Title:           "Harbour",
		VideoPrompt:     "Slow push-in on a misty harbour at dawn, fishing boats rocking, soft light",
		GenerateSeconds: 6,
		UseSeconds:      5.5,
		TransitionIn:    storyplan.Transition{Kind: storyplan.TransitionCut},
		Keyframe:        storyplan.Keyframe{Source: storyplan.KeyframeNone},
	}}
	return plan
}

// videoStudioPlanDeps returns the desktop service, manager and normalized config the plan store works with.
func videoStudioPlanDeps(t *testing.T, s *Server) (*desktop.Service, *videoStudioManager, *config.Config) {
	t.Helper()
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := normalizedVideoStudioConfig(s)
	if err != nil {
		t.Fatal(err)
	}
	return svc, manager, cfg
}

// videoStudioPlanSeed stores plan through the store, under the project lock, and returns its version.
func videoStudioPlanSeed(t *testing.T, s *Server, projectID string, plan storyplan.Plan, expectVersion string) string {
	t.Helper()
	svc, manager, cfg := videoStudioPlanDeps(t, s)
	lock := manager.projectLock(projectID)
	lock.Lock()
	defer lock.Unlock()
	version, err := writeVideoStudioPlan(context.Background(), svc, manager, cfg, projectID, plan, expectVersion)
	if err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	return version
}

func TestVideoStudioPlanStoreWritesConditionally(t *testing.T) {
	s, _, _, projectID := videoStudioPlanFixture(t)
	fixed := time.Date(2026, 11, 24, 9, 30, 0, 0, time.UTC)
	previousNow := videoStudioNow
	videoStudioNow = func() time.Time { return fixed }
	t.Cleanup(func() { videoStudioNow = previousNow })
	ctx := context.Background()
	svc, manager, cfg := videoStudioPlanDeps(t, s)
	lock := manager.projectLock(projectID)
	lock.Lock()
	defer lock.Unlock()

	if _, _, _, err := readVideoStudioPlan(ctx, svc, projectID); !errors.Is(err, errVideoStudioPlanNotFound) {
		t.Fatalf("read before the first write: %v", err)
	}
	plan := videoStudioPlanFixturePlan()
	plan.Scenes[0].Title = "  Harbour  "
	first, err := writeVideoStudioPlan(ctx, svc, manager, cfg, projectID, plan, "")
	if err != nil {
		t.Fatal(err)
	}
	stored, data, version, err := readVideoStudioPlan(ctx, svc, projectID)
	if err != nil || version != first || desktop.NoteVersion(data) != first {
		t.Fatalf("read back: version %q (wrote %q), err %v", version, first, err)
	}
	if stored.ID != "plan_fixture" || stored.Scenes[0].Title != "Harbour" || stored.Scenes[0].State != storyplan.SceneRuntimePending || !stored.UpdatedAt.Equal(fixed) {
		t.Fatalf("stored plan was not normalized and stamped: %+v", stored)
	}
	for _, expect := range []string{"", `"` + strings.Repeat("0", 64) + `"`} {
		var conflict *desktopFileConflict
		if _, err := writeVideoStudioPlan(ctx, svc, manager, cfg, projectID, plan, expect); !errors.As(err, &conflict) || conflict.Status != http.StatusPreconditionFailed {
			t.Fatalf("write expecting %q over %q: %v", expect, first, err)
		}
	}
	plan.Scenes[0].Title = "Harbour at dawn"
	second, err := writeVideoStudioPlan(ctx, svc, manager, cfg, projectID, plan, first)
	if err != nil || second == first {
		t.Fatalf("matching write: %q, %v", second, err)
	}
	if _, err := writeVideoStudioPlan(ctx, svc, manager, cfg, projectID, plan, "*"); err != nil {
		t.Fatalf("unconditional write: %v", err)
	}
	full := *cfg
	full.VideoStudio.MaxProjectSizeMB = 0
	if _, err := writeVideoStudioPlan(ctx, svc, manager, &full, projectID, plan, "*"); !errors.Is(err, errVideoStudioProjectSizeLimit) {
		t.Fatalf("write over the project limit: %v", err)
	}
	huge := plan
	huge.Idea = strings.Repeat("x", int(storyplan.MaxPlanBytes))
	if _, err := writeVideoStudioPlan(ctx, svc, manager, cfg, projectID, huge, "*"); !errors.Is(err, errVideoStudioPlanTooLarge) {
		t.Fatalf("oversized plan: %v", err)
	}
	missing := "00000000-0000-4000-8000-000000000001"
	if _, err := writeVideoStudioPlan(ctx, svc, manager, cfg, missing, plan, ""); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("write into a missing project: %v", err)
	}
	if _, err := svc.ListFiles(ctx, videoStudioProjectPath(missing)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the plan write created the missing project folder: %v", err)
	}
	if err := svc.WriteFileBytes(ctx, videoStudioPlanPath(projectID), []byte("{"), desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	if _, data, version, err := readVideoStudioPlan(ctx, svc, projectID); !errors.Is(err, errVideoStudioPlanInvalid) || string(data) != "{" || version != desktop.NoteVersion([]byte("{")) {
		t.Fatalf("damaged plan: %q %q %v", data, version, err)
	}
	if _, err := writeVideoStudioPlan(ctx, svc, manager, cfg, projectID, plan, "*"); err != nil {
		t.Fatalf("replacing a damaged plan: %v", err)
	}
}

func TestVideoStudioPlanEnvUsesThePlanResolution(t *testing.T) {
	s, _, _, projectID := videoStudioPlanFixture(t)
	ctx := context.Background()
	_, manager, cfg := videoStudioPlanDeps(t, s)
	env := videoStudioPlanEnv(ctx, s, cfg, manager, projectID)
	if env.Profile.Resolution != videoStudioGenerationProfile(cfg, "").Resolution || !env.Profile.PlanSupported {
		t.Fatalf("env without a plan: %+v", env.Profile)
	}
	if !slices.Contains(env.KnownTransitions, "dissolve") || len(env.UnavailableTransitions) != 0 || env.ImageAssetIDs == nil || len(env.ImageAssetIDs) != 0 {
		t.Fatalf("env transitions or assets: %+v", env)
	}
	plan := videoStudioPlanFixturePlan()
	plan.Profile.Resolution = "1080P"
	videoStudioPlanSeed(t, s, projectID, plan, "")
	if env := videoStudioPlanEnv(ctx, s, cfg, manager, projectID); env.Profile.Resolution != "1080P" {
		t.Fatalf("env resolution = %q, want the plan's 1080P", env.Profile.Resolution)
	}
}

func TestVideoStudioPlanAvailabilityFor(t *testing.T) {
	supported := storyplan.Profile{PlanSupported: true, Provider: "minimax"}
	agnes := storyplan.Profile{Provider: "agnes", Reason: "provider_images_public_url_only"}
	ready := &config.Config{}
	ready.VideoStudio.Plan.Enabled = true
	ready.ImageGeneration.Enabled = true
	ready.ImageGeneration.APIKey = "image-key"
	ready.TTS.Provider = "google"
	ready.MusicGeneration.Enabled = true
	ready.MusicGeneration.APIKey = "music-key"
	for _, tc := range []struct {
		name    string
		cfg     *config.Config
		profile storyplan.Profile
		want    videoStudioPlanAvailability
	}{
		{"ready", ready, supported, videoStudioPlanAvailability{Enabled: true, Supported: true, ImageGeneration: true, TTS: true, Music: true}},
		{"unsupported provider", ready, agnes, videoStudioPlanAvailability{Enabled: true, Reason: "provider_images_public_url_only", ImageGeneration: true, TTS: true, Music: true}},
		{"plan mode off", &config.Config{}, supported, videoStudioPlanAvailability{Supported: true, Reason: "plan_disabled"}},
	} {
		if got := videoStudioPlanAvailabilityFor(tc.cfg, tc.profile); got != tc.want {
			t.Errorf("%s: %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestVideoStudioPlanStatusReportsAvailability(t *testing.T) {
	s, readToken, _, _ := videoStudioPlanFixture(t)
	response := videoStudioAPIRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status: %d %s", response.Code, response.Body.String())
	}
	var status struct {
		Plan *videoStudioPlanAvailability `json:"plan"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil || status.Plan == nil {
		t.Fatalf("status without plan availability: %v %s", err, response.Body.String())
	}
	if !status.Plan.Enabled || !status.Plan.Supported || status.Plan.Reason != "" {
		t.Fatalf("status plan = %+v", *status.Plan)
	}
}

func TestVideoStudioPlanMergeServerFields(t *testing.T) {
	stored := storyplan.Plan{
		Version: 1, ID: "plan_a", Status: storyplan.StatusReview,
		Profile: storyplan.ProfileRef{Provider: "minimax", Model: "MiniMax-Hailuo-02", Resolution: "768P"},
		Cost:    storyplan.Cost{Estimate: 1.2, Spent: 0.4},
		Audio:   storyplan.Audio{Music: storyplan.Music{Prompt: "calm strings", AssetID: "music-1"}},
		Scenes: []storyplan.Scene{
			{ID: "s1", Title: "Old", State: storyplan.SceneReady, ClipAssetID: "clip-1", StillAssetID: "still-1", TimelineClipID: "tl-1", Cost: 0.2,
				Keyframe:  storyplan.Keyframe{Source: storyplan.KeyframeGenerated, AssetID: "kf-1"},
				Narration: storyplan.Narration{Text: "Old words", AssetID: "nar-1", MeasuredSeconds: 2.5}},
			{ID: "s2", Title: "Pick", State: storyplan.SceneFailed, Error: "provider_failed",
				Keyframe: storyplan.Keyframe{Source: storyplan.KeyframeLibrary, AssetID: "lib-1"}},
		},
	}
	forged := func(id, title string, keyframe storyplan.Keyframe) storyplan.Scene {
		return storyplan.Scene{ID: id, Title: title, Keyframe: keyframe, State: storyplan.SceneReady, ClipAssetID: "forged", StillAssetID: "forged",
			TimelineClipID: "forged", Cost: 9, Error: "forged", Narration: storyplan.Narration{Text: "New words", AssetID: "forged", MeasuredSeconds: 9}}
	}
	incoming := storyplan.Plan{
		Version: 1, ID: "plan_forged", Status: storyplan.StatusDone, Idea: "A new idea",
		Profile: storyplan.ProfileRef{Provider: "google"},
		Cost:    storyplan.Cost{Spent: 99},
		Audio:   storyplan.Audio{Music: storyplan.Music{Prompt: "loud drums", AssetID: "forged"}},
		Scenes: []storyplan.Scene{
			forged("s2", "Pick moved first", storyplan.Keyframe{Source: storyplan.KeyframeGenerated, AssetID: "forged"}),
			forged("s1", "New title", storyplan.Keyframe{Source: storyplan.KeyframeGenerated, AssetID: "forged"}),
			forged("s9", "Added", storyplan.Keyframe{Source: storyplan.KeyframeLibrary, AssetID: "lib-2"}),
			forged("s1", "Second s1", storyplan.Keyframe{Source: storyplan.KeyframeNone, AssetID: "forged"}),
		},
	}
	got := mergeVideoStudioPlanServerFields(stored, incoming)
	if got.ID != "plan_a" || got.Version != 1 || got.Status != storyplan.StatusReview || got.Profile != stored.Profile || got.Cost != stored.Cost ||
		got.Idea != "A new idea" || got.Audio.Music.Prompt != "loud drums" || got.Audio.Music.AssetID != "music-1" || len(got.Scenes) != 4 {
		t.Fatalf("plan fields = %+v", got)
	}
	for i, want := range []struct {
		title, state, clip, still, timeline, err, keyframe, narrationAsset string
		cost, measured                                                     float64
	}{
		{"Pick moved first", storyplan.SceneFailed, "", "", "", "provider_failed", "", "", 0, 0}, // a library pick is no server keyframe
		{"New title", storyplan.SceneReady, "clip-1", "still-1", "tl-1", "", "kf-1", "nar-1", 0.2, 2.5},
		{"Added", "", "", "", "", "", "lib-2", "", 0, 0}, // a new scene keeps the user's library pick
		{"Second s1", "", "", "", "", "", "", "", 0, 0},  // a repeated id counts as a new scene
	} {
		scene := got.Scenes[i]
		if scene.Title != want.title || scene.State != want.state || scene.ClipAssetID != want.clip || scene.StillAssetID != want.still ||
			scene.TimelineClipID != want.timeline || scene.Error != want.err || scene.Keyframe.AssetID != want.keyframe ||
			scene.Narration.AssetID != want.narrationAsset || scene.Cost != want.cost || scene.Narration.MeasuredSeconds != want.measured ||
			scene.Narration.Text != "New words" {
			t.Errorf("scene %d = %+v, want %+v", i, scene, want)
		}
	}
}

// Two env builds within the window run the FFmpeg check once.
func TestVideoStudioPlanEnvCachesRuntimeCheck(t *testing.T) {
	calls := 0
	previous := videoStudioPlanRuntimeCheck
	videoStudioPlanRuntimeCheck = func(_ context.Context, _ string) videostudio.RuntimeStatus {
		calls++
		return videostudio.RuntimeStatus{}
	}
	t.Cleanup(func() { videoStudioPlanRuntimeCheck = previous })
	cfg := &config.Config{}
	cfg.VideoStudio.FFmpegPath = filepath.Join(t.TempDir(), "ffmpeg-cache-probe")
	for i := 0; i < 2; i++ {
		videoStudioPlanEnvFor(context.Background(), cfg, nil, "project", "")
	}
	if calls != 1 {
		t.Fatalf("CheckRuntime ran %d times, want 1", calls)
	}
}
