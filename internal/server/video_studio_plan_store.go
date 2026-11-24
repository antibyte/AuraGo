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
func videoStudioPlanPath(projectID string) string {
	return videoStudioProjectPath(projectID) + "/plan.json"
}

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
