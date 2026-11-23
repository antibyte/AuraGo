# Noisemaker AI Helpers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Noisemaker gets provider-aware AI songwriting helpers (whole song, lyric revision, tag check, library variation) and every lyric-writing LLM in AuraGo learns the active provider's lyric tags from one Go source.

**Architecture:** A new `internal/tools/music_lyrics_guide.go` holds per-provider tag facts, LLM prompt rules and a deterministic lint. The Noisemaker server handlers use it for all enhance prompts, three new enhance kinds and a new `lyrics/check` endpoint; `GenerateMusicResult` uses it to reject MiniMax lyrics over 3500 characters before the paid call. The desktop app gets a song button, a lyrics AI menu, a check panel, an undo bar and a library "variation" submenu.

**Tech Stack:** Go (net/http, encoding/json, regexp), vanilla JS desktop modules, CSS, 16 JSON locale files, rod browser test.

Spec: `docs/superpowers/specs/2026-11-23-noisemaker-ai-helpers-design.md`.

---

## Ground rules for every task

- Repo: `C:\Users\Andi\Documents\repo\AuraGo`, branch `main`. Other agent sessions edit other files in the same checkout at the same time. **Never run `git add`, `git commit`, `git stash`, `git reset` or `git checkout`.** The controller commits your paths. Touch only the files your task lists.
- Before running Go tests, list untracked Go files from other sessions: `git ls-files --others --exclude-standard -- internal ui cmd | grep '\.go$'`. If `go test` fails to compile because of such a file (not yours), build an overlay JSON `{"Replace":{"<abs path>":""}}` for each and pass `-overlay <file>`; never edit or move those files.
- If `go test` fails with `no such tool "vet"`, prefix the command with `GOTOOLCHAIN=go1.27.4`.
- Keep `GOTMPDIR` as set by the environment (backslash path). Do not point it at a long scratch path.
- Go source written with the Write tool: never put a literal backslash-u sequence in the file; the plan avoids them.
- Report: files changed, test commands run with PASS/FAIL counts.

---

## File map

| File | Change | Responsibility |
|---|---|---|
| `internal/tools/music_lyrics_guide.go` | create | Provider tag facts, `PromptRules`, `StyleRules`, `LintSong` |
| `internal/tools/music_lyrics_guide_test.go` | create | Guide, prompt rules, lint, manual drift, MiniMax guard |
| `internal/tools/music_generation.go` | modify (`GenerateMusicResult`) | MiniMax lyrics length guard |
| `prompts/tools_manuals/generate_music.md` | modify | "Lyrics tags by provider" section |
| `internal/agent/native_tools_integrations.go` | modify (generate_music `lyrics` description) | Point to provider tags |
| `internal/server/noisemaker_handlers.go` | modify | State guide, local-mode helpers, new enhance kinds, check handler, generate message |
| `internal/server/noisemaker_ai_test.go` | create | Server tests for the new behaviour |
| `internal/server/music_local_handlers_test.go` | modify | Replace `TestNoisemakerLocalNoCloudLyrics` |
| `internal/server/server_routes.go` | modify (~line 597) | Register `/api/desktop/noisemaker/lyrics/check` |
| `ui/js/desktop/apps/noisemaker-create.js` | modify | Song button, lyrics menu, instruction row, check panel, undo bar, `suggestVariation` |
| `ui/js/desktop/apps/noisemaker.js` | modify | `menu` dependency, `templateFields`, `suggestVariation`, menu model |
| `ui/js/desktop/apps/noisemaker-menus.js` | modify (`trackContextItems`) | "Variante vorschlagen" submenu |
| `ui/css/desktop-app-noisemaker.css` | modify | Styles for the new elements |
| `ui/lang/desktop/*.json` (16 files) | modify | 36 new keys |
| `ui/desktop_noisemaker_browser_test.go` | modify | `ai_helpers` subtest + fake API additions |
| `documentation/ace-step-local.md`, `documentation/manuals/{en,de}/04-webui.md`, `documentation/manuals/{en,de}/21-api-reference.md` | modify | Docs |

---

### Task 1: Lyric guide and lint (`internal/tools`)

**Files:**
- Create: `internal/tools/music_lyrics_guide.go`
- Create: `internal/tools/music_lyrics_guide_test.go`

- [ ] **Step 1: Write the failing tests** in `internal/tools/music_lyrics_guide_test.go`:

```go
package tools

import (
	"strings"
	"testing"
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tools -run '(?i)(LyricsGuide|LintSong)' -count=1`
Expected: FAIL to compile (`undefined: LyricsGuideFor`).

- [ ] **Step 3: Write the implementation** in `internal/tools/music_lyrics_guide.go`:

```go
package tools

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// LyricsGuide describes the lyric tags and limits a music provider understands.
// It is the single source for Noisemaker's AI helpers, the lyrics check and the
// MiniMax length guard in GenerateMusicResult. Tag facts come from ACE-Step 1.5
// docs/en/Tutorial.md (commit ca1e85fe9430179831e6bc6be790c332190a3866) and the
// MiniMax POST /v1/music_generation reference; prompts/tools_manuals/generate_music.md
// must list the same tags (TestGenerateMusicManualListsProviderTags).
type LyricsGuide struct {
	Provider       string   `json:"provider"`
	SupportsLyrics bool     `json:"supports_lyrics"`
	MaxLyricsChars int      `json:"max_chars"`
	SectionTags    []string `json:"section_tags"`
	VoiceTags      []string `json:"voice_tags"`
	Modifiers      bool     `json:"modifiers"`
	Numbered       bool     `json:"numbered"`
	FreeformTags   bool     `json:"freeform_tags"`
	BPMInStyle     bool     `json:"bpm_in_style"`
}

// LyricsIssue is one LintSong finding. Code is stable; the desktop UI localizes it.
type LyricsIssue struct {
	Code     string            `json:"code"`
	Severity string            `json:"severity"` // error | warning | info
	Line     int               `json:"line,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
}

const (
	// MiniMaxMaxLyricsChars is the documented MiniMax limit for non-instrumental lyrics.
	MiniMaxMaxLyricsChars = 3500
	// DefaultMaxLyricsChars matches the Noisemaker lyrics field cap.
	DefaultMaxLyricsChars = 8000
)

var (
	aceStepSectionTags = []string{"Intro", "Verse", "Pre-Chorus", "Chorus", "Bridge", "Outro", "Build", "Drop", "Breakdown", "Instrumental", "Guitar Solo", "Piano Interlude", "Fade Out", "Silence"}
	aceStepVoiceTags   = []string{"raspy vocal", "whispered", "falsetto", "powerful belting", "spoken word", "harmonies", "call and response", "ad-lib", "high energy", "low energy", "building energy", "explosive", "melancholic", "euphoric", "dreamy", "aggressive"}
	miniMaxSectionTags = []string{"Intro", "Verse", "Pre Chorus", "Chorus", "Interlude", "Bridge", "Outro", "Post Chorus", "Transition", "Break", "Hook", "Build Up", "Inst", "Solo"}

	// lyricsTagGroups maps spellings of one section concept so a tag can be
	// suggested in the active provider's spelling. Members are lyricsTagKey values.
	lyricsTagGroups = [][]string{
		{"chorus", "refrain"},
		{"verse", "strophe"},
		{"pre chorus", "prechorus"},
		{"post chorus", "postchorus"},
		{"instrumental", "inst", "instrumental break"},
		{"solo", "guitar solo"},
		{"build up", "build", "buildup"},
		{"break", "breakdown"},
		{"interlude", "piano interlude"},
		{"fade out", "fadeout"},
	}

	// lyricsPlainHeadingWords are section names that count as a heading even
	// without decoration such as a colon, markdown or parentheses.
	lyricsPlainHeadingWords = map[string]bool{"verse": true, "chorus": true, "pre chorus": true, "post chorus": true, "bridge": true, "intro": true, "outro": true, "refrain": true, "strophe": true}

	lyricsInlineTagRe = regexp.MustCompile(`\[([^\[\]]+)\]`)
	lyricsModifierRe  = regexp.MustCompile(`\s+[-\x{2013}\x{2014}]\s+`)
	lyricsNumberRe    = regexp.MustCompile(`^(.*?)\s+(\d+)$`)
	lyricsRepeatRe    = regexp.MustCompile(`(?i)(\(\s*(?:[x\x{00D7}]\s*\d+|\d+\s*[x\x{00D7}])\s*\)|\s[x\x{00D7}]\s?\d+\s*$|^\(?\s*(?:repeat|wiederholung|wdh\.?)(?:\s+(?:chorus|verse|refrain|hook|strophe|x?\d+))?\s*\)?\s*:?\s*$)`)
	styleBPMRe        = regexp.MustCompile(`(?i)\b\d{2,3}\s*bpm\b`)
)

const lyricsHeadingTrim = "#*_():. \t"

// LyricsGuideFor returns the guide for a music provider type. Unknown types and
// Google Lyria take no separate lyrics.
func LyricsGuideFor(providerType string) LyricsGuide {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "acestep":
		return LyricsGuide{Provider: "acestep", SupportsLyrics: true, MaxLyricsChars: DefaultMaxLyricsChars,
			SectionTags: append([]string(nil), aceStepSectionTags...), VoiceTags: append([]string(nil), aceStepVoiceTags...),
			Modifiers: true, Numbered: true, FreeformTags: true}
	case "minimax":
		return LyricsGuide{Provider: "minimax", SupportsLyrics: true, MaxLyricsChars: MiniMaxMaxLyricsChars,
			SectionTags: append([]string(nil), miniMaxSectionTags...), VoiceTags: []string{}, BPMInStyle: true}
	case "google", "google_lyria":
		return LyricsGuide{Provider: "google", SectionTags: []string{}, VoiceTags: []string{}, BPMInStyle: true}
	default:
		return LyricsGuide{Provider: strings.ToLower(strings.TrimSpace(providerType)), SectionTags: []string{}, VoiceTags: []string{}, BPMInStyle: true}
	}
}

// FormatAllowedTags lists the section tags as "[Intro], [Verse], ...".
func (g LyricsGuide) FormatAllowedTags() string {
	return formatLyricsTags(g.SectionTags)
}

// StyleRules returns an extra sentence for prompts that write the style field.
func (g LyricsGuide) StyleRules() string {
	if g.BPMInStyle {
		return ""
	}
	return " Do not put BPM, key or time signature in the style; tempo is set separately."
}

// PromptRules returns compact English lyric-format rules for LLM system prompts.
func (g LyricsGuide) PromptRules() string {
	if !g.SupportsLyrics || len(g.SectionTags) == 0 {
		return "Use bracketed section tags on their own lines, such as [Verse], [Chorus], [Bridge] and [Outro]."
	}
	var b strings.Builder
	if g.Modifiers {
		fmt.Fprintf(&b, "Lyrics format for %s:\n", lyricsProviderName(g.Provider))
		fmt.Fprintf(&b, "- Put each section tag on its own line in square brackets. Known tags: %s. Number verses as [Verse 1], [Verse 2]; short names such as [Final Chorus] also work.\n", g.FormatAllowedTags())
		b.WriteString("- Optional: one short modifier after \" - \", e.g. [Chorus - anthemic] or [Verse - whispered]. Never more than two modifiers, or the words get sung.\n")
		if len(g.VoiceTags) > 0 {
			fmt.Fprintf(&b, "- Optional vocal or energy tags on their own line: %s.\n", formatLyricsTags(g.VoiceTags))
		}
		b.WriteString("- Text in (parentheses) is sung as backing vocals. Never use parentheses for stage directions and never write repeat notes such as (x2): write repeated lines out.\n")
		b.WriteString("- UPPERCASE words are sung with more intensity; use them sparingly.\n")
		b.WriteString("- Aim for 6-10 syllables per line and keep matching lines of a section similar in length. Separate sections with a blank line.\n")
		b.WriteString("- Keep style descriptions out of the lyrics, and never let a tag contradict the style.")
		return b.String()
	}
	fmt.Fprintf(&b, "Lyrics format for %s:\n", lyricsProviderName(g.Provider))
	fmt.Fprintf(&b, "- Use only these section tags, each on its own line, spelled exactly like this: %s.\n", g.FormatAllowedTags())
	b.WriteString("- No numbers in tags ([Verse], not [Verse 1]) and no modifiers ([Chorus], not [Chorus - soft]).\n")
	b.WriteString("- Never write stage directions or repeat notes such as (x2): write repeated lines out. Text in parentheses is sung.\n")
	if g.MaxLyricsChars > 0 {
		fmt.Fprintf(&b, "- Keep the whole lyrics under %d characters. Separate sections with a blank line.", g.MaxLyricsChars)
	}
	return b.String()
}

