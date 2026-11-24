package storyplan

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// registryTransitionNames are the transition ids of Video Studio's effect registry (videostudio.EffectRegistry()
// .Transitions, defined in internal/videostudio/effects.go registryTransitions). storyplan does not import
// videostudio: the server passes these names in Env.KnownTransitions.
var registryTransitionNames = []string{
	"dissolve", "black", "wipeleft", "wiperight", "fadewhite", "fadegrays", "pixelize", "wipeup", "wipedown",
	"smoothleft", "smoothright", "diagtl", "diagbr", "radial", "slideleft", "slideright", "slideup", "slidedown",
	"squeezeh", "zoomin", "circleopen", "circleclose", "circlecrop", "rectcrop",
}

// allIssueCodes lists every code of validate.go; TestValidateReportsEveryRule has a case for each one.
var allIssueCodes = []string{
	CodeNoScenes, CodeTooManyScenes, CodeTargetSecondsInvalid, CodeTotalLengthMismatch, CodeTotalLengthTooLong,
	CodeIdeaTooLong, CodeStyleTooLong, CodeTooManyCharacters, CodeCharacterInvalid, CodeCharacterReferenceInvalid,
	CodeTitleMissing, CodeTitleTooLong, CodePromptMissing, CodePromptTooLong, CodeNegativePromptTooLong,
	CodeDurationUnsupported, CodeUseSecondsInvalid, CodeTransitionUnknown, CodeTransitionUnsupported,
	CodeFirstSceneTransition, CodeChainTooLong, CodeBlendTypeUnknown, CodeBlendTypeUnavailable, CodeBlendOverlapInvalid,
	CodeKeyframeSourceInvalid, CodeKeyframePromptMissing, CodeKeyframeAssetInvalid, CodeNarrationUnavailable,
	CodeNarrationTooLong, CodeMusicUnavailable, CodeClipAudioInvalid, CodePlanUnsupportedProvider, CodeProfileChanged,
}

func validEnv() Env {
	return Env{
		Profile: Profile{
			PlanSupported: true, Provider: "minimax", Model: "MiniMax-Hailuo-02", Resolution: "768P",
			DurationsSeconds: []int{6, 10}, AspectRatios: []string{}, AspectFromFirstFrame: true,
			FirstFrame: true, LastFrame: true, MaxPromptChars: 2000, CostPerClipEstimate: 0.20, Parallel: 2,
		},
		ImageGeneration:        true,
		TTS:                    true,
		Music:                  true,
		UnavailableTransitions: []string{"fadegrays"},
		KnownTransitions:       append([]string{}, registryTransitionNames...),
		ImageAssetIDs:          map[string]bool{"img_harbor": true},
	}
}

// validPlan is a 28.5 s plan for a 30 s target: a cut, a seamless continuation and a one-second dissolve.
func validPlan() Plan {
	p := NewPlan("plan_fixture", "A fisherman heads out at dawn.", 30, "de", ProfileRef{Provider: "minimax", Model: "MiniMax-Hailuo-02", Resolution: "768P"})
	p.Status = StatusReview
	p.Style = "Muted dawn colours, slow handheld camera."
	p.Characters = []Character{{Name: "Fischer", Description: "Old man in a yellow raincoat."}}
	p.Scenes = []Scene{
		{ID: "s1", Title: "Hafen im Nebel", VideoPrompt: "Slow push-in on a foggy harbour at dawn, Fischer unties his boat.", GenerateSeconds: 10, UseSeconds: 9.5,
			TransitionIn: Transition{Kind: TransitionCut}, Keyframe: Keyframe{Source: KeyframeGenerated, ImagePrompt: "Foggy harbour at dawn, Fischer beside his boat."},
			Narration: Narration{Text: "Der Tag beginnt im Hafen."}, State: SceneRuntimePending},
		{ID: "s2", Title: "Hinaus", VideoPrompt: "The boat glides out of the harbour, the camera follows.", GenerateSeconds: 10, UseSeconds: 10,
			TransitionIn: Transition{Kind: TransitionContinue}, Keyframe: Keyframe{Source: KeyframeChain}, State: SceneRuntimePending},
		{ID: "s3", Title: "Die Bucht", VideoPrompt: "Wide shot of the bay as the sun rises.", GenerateSeconds: 10, UseSeconds: 10,
			TransitionIn: Transition{Kind: TransitionBlend, Type: "dissolve", Seconds: 1}, Keyframe: Keyframe{Source: KeyframeLibrary, AssetID: "img_harbor"}, State: SceneRuntimePending},
	}
	p.Audio.Music.Prompt = "Calm acoustic guitar."
	return p
}

