package server

import (
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"
	"aurago/internal/videostudio/storyplan"
)

// Values of the capability profile (spec 2026-11-23-video-studio-plan-mode-design.md §4.1). The clip costs are the
// estimates tools.GenerateVideoResult books when a provider reports none; the Google default model is
// tools.defaultGoogleVideoModel.
const (
	videoStudioDefaultMiniMaxResolution = "768P"
	videoStudioDefaultGoogleVideoModel  = "veo-3.1-generate-preview"
	videoStudioMiniMaxClipCostEstimate  = 0.20
	videoStudioGoogleClipCostEstimate   = 0.50
	videoStudioPlanMaxPromptChars       = 2000
	videoStudioPlanParallel             = 2
)

// videoStudioGenerationProfile is the capability profile of the configured video generation API (spec
// 2026-11-23-video-studio-plan-mode-design.md §4.1). It is the single source for the status JSON
// (generation.profile), the manual generate admission, the plan validator and the planning agent. resolution ""
// means cfg.VideoGeneration.DefaultResolution (MiniMax falls back to 768P). When generation is off or lacks a key or
// a provider type, the profile keeps the provider's capabilities, so manual durations stay visible, and reports
// plan_supported false with reason generation_unavailable.
func videoStudioGenerationProfile(cfg *config.Config, resolution string) storyplan.Profile {
	model := cfg.VideoGeneration.ResolvedModel
	if model == "" {
		model = cfg.VideoGeneration.Model
	}
	profile := videoStudioProviderProfile(cfg.VideoGeneration.ProviderType, model, firstNonEmptyVideoStudio(resolution, cfg.VideoGeneration.DefaultResolution))
	if !videoStudioGenerationConfigured(cfg) {
		profile.PlanSupported = false
		profile.Reason = storyplan.ReasonGenerationUnavailable
	}
	return profile
}

// videoStudioGenerationConfigured reports video generation switched on with an API key and a provider type.
func videoStudioGenerationConfigured(cfg *config.Config) bool {
	return cfg.VideoGeneration.Enabled && strings.TrimSpace(cfg.VideoGeneration.APIKey) != "" && cfg.VideoGeneration.ProviderType != ""
}

// videoStudioProviderProfile derives the profile from the provider type, the configured model (ResolvedModel, else
// Model) and the resolution, without looking at whether generation is configured. The durations are the ones manual
// generation has always offered: MiniMax 6 and 10 s (6 s at 1080P, or when the configured model name is not a Hailuo
// model), Google Veo 4, 6 and 8 s (8 s at 1080P), Agnes 4 to 12 s.
func videoStudioProviderProfile(providerType, model, resolution string) storyplan.Profile {
	provider := strings.ToLower(providerType)
	if provider == "google_veo" {
		provider = "google"
	}
	resolution = strings.TrimSpace(resolution)
	is1080 := strings.Contains(strings.ToLower(resolution), "1080")
	profile := storyplan.Profile{Provider: provider, Model: strings.TrimSpace(model), Resolution: resolution, DurationsSeconds: []int{}, AspectRatios: []string{}}
	switch provider {
	case "minimax":
		profile.PlanSupported = true
		profile.Model = tools.MiniMaxVideoAPIModel(model)
		if profile.Resolution == "" {
			profile.Resolution = videoStudioDefaultMiniMaxResolution
		}
		lowerModel := strings.ToLower(model)
		if is1080 || (!strings.Contains(lowerModel, "hailuo") && lowerModel != "") {
			profile.DurationsSeconds = []int{6}
		} else {
			profile.DurationsSeconds = []int{6, 10}
		}
		profile.FirstFrame = true
		// MiniMax documents first and last frame only for MiniMax-Hailuo-02 at 768P and 1080P (checked 2026-11-23).
		profile.LastFrame = profile.Model == "MiniMax-Hailuo-02" && (strings.EqualFold(profile.Resolution, "768P") || strings.EqualFold(profile.Resolution, "1080P"))
		profile.AspectFromFirstFrame = true
		profile.CostPerClipEstimate = videoStudioMiniMaxClipCostEstimate
	case "google":
		profile.PlanSupported = true
		profile.Model = firstNonEmptyVideoStudio(model, videoStudioDefaultGoogleVideoModel)
		if is1080 {
			profile.DurationsSeconds = []int{8}
		} else {
			profile.DurationsSeconds = []int{4, 6, 8}
		}
		profile.AspectRatios = []string{"16:9", "9:16"}
		// The adapter sends only instances[0].image; last frame stays off until a live check passes (spec §4.1).
		profile.FirstFrame = true
		profile.NativeAudio = strings.Contains(strings.ToLower(profile.Model), "veo-3")
		profile.CostPerClipEstimate = videoStudioGoogleClipCostEstimate
	case "agnes":
		// Agnes takes start and end images only as public URLs; manual text-only generation keeps its durations.
		profile.Reason = storyplan.ReasonProviderImagesPublicURLOnly
		profile.Model = tools.AgnesVideoModel(model)
		for seconds := tools.AgnesVideoMinSeconds; seconds <= tools.AgnesVideoMaxSeconds; seconds++ {
			profile.DurationsSeconds = append(profile.DurationsSeconds, seconds)
		}
	default:
		profile.Reason = storyplan.ReasonProviderUnsupported
	}
	if profile.PlanSupported {
		profile.MaxPromptChars = videoStudioPlanMaxPromptChars
		profile.Parallel = videoStudioPlanParallel
	}
	return profile
}

// videoStudioDurationOptions are the durations manual generation offers for a resolution ("" = default). Never nil.
// Google with reference images keeps the 8-second rule of manual generation.
func videoStudioDurationOptions(cfg *config.Config, resolution string, hasReferences bool) []int {
	profile := videoStudioGenerationProfile(cfg, resolution)
	if hasReferences && profile.Provider == "google" {
		return []int{8}
	}
	return profile.DurationsSeconds
}

// videoStudioImageModes are the image inputs manual generation accepts for a provider type: one first frame where the
// profile has first_frame (MiniMax, Google Veo), none otherwise. Never nil.
func videoStudioImageModes(provider string) []string {
	if videoStudioProviderProfile(provider, "", "").FirstFrame {
		return []string{"first_frame"}
	}
	return []string{}
}