// LintSong checks lyrics (and, for ACE-Step, the style) against the guide.
// Line numbers are 1-based on the lyrics after CRLF normalization.
func LintSong(g LyricsGuide, style, lyrics string) []LyricsIssue {
	issues := []LyricsIssue{}
	if !g.BPMInStyle && styleBPMRe.MatchString(style) {
		issues = append(issues, LyricsIssue{Code: "bpm_in_style", Severity: "info"})
	}
	if !g.SupportsLyrics || strings.TrimSpace(lyrics) == "" {
		return issues
	}
	lyrics = strings.ReplaceAll(lyrics, "\r\n", "\n")
	if n := utf8.RuneCountInString(strings.TrimSpace(lyrics)); g.MaxLyricsChars > 0 && n > g.MaxLyricsChars {
		issues = append(issues, LyricsIssue{Code: "lyrics_too_long", Severity: "error",
			Params: map[string]string{"length": strconv.Itoa(n), "max": strconv.Itoa(g.MaxLyricsChars)}})
	}
	known := map[string]bool{}
	for _, tag := range g.SectionTags {
		known[lyricsTagKey(tag)] = true
	}
	for _, tag := range g.VoiceTags {
		known[lyricsTagKey(tag)] = true
	}
	tags := 0
	for i, raw := range strings.Split(lyrics, "\n") {
		text := strings.TrimSpace(raw)
		if text == "" {
			continue
		}
		line := i + 1
		if lyricsRepeatRe.MatchString(text) {
			issues = append(issues, LyricsIssue{Code: "repeat_marker", Severity: "warning", Line: line})
			continue
		}
		matches := lyricsInlineTagRe.FindAllStringSubmatch(text, -1)
		if len(matches) == 0 {
			if base, number, ok := lyricsHeading(text); ok {
				suggestion := g.displayTag(base)
				if number != "" && g.Numbered {
					suggestion += " " + number
				}
				issues = append(issues, LyricsIssue{Code: "heading_not_tag", Severity: "warning", Line: line,
					Params: map[string]string{"text": text, "suggestion": "[" + suggestion + "]"}})
			}
			continue
		}
		for _, m := range matches {
			tags++
			issues = append(issues, g.lintTag(m[1], line, known)...)
		}
	}
	if tags == 0 {
		issues = append(issues, LyricsIssue{Code: "no_section_tags", Severity: "info"})
	}
	return issues
}

func (g LyricsGuide) lintTag(content string, line int, known map[string]bool) []LyricsIssue {
	content = strings.TrimSpace(content)
	parts := lyricsModifierRe.Split(content, -1)
	name := strings.TrimSpace(parts[0])
	var issues []LyricsIssue
	if len(parts) > 1 && !g.Modifiers {
		issues = append(issues, LyricsIssue{Code: "tag_modifier_unsupported", Severity: "warning", Line: line,
			Params: map[string]string{"tag": "[" + content + "]", "suggestion": "[" + g.displayTag(name) + "]"}})
	} else if len(parts) > 3 {
		issues = append(issues, LyricsIssue{Code: "tag_too_many_modifiers", Severity: "warning", Line: line,
			Params: map[string]string{"tag": "[" + content + "]"}})
	}
	base := name
	if m := lyricsNumberRe.FindStringSubmatch(name); m != nil {
		base = strings.TrimSpace(m[1])
		if !g.Numbered {
			issues = append(issues, LyricsIssue{Code: "tag_numbered", Severity: "warning", Line: line,
				Params: map[string]string{"tag": "[" + name + "]", "suggestion": "[" + g.displayTag(base) + "]"}})
		}
	}
	if known[lyricsTagKey(base)] {
		return issues
	}
	suggestion := g.suggestTag(base)
	if g.FreeformTags {
		if suggestion == "" {
			return issues
		}
		return append(issues, LyricsIssue{Code: "unknown_tag", Severity: "info", Line: line,
			Params: map[string]string{"tag": "[" + base + "]", "suggestion": "[" + suggestion + "]"}})
	}
	params := map[string]string{"tag": "[" + base + "]"}
	if suggestion != "" {
		params["suggestion"] = "[" + suggestion + "]"
	}
	return append(issues, LyricsIssue{Code: "unknown_tag", Severity: "warning", Line: line, Params: params})
}

// suggestTag returns the guide's spelling for the concept behind name, or "".
func (g LyricsGuide) suggestTag(name string) string {
	key := lyricsTagKey(name)
	for _, tag := range g.SectionTags {
		if lyricsTagKey(tag) == key {
			return tag
		}
	}
	group := lyricsTagGroup(key)
	for _, tag := range g.SectionTags {
		tagKey := lyricsTagKey(tag)
		for _, member := range group {
			if tagKey == member {
				return tag
			}
		}
	}
	return ""
}

// displayTag is suggestTag with the original name as fallback.
func (g LyricsGuide) displayTag(name string) string {
	if tag := g.suggestTag(name); tag != "" {
		return tag
	}
	return strings.TrimSpace(name)
}

func lyricsTagGroup(key string) []string {
	for _, group := range lyricsTagGroups {
		for _, member := range group {
			if member == key {
				return group
			}
		}
	}
	if strings.HasSuffix(key, " solo") {
		return lyricsTagGroup("solo")
	}
	return nil
}

// lyricsHeading reports a line such as "Chorus:", "**Verse 2**", "## Bridge"
// or "(Chorus)" that names a section without square brackets.
func lyricsHeading(text string) (string, string, bool) {
	core := strings.Trim(text, lyricsHeadingTrim)
	if core == "" || len(core) > 30 {
		return "", "", false
	}
	base, number := core, ""
	if m := lyricsNumberRe.FindStringSubmatch(core); m != nil {
		base, number = strings.TrimSpace(m[1]), m[2]
	}
	key := lyricsTagKey(base)
	if !lyricsSectionWord(key) {
		return "", "", false
	}
	if core == strings.TrimSpace(text) && !lyricsPlainHeadingWords[key] {
		return "", "", false
	}
	return base, number, true
}

func lyricsSectionWord(key string) bool {
	if lyricsPlainHeadingWords[key] || lyricsTagGroup(key) != nil {
		return true
	}
	for _, tags := range [][]string{aceStepSectionTags, miniMaxSectionTags} {
		for _, tag := range tags {
			if lyricsTagKey(tag) == key {
				return true
			}
		}
	}
	return false
}

func lyricsTagKey(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.NewReplacer("-", " ", "_", " ").Replace(name)
	return strings.Join(strings.Fields(name), " ")
}

func formatLyricsTags(tags []string) string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		out = append(out, "["+tag+"]")
	}
	return strings.Join(out, ", ")
}

func lyricsProviderName(provider string) string {
	switch provider {
	case "acestep":
		return "ACE-Step 1.5"
	case "minimax":
		return "MiniMax"
	case "google":
		return "Google Lyria"
	}
	if provider == "" {
		return "this provider"
	}
	return provider
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tools -run '(?i)(LyricsGuide|LintSong)' -count=1 -v`
Expected: PASS for `TestLyricsGuideForProviders`, `TestLyricsGuidePromptRules`, `TestLintSongMiniMax`, `TestLintSongACEStep`, `TestLintSongLengthAndEmpty`.

- [ ] **Step 5: Hand back** the two paths for the controller to commit as `feat(music): provider lyric tag guide and lint`.

---

### Task 2: MiniMax length guard in `GenerateMusicResult`

**Files:**
- Modify: `internal/tools/music_generation.go` (`GenerateMusicResult`, after the `MusicConfigured` check, before the local ACE-Step block)
- Test: `internal/tools/music_lyrics_guide_test.go` (append)

- [ ] **Step 1: Append the failing test** (add `"context"`, `"log/slog"` and `"aurago/internal/config"` to the imports):

```go
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
```

- [ ] **Step 2: Do not run it yet.** Without the guard this test would send a real request to the MiniMax API with the fake key. Go straight to Step 3.

- [ ] **Step 3: Add the guard** in `GenerateMusicResult` right after the `if !cfg.MusicConfigured() { ... }` block:

```go
	if guide := LyricsGuideFor(providerType); guide.Provider == "minimax" && !params.Instrumental && guide.MaxLyricsChars > 0 {
		if n := utf8.RuneCountInString(strings.TrimSpace(params.Lyrics)); n > guide.MaxLyricsChars {
			return MusicGenResult{
				Status:  "error",
				Error:   "invalid_lyrics_length",
				Message: fmt.Sprintf("MiniMax accepts at most %d characters of lyrics (got %d). Shorten the lyrics. Allowed section tags: %s.", guide.MaxLyricsChars, n, guide.FormatAllowedTags()),
			}
		}
	}
```

Add `"unicode/utf8"` to the imports of `music_generation.go` if missing.

- [ ] **Step 4: Run the tools tests**

Run: `go test ./internal/tools -run '(?i)(LyricsGuide|LintSong|GenerateMusicResultRejectsLong|Music)' -count=1`
Expected: PASS.

- [ ] **Step 5: Hand back** `internal/tools/music_generation.go` and the test file (commit `fix(music): reject MiniMax lyrics over 3500 characters before the provider call`).

---

### Task 3: Agent manual, tool description, drift test

**Files:**
- Modify: `prompts/tools_manuals/generate_music.md`
- Modify: `internal/agent/native_tools_integrations.go` (the `"lyrics"` property of `generate_music`, ~line 1847)
- Test: `internal/tools/music_lyrics_guide_test.go` (append; add `"os"` and `"path/filepath"` imports)

- [ ] **Step 1: Append the failing drift test:**

```go
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
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/tools -run TestGenerateMusicManualListsProviderTags -count=1`
Expected: FAIL listing missing tags.

- [ ] **Step 3: Edit the manual.** In `prompts/tools_manuals/generate_music.md`, change the `lyrics` table row to:

```markdown
| `lyrics` | string | no | Song lyrics with section tags in square brackets, each on its own line. The tags depend on the provider; see "Lyrics tags by provider". |
```

and insert this section directly before `## Examples`:

```markdown
## Lyrics tags by provider
Put each section tag on its own line in square brackets and separate sections with a blank line. Never write repeat notes such as `(x2)`; write repeated lines out.

### ACE-Step 1.5 (local)
- Section tags: `[Intro]`, `[Verse]` (numbered: `[Verse 1]`), `[Pre-Chorus]`, `[Chorus]`, `[Bridge]`, `[Outro]`, `[Build]`, `[Drop]`, `[Breakdown]`, `[Instrumental]`, `[Guitar Solo]`, `[Piano Interlude]`, `[Fade Out]`, `[Silence]`. Short free-form names such as `[Final Chorus]` also work.
- One short modifier after ` - ` is allowed: `[Chorus - anthemic]`, `[Verse - whispered]`. More than two modifiers get sung.
- Vocal and energy tags on their own line: `[raspy vocal]`, `[whispered]`, `[falsetto]`, `[powerful belting]`, `[spoken word]`, `[harmonies]`, `[call and response]`, `[ad-lib]`, `[high energy]`, `[low energy]`, `[building energy]`, `[explosive]`, `[melancholic]`, `[euphoric]`, `[dreamy]`, `[aggressive]`.
- `(parentheses)` are sung as backing vocals, UPPERCASE means more intensity, and 6-10 syllables per line work best.
- Put the tempo in `bpm`, not in `prompt`, and keep the tags consistent with the prompt.

### MiniMax
- Only these tags, spelled exactly: `[Intro]`, `[Verse]`, `[Pre Chorus]`, `[Chorus]`, `[Interlude]`, `[Bridge]`, `[Outro]`, `[Post Chorus]`, `[Transition]`, `[Break]`, `[Hook]`, `[Build Up]`, `[Inst]`, `[Solo]`. No numbers, no modifiers.
- At most 3500 characters of lyrics; longer lyrics are rejected before the provider call (`invalid_lyrics_length`).

### Google Lyria
- No separate lyrics field: lyrics are passed inside the text prompt, so keep them short.
```

Also replace the last bullet of `## Notes` ("MiniMax supports detailed lyrics with section tags; Google Lyria generates based on text prompts") with:

```markdown
- MiniMax and ACE-Step use different lyric tags (see "Lyrics tags by provider"); Google Lyria generates from the text prompt
```

- [ ] **Step 4: Edit the tool schema.** In `internal/agent/native_tools_integrations.go` replace the `"lyrics"` line with:

```go
				"lyrics":           prop("string", "Song lyrics with section tags in square brackets, each on its own line. Tags differ by provider (MiniMax: only its 14 tags such as [Verse], [Pre Chorus], [Chorus], no numbers, max 3500 characters; ACE-Step: [Verse 1], [Chorus - anthemic], (parentheses) = backing vocals). If empty and not instrumental, lyrics are auto-generated from the prompt."),
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/tools -run TestGenerateMusicManual -count=1` → PASS.
Run: `go test ./internal/agent -run '(?i)(Tool|Schema|Manual)' -count=1` → PASS (no test pins the old description; if one does, update its expected text to the new description).
Run: `go test ./internal/prompts -count=1` → PASS.

- [ ] **Step 6: Hand back** the three paths (commit `docs(agent): provider lyric tags in the generate_music manual`).

---

### Task 4: Noisemaker server (`internal/server`)

**Files:**
- Modify: `internal/server/noisemaker_handlers.go`
- Modify: `internal/server/server_routes.go` (after the `/api/desktop/noisemaker/enhance` line)
- Modify: `internal/server/music_local_handlers_test.go` (delete `TestNoisemakerLocalNoCloudLyrics`; remove imports that become unused)
- Create: `internal/server/noisemaker_ai_test.go`

- [ ] **Step 1: Write the failing tests** in `internal/server/noisemaker_ai_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

type noisemakerRecordingChatClient struct {
	content string
	calls   *int
	last    *openai.ChatCompletionRequest
}

func (c noisemakerRecordingChatClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if c.calls != nil {
		*c.calls++
	}
	if c.last != nil {
		*c.last = req
	}
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: c.content}}}}, nil
}

func (c noisemakerRecordingChatClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, fmt.Errorf("not implemented")
}

func noisemakerMiniMaxConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := noisemakerTestConfig(t)
	cfg.MusicGeneration.Enabled = true
	cfg.MusicGeneration.ProviderType = "minimax"
	cfg.MusicGeneration.APIKey = "test-key"
	return cfg
}

func noisemakerLocalConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := noisemakerTestConfig(t)
	cfg.MusicGeneration.Enabled = true
	cfg.MusicGeneration.Provider = config.LocalMusicProviderID
	cfg.ResolveProviders()
	return cfg
}

func noisemakerPost(t *testing.T, h http.HandlerFunc, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	var payload map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	return rec, payload
}

func noisemakerPromptText(req openai.ChatCompletionRequest) string {
	var b strings.Builder
	for _, m := range req.Messages {
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return b.String()
}

const noisemakerEnhancePath = "/api/desktop/noisemaker/enhance"

func TestNoisemakerLocalHelpersRunOnlyOnClick(t *testing.T) {
	calls := 0
	s := &Server{Cfg: noisemakerLocalConfig(t), Logger: slog.Default(), LLMClient: noisemakerRecordingChatClient{content: "[Verse 1]\nla", calls: &calls}}
	rec := httptest.NewRecorder()
	handleNoisemakerState(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/noisemaker/state", nil))
	var state map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &state)
	guide, _ := state["lyrics_guide"].(map[string]any)
	if state["llm_available"] != true || guide["provider"] != "acestep" || guide["modifiers"] != true {
		t.Fatalf("state = %v", state)
	}
	rec, payload := noisemakerPost(t, handleNoisemakerEnhance(s), noisemakerEnhancePath, `{"kind":"lyrics","text":"Piano"}`)
	if rec.Code != http.StatusOK || payload["text"] != "[Verse 1]\nla" || calls != 1 {
		t.Fatalf("explicit helper in local mode: %d %v calls=%d", rec.Code, payload, calls)
	}
	rec, _ = noisemakerPost(t, handleNoisemakerGenerate(s), "/api/desktop/noisemaker/generate", `{"prompt":"a song","style":"pop"}`)
	if calls != 1 {
		t.Fatalf("local generation called the LLM (status %d)", rec.Code)
	}
}

func TestNoisemakerEnhancePromptsCarryProviderTags(t *testing.T) {
	var last openai.ChatCompletionRequest
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default(), LLMClient: noisemakerRecordingChatClient{content: "[Verse]\nla", last: &last}}
	h := handleNoisemakerEnhance(s)
	if rec, _ := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"lyrics","context":"rain"}`); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	for _, want := range []string{"[Pre Chorus]", "[Build Up]", "3500", "No numbers"} {
		if !strings.Contains(noisemakerPromptText(last), want) {
			t.Fatalf("minimax lyrics prompt lacks %q", want)
		}
	}
	s.Cfg = noisemakerLocalConfig(t)
	if rec, _ := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"style","text":"synthwave"}`); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if !strings.Contains(noisemakerPromptText(last), "Do not put BPM") {
		t.Fatal("acestep style prompt lacks the BPM rule")
	}
	if rec, _ := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"lyrics"}`); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if !strings.Contains(noisemakerPromptText(last), "[Chorus - anthemic]") {
		t.Fatal("acestep lyrics prompt lacks the modifier rule")
	}
}

