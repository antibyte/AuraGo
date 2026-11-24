# Part B — plan storage and HTTP API

Part B adds `plan.json` storage, the plan availability block in the status JSON, the merge that protects server-owned fields when the user saves, and the four plan routes. All code is in `internal/server`.

**Order and dependencies (read first)**

- **Part A must land before B1.** B1 needs from Part A:
  - the `internal/videostudio/storyplan` package: types, `NewPlan`, `Decode`, `Marshal`, `Normalize`, `Validate`, and the constants;
  - `config.VideoStudioConfig.Plan`;
  - `videoStudioGenerationProfile`.
- **B1 and B2 need only Part A.** They come before Part C, because Part C's job uses B1's `readVideoStudioPlan`, `writeVideoStudioPlan`, `videoStudioPlanEnv`/`videoStudioPlanEnvFor` and `videoStudioPlanAvailabilityFor`, and may use B2's `mergeVideoStudioPlanServerFields`.
- **B3 and B4 do not compile without Part C's manager task.** They call:
  - `(*Server).videoStudioPlanAgents()`;
  - `Start`, `Stop`, `Status`, `Running`;
  - `videoStudioPlanAgentRequest` and `videoStudioPlanAgentStatus`;
  - `errVideoStudioPlanAgentBusy`, `errVideoStudioPlanAgentDailyLimit`, `errVideoStudioPlanAgentUnavailable`.
- **Execution order: A → B1 → B2 → C (at least the manager task) → B3 → B4.**
- **Tests that depend on how Part C behaves, not only on it compiling:**
  - `TestVideoStudioPlanAgentCreateUndoneWhenStartRefuses` needs `Start` to return `errVideoStudioPlanAgentUnavailable` synchronously when no LLM route resolves. The fixture server has no LLM.
  - `TestVideoStudioPlanAgentStopWithoutJob` needs `Stop` to return `false` for a project that never had a job.
  - `TestVideoStudioPlanAgentRunningRefusesPut` (B4) needs a live job. It uses Part C's test helpers from `video_studio_plan_agent_test.go` (`vspAgentEnv`, `vspScriptClient`, `vspCreateRequest`, `vspWait`) and checks that GET reports `planning`, PUT answers 409 `plan_agent_running` and the stop route answers `{"stopped":true}`.
  - Every other B3/B4 test only needs `Running` to be false and `Status` to be `(_, false)` for a project that never had a job.
- **Lock order for Part C.** B3 and B4 call `Running` while holding `manager.projectLock(id)`. The agents' own mutex must therefore never be held while waiting for a project lock.
- **Project rules (CLAUDE.md).** Before editing, run GitNexus `impact` upstream for:
  - `handleVideoStudioStatus` (B1);
  - `handleVideoStudio` (B3, B4).

  Run `detect-changes` before each commit. The GitNexus index does not cover worktrees, so the controller runs these from the main checkout (or records why not). Text search finds these callers:
  - `handleVideoStudio`: registered in `internal/server/server_routes.go:529-530`, and used by test servers (`video_studio_api_test.go:48`, `storymaker_app_browser_test.go:119`, `video_studio_browser_test.go`).
  - `handleVideoStudioStatus`: called only from `handleVideoStudio`.

---

### Task B1: plan.json storage, availability and status field

**Files:**
- Create: `internal/server/video_studio_plan_store.go`. It contains:
  - `videoStudioPlanTTSRoute`, `errVideoStudioPlanNotFound`, `errVideoStudioPlanInvalid`, `errVideoStudioPlanTooLarge`;
  - the `videoStudioPlanAvailability` type;
  - `videoStudioPlanPath`, `videoStudioPlanAvailabilityFor`, `readVideoStudioPlanBytes`, `readVideoStudioPlan`, `videoStudioPlanPrecondition`, `writeVideoStudioPlan`, `videoStudioPlanEnvFor`, `videoStudioPlanEnv`.
- Modify: `internal/server/video_studio_handlers.go`, function `handleVideoStudioStatus`: add a top-level `"plan"` field to the response map (around line 236-248).
- Test: `internal/server/video_studio_plan_store_test.go` (new).

- [ ] **Step 1: Write the failing test.** Create `internal/server/video_studio_plan_store_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
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
```

- [ ] **Step 2: Run the test and confirm it fails.**

```
go test ./internal/server -run '(?i)(VideoStudioPlanStore|VideoStudioPlanEnv|VideoStudioPlanAvailability|VideoStudioPlanStatus)' -count=1
```

Expected: `FAIL	aurago/internal/server [build failed]`, with `undefined: writeVideoStudioPlan` among other undefined plan-store names (`readVideoStudioPlan`, `videoStudioPlanAvailability`, `errVideoStudioPlanNotFound` …).

- [ ] **Step 3: Implement.** Create `internal/server/video_studio_plan_store.go`:

```go
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/tools"
	"aurago/internal/videostudio"
	"aurago/internal/videostudio/storyplan"
)

// videoStudioPlanTTSRoute is the speech route narration readiness is checked on. tools.BuildTTSConfig treats every
// route other than chat and glass like the tool route, so this is the default voice until Video Studio has its own.
const videoStudioPlanTTSRoute = "video_studio"

var (
	errVideoStudioPlanNotFound = errors.New("plan_not_found")
	// errVideoStudioPlanInvalid wraps a plan.json that exceeds storyplan.MaxPlanBytes or does not decode.
	errVideoStudioPlanInvalid = errors.New("plan_invalid")
	// errVideoStudioPlanTooLarge wraps a plan whose JSON would exceed storyplan.MaxPlanBytes.
	errVideoStudioPlanTooLarge = errors.New("plan_too_large")
)

// videoStudioPlanAvailability is what the plan panel and the planning agent may use (status JSON "plan", GET plan).
type videoStudioPlanAvailability struct {
	Enabled         bool   `json:"enabled"`          // cfg.VideoStudio.Plan.Enabled
	Supported       bool   `json:"supported"`        // profile.PlanSupported
	Reason          string `json:"reason,omitempty"` // profile.Reason, or "plan_disabled"
	ImageGeneration bool   `json:"image_generation"`
	TTS             bool   `json:"tts"`
	Music           bool   `json:"music"`
}

// videoStudioPlanPath is the project's plan document, next to project.json.
func videoStudioPlanPath(projectID string) string { return videoStudioProjectPath(projectID) + "/plan.json" }

// videoStudioPlanAvailabilityFor reports plan mode for cfg and a generation profile. Image generation counts as
// ready with an enabled provider and an API key (the Game Maker and Noisemaker rule), music with
// cfg.MusicConfigured, narration with tools.TTSReady.
func videoStudioPlanAvailabilityFor(cfg *config.Config, profile storyplan.Profile) videoStudioPlanAvailability {
	availability := videoStudioPlanAvailability{Supported: profile.PlanSupported, Reason: profile.Reason}
	if cfg == nil {
		availability.Reason = "plan_disabled"
		return availability
	}
	availability.Enabled = cfg.VideoStudio.Plan.Enabled
	if !availability.Enabled {
		availability.Reason = "plan_disabled"
	}
	availability.ImageGeneration = cfg.ImageGeneration.Enabled && strings.TrimSpace(cfg.ImageGeneration.APIKey) != ""
	availability.TTS = tools.TTSReady(cfg, videoStudioPlanTTSRoute)
	availability.Music = cfg.MusicConfigured()
	return availability
}

// readVideoStudioPlanBytes reads plan.json; a missing file (or project) is errVideoStudioPlanNotFound.
func readVideoStudioPlanBytes(ctx context.Context, svc *desktop.Service, projectID string) ([]byte, error) {
	file, entry, _, err := svc.OpenPreviewFile(ctx, videoStudioPlanPath(projectID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, errVideoStudioPlanNotFound
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	limit := int64(storyplan.MaxPlanBytes)
	if entry.Size > limit {
		return nil, fmt.Errorf("%w: plan.json exceeds %d bytes", errVideoStudioPlanInvalid, limit)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: plan.json exceeds %d bytes", errVideoStudioPlanInvalid, limit)
	}
	return data, nil
}

// readVideoStudioPlan returns the decoded and normalized plan, its raw bytes and its version (desktop.NoteVersion).
// A plan.json that does not decode is errVideoStudioPlanInvalid; its raw bytes and version are still returned, so a
// replace can put them back.
func readVideoStudioPlan(ctx context.Context, svc *desktop.Service, projectID string) (storyplan.Plan, []byte, string, error) {
	data, err := readVideoStudioPlanBytes(ctx, svc, projectID)
	if err != nil {
		return storyplan.Plan{}, nil, "", err
	}
	version := desktop.NoteVersion(data)
	plan, err := storyplan.Decode(data)
	if err != nil {
		return storyplan.Plan{}, data, version, fmt.Errorf("%w: %v", errVideoStudioPlanInvalid, err)
	}
	storyplan.Normalize(&plan)
	return plan, data, version, nil
}

// videoStudioPlanPrecondition accepts the current plan.json when it matches expectVersion: "" = the file must not
// exist, "*" = any state, else that version. A mismatch is a 412 file_conflict.
func videoStudioPlanPrecondition(path, expectVersion string) desktop.FileWritePrecondition {
	return func(state desktop.FileWriteState) error {
		version := ""
		if state.Exists {
			version = state.Version
			if version == "" {
				version = desktop.NoteVersion(state.Data)
			}
		}
		if expectVersion == "*" || (expectVersion == "" && !state.Exists) || (expectVersion != "" && state.Exists && version == expectVersion) {
			return nil
		}
		return &desktopFileConflict{Code: "file_conflict", Path: path, Version: version, Status: http.StatusPreconditionFailed}
	}
}

// writeVideoStudioPlan normalizes plan, stamps UpdatedAt (videoStudioNow) and stores it as plan.json when the current
// file matches expectVersion ("" = must not exist, "*" = unconditional, else that version; a mismatch is a
// *desktopFileConflict with status 412). The plan counts against the project storage limit like project.json
// (errVideoStudioProjectSizeLimit). A plan over storyplan.MaxPlanBytes is errVideoStudioPlanTooLarge, and a missing
// project is an os.ErrNotExist error. The caller holds manager.projectLock(projectID). It returns the new version.
func writeVideoStudioPlan(ctx context.Context, svc *desktop.Service, manager *videoStudioManager, cfg *config.Config, projectID string, plan storyplan.Plan, expectVersion string) (string, error) {
	if svc == nil || manager == nil || cfg == nil {
		return "", errors.New("video studio plan storage is unavailable")
	}
	// The desktop write creates missing folders, so a deleted project would come back. Project deletes hold the
	// project lock the caller holds, so the project still exists when the write runs.
	projectFile, _, _, err := svc.OpenPreviewFile(ctx, videoStudioProjectPath(projectID)+"/project.json")
	if err != nil {
		return "", fmt.Errorf("video studio project: %w", err)
	}
	_ = projectFile.Close()
	storyplan.Normalize(&plan)
	plan.UpdatedAt = videoStudioNow()
	data, err := storyplan.Marshal(plan)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errVideoStudioPlanTooLarge, err)
	}
	// An oversized damaged plan reads as no bytes, so its space is not credited (the check errs on the safe side).
	previous, err := readVideoStudioPlanBytes(ctx, svc, projectID)
	if err != nil && !errors.Is(err, errVideoStudioPlanNotFound) && !errors.Is(err, errVideoStudioPlanInvalid) {
		return "", err
	}
	used, err := manager.projectStorageSize(ctx, svc, projectID)
	if err != nil {
		return "", err
	}
	if used-int64(len(previous))+int64(len(data)) > int64(cfg.VideoStudio.MaxProjectSizeMB)<<20 {
		return "", errVideoStudioProjectSizeLimit
	}
	path := videoStudioPlanPath(projectID)
	if _, err := svc.WriteFileBytesConditional(ctx, path, data, desktop.SourceUser, videoStudioPlanPrecondition(path, expectVersion)); err != nil {
		return "", err
	}
	return desktop.NoteVersion(data), nil
}

// videoStudioPlanEnvFor builds the validator environment for a plan at resolution ("" = the configured default):
// the generation profile, the availability, the registry transitions and those this server's FFmpeg lacks, and the
// project's image assets (its media library records of kind image).
func videoStudioPlanEnvFor(ctx context.Context, cfg *config.Config, manager *videoStudioManager, projectID, resolution string) storyplan.Env {
	profile := videoStudioGenerationProfile(cfg, resolution)
	availability := videoStudioPlanAvailabilityFor(cfg, profile)
	registry := videostudio.EffectRegistry()
	known := make([]string, 0, len(registry.Transitions))
	for _, transition := range registry.Transitions {
		known = append(known, string(transition.ID))
	}
	// Like the status JSON: CheckRuntime lists nothing as missing when FFmpeg is not ready.
	// ponytail: CheckRuntime starts FFmpeg a few times per call; cache it if plan polling shows up in profiles.
	ffmpeg := videostudio.CheckRuntime(ctx, cfg.VideoStudio.FFmpegPath)
	unavailable := videoStudioUnavailableJSON(registry.Unavailable(ffmpeg.Missing)).Transitions
	images := map[string]bool{}
	if manager != nil {
		for _, record := range manager.mediaForProject(projectID) {
			if record.Asset.Kind == videostudio.AssetImage {
				images[record.Asset.ID] = true
			}
		}
	}
	return storyplan.Env{
		Profile:                profile,
		ImageGeneration:        availability.ImageGeneration,
		TTS:                    availability.TTS,
		Music:                  availability.Music,
		UnavailableTransitions: unavailable,
		KnownTransitions:       known,
		ImageAssetIDs:          images,
	}
}

// videoStudioPlanEnv builds the validator environment for a project's stored plan: its profile resolution, or the
// configured default when the project has no readable plan. Callers that already hold the plan use
// videoStudioPlanEnvFor.
func videoStudioPlanEnv(ctx context.Context, s *Server, cfg *config.Config, manager *videoStudioManager, projectID string) storyplan.Env {
	resolution := ""
	if s != nil {
		if svc, _, err := s.getDesktopService(ctx); err == nil {
			if plan, _, _, readErr := readVideoStudioPlan(ctx, svc, projectID); readErr == nil {
				resolution = plan.Profile.Resolution
			}
		}
	}
	return videoStudioPlanEnvFor(ctx, cfg, manager, projectID, resolution)
}
```

