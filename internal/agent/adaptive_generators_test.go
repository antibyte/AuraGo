package agent

import (
	"slices"
	"testing"

	"aurago/internal/config"
)

func TestAdaptiveGeneratorToolsForQuery(t *testing.T) {
	all := ToolFeatureFlags{ImageGenerationEnabled: true, MusicGenerationEnabled: true, VideoGenerationEnabled: true}
	for _, tc := range []struct {
		query string
		ff    ToolFeatureFlags
		want  []string
	}{
		// image
		{"generate an image of a fox", all, []string{"generate_image"}},
		{"draw a logo for my bakery", all, []string{"generate_image"}},
		{"I need a wallpaper with mountains", all, []string{"generate_image"}},
		{"make a photo of a sunset", all, []string{"generate_image"}},
		{"Erstelle ein Bild von einem Fuchs", all, []string{"generate_image"}},
		{"Male mir ein Logo", all, []string{"generate_image"}},
		{"Zeichne eine Skizze", all, []string{"generate_image"}},
		{"Ich brauche ein Hintergrundbild", all, []string{"generate_image"}},
		{"Mach ein neues Titelbild", all, []string{"generate_image"}},
		{"Erzeuge drei Fotos", all, []string{"generate_image"}},
		// music
		{"compose a short song", all, []string{"generate_music"}},
		{"make a jingle with a melody", all, []string{"generate_music"}},
		{"play some music", all, []string{"generate_music"}},
		{"Schreibe ein Lied über den Herbst", all, []string{"generate_music"}},
		{"Erstelle Musik für das Intro", all, []string{"generate_music"}},
		{"Eine Hintergrundmusik bitte", all, []string{"generate_music"}},
		// video
		{"generate a short video", all, []string{"generate_video"}},
		{"make an animated clip", all, []string{"generate_video"}},
		{"Erstelle ein Video über Katzen", all, []string{"generate_video"}},
		{"Ich brauche einen Kurzfilm", all, []string{"generate_video"}},
		{"Mach ein Erklärvideo", all, []string{"generate_video"}},
		{"Ein Musikvideo bitte", all, []string{"generate_video"}},
		{"Eine Hintergrundmusik und ein Musikstück", all, []string{"generate_music"}},
		{"Der Musiker kommt, die Begleitung ist musikalisch", all, nil},
		// several
		{"generate a short video with music", all, []string{"generate_music", "generate_video"}},
		// ambiguous words need a creation verb
		{"create a beat for my video", all, []string{"generate_music", "generate_video"}},
		{"Erstelle ein Lied", all, []string{"generate_music"}},
		{"make an image of a fox", all, []string{"generate_image"}},
		{"create a drawing of a house", all, []string{"generate_image"}},
		{"Mach einen Clip aus den Aufnahmen", all, []string{"generate_video"}},
		{"Erstelle eine Animation", all, []string{"generate_video"}},
		// ambiguous words without a creation verb, or a technical meaning
		{"you lied to me about the backup", all, nil},
		{"draw conclusions from the logs", all, nil},
		{"beat the timeout", all, nil},
		{"pull the latest nginx docker image", all, nil},
		{"Lade das Docker-Image neu", all, nil},
		{"create a VM from the ISO image", all, nil},
		{"create a docker image for my app", all, nil},
		{"make a disk image of the backup", all, nil},
		{"add a CSS animation to the button", all, nil},
		{"create a CSS animation", all, nil},
		{"the male users", all, nil},
		{"Der Film lief gestern", all, nil},
		{"Spielkonsole einschalten", all, nil},
		// no intent
		{"", all, nil},
		{"send an email to the team", all, nil},
		{"Starte den Docker Container neu", all, nil},
		{"Fine tuning the kernel parameters", all, nil},
		{"Das Vorbild und die Abbildung", all, nil},
		{"Kopiere die Datei", all, nil},
		{"turn on the lights", all, nil},
		// the feature switch comes first
		{"generate an image", ToolFeatureFlags{MusicGenerationEnabled: true}, nil},
		{"generate an image and a video", ToolFeatureFlags{VideoGenerationEnabled: true}, []string{"generate_video"}},
		{"make a video", ToolFeatureFlags{}, nil},
	} {
		got := adaptiveGeneratorToolsForQuery(tc.query, tc.ff)
		slices.Sort(got)
		slices.Sort(tc.want)
		if !slices.Equal(got, tc.want) {
			t.Errorf("adaptiveGeneratorToolsForQuery(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

// Without a request for media the three generators are no soft tools: they
// compete for the adaptive slots with every other tool.
func TestMediaGeneratorsAreSoftOnlyOnIntent(t *testing.T) {
	ff := allBuiltinToolFeatureFlags()
	if got := adaptiveGeneratorToolsForQuery("List the docker containers", ff); len(got) != 0 {
		t.Fatalf("a request without media intent got %v", got)
	}
	got := adaptiveGeneratorToolsForQuery("Erstelle ein Bild und ein Video dazu", ff)
	slices.Sort(got)
	if !slices.Equal(got, []string{"generate_image", "generate_video"}) {
		t.Fatalf("media request got %v", got)
	}
}

// The managed local model keeps a kernel that does not depend on the request,
// so its llama.cpp prompt prefix stays reusable between unrelated turns.
func TestManagedLocalKernelKeepsTheGeneratorsWithoutAnIntent(t *testing.T) {
	ff := allBuiltinToolFeatureFlags()
	schemas := BuildNativeToolSchemas("", nil, ff, nil)
	names := toolSchemaNames(stableLocalToolSchemas(schemas, &config.Config{}, RunConfig{}, ff, false, nil).Tools)
	for _, want := range []string{"generate_image", "generate_music", "generate_video"} {
		if !slices.Contains(names, want) {
			t.Errorf("the query-independent kernel lost %s: %v", want, names)
		}
	}
}