func TestNoisemakerEnhanceRevise(t *testing.T) {
	calls := 0
	var last openai.ChatCompletionRequest
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default(), LLMClient: noisemakerRecordingChatClient{content: "[Verse]\nnew", calls: &calls, last: &last}}
	h := handleNoisemakerEnhance(s)
	for _, body := range []string{
		`{"kind":"revise","action":"shorten"}`,
		`{"kind":"revise","action":"bogus","text":"[Verse]\nla"}`,
		`{"kind":"revise","action":"custom","text":"[Verse]\nla"}`,
	} {
		if rec, _ := noisemakerPost(t, h, noisemakerEnhancePath, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d", body, rec.Code)
		}
	}
	if calls != 0 {
		t.Fatal("invalid revise called the LLM")
	}
	rec, payload := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"revise","action":"custom","instruction":"translate to English","text":"[Verse]\nla"}`)
	if rec.Code != http.StatusOK || payload["text"] != "[Verse]\nnew" || !strings.Contains(noisemakerPromptText(last), "translate to English") {
		t.Fatalf("custom revise: %d %v", rec.Code, payload)
	}
	rec, payload = noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"revise","action":"fix_tags","text":"[Verse]\nla la"}`)
	if rec.Code != http.StatusOK || payload["text"] != "[Verse]\nla la" || calls != 1 {
		t.Fatalf("clean fix_tags: %d %v calls=%d", rec.Code, payload, calls)
	}
	rec, _ = noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"revise","action":"fix_tags","text":"[Verse 1]\nla la"}`)
	prompt := noisemakerPromptText(last)
	if rec.Code != http.StatusOK || calls != 2 || !strings.Contains(prompt, "Line 1: write [Verse] instead of [Verse 1].") {
		t.Fatalf("fix_tags prompt: %d calls=%d\n%s", rec.Code, calls, prompt)
	}
}

func TestNoisemakerEnhanceSongParsing(t *testing.T) {
	content := "Here you go:\n```json\n{\"idea\":\"A rainy night\",\"style\":\"lo-fi, mellow\",\"title\":\"Rain\",\"lyrics\":\"[Verse]\\nla\",\"bpm\":\"92\",\"vocal_language\":\"de\"}\n```"
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default(), LLMClient: noisemakerRecordingChatClient{content: content}}
	h := handleNoisemakerEnhance(s)
	rec, payload := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"song","text":"rain"}`)
	song, _ := payload["song"].(map[string]any)
	if rec.Code != http.StatusOK || song["idea"] != "A rainy night" || song["lyrics"] != "[Verse]\nla" || song["bpm"] != nil || song["vocal_language"] != nil {
		t.Fatalf("cloud song: %d %v", rec.Code, payload)
	}
	_, payload = noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"song","instrumental":true}`)
	if song, _ := payload["song"].(map[string]any); song["lyrics"] != "" {
		t.Fatalf("instrumental kept lyrics: %v", payload)
	}
	s.Cfg = noisemakerLocalConfig(t)
	_, payload = noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"song"}`)
	if song, _ := payload["song"].(map[string]any); song["bpm"] != float64(92) || song["vocal_language"] != "de" {
		t.Fatalf("local song: %v", payload)
	}
	s.LLMClient = noisemakerRecordingChatClient{content: "no json here"}
	rec, payload = noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"song"}`)
	if rec.Code != http.StatusBadGateway || payload["code"] != "ai_unreadable" {
		t.Fatalf("unreadable song: %d %v", rec.Code, payload)
	}
}

func TestNoisemakerEnhanceVariation(t *testing.T) {
	var last openai.ChatCompletionRequest
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default(), LLMClient: noisemakerRecordingChatClient{content: `{"idea":"B","style":"rock","title":"Part Two","lyrics":""}`, last: &last}}
	h := handleNoisemakerEnhance(s)
	for _, body := range []string{`{"kind":"variation","mode":"style"}`, `{"kind":"variation","mode":"remix","base":{"idea":"A"}}`} {
		if rec, _ := noisemakerPost(t, h, noisemakerEnhancePath, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d", body, rec.Code)
		}
	}
	rec, payload := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"variation","mode":"sequel","base":{"title":"Rain","idea":"A rainy night","style":"lo-fi","lyrics":"[Verse]\nla"}}`)
	prompt := noisemakerPromptText(last)
	if rec.Code != http.StatusOK || !strings.Contains(prompt, "Title: Rain") || !strings.Contains(prompt, "sequel") {
		t.Fatalf("variation: %d %v\n%s", rec.Code, payload, prompt)
	}
	if song, _ := payload["song"].(map[string]any); song["title"] != "Part Two" {
		t.Fatalf("variation song: %v", payload)
	}
}

func TestNoisemakerLyricsCheck(t *testing.T) {
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default()}
	h := handleNoisemakerLyricsCheck(s)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/noisemaker/lyrics/check", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status %d", rec.Code)
	}
	rec, payload := noisemakerPost(t, h, "/api/desktop/noisemaker/lyrics/check", `{"lyrics":"[Verse 1]\nla"}`)
	issues, _ := payload["issues"].([]any)
	if rec.Code != http.StatusOK || payload["provider"] != "minimax" || payload["max_chars"] != float64(3500) || len(issues) != 1 {
		t.Fatalf("check: %d %v", rec.Code, payload)
	}
	if first, _ := issues[0].(map[string]any); first["code"] != "tag_numbered" || first["line"] != float64(1) {
		t.Fatalf("issue = %v", issues[0])
	}
}

func TestNoisemakerGenerateRejectsLongMiniMaxLyrics(t *testing.T) {
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default()}
	body, _ := json.Marshal(map[string]any{"prompt": "pop", "lyrics": strings.Repeat("a", 3501)})
	rec, payload := noisemakerPost(t, handleNoisemakerGenerate(s), "/api/desktop/noisemaker/generate", string(body))
	if rec.Code != http.StatusBadRequest || payload["code"] != "invalid_lyrics_length" || !strings.Contains(fmt.Sprint(payload["message"]), "3500") {
		t.Fatalf("generate: %d %v", rec.Code, payload)
	}
}
```

- [ ] **Step 2: Delete `TestNoisemakerLocalNoCloudLyrics`** from `internal/server/music_local_handlers_test.go` (lines ~67-85) and drop imports that are no longer used there.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/server -run '(?i)Noisemaker' -count=1`
Expected: FAIL to compile (`undefined: handleNoisemakerLyricsCheck`).

- [ ] **Step 4: Implement in `internal/server/noisemaker_handlers.go`.**

4a. Imports: add `"errors"`, `"regexp"`, `"unicode/utf8"`.

4b. Next to the other constants add `noisemakerMaxInstructionLength = 500`. Below the `noisemakerFavoriteTag` constant add:

```go
var (
	errNoisemakerNothingToFix = errors.New("no tag problems to fix")
	errNoisemakerUnreadable   = errors.New("unreadable song suggestion")
	noisemakerVocalLanguageRe = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z]{2,4})?$`)
)

// noisemakerLyricsGuide returns the lyric guide of the active music provider and
// whether managed local ACE-Step is active. Callers hold CfgMu.
func noisemakerLyricsGuide(cfg *config.Config) (tools.LyricsGuide, bool) {
	if cfg == nil {
		return tools.LyricsGuideFor(""), false
	}
	if cfg.UsesLocalMusic() {
		return tools.LyricsGuideFor("acestep"), true
	}
	return tools.LyricsGuideFor(cfg.MusicGeneration.ProviderType), false
}
```