In `internal/server/video_studio_handlers.go`, `handleVideoStudioStatus`:

**Old:**
```go
		"unavailable":     unavailable,
		"encoder":         encoder,
	})
}
```
**New:**
```go
		"unavailable":     unavailable,
		"encoder":         encoder,
		"plan":            videoStudioPlanAvailabilityFor(cfg, videoStudioGenerationProfile(cfg, "")),
	})
}
```

- [ ] **Step 4: Run the test and confirm it passes.**

```
go test ./internal/server -run '(?i)(VideoStudioPlanStore|VideoStudioPlanEnv|VideoStudioPlanAvailability|VideoStudioPlanStatus)' -count=1
```

Expected: `ok  	aurago/internal/server`.

- [ ] **Step 5: Commit.**

```
git add internal/server/video_studio_plan_store.go internal/server/video_studio_plan_store_test.go internal/server/video_studio_handlers.go
git commit -m "feat(video-studio): store plan.json with conditional writes and report plan availability" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task B2: keep server-owned plan fields on user saves

**Files:**
- Modify: `internal/server/video_studio_plan_store.go`: append `mergeVideoStudioPlanServerFields` after `videoStudioPlanEnv`.
- Test: `internal/server/video_studio_plan_store_test.go`: append `TestVideoStudioPlanMergeServerFields` after `TestVideoStudioPlanStatusReportsAvailability`.

- [ ] **Step 1: Write the failing test.** In `internal/server/video_studio_plan_store_test.go`:

**Old:**
```go
	if !status.Plan.Enabled || !status.Plan.Supported || status.Plan.Reason != "" {
		t.Fatalf("status plan = %+v", *status.Plan)
	}
}
```
**New:**
```go
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
		cost, measured                                                      float64
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
```

- [ ] **Step 2: Run the test and confirm it fails.**

```
go test ./internal/server -run '(?i)VideoStudioPlanMerge' -count=1
```

Expected: `FAIL	aurago/internal/server [build failed]` with `undefined: mergeVideoStudioPlanServerFields`.

- [ ] **Step 3: Implement.** In `internal/server/video_studio_plan_store.go`:

**Old:**
```go
	return videoStudioPlanEnvFor(ctx, cfg, manager, projectID, resolution)
}
```
**New:**
```go
	return videoStudioPlanEnvFor(ctx, cfg, manager, projectID, resolution)
}