func TestValidateAcceptsAValidPlan(t *testing.T) {
	p, env := validPlan(), validEnv()
	before, _ := json.Marshal(p)
	issues := Validate(p, env)
	if issues == nil || len(issues) != 0 {
		t.Fatalf("issues = %#v, want an empty list", issues)
	}
	if after, _ := json.Marshal(p); string(before) != string(after) {
		t.Fatal("Validate changed the plan")
	}
	if got := TotalSeconds(p); got != 28.5 {
		t.Fatalf("TotalSeconds = %v, want 28.5", got)
	}
}

func TestValidateReportsEveryRule(t *testing.T) {
	type ruleCase struct {
		name   string
		mutate func(p *Plan, env *Env)
		code   string
		scene  string
	}
	cases := []ruleCase{
		{"provider without plan mode", func(p *Plan, env *Env) {
			env.Profile.PlanSupported, env.Profile.Reason = false, ReasonProviderImagesPublicURLOnly
		}, CodePlanUnsupportedProvider, ""},
		{"profile changed", func(p *Plan, env *Env) { p.Profile.Model = "MiniMax-Hailuo-2.3" }, CodeProfileChanged, ""},
		{"target below 5 s", func(p *Plan, env *Env) { p.TargetSeconds = 4 }, CodeTargetSecondsInvalid, ""},
		{"target above 120 s", func(p *Plan, env *Env) { p.TargetSeconds = 121 }, CodeTargetSecondsInvalid, ""},
		{"length off the target", func(p *Plan, env *Env) { p.TargetSeconds = 60 }, CodeTotalLengthMismatch, ""},
		{"longer than 120 s", func(p *Plan, env *Env) {
			env.Profile.DurationsSeconds = []int{6, 10, 120}
			p.Scenes[0].GenerateSeconds, p.Scenes[0].UseSeconds, p.TargetSeconds = 120, 120, 120
		}, CodeTotalLengthTooLong, ""},
		{"idea too long", func(p *Plan, env *Env) { p.Idea = strings.Repeat("i", MaxIdeaChars+1) }, CodeIdeaTooLong, ""},
		{"style too long", func(p *Plan, env *Env) { p.Style = strings.Repeat("s", MaxStyleChars+1) }, CodeStyleTooLong, ""},
		{"too many characters", func(p *Plan, env *Env) {
			for len(p.Characters) <= MaxCharacters {
				p.Characters = append(p.Characters, Character{Name: fmt.Sprintf("Figur %d", len(p.Characters))})
			}
		}, CodeTooManyCharacters, ""},
		{"character without name", func(p *Plan, env *Env) { p.Characters[0].Name = "" }, CodeCharacterInvalid, ""},
		{"character name twice", func(p *Plan, env *Env) {
			p.Characters = append(p.Characters, Character{Name: "fischer", Description: "The twin."})
		}, CodeCharacterInvalid, ""},
		{"character reference outside the project", func(p *Plan, env *Env) { p.Characters[0].ReferenceAssetID = "img_missing" }, CodeCharacterReferenceInvalid, ""},
		{"no scenes", func(p *Plan, env *Env) { p.Scenes = []Scene{} }, CodeNoScenes, ""},
		{"too many scenes", func(p *Plan, env *Env) {
			for len(p.Scenes) <= MaxScenes {
				extra := p.Scenes[2]
				extra.ID = fmt.Sprintf("x%d", len(p.Scenes))
				p.Scenes = append(p.Scenes, extra)
			}
		}, CodeTooManyScenes, ""},
		{"title missing", func(p *Plan, env *Env) { p.Scenes[1].Title = " " }, CodeTitleMissing, "s2"},
		{"title too long", func(p *Plan, env *Env) { p.Scenes[1].Title = strings.Repeat("t", MaxTitleChars+1) }, CodeTitleTooLong, "s2"},
		{"video prompt missing", func(p *Plan, env *Env) { p.Scenes[1].VideoPrompt = "" }, CodePromptMissing, "s2"},
		{"video prompt too long", func(p *Plan, env *Env) { p.Scenes[1].VideoPrompt = strings.Repeat("p", 2001) }, CodePromptTooLong, "s2"},
		{"image prompt too long", func(p *Plan, env *Env) { p.Scenes[0].Keyframe.ImagePrompt = strings.Repeat("p", 2001) }, CodePromptTooLong, "s1"},
		{"music prompt too long", func(p *Plan, env *Env) { p.Audio.Music.Prompt = strings.Repeat("m", MaxStyleChars+1) }, CodePromptTooLong, ""},
		{"negative prompt too long", func(p *Plan, env *Env) { p.Scenes[1].NegativePrompt = strings.Repeat("n", 2001) }, CodeNegativePromptTooLong, "s2"},
		{"duration the provider lacks", func(p *Plan, env *Env) { p.Scenes[0].GenerateSeconds, p.Scenes[0].UseSeconds = 7, 6 }, CodeDurationUnsupported, "s1"},
		{"use seconds zero", func(p *Plan, env *Env) { p.Scenes[1].UseSeconds = 0 }, CodeUseSecondsInvalid, "s2"},
		{"use seconds above generate", func(p *Plan, env *Env) { p.Scenes[1].UseSeconds = 10.5 }, CodeUseSecondsInvalid, "s2"},
		{"unknown transition kind", func(p *Plan, env *Env) { p.Scenes[1].TransitionIn.Kind = "fade" }, CodeTransitionUnknown, "s2"},
		{"continue without first frame", func(p *Plan, env *Env) { env.Profile.FirstFrame = false }, CodeTransitionUnsupported, "s2"},
		{"bridge without last frame", func(p *Plan, env *Env) {
			env.Profile.LastFrame = false
			p.Scenes[1].TransitionIn.Kind, p.Scenes[1].Keyframe.Source = TransitionBridge, KeyframeBridge
		}, CodeTransitionUnsupported, "s2"},
		{"first scene not cut", func(p *Plan, env *Env) { p.Scenes[0].TransitionIn.Kind = TransitionContinue }, CodeFirstSceneTransition, "s1"},
		{"chain of five scenes", func(p *Plan, env *Env) {
			p.Scenes[2].TransitionIn, p.Scenes[2].Keyframe = Transition{Kind: TransitionContinue}, Keyframe{Source: KeyframeChain}
			for _, id := range []string{"c4", "c5"} {
				p.Scenes = append(p.Scenes, Scene{ID: id, Title: "Weiter", VideoPrompt: "The boat keeps moving.", GenerateSeconds: 6, UseSeconds: 6,
					TransitionIn: Transition{Kind: TransitionContinue}, Keyframe: Keyframe{Source: KeyframeChain}, State: SceneRuntimePending})
			}
		}, CodeChainTooLong, "c5"},
		{"unknown blend type", func(p *Plan, env *Env) { p.Scenes[2].TransitionIn.Type = "spin" }, CodeBlendTypeUnknown, "s3"},
		{"blend type missing on this server", func(p *Plan, env *Env) { p.Scenes[2].TransitionIn.Type = "fadegrays" }, CodeBlendTypeUnavailable, "s3"},
		{"blend too short", func(p *Plan, env *Env) { p.Scenes[2].TransitionIn.Seconds = 0.2 }, CodeBlendOverlapInvalid, "s3"},
		{"blend longer than the scenes allow", func(p *Plan, env *Env) { p.Scenes[2].TransitionIn.Seconds = 9.6 }, CodeBlendOverlapInvalid, "s3"},
		{"unknown keyframe source", func(p *Plan, env *Env) { p.Scenes[0].Keyframe.Source = "photo" }, CodeKeyframeSourceInvalid, "s1"},
		{"chain keyframe without continue", func(p *Plan, env *Env) { p.Scenes[2].Keyframe = Keyframe{Source: KeyframeChain} }, CodeKeyframeSourceInvalid, "s3"},
		{"bridge keyframe without bridge", func(p *Plan, env *Env) { p.Scenes[2].Keyframe = Keyframe{Source: KeyframeBridge} }, CodeKeyframeSourceInvalid, "s3"},
		{"generated keyframe without image generation", func(p *Plan, env *Env) { env.ImageGeneration = false }, CodeKeyframeSourceInvalid, "s1"},
		{"generated keyframe without prompt", func(p *Plan, env *Env) { p.Scenes[0].Keyframe.ImagePrompt = "" }, CodeKeyframePromptMissing, "s1"},
		{"library keyframe outside the project", func(p *Plan, env *Env) { p.Scenes[2].Keyframe.AssetID = "img_other" }, CodeKeyframeAssetInvalid, "s3"},
		{"narration without TTS", func(p *Plan, env *Env) { env.TTS = false }, CodeNarrationUnavailable, "s1"},
		{"narration over the word budget", func(p *Plan, env *Env) { p.Scenes[0].Narration.Text = strings.Repeat("Wort ", 19) }, CodeNarrationTooLong, "s1"},
		{"narration over 600 characters", func(p *Plan, env *Env) { p.Scenes[0].Narration.Text = strings.Repeat("a", MaxNarrationChars+1) }, CodeNarrationTooLong, "s1"},
		{"measured narration spills over 1 s", func(p *Plan, env *Env) { p.Scenes[0].Narration.MeasuredSeconds = 10.5 }, CodeNarrationTooLong, "s1"},
		{"music without music generation", func(p *Plan, env *Env) { env.Music = false }, CodeMusicUnavailable, ""},
		{"unknown clip audio", func(p *Plan, env *Env) { p.Audio.ClipAudio = "loud" }, CodeClipAudioInvalid, ""},
	}
	covered := map[string]bool{}
	for _, tc := range cases {
		covered[tc.code] = true
		t.Run(tc.name, func(t *testing.T) {
			p, env := validPlan(), validEnv()
			tc.mutate(&p, &env)
			issues := Validate(p, env)
			for _, issue := range issues {
				if issue.Code == tc.code && issue.SceneID == tc.scene {
					if issue.Blocking != (tc.code != CodeProfileChanged) || issue.Message == "" || issue.Field == "" {
						t.Fatalf("issue %+v: wrong blocking flag, empty message or empty field", issue)
					}
					return
				}
			}
			t.Fatalf("no %s issue for scene %q in %+v", tc.code, tc.scene, issues)
		})
	}
	for _, code := range allIssueCodes {
		if !covered[code] {
			t.Errorf("no test case for %s", code)
		}
	}
}

