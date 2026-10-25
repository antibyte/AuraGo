package agent

import "strings"

// The media generators (generate_image, generate_music, generate_video) are
// large schemas that most requests never need. They join the soft tools of the
// adaptive first selection only when the request asks for that kind of media,
// and only while the feature is on; otherwise they stay reachable through the
// ranking and discover_tools like any other tool.

// adaptiveGeneratorRule is one generator: its feature switch, the whole words
// and phrases that ask for it (English and German, folded spelling), and the
// German compounds that end in one of the heads.
type adaptiveGeneratorRule struct {
	tool    string
	enabled func(ToolFeatureFlags) bool
	terms   *toolIntentTerms
	// heads are German words that name the media ("bild", "video"); a word
	// that ends in one of them counts when the rest is a modifier from the
	// explicit list: Hintergrundbild, Titelbild, Musikvideo.
	heads     []string
	modifiers map[string]bool
}

var adaptiveGeneratorRules = []adaptiveGeneratorRule{
	{
		tool:    "generate_image",
		enabled: func(ff ToolFeatureFlags) bool { return ff.ImageGenerationEnabled },
		terms: newToolIntentTerms([]string{
			"image", "picture", "photo", "illustration", "drawing", "draw", "logo", "wallpaper", "artwork", "poster", "avatar", "sketch",
			"bild", "bilder", "foto", "grafik", "zeichnung", "zeichnen", "malen", "gemaelde", "kunstwerk",
		}, nil),
		heads:     []string{"bild", "bilder", "foto", "fotos"},
		modifiers: adaptiveGeneratorModifiers("hintergrund", "titel", "profil", "vorschau", "wand", "cover", "werbe", "produkt", "banner", "logo"),
	},
	{
		tool:    "generate_music",
		enabled: func(ff ToolFeatureFlags) bool { return ff.MusicGenerationEnabled },
		terms: newToolIntentTerms([]string{
			"music", "song", "melody", "soundtrack", "jingle", "beat", "instrumental",
			"lied", "lieder", "komponieren",
		}, []string{"musik", "melodie"}),
	},
	{
		tool:    "generate_video",
		enabled: func(ff ToolFeatureFlags) bool { return ff.VideoGenerationEnabled },
		terms: newToolIntentTerms([]string{
			"video", "clip", "movie", "film", "animation", "animated", "trailer",
			"kurzfilm",
		}, nil),
		heads:     []string{"video", "videos"},
		modifiers: adaptiveGeneratorModifiers("musik", "erklaer", "lehr", "werbe", "kurz", "produkt", "intro"),
	},
}

func adaptiveGeneratorModifiers(modifiers ...string) map[string]bool {
	set := make(map[string]bool, len(modifiers))
	for _, modifier := range modifiers {
		set[modifier] = true
	}
	return set
}

// adaptiveGeneratorToolsForQuery names the media generators a request asks
// for: the feature is on and the message holds one of its words.
func adaptiveGeneratorToolsForQuery(userQuery string, ff ToolFeatureFlags) []string {
	var out []string
	var text toolIntentText
	for _, rule := range adaptiveGeneratorRules {
		if !rule.enabled(ff) {
			continue
		}
		if len(text.words) == 0 {
			text = newToolIntentText(userQuery)
		}
		if rule.terms.matches(text) || rule.matchesCompound(text) {
			out = append(out, rule.tool)
		}
	}
	return out
}

// matchesCompound reports a word such as "hintergrundbild": a listed modifier
// in front of one of the heads.
func (rule adaptiveGeneratorRule) matchesCompound(text toolIntentText) bool {
	for _, word := range text.words {
		for _, head := range rule.heads {
			if rest, ok := strings.CutSuffix(word, head); ok && rule.modifiers[rest] {
				return true
			}
		}
	}
	return false
}