// mergeVideoStudioPlanServerFields returns incoming with every server-owned field taken from stored:
//   - the plan's id, version, profile, status and cost, and the music asset;
//   - per scene: the state, the produced clip and still, the timeline clip, the cost, the error and the narration
//     asset and measurement.
//
// Scenes are matched by id. A new scene gets empty server fields, and so does a second scene with an id already used.
// The keyframe asset is the user's choice only for a library keyframe; otherwise it is the stored scene's
// server-made keyframe, never a former library pick. The function is pure; storage normalizes afterwards.
func mergeVideoStudioPlanServerFields(stored, incoming storyplan.Plan) storyplan.Plan {
	merged := incoming
	merged.ID = stored.ID
	merged.Version = stored.Version
	merged.Profile = stored.Profile
	merged.Status = stored.Status
	merged.Cost = stored.Cost
	merged.Audio.Music.AssetID = stored.Audio.Music.AssetID
	byID := make(map[string]storyplan.Scene, len(stored.Scenes))
	for _, scene := range stored.Scenes {
		id := strings.TrimSpace(scene.ID)
		if _, seen := byID[id]; id != "" && !seen {
			byID[id] = scene
		}
	}
	merged.Scenes = make([]storyplan.Scene, len(incoming.Scenes))
	for i, scene := range incoming.Scenes {
		id := strings.TrimSpace(scene.ID)
		previous, found := byID[id]
		if found {
			delete(byID, id) // a later scene with the same id counts as new
		}
		scene.State = previous.State
		scene.ClipAssetID = previous.ClipAssetID
		scene.StillAssetID = previous.StillAssetID
		scene.TimelineClipID = previous.TimelineClipID
		scene.Cost = previous.Cost
		scene.Error = previous.Error
		scene.Narration.AssetID = previous.Narration.AssetID
		scene.Narration.MeasuredSeconds = previous.Narration.MeasuredSeconds
		if strings.TrimSpace(scene.Keyframe.Source) != storyplan.KeyframeLibrary {
			scene.Keyframe.AssetID = ""
			if previous.Keyframe.Source != storyplan.KeyframeLibrary {
				scene.Keyframe.AssetID = previous.Keyframe.AssetID
			}
		}
		merged.Scenes[i] = scene
	}
	return merged
}
```

- [ ] **Step 4: Run the test and confirm it passes.**

```
go test ./internal/server -run '(?i)VideoStudioPlanMerge' -count=1
```

Expected: `ok  	aurago/internal/server`.

- [ ] **Step 5: Commit.**

```
git add internal/server/video_studio_plan_store.go internal/server/video_studio_plan_store_test.go
git commit -m "feat(video-studio): keep server-owned plan fields on user saves" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task B3: GET and PUT `/projects/{id}/plan`

Requires Part C's manager task (`videoStudioPlanAgents`, `Running`, `Status`, `videoStudioPlanAgentStatus`).

**Files:**
- Create: `internal/server/video_studio_plan_handlers.go`. It contains:
  - the `videoStudioPlanResponse` type;
  - `handleVideoStudioPlan`, `handleVideoStudioPlanGet`, `handleVideoStudioPlanPut`;
  - `writeVideoStudioPlanReply`, `videoStudioPlanReportedStatus`, `writeVideoStudioPlanReadError`, `writeVideoStudioPlanWriteError`.
- Modify: `internal/server/video_studio_handlers.go`, function `handleVideoStudio`: add the `plan` case to the `projects/{id}/...` switch (lines 104-110).
- Test: `internal/server/video_studio_plan_handlers_test.go` (new).

- [ ] **Step 1: Write the failing test.** Create `internal/server/video_studio_plan_handlers_test.go`:

```go
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/videostudio/storyplan"
)

// videoStudioPlanConfigure publishes a changed copy of the config snapshot, the way a saved config does.
func videoStudioPlanConfigure(s *Server, change func(cfg *config.Config)) {
	next := *s.ConfigSnapshot()
	change(&next)
	s.replaceConfigSnapshot(&next)
}

func decodeVideoStudioPlanReply(t *testing.T, body []byte) videoStudioPlanResponse {
	t.Helper()
	var reply videoStudioPlanResponse
	if err := json.Unmarshal(body, &reply); err != nil {
		t.Fatalf("plan reply: %v %s", err, body)
	}
	return reply
}

func TestVideoStudioPlanAPIGetAndPut(t *testing.T) {
	s, readToken, writeToken, projectID := videoStudioPlanFixture(t)
	path := "/projects/" + projectID + "/plan"
	if w := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"plan_not_found"`) {
		t.Fatalf("GET without plan = %d %s", w.Code, w.Body.String())
	}
	fixture, err := json.Marshal(videoStudioPlanFixturePlan())
	if err != nil {
		t.Fatal(err)
	}
	someVersion := `"` + strings.Repeat("a", 64) + `"`
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(fixture), map[string]string{"If-Match": someVersion}); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"plan_not_found"`) {
		t.Fatalf("PUT without plan = %d %s", w.Code, w.Body.String())
	}

	seed := videoStudioPlanFixturePlan()
	seed.Status = storyplan.StatusPlanning // a job that was interrupted: GET reports review
	seed.Scenes[0].State = storyplan.SceneReady
	seed.Scenes[0].ClipAssetID = "clip-1"
	seeded := videoStudioPlanSeed(t, s, projectID, seed, "")

	got := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil)
	if got.Code != http.StatusOK || got.Header().Get("ETag") != seeded {
		t.Fatalf("GET = %d (ETag %q, want %q) %s", got.Code, got.Header().Get("ETag"), seeded, got.Body.String())
	}
	if body := got.Body.String(); !strings.Contains(body, `"agent":null`) || !strings.Contains(body, `"issues":[`) {
		t.Fatalf("GET body lacks agent null or an issues array: %s", body)
	}
	reply := decodeVideoStudioPlanReply(t, got.Body.Bytes())
	if reply.ETag != seeded || reply.Plan.Status != storyplan.StatusReview || reply.Profile.Provider != "minimax" || !reply.Availability.Enabled || len(reply.Issues) == 0 {
		t.Fatalf("GET reply = %+v", reply)
	}

	edited := reply.Plan
	edited.ID = "plan_forged"
	edited.Scenes[0].Title = "Harbour at dawn"
	edited.Scenes[0].State = storyplan.SceneFailed
	edited.Scenes[0].ClipAssetID = "forged"
	body, err := json.Marshal(edited)
	if err != nil {
		t.Fatal(err)
	}
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), nil); w.Code != http.StatusPreconditionRequired || !strings.Contains(w.Body.String(), "precondition_required") {
		t.Fatalf("PUT without If-Match = %d %s", w.Code, w.Body.String())
	}
	if w := videoStudioAPIRequest(s, readToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": seeded}); w.Code != http.StatusForbidden {
		t.Fatalf("PUT with the read token = %d", w.Code)
	}
	saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": seeded})
	if saved.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", saved.Code, saved.Body.String())
	}
	savedReply := decodeVideoStudioPlanReply(t, saved.Body.Bytes())
	version := saved.Header().Get("ETag")
	if version == "" || version == seeded || savedReply.ETag != version {
		t.Fatalf("PUT ETag %q (reply %q, before %q)", version, savedReply.ETag, seeded)
	}
	scene := savedReply.Plan.Scenes[0]
	if savedReply.Plan.ID != "plan_fixture" || savedReply.Plan.Status != storyplan.StatusReview || scene.Title != "Harbour at dawn" || scene.State != storyplan.SceneReady || scene.ClipAssetID != "clip-1" {
		t.Fatalf("PUT kept the wrong fields: %+v", savedReply.Plan)
	}
	svc, _, _ := videoStudioPlanDeps(t, s)
	stored, _, storedVersion, err := readVideoStudioPlan(context.Background(), svc, projectID)
	if err != nil || storedVersion != version || stored.Status != storyplan.StatusReview {
		t.Fatalf("stored after PUT: status %q, version %q (reply %q), err %v", stored.Status, storedVersion, version, err)
	}
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": seeded}); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale PUT = %d %s", w.Code, w.Body.String())
	}
	if w := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, map[string]string{"If-None-Match": version}); w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("GET with the current If-None-Match = %d (%d bytes)", w.Code, w.Body.Len())
	}
	if w := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, map[string]string{"If-None-Match": seeded}); w.Code != http.StatusOK {
		t.Fatalf("GET with an old If-None-Match = %d", w.Code)
	}
	for _, invalid := range []string{`{`, `{"version":1,"bogus":true}`} {
		if w := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, strings.NewReader(invalid), map[string]string{"If-Match": version}); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_plan") {
			t.Fatalf("PUT %s = %d %s", invalid, w.Code, w.Body.String())
		}
	}
	videoStudioPlanConfigure(s, func(cfg *config.Config) { cfg.VideoStudio.ReadOnly = true })
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": version}); w.Code != http.StatusForbidden {
		t.Fatalf("PUT on read-only Video Studio = %d %s", w.Code, w.Body.String())
	}
	if w := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil); w.Code != http.StatusOK {
		t.Fatalf("read-only Video Studio cannot read the plan: %d", w.Code)
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails.**

```
go test ./internal/server -run '(?i)VideoStudioPlanAPI' -count=1
```

Expected: `--- FAIL: TestVideoStudioPlanAPIGetAndPut` with `GET without plan = 404 {"code":"not_found","error":"not_found","message":"Video Studio endpoint not found."}`. The route does not exist yet.

- [ ] **Step 3: Implement.** Create `internal/server/video_studio_plan_handlers.go`:

```go
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
```

In `internal/server/video_studio_handlers.go`, `handleVideoStudio`:

**Old:**
```go
			case len(parts) == 4 && parts[2] == "exports":
				handleVideoStudioExportFile(s, w, r, parts[1], parts[3])
			default:
				writeVideoStudioError(w, http.StatusNotFound, "not_found", "Video Studio endpoint not found.")
			}
```
**New:**
```go
			case len(parts) == 4 && parts[2] == "exports":
				handleVideoStudioExportFile(s, w, r, parts[1], parts[3])
			case len(parts) == 3 && parts[2] == "plan":
				handleVideoStudioPlan(s, w, r, parts[1])
			default:
				writeVideoStudioError(w, http.StatusNotFound, "not_found", "Video Studio endpoint not found.")
			}