4c. Replace `noisemakerEnhanceRequest` with:

```go
type noisemakerEnhanceRequest struct {
	Kind         string                `json:"kind"`         // idea | style | lyrics | title | revise | song | variation
	Text         string                `json:"text"`         // current field content (may be empty for random/from-scratch)
	Context      string                `json:"context"`      // supporting context (idea/style of the other field)
	Lang         string                `json:"lang"`         // UI language hint for from-scratch generation
	Action       string                `json:"action"`       // revise: catchier_chorus | shorten | rhymes | add_bridge | fix_tags | custom
	Instruction  string                `json:"instruction"`  // revise custom: the user's own request
	Instrumental bool                  `json:"instrumental"` // song/variation: no lyrics
	Mode         string                `json:"mode"`         // variation: style | verses | sequel
	Base         *noisemakerSongFields `json:"base"`         // variation: the library song
}

type noisemakerSongFields struct {
	Title  string `json:"title"`
	Idea   string `json:"idea"`
	Style  string `json:"style"`
	Lyrics string `json:"lyrics"`
}

type noisemakerSongSuggestion struct {
	Idea          string `json:"idea"`
	Style         string `json:"style"`
	Title         string `json:"title"`
	Lyrics        string `json:"lyrics"`
	BPM           int    `json:"bpm,omitempty"`
	VocalLanguage string `json:"vocal_language,omitempty"`
}
```

4d. In `handleNoisemakerState`, inside the existing `RLock` block add `guide, _ := noisemakerLyricsGuide(s.Cfg)`; in the response map set `"llm_available": s.LLMClient != nil,` and add `"lyrics_guide": guide,`.

4e. Replace `handleNoisemakerEnhance` with:

```go
// handleNoisemakerEnhance returns POST /api/desktop/noisemaker/enhance — one-shot
// LLM helpers: expand an idea, refine a style, write or revise lyrics, suggest a
// title, a whole song or a variation of a library song. They run only on an
// explicit request, also in local ACE-Step mode; generation itself never calls
// the LLM in local mode.
func handleNoisemakerEnhance(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.LLMClient == nil {
			jsonError(w, "LLM is not available", http.StatusServiceUnavailable)
			return
		}
		var body noisemakerEnhanceRequest
		if err := decodeDesktopJSON(w, r, &body, noisemakerEnhanceBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		body.Kind = strings.ToLower(strings.TrimSpace(body.Kind))
		body.Text = limitNoisemakerText(body.Text, noisemakerMaxLyricsLength)
		body.Context = limitNoisemakerText(body.Context, noisemakerMaxContextLength)
		body.Lang = limitNoisemakerText(body.Lang, 16)
		body.Action = strings.ToLower(strings.TrimSpace(body.Action))
		body.Mode = strings.ToLower(strings.TrimSpace(body.Mode))
		body.Instruction = limitNoisemakerText(body.Instruction, noisemakerMaxInstructionLength)
		if body.Base != nil {
			body.Base.Title = limitNoisemakerText(body.Base.Title, noisemakerMaxTitleLength)
			body.Base.Idea = limitNoisemakerText(body.Base.Idea, noisemakerMaxIdeaLength)
			body.Base.Style = limitNoisemakerText(body.Base.Style, noisemakerMaxStyleLength)
			body.Base.Lyrics = limitNoisemakerText(body.Base.Lyrics, noisemakerMaxLyricsLength)
		}

		s.CfgMu.RLock()
		guide, local := noisemakerLyricsGuide(s.Cfg)
		model := ""
		if s.Cfg != nil {
			model = s.Cfg.LLM.Model
		}
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		systemPrompt, userPrompt, maxTokens, err := noisemakerEnhancePrompts(body, guide, local)
		if errors.Is(err, errNoisemakerNothingToFix) {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "text": body.Text})
			return
		}
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), noisemakerEnhanceTimeout)
		defer cancel()
		text, err := noisemakerRunEnhance(budget.WithTracking(ctx, s.BudgetTracker, "music_prompt"), s.LLMClient, model, systemPrompt, userPrompt, maxTokens)
		if err != nil {
			if s.Logger != nil && !llm.IsContextError(err) {
				s.Logger.Warn("Noisemaker enhance failed", "kind", body.Kind, "error", err)
			}
			jsonError(w, "AI enhancement failed", http.StatusBadGateway)
			return
		}
		if body.Kind == "song" || body.Kind == "variation" {
			song, perr := parseNoisemakerSong(text, guide, body.Instrumental, local)
			if perr != nil {
				w.WriteHeader(http.StatusBadGateway)
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "code": "ai_unreadable", "message": "The AI answer could not be read. Try again."})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "song": song})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "text": stripNoisemakerFence(text)})
	}
}
```

4f. Edit `noisemakerEnhancePrompts` in place:
- change the signature to `func noisemakerEnhancePrompts(body noisemakerEnhanceRequest, guide tools.LyricsGuide, local bool) (string, string, int, error)` and its doc comment to "builds system/user prompts per enhancement kind. Every prompt that writes lyrics or a style carries the active provider's rules.";
- leave the `case "idea":` and `case "title":` branches exactly as they are (bodies and returns);
- replace the `case "style":` and `case "lyrics":` branches with the ones below;
- add the `revise`, `song` and `variation` cases below;
- change the `default:` error text to `unknown kind %q (expected idea, style, lyrics, title, revise, song or variation)`.

Then add the helper functions that follow.

```go
	case "style":
		sys := "You are a music expert. Turn the user's input into a concise, comma-separated list of music style tags " +
			"for an AI music generator (genre, subgenre, mood, era, instrumentation, vocals, tempo, production style). " +
			"Return ONLY the tag list, at most 12 tags, no explanations, no markdown. Answer in English unless the user input is clearly not English." +
			guide.StyleRules() + langRule
		usr := strings.TrimSpace(body.Text)
		if usr == "" {
			return "", "", 0, fmt.Errorf("text is required for style enhancement")
		}
		if body.Context != "" {
			usr += "\n\nSong idea for context: " + body.Context
		}
		return sys, usr, 200, nil
	case "lyrics":
		sys := "You are a professional songwriter. Write complete song lyrics with a clear structure that match the mood and story " +
			"of the given idea and style. Return ONLY the lyrics, no explanations, no markdown. Keep the language of the user's input." +
			langRule + "\n\n" + guide.PromptRules()
		usr := ""
		if strings.TrimSpace(body.Text) != "" {
			usr = "Continue, improve and complete these lyrics:\n" + strings.TrimSpace(body.Text)
		} else {
			usr = "Write full lyrics from scratch."
		}
		if body.Context != "" {
			usr += "\n\nSong idea / style:\n" + body.Context
		}
		return sys, usr, 1400, nil
	case "revise":
		return noisemakerRevisePrompts(body, guide)
	case "song":
		return noisemakerSongPrompts(body, guide, local, noisemakerSongSeed(body))
	case "variation":
		task, err := noisemakerVariationTask(body)
		if err != nil {
			return "", "", 0, err
		}
		return noisemakerSongPrompts(body, guide, local, task)
```

Helpers (new top-level code):

```go
var noisemakerReviseActions = map[string]string{
	"catchier_chorus": "Make the chorus catchier: a short, memorable hook line, simple repeated words and a strong rhythm. Change the verses only where needed to lead into it.",
	"shorten":         "Shorten the lyrics by about a third: drop weaker lines and repeated sections, keep the story, the chorus and the structure.",
	"rhymes":          "Improve the rhymes and the flow: natural end rhymes or near rhymes, even syllable counts within each section, no forced words.",
	"add_bridge":      "Add a bridge before the last chorus that brings a new perspective or an emotional turn. Keep the rest unchanged.",
}

func noisemakerRevisePrompts(body noisemakerEnhanceRequest, guide tools.LyricsGuide) (string, string, int, error) {
	text := strings.TrimSpace(body.Text)
	if text == "" {
		return "", "", 0, fmt.Errorf("text is required for lyric revision")
	}
	instruction := ""
	switch body.Action {
	case "custom":
		if body.Instruction == "" {
			return "", "", 0, fmt.Errorf("instruction is required for a custom revision")
		}
		instruction = "Apply this request from the user: " + body.Instruction
	case "fix_tags":
		fixes := noisemakerTagFixes(tools.LintSong(guide, "", text))
		if len(fixes) == 0 {
			return "", "", 0, errNoisemakerNothingToFix
		}
		instruction = "Fix only these problems and change nothing else:\n- " + strings.Join(fixes, "\n- ")
	default:
		preset, ok := noisemakerReviseActions[body.Action]
		if !ok {
			return "", "", 0, fmt.Errorf("unknown action %q (expected catchier_chorus, shorten, rhymes, add_bridge, fix_tags or custom)", body.Action)
		}
		instruction = preset
	}
	sys := "You are a professional songwriter and lyric editor. Revise the user's song lyrics as instructed. Keep everything the " +
		"instruction does not ask to change, keep valid section tags, and keep the language of the lyrics unless the instruction asks " +
		"for another language. Return ONLY the complete revised lyrics, no explanations, no markdown.\n\n" + guide.PromptRules()
	usr := "Instruction: " + instruction + "\n\nLyrics:\n" + text
	if body.Context != "" {
		usr += "\n\nSong idea / style for context:\n" + body.Context
	}
	return sys, usr, 1600, nil
}

// noisemakerTagFixes turns lint findings into instructions for the LLM.
func noisemakerTagFixes(issues []tools.LyricsIssue) []string {
	fixes := []string{}
	for _, issue := range issues {
		p := issue.Params
		line := ""
		if issue.Line > 0 {
			line = fmt.Sprintf("Line %d: ", issue.Line)
		}
		switch issue.Code {
		case "lyrics_too_long":
			fixes = append(fixes, fmt.Sprintf("The lyrics have %s characters; shorten them to under %s characters.", p["length"], p["max"]))
		case "unknown_tag":
			if p["suggestion"] != "" {
				fixes = append(fixes, fmt.Sprintf("%sreplace the tag %s with %s.", line, p["tag"], p["suggestion"]))
			} else {
				fixes = append(fixes, fmt.Sprintf("%sreplace the unsupported tag %s with one of the allowed tags or remove it.", line, p["tag"]))
			}
		case "tag_numbered", "tag_modifier_unsupported":
			fixes = append(fixes, fmt.Sprintf("%swrite %s instead of %s.", line, p["suggestion"], p["tag"]))
		case "tag_too_many_modifiers":
			fixes = append(fixes, fmt.Sprintf("%sshorten %s to at most two modifiers.", line, p["tag"]))
		case "heading_not_tag":
			fixes = append(fixes, fmt.Sprintf("%sturn the heading %q into the tag %s.", line, p["text"], p["suggestion"]))
		case "repeat_marker":
			fixes = append(fixes, line+"remove the repeat note and write the repeated lines out.")
		case "no_section_tags":
			fixes = append(fixes, "Add section tags on their own lines.")
		}
	}
	return fixes
}

func noisemakerSongSeed(body noisemakerEnhanceRequest) string {
	parts := []string{}
	if idea := strings.TrimSpace(body.Text); idea != "" {
		parts = append(parts, "Build the song around this idea: "+idea)
	}
	if body.Context != "" {
		parts = append(parts, "Style direction: "+body.Context)
	}
	if len(parts) == 0 {
		return "Invent one surprising, specific song (an unusual theme or genre blend)."
	}
	return strings.Join(parts, "\n")
}

var noisemakerVariationModes = map[string]string{
	"style":  "Keep the theme and the story but move the song into a clearly different genre and mood. Adapt the lyrics only where the new genre needs it. Write a new style and a new title.",
	"verses": "Keep the style, the theme and the chorus. Write new verses and a new bridge that tell the story differently. Keep the title or vary it slightly.",
	"sequel": "Write a sequel: same style and characters, the story continues after the original ends. New title, new lyrics and a new chorus that echoes the original.",
}

func noisemakerVariationTask(body noisemakerEnhanceRequest) (string, error) {
	base := body.Base
	if base == nil || (base.Idea == "" && base.Style == "" && base.Lyrics == "") {
		return "", fmt.Errorf("base song is required for a variation")
	}
	task, ok := noisemakerVariationModes[body.Mode]
	if !ok {
		return "", fmt.Errorf("unknown mode %q (expected style, verses or sequel)", body.Mode)
	}
	var b strings.Builder
	b.WriteString("Original song:\n")
	fmt.Fprintf(&b, "Title: %s\nIdea: %s\nStyle: %s\n", base.Title, base.Idea, base.Style)
	if base.Lyrics != "" {
		b.WriteString("Lyrics:\n" + base.Lyrics + "\n")
	}
	b.WriteString("\nTask: " + task)
	return b.String(), nil
}

func noisemakerSongPrompts(body noisemakerEnhanceRequest, guide tools.LyricsGuide, local bool, userPrompt string) (string, string, int, error) {
	withLyrics := !body.Instrumental && guide.SupportsLyrics
	keys := `{"idea": "...", "style": "...", "title": "...", "lyrics": "..."`
	if local {
		keys += `, "bpm": 100, "vocal_language": "en"`
	}
	keys += "}"
	var b strings.Builder
	b.WriteString("You are a creative songwriter and music producer. Create one complete song for an AI music generator. " +
		"Answer with ONE JSON object only, no markdown, no code fences, with exactly these keys:\n" + keys + "\n")
	b.WriteString("- idea: 2-3 vivid sentences about subject, mood and story.\n")
	b.WriteString("- style: comma-separated style tags (genre, subgenre, mood, era, instruments, vocals, production), at most 12, in English." + guide.StyleRules() + "\n")
	b.WriteString("- title: 1-5 words.\n")
	if withLyrics {
		b.WriteString("- lyrics: complete lyrics with section tags: two verses, a memorable chorus and a bridge. Encode line breaks as JSON newlines.\n")
	} else {
		b.WriteString("- lyrics: always an empty string (instrumental).\n")
	}
	if local {
		b.WriteString("- bpm: a fitting tempo as a number from 60 to 180.\n- vocal_language: ISO code of the lyrics language, e.g. en or de.\n")
	}
	if lang := strings.TrimSpace(body.Lang); lang != "" {
		fmt.Fprintf(&b, "Write idea, title and lyrics in the language with ISO code %q unless the user's input is clearly in another language.\n", lang)
	}
	if withLyrics {
		b.WriteString("\n" + guide.PromptRules())
	}
	return b.String(), userPrompt, 2000, nil
}

// parseNoisemakerSong reads the JSON object of a song/variation answer. It
// tolerates code fences and prose around the object and caps every field.
func parseNoisemakerSong(raw string, guide tools.LyricsGuide, instrumental, local bool) (noisemakerSongSuggestion, error) {
	text := strings.TrimSpace(raw)
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return noisemakerSongSuggestion{}, errNoisemakerUnreadable
	}
	var payload struct {
		Idea          string      `json:"idea"`
		Style         string      `json:"style"`
		Title         string      `json:"title"`
		Lyrics        string      `json:"lyrics"`
		BPM           interface{} `json:"bpm"`
		VocalLanguage string      `json:"vocal_language"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &payload); err != nil {
		return noisemakerSongSuggestion{}, errNoisemakerUnreadable
	}
	song := noisemakerSongSuggestion{
		Idea:   limitNoisemakerText(payload.Idea, noisemakerMaxIdeaLength),
		Style:  limitNoisemakerText(payload.Style, noisemakerMaxStyleLength),
		Title:  limitNoisemakerText(payload.Title, noisemakerMaxTitleLength),
		Lyrics: limitNoisemakerText(payload.Lyrics, noisemakerMaxLyricsLength),
	}
	if song.Idea == "" && song.Style == "" {
		return noisemakerSongSuggestion{}, errNoisemakerUnreadable
	}
	if instrumental || !guide.SupportsLyrics {
		song.Lyrics = ""
	}
	if local {
		if bpm := noisemakerBPMValue(payload.BPM); bpm >= 30 && bpm <= 300 {
			song.BPM = bpm
		}
		if lang := strings.TrimSpace(payload.VocalLanguage); noisemakerVocalLanguageRe.MatchString(lang) {
			song.VocalLanguage = lang
		}
	}
	return song, nil
}

