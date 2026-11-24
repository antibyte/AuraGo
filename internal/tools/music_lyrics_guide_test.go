package tools

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

type lintHit struct {
	code string
	line int
}

func lintHits(issues []LyricsIssue) []lintHit {
	hits := make([]lintHit, 0, len(issues))
	for _, issue := range issues {
		hits = append(hits, lintHit{issue.Code, issue.Line})
	}
	return hits
}

func sameHits(got, want []lintHit) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestLyricsGuideForProviders(t *testing.T) {
	ace := LyricsGuideFor("acestep")
	if !ace.SupportsLyrics || !ace.Modifiers || !ace.Numbered || !ace.FreeformTags || ace.BPMInStyle || ace.MaxLyricsChars != DefaultMaxLyricsChars || len(ace.VoiceTags) == 0 {
		t.Fatalf("acestep guide = %+v", ace)
	}
	mm := LyricsGuideFor(" MiniMax ")
	if mm.Provider != "minimax" || !mm.SupportsLyrics || mm.Modifiers || mm.Numbered || mm.FreeformTags || mm.MaxLyricsChars != MiniMaxMaxLyricsChars || len(mm.SectionTags) != 14 {
		t.Fatalf("minimax guide = %+v", mm)
	}
	for _, provider := range []string{"google", "google_lyria", "", "bogus"} {
		g := LyricsGuideFor(provider)
		if g.SupportsLyrics || g.SectionTags == nil || g.VoiceTags == nil {
			t.Fatalf("%q guide = %+v, want no lyrics and non-nil tag lists", provider, g)
		}
	}
}

func TestLyricsGuidePromptRules(t *testing.T) {
	mm := LyricsGuideFor("minimax").PromptRules()
	for _, want := range []string{"[Pre Chorus]", "[Build Up]", "[Inst]", "3500", "No numbers"} {
		if !strings.Contains(mm, want) {
			t.Errorf("minimax rules lack %q:\n%s", want, mm)
		}
	}
	ace := LyricsGuideFor("acestep").PromptRules()
	for _, want := range []string{"[Pre-Chorus]", "[Chorus - anthemic]", "[whispered]", "backing vocals", "6-10 syllables"} {
		if !strings.Contains(ace, want) {
			t.Errorf("acestep rules lack %q:\n%s", want, ace)
		}
	}
	if LyricsGuideFor("google").PromptRules() == "" {
		t.Error("fallback rules are empty")
	}
	if LyricsGuideFor("acestep").StyleRules() == "" || LyricsGuideFor("minimax").StyleRules() != "" {
		t.Error("only ACE-Step keeps BPM out of the style")
	}
}

func TestLintSongMiniMax(t *testing.T) {
	lyrics := "[Verse 1]\nWalking down the street\n\n[Chorus - soft]\nSing it loud (x2)\n[Guitar Solo]\n[pre-chorus]\nChorus:\n[whispered]"
	issues := LintSong(LyricsGuideFor("minimax"), "pop, 120 bpm", lyrics)
	want := []lintHit{{"tag_numbered", 1}, {"tag_modifier_unsupported", 4}, {"repeat_marker", 5}, {"unknown_tag", 6}, {"heading_not_tag", 8}, {"unknown_tag", 9}}
	if got := lintHits(issues); !sameHits(got, want) {
		t.Fatalf("hits = %v, want %v", got, want)
	}
	if issues[0].Params["suggestion"] != "[Verse]" || issues[3].Params["suggestion"] != "[Solo]" || issues[4].Params["suggestion"] != "[Chorus]" {
		t.Fatalf("suggestions = %v / %v / %v", issues[0].Params, issues[3].Params, issues[4].Params)
	}
	if _, ok := issues[5].Params["suggestion"]; ok || issues[5].Severity != "warning" {
		t.Fatalf("unknown voice tag on minimax = %+v", issues[5])
	}
}

func TestLintSongACEStep(t *testing.T) {
	lyrics := "[Verse 1]\nLine one\n[Chorus - anthemic]\nWE ARE HERE (oh oh)\n[Final Chorus]\n[Inst]\n[Bridge - soft - airy - slow]\n[whispered]\nRepeat chorus"
	issues := LintSong(LyricsGuideFor("acestep"), "synthwave, 110 bpm", lyrics)
	want := []lintHit{{"bpm_in_style", 0}, {"unknown_tag", 6}, {"tag_too_many_modifiers", 7}, {"repeat_marker", 9}}
	if got := lintHits(issues); !sameHits(got, want) {
		t.Fatalf("hits = %v, want %v", got, want)
	}
	if issues[1].Severity != "info" || issues[1].Params["suggestion"] != "[Instrumental]" {
		t.Fatalf("ACE-Step alias = %+v", issues[1])
	}
}

