package storyplan

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Env is everything the validator needs besides the plan. The server builds it (videoStudioPlanEnv).
type Env struct {
	// Profile is the capability profile for the plan's resolution.
	Profile Profile
	// ImageGeneration reports configured image generation (keyframe source generated).
	ImageGeneration bool
	// TTS reports text-to-speech ready for Video Studio (tools.TTSReady(cfg, "video_studio")).
	TTS bool
	// Music reports configured music generation.
	Music bool
	// UnavailableTransitions are registry transitions this server's FFmpeg lacks.
	UnavailableTransitions []string
	// KnownTransitions are the transition names of the Video Studio effect registry.
	KnownTransitions []string
	// ImageAssetIDs are the image assets of the project (keyframe source library, character references).
	ImageAssetIDs map[string]bool
}

// Issue is one finding. Field is a JSON path into the plan with 0-based indexes ("scenes[2].generate_seconds",
// "target_seconds"); Message is short English for the agent and logs (the UI translates by Code).
type Issue struct {
	Code     string `json:"code"`
	SceneID  string `json:"scene_id,omitempty"`
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
}

// Issue codes (spec §8). Every code blocks except CodeProfileChanged, which warns.
const (
	CodeNoScenes                  = "no_scenes"
	CodeTooManyScenes             = "too_many_scenes"
	CodeTargetSecondsInvalid      = "target_seconds_invalid"
	CodeTotalLengthMismatch       = "total_length_mismatch"
	CodeTotalLengthTooLong        = "total_length_too_long"
	CodeIdeaTooLong               = "idea_too_long"
	CodeStyleTooLong              = "style_too_long"
	CodeTooManyCharacters         = "too_many_characters"
	CodeCharacterInvalid          = "character_invalid"
	CodeCharacterReferenceInvalid = "character_reference_invalid"
	CodeTitleMissing              = "title_missing"
	CodeTitleTooLong              = "title_too_long"
	CodePromptMissing             = "prompt_missing"
	CodePromptTooLong             = "prompt_too_long"
	CodeNegativePromptTooLong     = "negative_prompt_too_long"
	CodeDurationUnsupported       = "duration_unsupported"
	CodeUseSecondsInvalid         = "use_seconds_invalid"
	CodeTransitionUnknown         = "transition_unknown"
	CodeTransitionUnsupported     = "transition_unsupported"
	CodeFirstSceneTransition      = "first_scene_transition"
	CodeChainTooLong              = "chain_too_long"
	CodeBlendTypeUnknown          = "blend_type_unknown"
	CodeBlendTypeUnavailable      = "blend_type_unavailable"
	CodeBlendOverlapInvalid       = "blend_overlap_invalid"
	CodeKeyframeSourceInvalid     = "keyframe_source_invalid"
	CodeKeyframePromptMissing     = "keyframe_prompt_missing"
	CodeKeyframeAssetInvalid      = "keyframe_asset_invalid"
	CodeNarrationUnavailable      = "narration_unavailable"
	CodeNarrationTooLong          = "narration_too_long"
	CodeMusicUnavailable          = "music_unavailable"
	CodeClipAudioInvalid          = "clip_audio_invalid"
	CodePlanUnsupportedProvider   = "plan_unsupported_provider"
	CodeProfileChanged            = "profile_changed"
)

// Phases of a planning job (PhaseComplete).
const (
	PhaseOutline = "outline"
	PhaseDetails = "details"
	PhaseCheck   = "check"
)

const (
	lengthTolerance          = 0.15 // share of target_seconds the total length may differ by
	minBlendSeconds          = 0.5  // shortest blend
	blendMargin              = 0.5  // part of each clip a blend must leave unblended
	narrationStartSeconds    = 0.3  // a narration starts this far into its scene (spec §10)
	narrationMaxSpillSeconds = 1.0  // and may run this far into the next scene
	epsilon                  = 1e-9 // absorbs float rounding in second comparisons
	maxQuotedRunes           = 40   // plan values quoted in messages are cut to this length
)

