package server

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"aurago/internal/config"
	"aurago/internal/videostudio/storyplan"
)

func videoStudioProfileConfig(provider, model, defaultResolution string) *config.Config {
	cfg := &config.Config{}
	cfg.VideoGeneration.Enabled = true
	cfg.VideoGeneration.APIKey = "fixture-key"
	cfg.VideoGeneration.ProviderType = provider
	cfg.VideoGeneration.Model = model
	cfg.VideoGeneration.DefaultResolution = defaultResolution
	return cfg
}

// The spec §4.1 table: MiniMax Hailuo-2.3 and -02, Google Veo, Agnes and an unknown provider.
func TestVideoStudioGenerationProfileTable(t *testing.T) {
	minimax := func(model, resolution string, durations []int, lastFrame bool) storyplan.Profile {
		return storyplan.Profile{PlanSupported: true, Provider: "minimax", Model: model, Resolution: resolution, DurationsSeconds: durations,
			AspectRatios: []string{}, AspectFromFirstFrame: true, FirstFrame: true, LastFrame: lastFrame, MaxPromptChars: 2000, CostPerClipEstimate: 0.20, Parallel: 2}
	}
	google := func(model, resolution string, durations []int, nativeAudio bool) storyplan.Profile {
		return storyplan.Profile{PlanSupported: true, Provider: "google", Model: model, Resolution: resolution, DurationsSeconds: durations,
			AspectRatios: []string{"16:9", "9:16"}, FirstFrame: true, NativeAudio: nativeAudio, MaxPromptChars: 2000, CostPerClipEstimate: 0.50, Parallel: 2}
	}
	generationOff := minimax("MiniMax-Hailuo-2.3", "768P", []int{6, 10}, false)
	generationOff.PlanSupported, generationOff.Reason = false, storyplan.ReasonGenerationUnavailable
	for _, tc := range []struct {
		name                                    string
		provider, model, defaultRes, resolution string
		enabled                                 bool
		want                                    storyplan.Profile
	}{
		{"hailuo 2.3", "minimax", "MiniMax-Hailuo-2.3", "", "768P", true, minimax("MiniMax-Hailuo-2.3", "768P", []int{6, 10}, false)},
		{"hailuo 2.3 fast at 1080P", "minimax", "MiniMax-Hailuo-2.3-Fast", "", "1080P", true, minimax("MiniMax-Hailuo-2.3-Fast", "1080P", []int{6}, false)},
		{"hailuo 02 alias at 768P", "minimax", "hailuo-02", "", "768P", true, minimax("MiniMax-Hailuo-02", "768P", []int{6, 10}, true)},
		{"hailuo 02 at 1080P", "minimax", "MiniMax-Hailuo-02", "", "1080P", true, minimax("MiniMax-Hailuo-02", "1080P", []int{6}, true)},
		{"hailuo 02 at 512P has no last frame", "minimax", "MiniMax-Hailuo-02", "", "512P", true, minimax("MiniMax-Hailuo-02", "512P", []int{6, 10}, false)},
		{"minimax defaults", "minimax", "", "", "", true, minimax("MiniMax-Hailuo-2.3", "768P", []int{6, 10}, false)},
		{"default resolution from the config", "minimax", "MiniMax-Hailuo-02", "1080P", "", true, minimax("MiniMax-Hailuo-02", "1080P", []int{6}, true)},
		{"non-hailuo minimax model", "minimax", "S2V-01", "", "768P", true, minimax("S2V-01", "768P", []int{6}, false)},
		{"veo 3.1", "google_veo", "veo-3.1-generate-preview", "", "720p", true, google("veo-3.1-generate-preview", "720p", []int{4, 6, 8}, true)},
		{"veo default model at 1080p", "google", "", "", "1080p", true, google("veo-3.1-generate-preview", "1080p", []int{8}, true)},
		{"veo 2 has no audio", "google", "veo-2.0-generate-001", "", "720p", true, google("veo-2.0-generate-001", "720p", []int{4, 6, 8}, false)},
		{"agnes", "agnes", "agnes-3.0-flash", "", "720P", true, storyplan.Profile{Reason: storyplan.ReasonProviderImagesPublicURLOnly, Provider: "agnes",
			Model: "agnes-video-2.5-flash", Resolution: "720P", DurationsSeconds: []int{4, 5, 6, 7, 8, 9, 10, 11, 12}, AspectRatios: []string{}}},
		{"unknown provider", "acme", "acme-video", "", "720p", true, storyplan.Profile{Reason: storyplan.ReasonProviderUnsupported, Provider: "acme",
			Model: "acme-video", Resolution: "720p", DurationsSeconds: []int{}, AspectRatios: []string{}}},
		{"generation off keeps the capabilities", "minimax", "", "", "", false, generationOff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := videoStudioProfileConfig(tc.provider, tc.model, tc.defaultRes)
			cfg.VideoGeneration.Enabled = tc.enabled
			if got := videoStudioGenerationProfile(cfg, tc.resolution); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("profile = %+v\nwant      %+v", got, tc.want)
			}
		})
	}
}