```

- [ ] **Step 4: Run the test and confirm it passes.**

```
go test ./internal/server -run '(?i)VideoStudioPlanAPI' -count=1
```

Expected: `ok  	aurago/internal/server`.

- [ ] **Step 5: Commit.**

```
git add internal/server/video_studio_plan_handlers.go internal/server/video_studio_plan_handlers_test.go internal/server/video_studio_handlers.go
git commit -m "feat(video-studio): GET and PUT plan routes" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task B4: plan agent start and stop routes

Requires Part C's manager task (`Start`, `Stop`, `videoStudioPlanAgentRequest`, the three `errVideoStudioPlanAgent*` errors). Three tests also depend on Part C's behaviour, as stated at the top; one of them drives a live job with Part C's test helpers.

**Files:**
- Modify: `internal/server/video_studio_plan_handlers.go`:
  - extend the import block;
  - append `videoStudioPlanAgentBody`, `handleVideoStudioPlanAgent`, `videoStudioPlanAgentRequestFrom`, `videoStudioPlanLanguage`, `videoStudioPlanAgentPrepare`, `videoStudioPlanAgentCheckRevise`, `videoStudioPlanAgentCreatePlan`, `restoreVideoStudioPlan`, `writeVideoStudioPlanAgentStartError`, `handleVideoStudioPlanAgentStop` after `writeVideoStudioPlanWriteError`.
- Modify: `internal/server/video_studio_handlers.go`, function `handleVideoStudio`: add the `plan/agent` and `plan/agent/stop` cases.
- Test: `internal/server/video_studio_plan_handlers_test.go`: extend the imports; append five tests after `TestVideoStudioPlanAPIGetAndPut`.

- [ ] **Step 1: Write the failing tests.** In `internal/server/video_studio_plan_handlers_test.go`:

**Old:**
```go
import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/videostudio/storyplan"
)
```
**New:**
```go
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/videostudio/storyplan"
)
```