// Validate checks p against env (spec §8) and returns every issue: plan-level issues first, then each scene in
// order. It never changes p and returns an empty, non-nil list for a valid plan.
func Validate(p Plan, env Env) []Issue {
	v := &validator{plan: p, env: env, issues: []Issue{}}
	v.checkPlan()
	for i := range p.Scenes {
		v.checkScene(i)
	}
	return v.issues
}

// HasBlocking reports whether any issue blocks.
func HasBlocking(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Blocking {
			return true
		}
	}
	return false
}

// PhaseComplete reports whether a planning phase has done its work:
//   - outline: at least one scene, and every scene has a title, generate_seconds, use_seconds and a transition kind;
//   - details: outline, and every scene has a video prompt and, for a generated keyframe, an image prompt (narration
//     stays optional);
//   - check: no blocking issue.
//
// Any other phase is not complete.
func PhaseComplete(p Plan, phase string, env Env) bool {
	switch phase {
	case PhaseOutline:
		return outlineComplete(p)
	case PhaseDetails:
		if !outlineComplete(p) {
			return false
		}
		for _, s := range p.Scenes {
			if strings.TrimSpace(s.VideoPrompt) == "" {
				return false
			}
			if s.Keyframe.Source == KeyframeGenerated && strings.TrimSpace(s.Keyframe.ImagePrompt) == "" {
				return false
			}
		}
		return true
	case PhaseCheck:
		return !HasBlocking(Validate(p, env))
	}
	return false
}

// TotalSeconds is the length the scenes fill on the timeline: the sum of use_seconds minus every blend overlap.
func TotalSeconds(p Plan) float64 {
	total := 0.0
	for i, s := range p.Scenes {
		total += s.UseSeconds
		if i > 0 && s.TransitionIn.Kind == TransitionBlend {
			total -= s.TransitionIn.Seconds
		}
	}
	return total
}

func outlineComplete(p Plan) bool {
	if len(p.Scenes) == 0 {
		return false
	}
	for _, s := range p.Scenes {
		if strings.TrimSpace(s.Title) == "" || s.GenerateSeconds <= 0 || s.UseSeconds <= 0 || strings.TrimSpace(s.TransitionIn.Kind) == "" {
			return false
		}
	}
	return true
}

type validator struct {
	plan   Plan
	env    Env
	issues []Issue
}

func (v *validator) add(code, sceneID, field, message string) {
	v.issues = append(v.issues, Issue{Code: code, SceneID: sceneID, Field: field, Message: message, Blocking: code != CodeProfileChanged})
}