func noisemakerBPMValue(v interface{}) int {
	switch value := v.(type) {
	case float64:
		return int(value)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return n
		}
	}
	return 0
}

// stripNoisemakerFence removes a code fence the model wrapped around its answer.
func stripNoisemakerFence(text string) string {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "```") {
		return t
	}
	if nl := strings.IndexByte(t, '\n'); nl >= 0 {
		t = t[nl+1:]
	} else {
		t = strings.TrimPrefix(t, "```")
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(t), "```"))
}

// handleNoisemakerLyricsCheck returns POST /api/desktop/noisemaker/lyrics/check —
// a deterministic check of lyrics (and style) against the active provider's tags.
func handleNoisemakerLyricsCheck(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Lyrics string `json:"lyrics"`
			Style  string `json:"style"`
		}
		if err := decodeDesktopJSON(w, r, &body, noisemakerEnhanceBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		s.CfgMu.RLock()
		guide, _ := noisemakerLyricsGuide(s.Cfg)
		s.CfgMu.RUnlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "ok",
			"issues":    tools.LintSong(guide, body.Style, body.Lyrics),
			"length":    utf8.RuneCountInString(strings.TrimSpace(body.Lyrics)),
			"max_chars": guide.MaxLyricsChars,
			"provider":  guide.Provider,
		})
	}
}
```

Task 4 runs after Task 2: `TestNoisemakerGenerateRejectsLongMiniMaxLyrics` relies on the guard from Task 2 and would otherwise call the MiniMax API.

4g. In `handleNoisemakerGenerate`:
- the auto-lyrics call becomes `noisemakerEnhancePrompts(noisemakerEnhanceRequest{Kind: "lyrics", Context: strings.TrimSpace(body.Prompt + "\n" + body.Style), Lang: body.Lang}, tools.LyricsGuideFor(providerType), false)`;
- the `result.Status != "ok"` branch encodes the message from `result.Message` when it is set:

```go
			message := result.Error
			if result.Message != "" {
				message = result.Message
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "code": result.Error, "message": message})
			return
```

4h. In `internal/server/server_routes.go`, after the enhance route add:

```go
	mux.HandleFunc("/api/desktop/noisemaker/lyrics/check", handleNoisemakerLyricsCheck(s))
```

- [ ] **Step 5: Run the server tests**

Run: `go test ./internal/server -run '(?i)Noisemaker' -count=1 -v`
Expected: all `TestNoisemaker*` PASS, including the existing ones (`TestNoisemakerEnhanceIdeaReturnsText`, `TestNoisemakerStateEnabled`, ...).

Run: `go vet ./internal/server ./internal/tools`
Expected: no output.

- [ ] **Step 6: Hand back** the four server paths (commit `feat(noisemaker): provider-aware AI helpers, song and variation suggestions, lyrics check`).

---

### Task 5: Create panel (`ui/js/desktop/apps/noisemaker-create.js`)

**Files:**
- Modify: `ui/js/desktop/apps/noisemaker-create.js`

Keep the export line `window.NoisemakerCreate = { create, PRESETS };` exactly (a static test pins it).

- [ ] **Step 1: Constants.** Update the header comment's deps list to include `menu`. After `PRESETS` add:

```js
    const SONG_FIELDS = ['idea', 'style', 'title', 'lyrics'];
    const FIELD_LABELS = {
        idea: { labelKey: 'desktop.noisemaker_idea_label' },
        style: { labelKey: 'desktop.noisemaker_style_label' },
        title: { labelKey: 'desktop.noisemaker_title_label' },
        lyrics: { labelKey: 'desktop.noisemaker_lyrics_label' },
        bpm: { labelKey: 'desktop.noisemaker_bpm' },
        vocal_language: { labelKey: 'desktop.noisemaker_vocal_language' }
    };
    const ISSUE_TEXT = {
        lyrics_too_long: { messageKey: 'desktop.noisemaker_issue_lyrics_too_long' },
        unknown_tag: { messageKey: 'desktop.noisemaker_issue_unknown_tag' },
        unknown_tag_suggest: { messageKey: 'desktop.noisemaker_issue_unknown_tag_suggest' },
        tag_numbered: { messageKey: 'desktop.noisemaker_issue_tag_numbered' },
        tag_modifier_unsupported: { messageKey: 'desktop.noisemaker_issue_tag_modifier_unsupported' },
        tag_too_many_modifiers: { messageKey: 'desktop.noisemaker_issue_tag_too_many_modifiers' },
        heading_not_tag: { messageKey: 'desktop.noisemaker_issue_heading_not_tag' },
        repeat_marker: { messageKey: 'desktop.noisemaker_issue_repeat_marker' },
        no_section_tags: { messageKey: 'desktop.noisemaker_issue_no_section_tags' },
        bpm_in_style: { messageKey: 'desktop.noisemaker_issue_bpm_in_style' }
    };
    const PROVIDER_NAMES = { acestep: 'ACE-Step', minimax: 'MiniMax', google: 'Google Lyria' };
```

- [ ] **Step 2: State.** In `create(deps)` after `const readonly = ...` add `const menu = deps.menu || (() => {});` and after `let disposed = false;` add:

```js
        let undoState = null;      // { before: { field: previousValue } }
        let checkState = null;     // { loading, issues, provider, error, token }
        let songRequestId = 0;
```

- [ ] **Step 3: Markup helpers.** Add next to `aiButton`:

```js
        function aiAvailable() {
            return !!(caps && caps.llm_available !== false);
        }

        function songRowMarkup() {
            if (!aiAvailable()) return '';
            return '<div class="nm-ai-row">' +
                '<button type="button" class="nm-ai nm-ai--song" data-nm-song title="' + esc(t('desktop.noisemaker_song_suggest_hint')) + '"' + (readonly ? ' disabled' : '') + '>' +
                    '<span class="nm-ai-glyph" aria-hidden="true">✨</span>' + esc(t('desktop.noisemaker_song_suggest')) + '</button>' +
            '</div>';
        }

        function undoMarkup() {
            return '<div class="nm-undo" data-nm-undo role="status" aria-live="polite" hidden></div>';
        }

        function lyricsToolsMarkup() {
            if (aiAvailable()) {
                return '<button type="button" class="nm-ai" data-nm-lyrics-menu aria-haspopup="menu">' +
                    '<span class="nm-ai-glyph" aria-hidden="true">✨</span>' + esc(t('desktop.noisemaker_lyrics_ai_menu')) + ' <span aria-hidden="true">▾</span></button>';
            }
            return '<button type="button" class="nm-ai" data-nm-lyrics-check>' +
                '<span class="nm-ai-glyph" aria-hidden="true">✓</span>' + esc(t('desktop.noisemaker_lyrics_check')) + '</button>';
        }

        function instructionMarkup() {
            return '<div class="nm-instruction" data-nm-instruction hidden>' +
                '<input class="nm-input" data-nm-instruction-input maxlength="500" placeholder="' + esc(t('desktop.noisemaker_revise_custom_placeholder')) + '" aria-label="' + esc(t('desktop.noisemaker_revise_custom')) + '">' +
                '<button type="button" class="nm-btn nm-btn--primary" data-nm-instruction-apply>' + esc(t('desktop.noisemaker_revise_apply')) + '</button>' +
                '<button type="button" class="nm-icon-btn nm-icon-btn--small" data-nm-instruction-close aria-label="' + esc(t('desktop.noisemaker_panel_close')) + '" title="' + esc(t('desktop.noisemaker_panel_close')) + '">×</button>' +
            '</div>';
        }
```

- [ ] **Step 4: Lyrics markup.** In `lyricsMarkup()`, replace the non-`unsupported` branch of `body` with:

```js
                : '<div class="nm-field-head">' + lyricsToolsMarkup() + '</div>' +
                  '<textarea class="nm-textarea nm-textarea--lyrics" data-nm-field="lyrics" maxlength="' + LYRICS_MAX + '" placeholder="' + esc(t('desktop.noisemaker_lyrics_placeholder')) + '"></textarea>' +
                  '<div class="nm-field-foot"><span class="nm-counter" data-nm-counter="lyrics"></span></div>' +
                  instructionMarkup() +
                  '<div class="nm-check" data-nm-check aria-live="polite" hidden></div>' +
                  '<p class="nm-hint" data-nm-lyrics-required-hint' + (required && c.llm_available === false ? '' : ' hidden') + '>' + esc(t('desktop.noisemaker_lyrics_required_hint')) + '</p>';
