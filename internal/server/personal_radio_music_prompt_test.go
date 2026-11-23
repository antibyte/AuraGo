package server

import (
	"strings"
	"testing"
)

func TestRadioMusicPrompt(t *testing.T) {
	cases := []struct {
		name, style, idea, want string
	}{
		{"plain", "Lo-fi, chillig, instrumental", "Warm Rhodes and vinyl crackle.", "Lo-fi, chillig, instrumental. Warm Rhodes and vinyl crackle."},
		{"markdown", "Lo-fi, instrumental", "**Music Generation Prompt:**\n\nInstrumental lo-fi beat, 78 BPM.\n- Rhodes\n- vinyl..", "Lo-fi, instrumental. Instrumental lo-fi beat, 78 BPM. Rhodes vinyl."},
		{"heading and label", "Jazz", "# Prompt:\n## Soft `brass`\n* __warm__ bass\n• drums", "Jazz. Soft brass warm bass drums."},
		{"empty idea", "Lo-fi, chillig, instrumental", "  \n ", "Lo-fi, chillig, instrumental."},
		{"label only", "Lo-fi", "**Prompt:**", "Lo-fi."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := radioMusicPrompt(tc.style, tc.idea)
			if got != tc.want {
				t.Fatalf("radioMusicPrompt() = %q, want %q", got, tc.want)
			}
			if strings.Contains(got, "..") || strings.Contains(got, "**") || strings.Contains(got, "  ") || strings.Contains(got, "\n") {
				t.Fatalf("unclean prompt %q", got)
			}
		})
	}
}

func TestPersonalRadioEditorPromptMusicIdeaEnglish(t *testing.T) {
	if !strings.Contains(personalRadioEditorPrompt, "music_idea is always English") {
		t.Fatal("editor prompt must require an English music_idea")
	}
}

func TestRadioLyricsInput(t *testing.T) {
	with := radioLyricsInput("German", "Alles aus der Welt der KI", "Lo-fi. Warm Rhodes.")
	if want := "German\nThemes: Alles aus der Welt der KI\nLo-fi. Warm Rhodes."; with != want {
		t.Fatalf("with topics = %q, want %q", with, want)
	}
	without := radioLyricsInput("German", "  ", "Lo-fi. Warm Rhodes.")
	if strings.Contains(without, "Themes:") || without != "German\nLo-fi. Warm Rhodes." {
		t.Fatalf("without topics = %q", without)
	}
}