func (v *validator) checkPlan() {
	p, env := v.plan, v.env
	if !env.Profile.PlanSupported {
		reason := env.Profile.Reason
		if reason == "" {
			reason = ReasonProviderUnsupported
		}
		v.add(CodePlanUnsupportedProvider, "", "profile", "The configured video provider cannot be used for plan mode ("+reason+").")
	}
	if !sameRef(p.Profile, env.Profile) {
		v.add(CodeProfileChanged, "", "profile", fmt.Sprintf("The plan was made for %s; the server now uses %s.",
			describeRef(p.Profile.Provider, p.Profile.Model, p.Profile.Resolution), describeRef(env.Profile.Provider, env.Profile.Model, env.Profile.Resolution)))
	}
	targetValid := p.TargetSeconds >= MinTargetSeconds && p.TargetSeconds <= MaxTotalSeconds
	if !targetValid {
		v.add(CodeTargetSecondsInvalid, "", "target_seconds", fmt.Sprintf("target_seconds must be %d to %d.", MinTargetSeconds, MaxTotalSeconds))
	}
	if n := utf8.RuneCountInString(p.Idea); n > MaxIdeaChars {
		v.add(CodeIdeaTooLong, "", "idea", fmt.Sprintf("The idea has %d characters; at most %d are allowed.", n, MaxIdeaChars))
	}
	if n := utf8.RuneCountInString(p.Style); n > MaxStyleChars {
		v.add(CodeStyleTooLong, "", "style", fmt.Sprintf("The style has %d characters; at most %d are allowed.", n, MaxStyleChars))
	}
	v.checkCharacters()
	switch {
	case len(p.Scenes) == 0:
		v.add(CodeNoScenes, "", "scenes", "The plan has no scenes.")
	case len(p.Scenes) > MaxScenes:
		v.add(CodeTooManyScenes, "", "scenes", fmt.Sprintf("The plan has %d scenes; at most %d are allowed.", len(p.Scenes), MaxScenes))
	}
	if len(p.Scenes) > 0 {
		total := TotalSeconds(p)
		if total > MaxTotalSeconds+epsilon {
			v.add(CodeTotalLengthTooLong, "", "scenes", fmt.Sprintf("The scenes run %.1f s; a plan holds at most %d s.", total, MaxTotalSeconds))
		}
		target := float64(p.TargetSeconds)
		if targetValid && math.Abs(total-target) > lengthTolerance*target+epsilon {
			v.add(CodeTotalLengthMismatch, "", "target_seconds", fmt.Sprintf("The scenes run %.1f s (use_seconds minus blends); a target of %d s allows %.1f to %.1f s.",
				total, p.TargetSeconds, target*(1-lengthTolerance), target*(1+lengthTolerance)))
		}
	}
	if prompt := strings.TrimSpace(p.Audio.Music.Prompt); prompt != "" {
		if !env.Music {
			v.add(CodeMusicUnavailable, "", "audio.music.prompt", "Music generation is not configured; leave the music prompt empty.")
		}
		if n := utf8.RuneCountInString(prompt); n > MaxStyleChars {
			v.add(CodePromptTooLong, "", "audio.music.prompt", fmt.Sprintf("The music prompt has %d characters; at most %d are allowed.", n, MaxStyleChars))
		}
	}
	switch p.Audio.ClipAudio {
	case ClipAudioKeep, ClipAudioDuck, ClipAudioMute:
	default:
		v.add(CodeClipAudioInvalid, "", "audio.clip_audio", fmt.Sprintf("clip_audio %s is not keep, duck or mute.", quote(p.Audio.ClipAudio)))
	}
}

func (v *validator) checkCharacters() {
	characters := v.plan.Characters
	if len(characters) > MaxCharacters {
		v.add(CodeTooManyCharacters, "", "characters", fmt.Sprintf("The plan has %d characters; at most %d are allowed.", len(characters), MaxCharacters))
	}
	seen := make(map[string]bool, len(characters))
	for i, c := range characters {
		field := fmt.Sprintf("characters[%d]", i)
		name := strings.TrimSpace(c.Name)
		key := strings.ToLower(name)
		switch {
		case name == "":
			v.add(CodeCharacterInvalid, "", field+".name", fmt.Sprintf("Character %d has no name.", i+1))
		case utf8.RuneCountInString(name) > MaxTitleChars:
			v.add(CodeCharacterInvalid, "", field+".name", fmt.Sprintf("Character %d has a name longer than %d characters.", i+1, MaxTitleChars))
		case seen[key]:
			v.add(CodeCharacterInvalid, "", field+".name", fmt.Sprintf("The character name %s is used twice.", quote(name)))
		}
		if name != "" {
			seen[key] = true
		}
		if utf8.RuneCountInString(c.Description) > MaxCharacterDescriptionChars {
			v.add(CodeCharacterInvalid, "", field+".description", fmt.Sprintf("Character %d has a description longer than %d characters.", i+1, MaxCharacterDescriptionChars))
		}
		if id := strings.TrimSpace(c.ReferenceAssetID); id != "" && !v.env.ImageAssetIDs[id] {
			v.add(CodeCharacterReferenceInvalid, "", field+".reference_asset_id", fmt.Sprintf("Character %d refers to %s, which is not an image of this project.", i+1, quote(id)))
		}
	}
}