```

- [ ] **Step 5: renderForm.** Change the `formEl.innerHTML` assignment to insert `songRowMarkup() + undoMarkup()` right after the readonly hint, and call `renderUndo(); renderCheck();` at the end of `renderForm()`:

```js
            formEl.innerHTML = (readonly ? '<p class="nm-hint" role="status">' + esc(t('desktop.noisemaker_readonly_hint')) + '</p>' : '') +
                songRowMarkup() + undoMarkup() + (mode === 'simple'
                ? ideaFieldMarkup() + presetsMarkup() + switchesMarkup()
                : ideaFieldMarkup() + styleFieldMarkup() + switchesMarkup() + lyricsMarkup() + titleMarkup() + (c.supports_controls ? localControlsMarkup() : ''));
```

- [ ] **Step 6: Counter.** Replace `syncCounters` with:

```js
        function lyricsMax() {
            const guide = caps && caps.lyrics_guide;
            const max = guide ? Number(guide.max_chars) : 0;
            return max > 0 ? Math.min(max, LYRICS_MAX) : LYRICS_MAX;
        }

        function syncCounters() {
            root.querySelectorAll('[data-nm-counter]').forEach(el => {
                const field = el.dataset.nmCounter;
                const len = String(form[field] || '').length;
                const max = field === 'lyrics' ? lyricsMax() : IDEA_MAX;
                el.textContent = len > 0 ? len + ' / ' + max : '';
                el.classList.toggle('is-over', len > max);
            });
        }
```

- [ ] **Step 7: Form writes and undo.** Replace `setForm` with `writeForm` + `setForm`, and add the undo helpers:

```js
        function writeForm(patch) {
            assignForm(patch);
            applyFormToInputs();
            syncPresets();
            syncCounters();
            syncCreateButton();
            if (patch && Object.prototype.hasOwnProperty.call(patch, 'lyrics')) closeCheck();
        }

        // setForm is used for user-driven and external changes; it drops the AI undo.
        function setForm(patch) {
            clearUndo();
            writeForm(patch);
        }

        function applyAiChange(patch) {
            const before = {};
            Object.keys(patch).forEach(field => { before[field] = form[field]; });
            writeForm(patch);
            undoState = { before };
            renderUndo();
            emit('change', getForm());
        }

        function clearUndo() {
            if (!undoState) return;
            undoState = null;
            renderUndo();
        }

        function undoAi() {
            if (!undoState) return;
            const before = undoState.before;
            undoState = null;
            writeForm(before);
            renderUndo();
            emit('change', getForm());
        }

        function renderUndo() {
            const bar = qs('[data-nm-undo]');
            if (!bar) return;
            if (!undoState) { bar.hidden = true; bar.innerHTML = ''; return; }
            const names = Object.keys(undoState.before).filter(field => FIELD_LABELS[field]).map(field => t(FIELD_LABELS[field].labelKey));
            bar.hidden = false;
            bar.innerHTML = '<span class="nm-undo-text">' + esc(t('desktop.noisemaker_ai_applied', { fields: names.join(', ') })) + '</span>' +
                '<button type="button" class="nm-link-btn" data-nm-undo-btn>' + esc(t('desktop.noisemaker_ai_undo')) + '</button>';
        }

        function openLyrics() {
            const wrap = qs('[data-nm-lyrics-wrap]');
            if (wrap) wrap.open = true;
        }
```

- [ ] **Step 8: setMode.** In `setMode`, before `renderForm();` add `clearUndo(); checkState = null;`.

- [ ] **Step 9: AI requests.** Replace the whole `enhance` function with:

```js
        function setBusy(el, on) {
            if (!el) return;
            el.classList.toggle('is-busy', on);
            el.disabled = on;
        }

        function aiErrorMessage(err) {
            if (err && err.body && err.body.code === 'ai_unreadable') return t('desktop.noisemaker_ai_unreadable');
            return (err && err.message) || t('desktop.noisemaker_enhance_failed');
        }

        async function runFieldRequest(field, busyEl, body) {
            if (readonly || disposed) return;
            const revision = fieldRevisions[field] || 0;
            const requestId = (enhancementRequests[field] || 0) + 1;
            enhancementRequests[field] = requestId;
            const isCurrent = () => !disposed && enhancementRequests[field] === requestId && (fieldRevisions[field] || 0) === revision;
            setBusy(busyEl, true);
            try {
                const data = await request('/api/desktop/noisemaker/enhance', { method: 'POST', body });
                if (!isCurrent()) return;
                if (data && typeof data.text === 'string' && data.text && data.text !== form[field]) {
                    applyAiChange({ [field]: data.text });
                    if (field === 'lyrics') openLyrics();
                }
            } catch (err) {
                if (!isCurrent()) return;
                notify(aiErrorMessage(err));
            } finally {
                setBusy(busyEl, false);
            }
        }

        function enhance(kind, button) {
            const apiKind = kind === 'random' ? 'idea' : kind;
            const fieldFor = { idea: 'idea', random: 'idea', style: 'style', lyrics: 'lyrics', title: 'title' };
            const field = fieldFor[kind];
            if (!field) return;
            const currentValue = kind === 'random' ? '' : String(form[field] || '');
            if (kind === 'style' && !currentValue.trim()) return;
            runFieldRequest(field, button, { kind: apiKind, text: currentValue, context: enhanceContextFor(apiKind), lang });
        }

        function revise(action, instruction) {
            const text = String(form.lyrics || '');
            if (!text.trim()) return;
            runFieldRequest('lyrics', qs('[data-nm-lyrics-menu]'), {
                kind: 'revise', action, instruction: instruction || '', text, context: enhanceContextFor('lyrics'), lang
            });
        }

        async function suggestSong(body, overrides) {
            if (readonly || disposed || !aiAvailable()) return;
            const extra = overrides || {};
            const instrumental = Object.prototype.hasOwnProperty.call(extra, 'instrumental') ? !!extra.instrumental : !!form.instrumental;
            const controls = !!(caps && caps.supports_controls);
            const fields = SONG_FIELDS.concat(controls ? ['bpm', 'vocal_language'] : []);
            fields.forEach(field => { enhancementRequests[field] = (enhancementRequests[field] || 0) + 1; });
            const revisions = fields.map(field => fieldRevisions[field] || 0);
            const requestId = ++songRequestId;
            const isCurrent = () => !disposed && songRequestId === requestId && fields.every((field, i) => (fieldRevisions[field] || 0) === revisions[i]);
            setBusy(qs('[data-nm-song]'), true);
            root.classList.add('is-suggesting');
            try {
                const data = await request('/api/desktop/noisemaker/enhance', { method: 'POST', body: Object.assign({ lang, instrumental }, body) });
                if (!isCurrent() || !data || !data.song) return;
                const song = data.song;
                const patch = { idea: String(song.idea || ''), style: String(song.style || ''), title: String(song.title || '') };
                if (instrumental !== !!form.instrumental) patch.instrumental = instrumental;
                if (!instrumental && !(caps && caps.supports_lyrics === false)) patch.lyrics = String(song.lyrics || '');
                if (controls && song.bpm) patch.bpm = String(song.bpm);
                if (controls && song.vocal_language) patch.vocal_language = String(song.vocal_language);
                if (mode !== 'custom') setMode('custom');
                applyAiChange(patch);
                openLyrics();
            } catch (err) {
                if (isCurrent()) notify(aiErrorMessage(err));
            } finally {
                if (songRequestId === requestId) {
                    setBusy(qs('[data-nm-song]'), false);
                    root.classList.remove('is-suggesting');
                }
            }
        }

        function suggestVariation(base, variationMode) {
            if (!base) return;
            suggestSong({
                kind: 'variation',
                mode: variationMode,
                base: { title: String(base.title || ''), idea: String(base.idea || ''), style: String(base.style || ''), lyrics: String(base.lyrics || '') }
            }, { instrumental: !!base.instrumental });
        }
```

- [ ] **Step 10: Lyrics menu, instruction row, check panel.** Add below the AI request functions:

```js
        function openLyricsMenu(button) {
            const hasText = !!String(form.lyrics || '').trim();
            const aiOff = readonly || !aiAvailable();
            const items = [
                { id: 'lyrics-write', label: hasText ? t('desktop.noisemaker_lyrics_continue') : t('desktop.noisemaker_lyrics_generate'), icon: 'edit', fallback: '+', disabled: aiOff, action: () => enhance('lyrics', button) },
                { separator: true },
                { id: 'lyrics-chorus', label: t('desktop.noisemaker_revise_catchier_chorus'), icon: 'audio', fallback: '♪', disabled: aiOff || !hasText, action: () => revise('catchier_chorus') },
                { id: 'lyrics-shorten', label: t('desktop.noisemaker_revise_shorten'), icon: 'x', fallback: '-', disabled: aiOff || !hasText, action: () => revise('shorten') },
                { id: 'lyrics-rhymes', label: t('desktop.noisemaker_revise_rhymes'), icon: 'refresh', fallback: '~', disabled: aiOff || !hasText, action: () => revise('rhymes') },
                { id: 'lyrics-bridge', label: t('desktop.noisemaker_revise_add_bridge'), icon: 'plus', fallback: '+', disabled: aiOff || !hasText, action: () => revise('add_bridge') },
                { id: 'lyrics-custom', label: t('desktop.noisemaker_revise_custom'), icon: 'edit', fallback: '…', disabled: aiOff || !hasText, action: () => openInstruction() },
                { separator: true },
                { id: 'lyrics-check', label: t('desktop.noisemaker_lyrics_check'), icon: 'check-square', fallback: '✓', disabled: !hasText, action: () => checkLyrics() }
            ];
            const rect = button.getBoundingClientRect();
            menu(rect.left, rect.bottom + 4, items);
        }

        function openInstruction() {
            const row = qs('[data-nm-instruction]');
            if (!row) return;
            openLyrics();
            row.hidden = false;
            const input = row.querySelector('[data-nm-instruction-input]');
            if (input) input.focus();
        }

        function closeInstruction() {
            const row = qs('[data-nm-instruction]');
            if (row) row.hidden = true;
        }

        function applyInstruction() {
            const input = qs('[data-nm-instruction-input]');
            const value = input ? input.value.trim() : '';
            if (!value) { if (input) input.focus(); return; }
            input.value = '';
            closeInstruction();
            revise('custom', value);
        }

        function closeCheck() {
            if (!checkState) return;
            checkState = null;
            renderCheck();
        }

        async function checkLyrics() {
            if (disposed) return;
            const lyrics = String(form.lyrics || '');
            if (!lyrics.trim()) return;
            const revision = fieldRevisions.lyrics || 0;
            const token = {};
            checkState = { loading: true, issues: [], token };
            openLyrics();
            renderCheck();
            try {
                const data = await request('/api/desktop/noisemaker/lyrics/check', { method: 'POST', body: { lyrics, style: String(form.style || '') } });
                if (disposed || !checkState || checkState.token !== token || (fieldRevisions.lyrics || 0) !== revision) return;
                checkState = { loading: false, issues: Array.isArray(data && data.issues) ? data.issues : [], provider: String((data && data.provider) || ''), token };
            } catch (err) {
                if (disposed || !checkState || checkState.token !== token) return;
                checkState = { loading: false, issues: [], error: (err && err.message) || t('desktop.noisemaker_check_failed'), token };
            }
            renderCheck();
        }

        function issueText(issue) {
            const params = Object.assign({}, issue.params || {});
            let entry = ISSUE_TEXT[issue.code];
            if (issue.code === 'unknown_tag' && params.suggestion) entry = ISSUE_TEXT.unknown_tag_suggest;
            return entry ? t(entry.messageKey, params) : String(issue.code || '');
        }

        function renderCheck() {
            const el = qs('[data-nm-check]');
            if (!el) return;
            if (!checkState) { el.hidden = true; el.innerHTML = ''; return; }
            el.hidden = false;
            const closeLabel = esc(t('desktop.noisemaker_panel_close'));
            const head = '<div class="nm-check-head"><strong>' + esc(t('desktop.noisemaker_check_title')) + '</strong>' +
                '<button type="button" class="nm-icon-btn nm-icon-btn--small" data-nm-check-close aria-label="' + closeLabel + '" title="' + closeLabel + '">×</button></div>';
            if (checkState.loading) { el.innerHTML = head + '<p class="nm-hint">' + esc(t('desktop.noisemaker_check_running')) + '</p>'; return; }
            if (checkState.error) { el.innerHTML = head + '<p class="nm-hint nm-check-error">' + esc(checkState.error) + '</p>'; return; }
            const issues = checkState.issues;
            if (!issues.length) {
                const provider = PROVIDER_NAMES[checkState.provider] || checkState.provider || '';
                el.innerHTML = head + '<p class="nm-check-ok">✓ ' + esc(t('desktop.noisemaker_check_ok', { provider })) + '</p>';
                return;
            }
            const fixable = issues.some(issue => issue.severity === 'error' || issue.severity === 'warning');
            const canFix = fixable && aiAvailable() && !readonly;
            const glyph = { error: '⛔', warning: '⚠', info: 'ℹ' };
            el.innerHTML = head + '<ul class="nm-check-list">' + issues.map(issue =>
                '<li class="nm-check-item nm-check-item--' + esc(issue.severity) + '">' +
                    '<span class="nm-check-sev" aria-hidden="true">' + (glyph[issue.severity] || 'ℹ') + '</span>' +
                    (issue.line ? '<button type="button" class="nm-link-btn nm-check-line" data-nm-check-line="' + Number(issue.line) + '">' + esc(t('desktop.noisemaker_check_line', { line: issue.line })) + '</button>' : '') +
                    '<span class="nm-check-text">' + esc(issueText(issue)) + '</span></li>').join('') + '</ul>' +
                (canFix ? '<button type="button" class="nm-btn nm-btn--primary nm-check-fix" data-nm-check-fix>✨ ' + esc(t('desktop.noisemaker_check_fix')) + '</button>' : '');
        }

        function selectLyricsLine(lineNo) {
            const area = qs('[data-nm-field="lyrics"]');
            if (!area || !(lineNo > 0)) return;
            const lines = area.value.split('\n');
            let start = 0;
            for (let i = 0; i < lineNo - 1 && i < lines.length; i++) start += lines[i].length + 1;
            const end = start + (lines[lineNo - 1] || '').length;
            area.focus();
            area.setSelectionRange(start, end);
        }
