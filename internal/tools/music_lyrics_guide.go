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

	// lyricsTagSynonyms lists spellings of one section so a tag can be suggested
	// in the active provider's spelling. Members are lyricsTagKey values.
	lyricsTagSynonyms = [][]string{
		{"chorus", "refrain"},
		{"verse", "strophe"},
		{"pre chorus", "prechorus"},
		{"post chorus", "postchorus"},
		{"instrumental", "inst", "instrumental break"},
		{"build up", "buildup"},
		{"fade out", "fadeout"},
	}

	// lyricsTagBroader maps a specific tag to a broader one. Only providers with
	// a closed tag list (not FreeformTags) use it, because it changes the music.
	lyricsTagBroader = map[string]string{
		"guitar solo":     "solo",
		"piano interlude": "interlude",
		"breakdown":       "break",
		"build":           "build up",
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

const lyricsHeadingTrim = "#*_(): \t"

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
		repeat := lyricsRepeatRe.MatchString(text)
		if repeat {
			issues = append(issues, LyricsIssue{Code: "repeat_marker", Severity: "warning", Line: line})
		}
		matches := lyricsInlineTagRe.FindAllStringSubmatch(text, -1)
		if len(matches) == 0 {
			if repeat {
				continue
			}
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
	candidates := lyricsTagSynonymGroup(key)
	if !g.FreeformTags {
		if broader := lyricsBroaderKey(key); broader != "" {
			candidates = append(candidates, lyricsTagSynonymGroup(broader)...)
		}
	}
	for _, tag := range g.SectionTags {
		tagKey := lyricsTagKey(tag)
		for _, member := range candidates {
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

// lyricsTagSynonymGroup returns the synonym group of key, or just key.
func lyricsTagSynonymGroup(key string) []string {
	for _, group := range lyricsTagSynonyms {
		for _, member := range group {
			if member == key {
				return append([]string(nil), group...)
			}
		}
	}
	return []string{key}
}

// lyricsBroaderKey returns the broader tag key for a specific one, or "".
func lyricsBroaderKey(key string) string {
	if broader, ok := lyricsTagBroader[key]; ok {
		return broader
	}
	if strings.HasSuffix(key, " solo") {
		return "solo"
	}
	return ""
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
	if !lyricsPlainHeadingWords[key] && !strings.ContainsAny(text, ":#*_") {
		return "", "", false
	}
	return base, number, true
}

func lyricsSectionWord(key string) bool {
	if lyricsPlainHeadingWords[key] || lyricsBroaderKey(key) != "" {
		return true
	}
	for _, group := range lyricsTagSynonyms {
		for _, member := range group {
			if member == key {
				return true
			}
		}
	}
	for broader := range lyricsTagBroader {
		if lyricsBroaderKey(broader) == key {
			return true
		}
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