func (v *validator) checkScene(i int) {
	s := v.plan.Scenes[i]
	path := fmt.Sprintf("scenes[%d]", i)
	label := "Scene " + s.ID
	if s.ID == "" {
		label = fmt.Sprintf("Scene %d", i+1)
	}
	maxPrompt := v.env.Profile.MaxPromptChars
	if title := strings.TrimSpace(s.Title); title == "" {
		v.add(CodeTitleMissing, s.ID, path+".title", label+" has no title.")
	} else if n := utf8.RuneCountInString(title); n > MaxTitleChars {
		v.add(CodeTitleTooLong, s.ID, path+".title", fmt.Sprintf("%s: the title has %d characters; at most %d are allowed.", label, n, MaxTitleChars))
	}
	if prompt := strings.TrimSpace(s.VideoPrompt); prompt == "" {
		v.add(CodePromptMissing, s.ID, path+".video_prompt", label+" has no video prompt.")
	} else if n := utf8.RuneCountInString(prompt); maxPrompt > 0 && n > maxPrompt {
		v.add(CodePromptTooLong, s.ID, path+".video_prompt", fmt.Sprintf("%s: the video prompt has %d characters; at most %d are allowed.", label, n, maxPrompt))
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(s.NegativePrompt)); maxPrompt > 0 && n > maxPrompt {
		v.add(CodeNegativePromptTooLong, s.ID, path+".negative_prompt", fmt.Sprintf("%s: the negative prompt has %d characters; at most %d are allowed.", label, n, maxPrompt))
	}
	if !slices.Contains(v.env.Profile.DurationsSeconds, s.GenerateSeconds) {
		v.add(CodeDurationUnsupported, s.ID, path+".generate_seconds", fmt.Sprintf("%s: generate_seconds %d is not offered by the provider (allowed: %s).", label, s.GenerateSeconds, joinInts(v.env.Profile.DurationsSeconds)))
	}
	if s.UseSeconds <= 0 || s.UseSeconds > float64(s.GenerateSeconds)+epsilon {
		v.add(CodeUseSecondsInvalid, s.ID, path+".use_seconds", fmt.Sprintf("%s: use_seconds %.2f must be above 0 and at most generate_seconds (%d).", label, s.UseSeconds, s.GenerateSeconds))
	}
	v.checkTransition(i, path, label)
	v.checkKeyframe(i, path, label)
	v.checkNarration(i, path, label)
}

func (v *validator) checkTransition(i int, path, label string) {
	scenes := v.plan.Scenes
	s := scenes[i]
	kind := s.TransitionIn.Kind
	field := path + ".transition_in.kind"
	if i == 0 {
		if kind != TransitionCut {
			v.add(CodeFirstSceneTransition, s.ID, field, label+" opens the clip; its transition_in must be cut.")
		}
		return
	}
	profile := v.env.Profile
	switch kind {
	case TransitionCut:
	case TransitionContinue:
		if !profile.FirstFrame {
			v.add(CodeTransitionUnsupported, s.ID, field, label+": continue needs a provider that takes a first frame.")
		}
		if n := chainLength(scenes, i); n > MaxChainScenes {
			v.add(CodeChainTooLong, s.ID, field, fmt.Sprintf("%s is scene %d of a seamless chain; a chain holds at most %d scenes.", label, n, MaxChainScenes))
		}
	case TransitionBridge:
		if !profile.FirstFrame || !profile.LastFrame {
			v.add(CodeTransitionUnsupported, s.ID, field, label+": bridge needs a provider that takes a first and a last frame.")
		}
	case TransitionBlend:
		v.checkBlend(i, path, label)
	default:
		v.add(CodeTransitionUnknown, s.ID, field, fmt.Sprintf("%s: transition_in.kind %s is not cut, continue, bridge or blend.", label, quote(kind)))
	}
}