```

- [ ] **Step 11: Events.** Update the handlers:

```js
        root.addEventListener('input', event => {
            const input = event.target.closest('[data-nm-field]');
            if (!input) return;
            const field = input.dataset.nmField;
            assignForm({ [field]: input.type === 'checkbox' ? input.checked : input.value });
            if (undoState && Object.prototype.hasOwnProperty.call(undoState.before, field)) clearUndo();
            if (field === 'lyrics') closeCheck();
            syncCounters();
            syncPresets();
            syncCreateButton();
            emit('change', getForm());
        });

        root.addEventListener('change', event => {
            const input = event.target.closest('[data-nm-field]');
            if (!input || input.type !== 'checkbox') return;
            const field = input.dataset.nmField;
            assignForm({ [field]: input.checked });
            if (undoState && Object.prototype.hasOwnProperty.call(undoState.before, field)) clearUndo();
            syncCreateButton();
            emit('change', getForm());
        });
```

In the `click` handler insert these lines directly before `const ai = event.target.closest('[data-nm-enhance]');`:

```js
            if (event.target.closest('[data-nm-song]')) { suggestSong({ kind: 'song', text: String(form.idea || '').trim(), context: String(form.style || '').trim() }); return; }
            const lyricsMenuBtn = event.target.closest('[data-nm-lyrics-menu]');
            if (lyricsMenuBtn) { openLyricsMenu(lyricsMenuBtn); return; }
            if (event.target.closest('[data-nm-lyrics-check]')) { checkLyrics(); return; }
            if (event.target.closest('[data-nm-undo-btn]')) { undoAi(); return; }
            if (event.target.closest('[data-nm-instruction-apply]')) { applyInstruction(); return; }
            if (event.target.closest('[data-nm-instruction-close]')) { closeInstruction(); return; }
            if (event.target.closest('[data-nm-check-close]')) { closeCheck(); return; }
            if (event.target.closest('[data-nm-check-fix]')) { closeCheck(); revise('fix_tags'); return; }
            const lineBtn = event.target.closest('[data-nm-check-line]');
            if (lineBtn) { selectLyricsLine(Number(lineBtn.dataset.nmCheckLine)); return; }
```

At the top of the `keydown` handler add:

```js
            if (event.target.closest('[data-nm-instruction-input]')) {
                if (event.key === 'Enter') { event.preventDefault(); applyInstruction(); }
                else if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); closeInstruction(); }
                return;
            }
```

- [ ] **Step 12: Public API.** Add `suggestVariation` to the returned object:

```js
        return {
            element: root,
            setCaps, setMode, getMode: () => mode,
            setForm, getForm, setGeneration, focusIdea, suggestVariation,
            on, dispose
        };
```

- [ ] **Step 13: Syntax check**

Run: `node --check ui/js/desktop/apps/noisemaker-create.js` → no output.

---

### Task 6: Shell and menus

**Files:**
- Modify: `ui/js/desktop/apps/noisemaker.js`
- Modify: `ui/js/desktop/apps/noisemaker-menus.js`

- [ ] **Step 1:** In `mountModules`, add to the `NoisemakerCreate.create` deps:

```js
            menu: (x, y, items) => { if (typeof S.ctx.showContextMenu === 'function') S.ctx.showContextMenu(x, y, items); },
```

- [ ] **Step 2:** Replace `useTemplate` with `templateFields` + `useTemplate` + `suggestVariation`:

```js
    function templateFields(track) {
        let idea = String(track.prompt || '');
        let style = String(track.style || '');
        const sep = idea.indexOf(' — ');
        if (sep > 0) {
            if (!style) style = idea.slice(0, sep);
            idea = idea.startsWith(style + ' — ') ? idea.slice(style.length + 3) : idea.slice(sep + 3);
        }
        return { idea, style, lyrics: String(track.lyrics || ''), instrumental: !!track.instrumental };
    }

    function useTemplate(S, track) {
        if (!track || !S.create) return;
        const fields = templateFields(track);
        if (fields.lyrics) S.create.setMode('custom');
        S.create.setForm({ idea: fields.idea, style: fields.style, lyrics: fields.lyrics, title: '', instrumental: fields.instrumental });
        S.prefs.style = fields.style;
        S.prefs.instrumental = fields.instrumental;
        savePrefs(S.prefs);
        focusCreate(S);
    }

    function suggestVariation(S, track, mode) {
        if (!track || !S.create || S.readonly) return;
        focusCreate(S);
        S.create.suggestVariation(Object.assign({ title: String(track.title || '') }, templateFields(track)), mode);
    }
```

- [ ] **Step 3:** In `menuModel(S).s` add `aiAvailable: !!(S.caps && S.caps.llm_available),`. In `menuActions(S)` add `suggestVariation: (track, mode) => suggestVariation(S, track, mode),`.

- [ ] **Step 4:** In `noisemaker-menus.js` `trackContextItems`, destructure `s` (`const { t, s, readonly, actions } = m;`) and directly after the `template` push add:

```js
        if (!multi && s && s.aiAvailable) {
            items.push({
                id: 'variation', label: t('desktop.noisemaker_variation'), icon: 'edit', fallback: '*', disabled: readonly,
                items: [
                    { id: 'variation-style', label: t('desktop.noisemaker_variation_style'), action: () => actions.suggestVariation(track, 'style') },
                    { id: 'variation-verses', label: t('desktop.noisemaker_variation_verses'), action: () => actions.suggestVariation(track, 'verses') },
                    { id: 'variation-sequel', label: t('desktop.noisemaker_variation_sequel'), action: () => actions.suggestVariation(track, 'sequel') }
                ]
            });
        }
