package storyplan

// Reasons a Profile reports plan_supported false (Profile.Reason).
const (
	ReasonGenerationUnavailable       = "generation_unavailable"          // video generation is off or lacks a key or provider type
	ReasonProviderImagesPublicURLOnly = "provider_images_public_url_only" // start and end images only as public URLs (Agnes)
	ReasonProviderUnsupported         = "provider_unsupported"            // a provider type without a Video Studio generation mode
)

// Profile is what the configured video generation API can do at one resolution (spec §4.1). The server derives it
// (videoStudioGenerationProfile in internal/server); storyplan only reads it.
//
// PlanSupported says whether plan mode can use the provider and Reason says why not. Provider is the lowercased
// provider type ("google_veo" is reported as "google"), Model the model id sent to the API and Resolution the
// resolution the profile is for. DurationsSeconds are the allowed generate_seconds. AspectRatios is empty when the clip
// follows the first frame (AspectFromFirstFrame). FirstFrame and LastFrame are the image inputs; NativeAudio says the
// clip has sound. MaxPromptChars caps every prompt, CostPerClipEstimate is in USD and Parallel bounds concurrent
// generations.
type Profile struct {
	PlanSupported        bool     `json:"plan_supported"`
	Reason               string   `json:"reason,omitempty"`
	Provider             string   `json:"provider"`
	Model                string   `json:"model"`
	Resolution           string   `json:"resolution"`
	DurationsSeconds     []int    `json:"durations_seconds"`
	AspectRatios         []string `json:"aspect_ratios"`
	AspectFromFirstFrame bool     `json:"aspect_from_first_frame"`
	FirstFrame           bool     `json:"first_frame"`
	LastFrame            bool     `json:"last_frame"`
	NativeAudio          bool     `json:"native_audio"`
	MaxPromptChars       int      `json:"max_prompt_chars"`
	CostPerClipEstimate  float64  `json:"cost_per_clip_estimate"`
	Parallel             int      `json:"parallel"`
}