**Old:**
```go
	if w := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil); w.Code != http.StatusOK {
		t.Fatalf("read-only Video Studio cannot read the plan: %d", w.Code)
	}
}
```
**New:**
```go
	if w := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil); w.Code != http.StatusOK {
		t.Fatalf("read-only Video Studio cannot read the plan: %d", w.Code)
	}
}

func TestVideoStudioPlanAgentRouteRefusals(t *testing.T) {
	s, readToken, writeToken, projectID := videoStudioPlanFixture(t)
	path := "/projects/" + projectID + "/plan/agent"
	create := `{"mode":"create","idea":"A fisherman heads out at dawn","target_seconds":30,"language":"de"}`
	expect := func(name, token, body string, status int, code string) *httptest.ResponseRecorder {
		t.Helper()
		w := videoStudioAPIRequest(s, token, http.MethodPost, path, strings.NewReader(body), nil)
		if w.Code != status || !strings.Contains(w.Body.String(), `"`+code+`"`) {
			t.Fatalf("%s = %d %s, want %d %s", name, w.Code, w.Body.String(), status, code)
		}
		return w
	}
	longIdea := strings.Repeat("x", int(storyplan.MaxIdeaChars)+1)
	for _, tc := range []struct {
		name, token, body string
		status            int
		code              string
	}{
		{"read token", readToken, create, http.StatusForbidden, "desktop_scope_required"},
		{"unknown field", writeToken, `{"mode":"create","bogus":1}`, http.StatusBadRequest, "invalid_request"},
		{"unknown mode", writeToken, `{"mode":"run"}`, http.StatusBadRequest, "invalid_mode"},
		{"blank idea", writeToken, `{"mode":"create","idea":"  ","target_seconds":30}`, http.StatusBadRequest, "invalid_idea"},
		{"long idea", writeToken, `{"mode":"create","idea":"` + longIdea + `","target_seconds":30}`, http.StatusBadRequest, "invalid_idea"},
		{"short target", writeToken, `{"mode":"create","idea":"Dawn","target_seconds":4}`, http.StatusBadRequest, "invalid_target_seconds"},
		{"long target", writeToken, `{"mode":"create","idea":"Dawn","target_seconds":121}`, http.StatusBadRequest, "invalid_target_seconds"},
		{"bad language", writeToken, `{"mode":"create","idea":"Dawn","target_seconds":30,"language":"de/x"}`, http.StatusBadRequest, "invalid_language"},
		{"revise without scene", writeToken, `{"mode":"revise","note":"Brighter"}`, http.StatusBadRequest, "invalid_scene_id"},
		{"revise without note", writeToken, `{"mode":"revise","scene_id":"s1"}`, http.StatusBadRequest, "invalid_note"},
		{"revise without plan", writeToken, `{"mode":"revise","scene_id":"s1","note":"Brighter"}`, http.StatusNotFound, "plan_not_found"},
	} {
		expect(tc.name, tc.token, tc.body, tc.status, tc.code)
	}
	videoStudioPlanSeed(t, s, projectID, videoStudioPlanFixturePlan(), "")
	expect("create over a plan", writeToken, create, http.StatusConflict, "plan_exists")
	expect("revise an unknown scene", writeToken, `{"mode":"revise","scene_id":"s9","note":"Brighter"}`, http.StatusBadRequest, "scene_not_found")
	videoStudioPlanConfigure(s, func(cfg *config.Config) { cfg.VideoStudio.Plan.Enabled = false })
	expect("plan mode off", writeToken, create, http.StatusConflict, "plan_disabled")
	videoStudioPlanConfigure(s, func(cfg *config.Config) {
		cfg.VideoStudio.Plan.Enabled = true
		cfg.VideoGeneration.ProviderType = "agnes"
	})
	if w := expect("Agnes", writeToken, create, http.StatusConflict, "plan_unsupported_provider"); !strings.Contains(w.Body.String(), `"reason":"provider_images_public_url_only"`) {
		t.Fatalf("unsupported provider without its reason: %s", w.Body.String())
	}
	videoStudioPlanConfigure(s, func(cfg *config.Config) { cfg.VideoStudio.ReadOnly = true })
	expect("read-only Video Studio", writeToken, create, http.StatusForbidden, "video_studio_readonly")
	if w := videoStudioAPIRequest(s, writeToken, http.MethodGet, path, nil, nil); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET plan/agent = %d", w.Code)
	}
}

func TestVideoStudioPlanAgentStartErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{errVideoStudioPlanAgentBusy, http.StatusConflict, "plan_agent_running"},
		{fmt.Errorf("start: %w", errVideoStudioPlanAgentDailyLimit), http.StatusTooManyRequests, "plan_daily_limit"},
		{errVideoStudioPlanAgentUnavailable, http.StatusServiceUnavailable, "plan_agent_unavailable"},
		{errors.New("disk full"), http.StatusInternalServerError, "plan_agent_start_failed"},
	} {
		w := httptest.NewRecorder()
		writeVideoStudioPlanAgentStartError(w, tc.err)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
			t.Errorf("%v = %d %s, want %d %s", tc.err, w.Code, w.Body.String(), tc.status, tc.code)
		}
	}
}

// Depends on Part C: the fixture has no LLM, so Start refuses synchronously with errVideoStudioPlanAgentUnavailable.
func TestVideoStudioPlanAgentCreateUndoneWhenStartRefuses(t *testing.T) {
	s, _, writeToken, projectID := videoStudioPlanFixture(t)
	path := "/projects/" + projectID + "/plan/agent"
	create := `{"mode":"create","idea":"A fisherman heads out at dawn","target_seconds":30,"language":"de"}`
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPost, path, strings.NewReader(create), nil); w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "plan_agent_unavailable") {
		t.Fatalf("create without an LLM = %d %s", w.Code, w.Body.String())
	}
	svc, _, _ := videoStudioPlanDeps(t, s)
	ctx := context.Background()
	if _, _, _, err := readVideoStudioPlan(ctx, svc, projectID); !errors.Is(err, errVideoStudioPlanNotFound) {
		t.Fatalf("a refused create left a plan: %v", err)
	}
	kept := videoStudioPlanSeed(t, s, projectID, videoStudioPlanFixturePlan(), "")
	replace := `{"mode":"create","idea":"Something else","target_seconds":20,"replace":true}`
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPost, path, strings.NewReader(replace), nil); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("replace without an LLM = %d %s", w.Code, w.Body.String())
	}
	plan, _, version, err := readVideoStudioPlan(ctx, svc, projectID)
	if err != nil || version != kept || plan.Idea != "A fisherman heads out at dawn" {
		t.Fatalf("a refused replace changed the plan: version %q (kept %q), idea %q, err %v", version, kept, plan.Idea, err)
	}
}

// Depends on Part C: Stop of a project without a job returns false.
func TestVideoStudioPlanAgentStopWithoutJob(t *testing.T) {
	s, readToken, writeToken, projectID := videoStudioPlanFixture(t)
	path := "/projects/" + projectID + "/plan/agent/stop"
	if w := videoStudioAPIRequest(s, readToken, http.MethodPost, path, nil, nil); w.Code != http.StatusForbidden {
		t.Fatalf("stop with the read token = %d", w.Code)
	}
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPost, path, nil, nil); w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"stopped":false}` {
		t.Fatalf("stop without a job = %d %s", w.Code, w.Body.String())
	}
	if w := videoStudioAPIRequest(s, writeToken, http.MethodGet, path, nil, nil); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET plan/agent/stop = %d", w.Code)
	}
}