func TestValidateOrdersPlanIssuesFirst(t *testing.T) {
	p, env := validPlan(), validEnv()
	p.Scenes[1].Title = ""
	p.Scenes[0].VideoPrompt = ""
	p.Audio.ClipAudio = "loud"
	var got []string
	for _, issue := range Validate(p, env) {
		got = append(got, issue.Code+"@"+issue.SceneID)
	}
	want := []string{CodeClipAudioInvalid + "@", CodePromptMissing + "@s1", CodeTitleMissing + "@s2"}
	if !slices.Equal(got, want) {
		t.Fatalf("issue order = %v, want %v", got, want)
	}
}

func TestHasBlockingIgnoresWarnings(t *testing.T) {
	p, env := validPlan(), validEnv()
	p.Profile.Resolution = "1080P"
	issues := Validate(p, env)
	if len(issues) != 1 || issues[0].Code != CodeProfileChanged || HasBlocking(issues) {
		t.Fatalf("a changed profile alone must only warn: %+v", issues)
	}
	p.Scenes[0].Title = ""
	if !HasBlocking(Validate(p, env)) {
		t.Fatal("a missing title must block")
	}
}

// A bridge's boundary keyframe K is an image of the project, or a generated image (image_prompt) with image
// generation configured.
func TestValidateBridgeKeyframe(t *testing.T) {
	bridged := func(keyframe Keyframe) Plan {
		p := validPlan()
		p.Scenes[1].TransitionIn, p.Scenes[1].Keyframe = Transition{Kind: TransitionBridge}, keyframe
		return p
	}
	noImages := validEnv()
	noImages.ImageGeneration = false
	prompt := "The boat at the harbour mouth, wide shot, dawn light."
	for _, tc := range []struct {
		name string
		plan Plan
		env  Env
		want string // the keyframe issue of s2; "" for none
	}{
		{"generated K", bridged(Keyframe{Source: KeyframeBridge, ImagePrompt: prompt}), validEnv(), ""},
		{"project image K", bridged(Keyframe{Source: KeyframeBridge, AssetID: "img_harbor"}), validEnv(), ""},
		{"project image K without image generation", bridged(Keyframe{Source: KeyframeBridge, AssetID: "img_harbor"}), noImages, ""},
		{"no K", bridged(Keyframe{Source: KeyframeBridge}), validEnv(), CodeKeyframePromptMissing},
		{"K outside the project", bridged(Keyframe{Source: KeyframeBridge, AssetID: "img_other"}), validEnv(), CodeKeyframeAssetInvalid},
		{"prompt without image generation", bridged(Keyframe{Source: KeyframeBridge, ImagePrompt: prompt}), noImages, CodeKeyframeAssetInvalid},
	} {
		var got []string
		for _, issue := range Validate(tc.plan, tc.env) {
			if issue.SceneID == "s2" && strings.HasPrefix(issue.Code, "keyframe_") {
				got = append(got, issue.Code)
			}
		}
		var want []string
		if tc.want != "" {
			want = []string{tc.want}
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: keyframe issues of s2 = %v, want %v", tc.name, got, want)
		}
	}
}

