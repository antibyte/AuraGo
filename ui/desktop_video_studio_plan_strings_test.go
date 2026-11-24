package ui

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The validator's issue codes (brief §4); the board reads videoStudio.plan.issue.<code>.
var videoStudioPlanIssueCodes = []string{
	"no_scenes", "too_many_scenes", "target_seconds_invalid", "total_length_mismatch", "total_length_too_long",
	"idea_too_long", "style_too_long", "too_many_characters", "character_invalid", "character_reference_invalid",
	"title_missing", "title_too_long", "prompt_missing", "prompt_too_long", "negative_prompt_too_long",
	"duration_unsupported", "use_seconds_invalid", "transition_unknown", "transition_unsupported",
	"first_scene_transition", "chain_too_long", "blend_type_unknown", "blend_type_unavailable", "blend_overlap_invalid",
	"keyframe_source_invalid", "keyframe_prompt_missing", "keyframe_asset_invalid", "narration_unavailable",
	"narration_too_long", "music_unavailable", "clip_audio_invalid", "plan_unsupported_provider", "profile_changed",
}

// Every plan-mode string the storyboard reads (each 'plan.*' literal in the two plan scripts, the _one form of the
// trCount keys, the dynamic families and one issue text per validator code) exists in all sixteen desktop locales
// with the English placeholders.
func TestDesktopVideoStudioPlanStrings(t *testing.T) {
	literal := regexp.MustCompile(`'(plan\.[A-Za-z0-9_.]*[A-Za-z0-9_])'`)
	counted := regexp.MustCompile(`trCount\(s, '(plan\.[A-Za-z0-9_.]*[A-Za-z0-9_])'`)
	keys := map[string]bool{}
	for _, file := range []string{"js/desktop/apps/video-studio-plan.js", "js/desktop/apps/video-studio-plan-board.js"} {
		source := readDesktopAssetText(t, file)
		for _, match := range literal.FindAllStringSubmatch(source, -1) {
			keys[match[1]] = true
		}
		for _, match := range counted.FindAllStringSubmatch(source, -1) {
			keys[match[1]+"_one"] = true
		}
	}
	if len(keys) < 60 {
		t.Fatalf("found only %d plan keys in the plan scripts", len(keys))
	}
	families := map[string][]string{
		"phase":     {"outline", "details", "check", "revise"},
		"kind":      {"cut", "continue", "bridge", "blend"},
		"source":    {"generated", "library", "none", "chain", "bridge"},
		"clipAudio": {"keep", "duck", "mute"},
		"reason":    {"provider_images_public_url_only", "provider_unsupported", "generation_unavailable"},
		"issue":     videoStudioPlanIssueCodes,
	}
	for family, names := range families {
		for _, name := range names {
			keys["plan."+family+"."+name] = true
		}
	}
	placeholder := regexp.MustCompile(`\{\{[a-z]+\}\}`)
	placeholders := func(text string) string {
		found := placeholder.FindAllString(text, -1)
		sort.Strings(found)
		return strings.Join(found, ",")
	}
	english := readDesktopLocale(t, "en")
	for _, lang := range videoStudioLocales {
		words := readDesktopLocale(t, lang)
		for key := range keys {
			full := "videoStudio." + key
			value := strings.TrimSpace(words[full])
			if value == "" {
				t.Errorf("%s lacks %s", lang, full)
				continue
			}
			if got, want := placeholders(value), placeholders(english[full]); got != want {
				t.Errorf("%s %s has placeholders %q, English has %q", lang, full, got, want)
			}
		}
	}
}