// The wrappers return exactly what manual generation offered before the profile existed.
func TestVideoStudioGenerationProfileKeepsManualDurationsAndImageModes(t *testing.T) {
	for _, tc := range []struct {
		provider, model, defaultRes, resolution string
		references                              bool
		want                                    []int
	}{
		{"google", "", "", "720p", false, []int{4, 6, 8}},
		{"google", "", "", "720p", true, []int{8}},
		{"google", "", "", "1080p", false, []int{8}},
		{"google_veo", "", "1080P", "", false, []int{8}},
		{"minimax", "hailuo-02", "", "", false, []int{6, 10}},
		{"minimax", "hailuo-02", "", "", true, []int{6, 10}},
		{"minimax", "hailuo-02", "", "1080P", false, []int{6}},
		{"minimax", "S2V-01", "", "", false, []int{6}},
		{"minimax", "", "", "", false, []int{6, 10}},
		{"agnes", "", "", "", false, []int{4, 5, 6, 7, 8, 9, 10, 11, 12}},
		{"", "", "", "", false, []int{}},
		{"acme", "", "", "", false, []int{}},
	} {
		cfg := videoStudioProfileConfig(tc.provider, tc.model, tc.defaultRes)
		if got := videoStudioDurationOptions(cfg, tc.resolution, tc.references); got == nil || !slices.Equal(got, tc.want) {
			t.Errorf("videoStudioDurationOptions(%q %q %q %q refs=%v) = %#v, want %v", tc.provider, tc.model, tc.defaultRes, tc.resolution, tc.references, got, tc.want)
		}
	}
	for provider, want := range map[string][]string{
		"minimax": {"first_frame"}, "google": {"first_frame"}, "google_veo": {"first_frame"}, "GOOGLE_VEO": {"first_frame"},
		"agnes": {}, "": {}, "acme": {},
	} {
		if got := videoStudioImageModes(provider); got == nil || !slices.Equal(got, want) {
			t.Errorf("videoStudioImageModes(%q) = %#v, want %v", provider, got, want)
		}
	}
}

func TestVideoStudioGenerationProfileInStatus(t *testing.T) {
	s, readToken, _ := videoStudioAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = "video-studio-test-missing-ffmpeg"
	s.Cfg.VideoGeneration.Enabled = true
	s.Cfg.VideoGeneration.APIKey = "fixture-key"
	s.Cfg.VideoGeneration.ProviderType = "minimax"
	s.Cfg.VideoGeneration.Model = "hailuo-02"
	s.Cfg.VideoGeneration.ResolvedModel = ""
	s.Cfg.VideoGeneration.DefaultResolution = "768P"
	response := videoStudioAPIRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status: %d %s", response.Code, response.Body.String())
	}
	var status struct {
		Generation struct {
			Profile *storyplan.Profile `json:"profile"`
		} `json:"generation"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	want := storyplan.Profile{PlanSupported: true, Provider: "minimax", Model: "MiniMax-Hailuo-02", Resolution: "768P", DurationsSeconds: []int{6, 10},
		AspectRatios: []string{}, AspectFromFirstFrame: true, FirstFrame: true, LastFrame: true, MaxPromptChars: 2000, CostPerClipEstimate: 0.20, Parallel: 2}
	if status.Generation.Profile == nil || !reflect.DeepEqual(*status.Generation.Profile, want) {
		t.Fatalf("generation.profile = %+v, want %+v", status.Generation.Profile, want)
	}
}