// Depends on Part C (C4 job and its vsp* test helpers): while a planning job runs, GET reports it, a user save is
// refused and the stop route ends the job.
func TestVideoStudioPlanAgentRunningRefusesPut(t *testing.T) {
	reached := make(chan struct{}, 1)
	client := &vspScriptClient{
		turns: map[string][]vspTurn{agent.VideoStudioPlanPhaseOutline: {{block: true}}},
		onBlock: func() {
			select {
			case reached <- struct{}{}:
			default:
			}
		},
	}
	s, writeToken, projectID := vspAgentEnv(t, vspScriptNativeModel, client)
	readToken, _, err := s.TokenManager.Create("video studio plan read fixture", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := s.videoStudioPlanAgents()
	t.Cleanup(func() { m.Stop(projectID) })
	if _, err := m.Start(vspCreateRequest(projectID)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reached:
	case <-time.After(60 * time.Second):
		t.Fatal("the model never got the outline request")
	}
	path := "/projects/" + projectID + "/plan"
	got := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil)
	if got.Code != http.StatusOK {
		t.Fatalf("GET while the agent runs = %d %s", got.Code, got.Body.String())
	}
	reply := decodeVideoStudioPlanReply(t, got.Body.Bytes())
	if reply.Plan.Status != storyplan.StatusPlanning || reply.Agent == nil || reply.Agent.State != videoStudioPlanAgentRunning {
		t.Fatalf("GET while the agent runs = %s", got.Body.String())
	}
	body, err := json.Marshal(reply.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": reply.ETag}); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"plan_agent_running"`) {
		t.Fatalf("PUT while the agent runs = %d %s", w.Code, w.Body.String())
	}
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPost, path+"/agent/stop", nil, nil); w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"stopped":true}` {
		t.Fatalf("stop while the agent runs = %d %s", w.Code, w.Body.String())
	}
	if status := vspWait(t, m, projectID); status.State != videoStudioPlanAgentStopped {
		t.Fatalf("status after the stop route = %+v", status)
	}
}
```

- [ ] **Step 2: Run the tests and confirm they fail.**

```
go test ./internal/server -run '(?i)(VideoStudioPlanAgentRoute|VideoStudioPlanAgentStartError|VideoStudioPlanAgentCreateUndone|VideoStudioPlanAgentStopWithoutJob|VideoStudioPlanAgentRunningRefusesPut)' -count=1
```

Expected: `FAIL	aurago/internal/server [build failed]` with `undefined: writeVideoStudioPlanAgentStartError`.

- [ ] **Step 3: Implement.** In `internal/server/video_studio_plan_handlers.go`:

**Old:**
```go
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
```
**New:**
```go
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
```

**Old:**
```go
	default:
		writeVideoStudioError(w, http.StatusInternalServerError, "plan_save_failed", "Plan could not be saved.")
	}
}
```
**New:**
```go
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
	undo, ok := videoStudioPlanAgentPrepare(w, r, s, svc, manager, cfg, &req, profile, body.Replace)
	if !ok {
		return
	}
	status, err := s.videoStudioPlanAgents().Start(req)
	if err != nil {
		if undo != nil {
			undo()
		}
		writeVideoStudioPlanAgentStartError(w, err)
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
// its new plan (and returns how to undo that), a revise needs the plan and its scene. On a refusal it has answered.
// Start is called after the lock is released.
func videoStudioPlanAgentPrepare(w http.ResponseWriter, r *http.Request, s *Server, svc *desktop.Service, manager *videoStudioManager, cfg *config.Config, req *videoStudioPlanAgentRequest, profile storyplan.Profile, replace bool) (func(), bool) {
	lock := manager.projectLock(req.ProjectID)
	lock.Lock()
	defer lock.Unlock()
	if s.videoStudioPlanAgents().Running(req.ProjectID) {
		writeVideoStudioError(w, http.StatusConflict, "plan_agent_running", "The planning agent is already working on this project.")
		return nil, false
	}
	if req.Mode == agent.VideoStudioPlanModeRevise {
		return nil, videoStudioPlanAgentCheckRevise(w, r, svc, req)
	}
	return videoStudioPlanAgentCreatePlan(w, r, svc, manager, cfg, *req, profile, replace)
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
// start. The caller holds the project lock; on a refusal it has answered.
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
	undo := func() {
		lock := manager.projectLock(req.ProjectID)
		lock.Lock()
		defer lock.Unlock()
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
```

In `internal/server/video_studio_handlers.go`, `handleVideoStudio`:

**Old:**
```go
			case len(parts) == 3 && parts[2] == "plan":
				handleVideoStudioPlan(s, w, r, parts[1])
			default:
```
**New:**
```go
			case len(parts) == 3 && parts[2] == "plan":
				handleVideoStudioPlan(s, w, r, parts[1])
			case len(parts) == 4 && parts[2] == "plan" && parts[3] == "agent":
				handleVideoStudioPlanAgent(s, w, r, parts[1])
			case len(parts) == 5 && parts[2] == "plan" && parts[3] == "agent" && parts[4] == "stop":
				handleVideoStudioPlanAgentStop(s, w, r, parts[1])
			default:
```

- [ ] **Step 4: Run the tests and confirm they pass.**