// chainLength is the number of scenes in the seamless chain that ends at scene i.
func chainLength(scenes []Scene, i int) int {
	n := 1
	for j := i; j > 0 && scenes[j].TransitionIn.Kind == TransitionContinue; j-- {
		n++
	}
	return n
}

func (v *validator) checkBlend(i int, path, label string) {
	s := v.plan.Scenes[i]
	blendType := s.TransitionIn.Type
	typeField := path + ".transition_in.type"
	switch {
	case !slices.Contains(v.env.KnownTransitions, blendType):
		v.add(CodeBlendTypeUnknown, s.ID, typeField, fmt.Sprintf("%s: blend type %s is not a Video Studio transition.", label, quote(blendType)))
	case slices.Contains(v.env.UnavailableTransitions, blendType):
		v.add(CodeBlendTypeUnavailable, s.ID, typeField, fmt.Sprintf("%s: blend type %s is not available on this server.", label, quote(blendType)))
	}
	limit := math.Min(v.plan.Scenes[i-1].UseSeconds, s.UseSeconds) - blendMargin
	if seconds := s.TransitionIn.Seconds; seconds < minBlendSeconds-epsilon || seconds > limit+epsilon {
		v.add(CodeBlendOverlapInvalid, s.ID, path+".transition_in.seconds", fmt.Sprintf("%s: the blend lasts %.2f s; it must last %.1f to %.2f s (the shorter of both scenes minus %.1f s).",
			label, seconds, minBlendSeconds, math.Max(limit, 0), blendMargin))
	}
}

func (v *validator) checkKeyframe(i int, path, label string) {
	s := v.plan.Scenes[i]
	field := path + ".keyframe.source"
	maxPrompt := v.env.Profile.MaxPromptChars
	switch s.Keyframe.Source {
	case KeyframeNone:
	case KeyframeGenerated:
		if !v.env.ImageGeneration {
			v.add(CodeKeyframeSourceInvalid, s.ID, field, label+": image generation is not configured; use a library image or none.")
		}
		if prompt := strings.TrimSpace(s.Keyframe.ImagePrompt); prompt == "" {
			v.add(CodeKeyframePromptMissing, s.ID, path+".keyframe.image_prompt", label+": a generated keyframe needs an image_prompt.")
		} else if n := utf8.RuneCountInString(prompt); maxPrompt > 0 && n > maxPrompt {
			v.add(CodePromptTooLong, s.ID, path+".keyframe.image_prompt", fmt.Sprintf("%s: the image prompt has %d characters; at most %d are allowed.", label, n, maxPrompt))
		}
	case KeyframeLibrary:
		if !v.env.ImageAssetIDs[strings.TrimSpace(s.Keyframe.AssetID)] {
			v.add(CodeKeyframeAssetInvalid, s.ID, path+".keyframe.asset_id", label+": keyframe.asset_id must be an image of this project.")
		}
	case KeyframeChain:
		if i == 0 || s.TransitionIn.Kind != TransitionContinue {
			v.add(CodeKeyframeSourceInvalid, s.ID, field, label+": keyframe source chain belongs to a continue transition.")
		}
	case KeyframeBridge:
		if i == 0 || s.TransitionIn.Kind != TransitionBridge {
			v.add(CodeKeyframeSourceInvalid, s.ID, field, label+": keyframe source bridge belongs to a bridge transition.")
		} else {
			v.checkBridgeKeyframe(s, path, label)
		}
	default:
		v.add(CodeKeyframeSourceInvalid, s.ID, field, fmt.Sprintf("%s: keyframe.source %s is not generated, library, chain, bridge or none.", label, quote(s.Keyframe.Source)))
	}
}