func TestPhaseComplete(t *testing.T) {
	env := validEnv()
	outline := validPlan()
	for i := range outline.Scenes {
		outline.Scenes[i].VideoPrompt = ""
		outline.Scenes[i].Keyframe.ImagePrompt = ""
	}
	full := validPlan()
	broken := validPlan()
	broken.Scenes[2].TransitionIn.Type = "spin"
	empty := validPlan()
	empty.Scenes = []Scene{}
	noKind := validPlan()
	noKind.Scenes[1].TransitionIn.Kind = ""
	for _, tc := range []struct {
		name  string
		plan  Plan
		phase string
		want  bool
	}{
		{"outline done", outline, PhaseOutline, true},
		{"details missing prompts", outline, PhaseDetails, false},
		{"details done", full, PhaseDetails, true},
		{"details ignore blocking issues", broken, PhaseDetails, true},
		{"check passes", full, PhaseCheck, true},
		{"check blocked", broken, PhaseCheck, false},
		{"no scenes", empty, PhaseOutline, false},
		{"missing transition kind", noKind, PhaseOutline, false},
		{"unknown phase", full, "revise", false},
	} {
		if got := PhaseComplete(tc.plan, tc.phase, env); got != tc.want {
			t.Errorf("%s: PhaseComplete(%s) = %v, want %v", tc.name, tc.phase, got, tc.want)
		}
	}
}