```
go test ./internal/server -run '(?i)(VideoStudioPlanAgentRoute|VideoStudioPlanAgentStartError|VideoStudioPlanAgentCreateUndone|VideoStudioPlanAgentStopWithoutJob|VideoStudioPlanAgentRunningRefusesPut)' -count=1
```

Expected: `ok  	aurago/internal/server`.

- [ ] **Step 5: Commit.**

```
git add internal/server/video_studio_plan_handlers.go internal/server/video_studio_plan_handlers_test.go internal/server/video_studio_handlers.go
git commit -m "feat(video-studio): plan agent start and stop routes" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Part B batch check (no commit)

- [ ] Run `go test ./internal/server -run '(?i)(VideoStudioGenerationProfile|VideoStudioPlan)' -count=1`. Expected: `ok  	aurago/internal/server`.
- [ ] Run `go test ./internal/server -run '(?i)VideoStudio' -count=1`. Expected: `ok  	aurago/internal/server`. The existing status, project and job tests must stay green.

## Contract notes

1. **`videoStudioPlanEnv` takes no plan argument.** The brief wants the profile for `plan.Profile.Resolution`, so it reads `plan.json` itself through `s.getDesktopService`. I added `videoStudioPlanEnvFor(ctx, cfg, manager, projectID, resolution)` for callers that already hold the plan. The handlers use it, and Part C's studio should too.
2. **New private errors and what `writeVideoStudioPlan` returns.**
   - `errVideoStudioPlanInvalid` means `plan.json` is oversized or does not decode. For a decode failure, `readVideoStudioPlan` still returns the raw bytes and version.
   - `errVideoStudioPlanTooLarge` means `Marshal` went over `MaxPlanBytes`.
   - On a version mismatch `writeVideoStudioPlan` returns `*desktopFileConflict` (412); Part C maps this to `plan_conflict`.
   - A full project returns `errVideoStudioProjectSizeLimit`, an oversized plan `errVideoStudioPlanTooLarge`, and a missing project an error wrapping `os.ErrNotExist`. The function checks that `project.json` exists, so it never recreates a deleted project folder.
3. **Reported status.** GET reports `status: "planning"` while a job runs for the project, in addition to the brief's planning → review rule. This matters for revise, whose stored status stays `review`. PUT stores `review` when the stored status is still `planning`; no job can be running at that point.
4. **304 rule.** GET answers 304 only when no agent status (live or finished) is known for the project and the stored status is not `planning`. A finished job's state therefore always reaches the poller, even if its last write failed. The `ETag` header and the body `etag` are both the plan file version, so either can be sent as `If-Match`.
5. **Stop access.** `stop` uses `requireDesktopOperation(..., desktopScopeWrite, desktopStop)`, like `handleVideoStudioJobCancel`, instead of `videoStudioMutationAccess`. A running job can then be stopped on a read-only desktop or Video Studio. `desktopRequestOperation` in `desktop_operation.go` is not changed, because that file has uncommitted edits from another session.
6. **Undo on a refused create.** If `Start` refuses a create, the handler undoes its plan write: it restores the previous bytes, or removes `plan.json` if there was none. A 429 or 503 therefore never costs the user an existing plan.
   - The handler releases the project lock before calling `Start`, then re-checks `Running` under the lock.
   - A double-clicked replace can still overlap a just-started job. The undo restores the previous bytes, so they have the same version again.
7. **Revise request.** For revise, the handler copies `Idea`, `TargetSeconds` and `Language` from the stored plan into `videoStudioPlanAgentRequest`.
8. **Language.** If empty it defaults to `en`. Otherwise it is lower-cased and must be 2 to 16 characters from `[a-z0-9_-]`.
9. **Error codes not listed in the brief** (Part D may want `tr` keys for them):
   - 400: `invalid_request`, `invalid_mode`, `invalid_idea`, `invalid_target_seconds`, `invalid_language`, `invalid_scene_id`, `invalid_note`, `scene_not_found`, `invalid_plan`
   - 413: `plan_too_large`, `project_size_limit`
   - 409: `plan_invalid`, a damaged `plan.json`; the UI should offer "Neu planen", since create with `replace` overwrites it
   - 404: `project_not_found`
   - 500: `plan_read_failed`, `plan_save_failed`, `plan_agent_start_failed`
10. **Readiness rules.**
    - TTS uses route `"video_studio"`. It is not a constant in `tools`; `BuildTTSConfig` treats any route other than chat and glass like the tool route, so it works as written.
    - Image generation counts as ready when it is enabled and has a non-empty API key (the Game Maker / Noisemaker rule).
    - Music uses `cfg.MusicConfigured()`.
    - `ImageAssetIDs` come from the media library records (`manager.mediaForProject`, kind `image`), not from `project.json` assets.
11. **For Part C.**
    - `Start` must resolve the LLM route synchronously and return `errVideoStudioPlanAgentUnavailable` when that fails; the undo test relies on it.
    - The agents' mutex must not be held while waiting for a project lock (lock order: project lock → agents mutex).
    - "PUT while a job runs → 409 `plan_agent_running`" is covered by B4's `TestVideoStudioPlanAgentRunningRefusesPut`, which drives a live job with Part C's test helpers (the PUT route lands after C4, so C cannot test it).
    - `vs_plan_write` can reuse `mergeVideoStudioPlanServerFields`, then restore `idea`, `target_seconds` and `language` from the stored plan itself.
12. **For Part A.** `videoStudioConfigRootsChanged` compares `previous.VideoStudio != next.VideoStudio`, so `VideoStudioPlanConfig` must stay comparable. It is as the brief specifies it. Part A (A7) compares with `Plan` zeroed on both sides, so saving a `video_studio.plan.*` setting does not cancel running Video Studio FFmpeg jobs.
13. **Status JSON anchor.** The `"plan"` field is anchored on the `"unavailable"`/`"encoder"` lines of the response map in `handleVideoStudioStatus`. If Part A's edit touched those lines, re-anchor B1's Old/New block there.