// checkBridgeKeyframe checks the boundary keyframe K of a bridge scene (spec §4.2): an image of the project
// (keyframe.asset_id), or a generated image (keyframe.image_prompt) when image generation is configured.
func (v *validator) checkBridgeKeyframe(s Scene, path, label string) {
	maxPrompt := v.env.Profile.MaxPromptChars
	assetID, prompt := strings.TrimSpace(s.Keyframe.AssetID), strings.TrimSpace(s.Keyframe.ImagePrompt)
	switch {
	case assetID != "":
		if !v.env.ImageAssetIDs[assetID] {
			v.add(CodeKeyframeAssetInvalid, s.ID, path+".keyframe.asset_id", label+": the bridge keyframe.asset_id must be an image of this project.")
		}
	case !v.env.ImageGeneration:
		v.add(CodeKeyframeAssetInvalid, s.ID, path+".keyframe.asset_id", label+": image generation is not configured, so a bridge needs an image of this project as its keyframe; otherwise use another transition.")
	case prompt == "":
		v.add(CodeKeyframePromptMissing, s.ID, path+".keyframe.image_prompt", label+": a bridge needs an image_prompt for the frame where the scene before ends and this scene starts.")
	default:
		if n := utf8.RuneCountInString(prompt); maxPrompt > 0 && n > maxPrompt {
			v.add(CodePromptTooLong, s.ID, path+".keyframe.image_prompt", fmt.Sprintf("%s: the image prompt has %d characters; at most %d are allowed.", label, n, maxPrompt))
		}
	}
}

func (v *validator) checkNarration(i int, path, label string) {
	s := v.plan.Scenes[i]
	text := strings.TrimSpace(s.Narration.Text)
	if text == "" {
		return
	}
	field := path + ".narration.text"
	if !v.env.TTS {
		v.add(CodeNarrationUnavailable, s.ID, field, label+": text-to-speech is not ready; leave the narration empty.")
	}
	if n := utf8.RuneCountInString(text); n > MaxNarrationChars {
		v.add(CodeNarrationTooLong, s.ID, field, fmt.Sprintf("%s: the narration has %d characters; at most %d are allowed.", label, n, MaxNarrationChars))
		return
	}
	if measured := s.Narration.MeasuredSeconds; measured > 0 {
		// After the preview step the measured voice length replaces the word estimate (spec §8).
		if narrationStartSeconds+measured > s.UseSeconds+narrationMaxSpillSeconds+epsilon {
			v.add(CodeNarrationTooLong, s.ID, field, fmt.Sprintf("%s: the narration lasts %.1f s and runs more than %.0f s into the next scene; shorten it.", label, measured, narrationMaxSpillSeconds))
		}
		return
	}
	if words, budget := CountWords(text), WordBudget(v.plan.Language, s.UseSeconds); words > budget {
		v.add(CodeNarrationTooLong, s.ID, field, fmt.Sprintf("%s: the narration has %d words; %.1f s fit at most %d.", label, words, s.UseSeconds, budget))
	}
}

func sameRef(ref ProfileRef, profile Profile) bool {
	return strings.EqualFold(strings.TrimSpace(ref.Provider), profile.Provider) &&
		strings.EqualFold(strings.TrimSpace(ref.Model), profile.Model) &&
		strings.EqualFold(strings.TrimSpace(ref.Resolution), profile.Resolution)
}

func describeRef(provider, model, resolution string) string {
	var parts []string
	for _, part := range []string{provider, model, resolution} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return "no provider"
	}
	return strings.Join(parts, " / ")
}

func joinInts(values []int) string {
	if len(values) == 0 {
		return "none"
	}
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.Itoa(value)
	}
	return strings.Join(parts, ", ")
}

// quote quotes a plan value for a message, cut to maxQuotedRunes characters.
func quote(value string) string {
	if utf8.RuneCountInString(value) > maxQuotedRunes {
		value = string([]rune(value)[:maxQuotedRunes]) + "…"
	}
	return strconv.Quote(value)
}