func TestLintSongLengthAndEmpty(t *testing.T) {
	mm := LyricsGuideFor("minimax")
	issues := LintSong(mm, "", "[Verse]\n"+strings.Repeat("la ", 1200))
	if len(issues) != 1 || issues[0].Code != "lyrics_too_long" || issues[0].Severity != "error" || issues[0].Params["max"] != "3500" {
		t.Fatalf("too long = %+v", issues)
	}
	if issues := LintSong(mm, "", "  \n "); len(issues) != 0 {
		t.Fatalf("empty lyrics = %+v", issues)
	}
	if issues := LintSong(LyricsGuideFor("google"), "", "[Verse 1]\nla"); len(issues) != 0 {
		t.Fatalf("google lyrics = %+v", issues)
	}
	if got := lintHits(LintSong(mm, "", "just words\nmore words")); !sameHits(got, []lintHit{{"no_section_tags", 0}}) {
		t.Fatalf("no tags = %v", got)
	}
}

func TestGenerateMusicResultRejectsLongMiniMaxLyrics(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.MusicGeneration.Enabled = true
	cfg.MusicGeneration.ProviderType = "minimax"
	cfg.MusicGeneration.APIKey = "test-key"
	before := MusicCounterGet()
	res := GenerateMusicResult(context.Background(), cfg, nil, slog.Default(), MusicGenParams{
		Prompt: "pop",
		Lyrics: strings.Repeat("a", MiniMaxMaxLyricsChars+1),
	})
	if res.Status != "error" || res.Error != "invalid_lyrics_length" || !strings.Contains(res.Message, "3500") || !strings.Contains(res.Message, "[Pre Chorus]") {
		t.Fatalf("result = %+v", res)
	}
	if MusicCounterGet() != before {
		t.Fatal("rejected lyrics consumed the daily quota")
	}
}

func TestGenerateMusicManualListsProviderTags(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "prompts", "tools_manuals", "generate_music.md"))
	if err != nil {
		t.Fatal(err)
	}
	manual := string(data)
	for _, provider := range []string{"acestep", "minimax"} {
		g := LyricsGuideFor(provider)
		for _, tag := range append(append([]string{}, g.SectionTags...), g.VoiceTags...) {
			if !strings.Contains(manual, "`["+tag+"]`") {
				t.Errorf("generate_music.md lacks %s tag `[%s]`", provider, tag)
			}
		}
	}
	if !strings.Contains(manual, "3500") {
		t.Error("generate_music.md lacks the MiniMax lyrics limit")
	}
}

func TestLintSongSuggestionsKeepTheMusic(t *testing.T) {
	ace := LyricsGuideFor("acestep")
	if issues := LintSong(ace, "", "[Verse 1]\nla\n[Interlude]\n[Sax Solo]\n[Break]\nSilence.\n(Break)\nDrop."); len(issues) != 0 {
		t.Fatalf("acestep issues = %+v", issues)
	}
	mm := LyricsGuideFor("minimax")
	for tag, want := range map[string]string{"[Breakdown]": "[Break]", "[Build]": "[Build Up]", "[Guitar Solo]": "[Solo]"} {
		issues := LintSong(mm, "", tag+"\nla")
		if len(issues) != 1 || issues[0].Code != "unknown_tag" || issues[0].Params["suggestion"] != want {
			t.Fatalf("minimax %s: %+v", tag, issues)
		}
	}
	if issues := LintSong(mm, "", "[Verse]\nla\nDrop:"); len(issues) != 1 || issues[0].Code != "heading_not_tag" {
		t.Fatalf("minimax Drop: %+v", issues)
	}
	issues := LintSong(mm, "", "[Chorus] (x2)\nla")
	codes := map[string]bool{}
	for _, issue := range issues {
		codes[issue.Code] = true
	}
	if !codes["repeat_marker"] || codes["no_section_tags"] {
		t.Fatalf("repeat marker line: %+v", issues)
	}
}