```

- [ ] **Step 5:** `node --check ui/js/desktop/apps/noisemaker.js && node --check ui/js/desktop/apps/noisemaker-menus.js` → no output.

---

### Task 7: Styles (`ui/css/desktop-app-noisemaker.css`)

- [ ] **Step 1:** Insert after the `.nm-ai.is-busy .nm-ai-glyph` rule (~line 337):

```css
.nm-ai-row { align-items: center; display: flex; gap: 8px; }
.nm-ai--song { font-size: 12.5px; margin-left: 0; padding: 7px 12px; }
.nm-create.is-suggesting .nm-create-form > :not(.nm-ai-row) { opacity: .6; transition: opacity .2s ease; }
.nm-undo { align-items: center; background: var(--nm-accent-soft); border-radius: var(--nm-field-radius); display: flex; flex-wrap: wrap; font-size: 12px; gap: 8px; padding: 7px 10px; }
.nm-undo[hidden], .nm-instruction[hidden], .nm-check[hidden] { display: none; }
.nm-undo-text { flex: 1; min-width: 0; }
.nm-instruction { align-items: center; display: flex; gap: 6px; }
.nm-instruction .nm-input { flex: 1; min-width: 0; }
.nm-check { background: var(--nm-field); border: 1px solid var(--nm-field-border); border-radius: var(--nm-field-radius); display: grid; gap: 8px; padding: 10px 12px; }
.nm-check-head { align-items: center; display: flex; font-size: 12.5px; gap: 8px; justify-content: space-between; }
.nm-check-list { display: grid; font-size: 12px; gap: 6px; list-style: none; margin: 0; padding: 0; }
.nm-check-item { align-items: baseline; display: flex; gap: 6px; }
.nm-check-item--error .nm-check-text { color: var(--nm-danger); }
.nm-check-item--warning .nm-check-sev { color: var(--nm-warn); }
.nm-check-item--info .nm-check-sev { color: var(--nm-info); }
.nm-check-line { flex: none; font-variant-numeric: tabular-nums; }
.nm-check-ok { color: var(--nm-ok); font-size: 12px; margin: 0; }
.nm-check-error { color: var(--nm-danger); }
.nm-check-fix { justify-self: start; }
.nm-counter.is-over { color: var(--nm-danger); font-weight: 600; }
```

- [ ] **Step 2:** In the existing reduced-motion block (the `@media` that lists `.nm-ai.is-busy .nm-ai-glyph` around line 810) add `.nm-create.is-suggesting .nm-create-form > :not(.nm-ai-row),` to its selector list so the opacity transition is disabled there too.

---

### Task 8: Locale keys (16 files)

**Files:** `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`

- [ ] **Step 1:** Write `C:\Users\Andi\AppData\Local\Temp\claude\3e6417243ebbf17a\f6769417-beca-4c78-8da5-bb505c974fb7\scratchpad\nm-keys.json` with one object per language: `{"en": {...}, "de": {...}, "cs": {...}, ...}`. English and German values are below; translate the English text into the other 14 languages (natural UI wording, keep `{{placeholders}}` and the `[Verse]`/`[Chorus]`/`BPM` tokens unchanged).

| Key | en | de |
|---|---|---|
| `desktop.noisemaker_song_suggest` | Suggest a song | Song vorschlagen |
| `desktop.noisemaker_song_suggest_hint` | AI fills in idea, style, title and lyrics | KI füllt Idee, Stil, Titel und Songtext aus |
| `desktop.noisemaker_lyrics_ai_menu` | AI | KI |
| `desktop.noisemaker_lyrics_continue` | Continue lyrics with AI | Songtext mit KI fortsetzen |
| `desktop.noisemaker_revise_catchier_chorus` | Make the chorus catchier | Refrain eingängiger machen |
| `desktop.noisemaker_revise_shorten` | Shorten | Kürzen |
| `desktop.noisemaker_revise_rhymes` | Improve rhymes and flow | Reime und Fluss verbessern |
| `desktop.noisemaker_revise_add_bridge` | Add a bridge | Bridge ergänzen |
| `desktop.noisemaker_revise_custom` | Your own request… | Eigener Wunsch… |
| `desktop.noisemaker_revise_custom_placeholder` | e.g. translate to English, more hopeful, add a rap part | z. B. ins Englische übersetzen, hoffnungsvoller, Rap-Teil einbauen |
| `desktop.noisemaker_revise_apply` | Apply | Anwenden |
| `desktop.noisemaker_panel_close` | Close | Schließen |
| `desktop.noisemaker_lyrics_check` | Check tags | Tags prüfen |
| `desktop.noisemaker_check_title` | Tag check | Tag-Prüfung |
| `desktop.noisemaker_check_running` | Checking… | Prüfe… |
| `desktop.noisemaker_check_ok` | Everything fits {{provider}}. | Alles passt zu {{provider}}. |
| `desktop.noisemaker_check_failed` | The check failed. | Die Prüfung ist fehlgeschlagen. |
| `desktop.noisemaker_check_line` | Line {{line}} | Zeile {{line}} |
| `desktop.noisemaker_check_fix` | Fix with AI | Mit KI korrigieren |
| `desktop.noisemaker_issue_lyrics_too_long` | The lyrics have {{length}} characters; this provider accepts at most {{max}}. | Der Songtext hat {{length}} Zeichen, dieser Anbieter nimmt höchstens {{max}}. |
| `desktop.noisemaker_issue_unknown_tag` | {{tag}} is not a tag this provider knows. | {{tag}} kennt dieser Anbieter nicht. |
| `desktop.noisemaker_issue_unknown_tag_suggest` | {{tag}} is not a tag this provider knows; use {{suggestion}}. | {{tag}} kennt dieser Anbieter nicht, besser {{suggestion}}. |
| `desktop.noisemaker_issue_tag_numbered` | This provider expects {{suggestion}} without a number instead of {{tag}}. | Dieser Anbieter erwartet {{suggestion}} ohne Nummer statt {{tag}}. |
| `desktop.noisemaker_issue_tag_modifier_unsupported` | This provider does not understand additions like in {{tag}}; use {{suggestion}}. | Zusätze wie in {{tag}} versteht dieser Anbieter nicht, besser {{suggestion}}. |
| `desktop.noisemaker_issue_tag_too_many_modifiers` | {{tag}} has too many additions; the extra words may be sung. | {{tag}} hat zu viele Zusätze, die Wörter werden womöglich mitgesungen. |
| `desktop.noisemaker_issue_heading_not_tag` | "{{text}}" looks like a heading; write it as the tag {{suggestion}}. | „{{text}}“ sieht wie eine Überschrift aus, schreibe sie als Tag {{suggestion}}. |
| `desktop.noisemaker_issue_repeat_marker` | Repeat notes get sung; write the repeated lines out. | Wiederholungs-Notizen werden mitgesungen, schreibe die Zeilen lieber aus. |
| `desktop.noisemaker_issue_no_section_tags` | No section tags yet, e.g. [Verse] and [Chorus]. | Noch keine Abschnitts-Tags, z. B. [Verse] und [Chorus]. |
| `desktop.noisemaker_issue_bpm_in_style` | Put the tempo in the BPM field instead of the style. | Das Tempo gehört ins BPM-Feld statt in den Stil. |
| `desktop.noisemaker_ai_applied` | AI suggestion applied: {{fields}} | KI-Vorschlag übernommen: {{fields}} |
| `desktop.noisemaker_ai_undo` | Undo | Rückgängig |
| `desktop.noisemaker_ai_unreadable` | The AI answer could not be read. Please try again. | Die KI-Antwort war nicht lesbar. Bitte noch einmal versuchen. |
| `desktop.noisemaker_variation` | Suggest a variation | Variante vorschlagen |
| `desktop.noisemaker_variation_style` | New style | Neuer Stil |
| `desktop.noisemaker_variation_verses` | New verses | Neue Strophen |
| `desktop.noisemaker_variation_sequel` | Sequel | Fortsetzung |

- [ ] **Step 2:** Write `scratchpad\nm-add-keys.cjs` (same scratchpad folder) and run it with `node <scratchpad>\nm-add-keys.cjs ui/lang/desktop <scratchpad>\nm-keys.json`:

```js
// Inserts new keys after the last "desktop.noisemaker_" line of each locale file,
// keeps the file's line endings and existing escapes, skips keys that already exist,
// and validates the JSON afterwards.
const fs = require('fs');
const path = require('path');
const [dir, keysFile] = process.argv.slice(2);
const all = JSON.parse(fs.readFileSync(keysFile, 'utf8'));
const langs = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
for (const lang of langs) {
    const entries = all[lang];
    if (!entries) throw new Error('missing translations for ' + lang);
    const file = path.join(dir, lang + '.json');
    const text = fs.readFileSync(file, 'utf8');
    const eol = text.includes('\r\n') ? '\r\n' : '\n';
    const data = JSON.parse(text);
    const lines = text.split(eol);
    let idx = -1;
    lines.forEach((line, i) => { if (line.startsWith('  "desktop.noisemaker_')) idx = i; });
    if (idx < 0) throw new Error(lang + ': no noisemaker block');
    const add = Object.entries(entries).filter(([key]) => !(key in data)).map(([key, value]) => '  ' + JSON.stringify(key) + ': ' + JSON.stringify(value));
    if (!add.length) continue;
    const wasLast = !lines[idx].trimEnd().endsWith(',');
    if (wasLast) lines[idx] = lines[idx].replace(/\s*$/, '') + ',';
    const block = add.map((line, i) => (i < add.length - 1 || !wasLast) ? line + ',' : line);
    lines.splice(idx + 1, 0, ...block);
    const out = lines.join(eol);
    const parsed = JSON.parse(out);
    for (const key of Object.keys(entries)) if (!parsed[key]) throw new Error(lang + ': ' + key + ' missing after insert');
    fs.writeFileSync(file, out);
    console.log(lang + ': +' + add.length);
}
```

Expected output: one line per language with `+36`.

- [ ] **Step 3:** Run the locale tests:

Run: `go test ./ui -run '(?i)(DesktopUsedI18nKeysExistInAllLanguages|Noisemaker)' -count=1`
Expected: PASS (browser subtests skip without `AURAGO_RUN_BROWSER_SMOKE=1`).

---

### Task 9: Browser test (`ui/desktop_noisemaker_browser_test.go`)

- [ ] **Step 1:** In the fake page script of `noisemakerBrowserOrigin`:
- in `window.api`, before the `/enhance` line add: `if(path.endsWith('/lyrics/check'))return structuredClone(window.checkResponse||{issues:[]});`
- in `window.mount`, add `showContextMenu:(x,y,items)=>{window.lastMenu=items}` to the ctx object passed to `NoisemakerApp.render`.

- [ ] **Step 2:** Add this subtest inside `TestDesktopNoisemakerAuditBrowser` (after `enhancement_preserves_edits_and_newer_requests`):

```go
	t.Run("ai_helpers", func(t *testing.T) {
		page := pageFor(t)
		page.MustEval(`() => {caps.lyrics_guide={provider:'minimax',max_chars:3500};requests=[];enhancements=[];mount()}`)
		waitForJSBool(t, page, `() => !!document.querySelector('[data-nm-song]')`)
		page.MustEval(`() => {input('idea','Rain');document.querySelector('[data-nm-song]').click()}`)
		waitForJSBool(t, page, `() => enhancements.length === 1`)
		if body := page.MustEval(`() => JSON.parse(requests.filter(r=>r.path.endsWith('/enhance')).pop().body)`); body.Get("kind").Str() != "song" || body.Get("text").Str() != "Rain" {
			t.Fatalf("song request = %v", body)
		}
		page.MustEval(`() => enhancements[0]({status:'ok',song:{idea:'A rainy night',style:'lo-fi',title:'Rain',lyrics:'[Verse 1]\nla'}})`)
		waitForJSBool(t, page, `() => document.querySelector('[data-nm-field="lyrics"]')?.value === '[Verse 1]\nla' && !document.querySelector('[data-nm-undo]').hidden`)
		page.MustEval(`() => document.querySelector('[data-nm-undo-btn]').click()`)
		waitForJSBool(t, page, `() => document.querySelector('[data-nm-field="idea"]').value === 'Rain' && document.querySelector('[data-nm-field="lyrics"]').value === '' && document.querySelector('[data-nm-undo]').hidden`)

		page.MustEval(`() => {input('lyrics','[Verse 1]\nla');checkResponse={provider:'minimax',issues:[{code:'tag_numbered',severity:'warning',line:1,params:{tag:'[Verse 1]',suggestion:'[Verse]'}}]};document.querySelector('[data-nm-lyrics-menu]').click()}`)
		waitForJSBool(t, page, `() => Array.isArray(lastMenu) && lastMenu.some(i=>i.id==='lyrics-check')`)
		page.MustEval(`() => lastMenu.find(i=>i.id==='lyrics-check').action()`)
		waitForJSBool(t, page, `() => document.querySelector('[data-nm-check]')?.innerText.includes('[Verse 1]') && !!document.querySelector('[data-nm-check-fix]')`)
		page.MustEval(`() => document.querySelector('[data-nm-check-fix]').click()`)
		waitForJSBool(t, page, `() => enhancements.length === 2`)
		if body := page.MustEval(`() => JSON.parse(requests.filter(r=>r.path.endsWith('/enhance')).pop().body)`); body.Get("kind").Str() != "revise" || body.Get("action").Str() != "fix_tags" {
			t.Fatalf("fix request = %v", body)
		}
		page.MustEval(`() => enhancements[1]({status:'ok',text:'[Verse]\nla'})`)
		waitForJSBool(t, page, `() => document.querySelector('[data-nm-field="lyrics"]').value === '[Verse]\nla'`)

		page.MustEval(`() => {const card=document.querySelector('.nm-card, .nm-row');card.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,clientX:40,clientY:40}))}`)
		waitForJSBool(t, page, `() => Array.isArray(lastMenu) && lastMenu.some(i=>i.id==='variation')`)
		page.MustEval(`() => lastMenu.find(i=>i.id==='variation').items.find(i=>i.id==='variation-sequel').action()`)
		waitForJSBool(t, page, `() => enhancements.length === 3`)
		body := page.MustEval(`() => JSON.parse(requests.filter(r=>r.path.endsWith('/enhance')).pop().body)`)
		if body.Get("kind").Str() != "variation" || body.Get("mode").Str() != "sequel" || body.Get("base").Get("title").Str() == "" {
			t.Fatalf("variation request = %v", body)
		}
	})
```

If the library card ignores a synthetic `contextmenu` event, open the menu through the card's "more" button (`.nm-act-more`) instead; keep the assertions.

- [ ] **Step 3:** Run it:

Run: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopNoisemakerAuditBrowser' -count=1 -timeout 10m -v`
Expected: all subtests PASS, including `ai_helpers`. (Needs a local Chrome; if rod cannot launch Chrome from the repo build dir, report it instead of working around it.)

- [ ] **Step 4: Hand back** Tasks 5–9 paths (commit `feat(noisemaker): AI song suggestion, lyrics menu, tag check and variations in the app`).

---

### Task 10: Documentation

- [ ] `documentation/ace-step-local.md`: replace "Local mode never silently invokes the chat/cloud LLM to write lyrics." with "Local mode never silently invokes the chat/cloud LLM to write lyrics; Noisemaker's AI helper buttons use the chat LLM only when you click them."
- [ ] `documentation/manuals/en/04-webui.md` and `documentation/manuals/de/04-webui.md`, Noisemaker section: one short paragraph each on "Suggest a song", the lyrics AI menu (write/continue, catchier chorus, shorten, rhymes, bridge, own request), "Check tags" with "Fix with AI", the undo bar and "Suggest a variation" in the library context menu. Mention that the check knows the tags of the active provider (ACE-Step, MiniMax) and that MiniMax accepts at most 3500 characters.
- [ ] `documentation/manuals/en/21-api-reference.md` and `.../de/21-api-reference.md`, Noisemaker API block: add `POST   /api/desktop/noisemaker/lyrics/check` after the enhance line, and one sentence that `enhance` accepts `kind` = `idea | style | lyrics | title | revise | song | variation` (`revise` with `action` and optional `instruction`, `variation` with `mode` and `base`), and that `state` returns `lyrics_guide`.
- [ ] Hand back the five paths (commit `docs(noisemaker): AI helpers and lyrics check`).

---

### Task 11: Verification (controller)

- [ ] `go test ./internal/tools -run '(?i)(LyricsGuide|LintSong|GenerateMusic)' -count=1`
- [ ] `go test ./internal/server -run '(?i)Noisemaker' -count=1`
- [ ] `go test ./internal/agent -run '(?i)(Tool|Schema|Manual)' -count=1` and `go test ./internal/prompts -count=1`
- [ ] `go test ./ui -run '(?i)(I18n|Noisemaker)' -count=1`
- [ ] `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopNoisemakerAuditBrowser -count=1 -timeout 10m`
- [ ] One combined review of the whole change (Critical/Important only).
