package agent

import "strings"

// The media generators (generate_image, generate_music, generate_video) are
// large schemas that most requests never need. They join the soft tools of the
// adaptive first selection only when the request asks for that kind of media,
// and only while the feature is on; otherwise they compete for a slot like any
// other tool and stay reachable through discover_tools.
//
// A request asks for media with a word that names it (Bild, photo, Musik,
// song, Video, ...). Some words also have an everyday or technical meaning:
// "image" is a docker or ISO image, "draw conclusions", "he lied", "beat the
// timeout", an "animation" in CSS, a "clip" on a desk. Those count only
// together with a creation verb (generate, create, make, erstelle, mach, ...),
// and "image" never after a technical word ("docker image", "ISO image").

// adaptiveGeneratorRule is one generator.
type adaptiveGeneratorRule struct {
	tool    string
	enabled func(ToolFeatureFlags) bool
	// terms are whole words and phrases that name the media; heads are German
	// nouns that also match inside compounds (Musikvideo, Hintergrundmusik).
	terms *toolIntentTerms
	// ambiguous words ask for the media only with a creation verb of cues, and
	// not directly after a word of qualifiers.
	ambiguous  *toolIntentTerms
	cues       *toolIntentTerms
	qualifiers map[string]bool
	// compoundHeads are German words that name the media ("bild", "video"); a
	// word that ends in one of them counts when the rest is a modifier from
	// the explicit list: Hintergrundbild, Titelbild, Musikvideo.
	compoundHeads []string
	modifiers     map[string]bool
}

// adaptiveCreationCues are the creation verbs that make an ambiguous media word
// a request. "compose" is no shared cue: it is a docker command.
var adaptiveCreationCues = []string{
	"generate", "create", "make", "made", "design", "produce", "render", "paint", "illustrate",
	"erstellen", "erzeugen", "generieren", "mach", "mache", "machen", "gestalten", "entwerfen", "entwirf",
	"zeichnen", "produzieren", "kreieren",
}

func adaptiveCues(extra ...string) *toolIntentTerms {
	return newToolIntentTerms(append(append([]string(nil), adaptiveCreationCues...), extra...), nil)
}

func adaptiveWordSet(words ...string) map[string]bool {
	set := make(map[string]bool, len(words))
	for _, word := range words {
		set[word] = true
	}
	return set
}

var adaptiveGeneratorRules = []adaptiveGeneratorRule{
	{
		tool:    "generate_image",
		enabled: func(ff ToolFeatureFlags) bool { return ff.ImageGenerationEnabled },
		terms: newToolIntentTerms([]string{
			"picture", "photo", "illustration", "logo", "wallpaper", "artwork",
			"bild", "bilder", "foto", "grafik", "gemaelde", "kunstwerk",
			// "zeichnen" also covers Zeichnung and "zeichne"; "malen" and "male mir"
			// are exact, the English "male" shares their stem.
			"zeichnen", "=malen", "=male mir", "=male ein", "=male eine", "=male einen",
		}, nil),
		ambiguous: newToolIntentTerms([]string{"image", "draw", "drawing", "sketch", "avatar", "poster"}, nil),
		cues:      adaptiveCues(),
		qualifiers: adaptiveWordSet(
			"docker", "container", "containers", "iso", "vm", "virtual", "disk", "os", "system", "backup", "boot", "bootable",
			"base", "cloud", "golden", "kernel", "firmware", "linux", "ubuntu", "debian", "windows", "vagrant", "oci", "raw",
			"qcow2", "installation", "install", "rootfs",
		),
		compoundHeads: []string{"bild", "bilder", "foto", "fotos"},
		modifiers:     adaptiveWordSet("hintergrund", "titel", "profil", "vorschau", "wand", "cover", "werbe", "produkt", "banner", "logo"),
	},
	{
		tool:    "generate_music",
		enabled: func(ff ToolFeatureFlags) bool { return ff.MusicGenerationEnabled },
		terms: newToolIntentTerms([]string{
			"music", "song", "melody", "soundtrack", "jingle", "instrumental",
			"musik", "musikstueck", "lieder", "komponieren",
			// Hintergrundmusik, Filmmusik: "musik" ends the word, but Musiker and
			// musikalisch only start with it.
		}, []string{"$musik", "melodie"}),
		ambiguous: newToolIntentTerms([]string{"lied", "beat"}, nil),
		cues:      adaptiveCues("compose", "write", "schreiben"),
	},
	{
		tool:    "generate_video",
		enabled: func(ff ToolFeatureFlags) bool { return ff.VideoGenerationEnabled },
		terms:   newToolIntentTerms([]string{"video", "movie", "trailer", "kurzfilm"}, nil),
		ambiguous: newToolIntentTerms([]string{
			"clip", "film", "animation", "animated",
		}, nil),
		cues:          adaptiveCues(),
		qualifiers:    adaptiveWordSet("css", "ui", "svg", "html", "js", "javascript", "loading", "hover", "button"),
		compoundHeads: []string{"video", "videos"},
		modifiers:     adaptiveWordSet("musik", "erklaer", "lehr", "werbe", "kurz", "produkt", "intro"),
	},
}

// adaptiveGeneratorToolsForQuery names the media generators a request asks
// for: the feature is on and the message asks for that media.
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
		if rule.asks(text) {
			out = append(out, rule.tool)
		}
	}
	return out
}

// asks reports whether the message asks for the rule's media.
func (rule adaptiveGeneratorRule) asks(text toolIntentText) bool {
	if rule.terms.matches(text) || rule.matchesCompound(text) {
		return true
	}
	return rule.cues.matches(text) && rule.ambiguous.matchesUnless(text, func(previous string) bool { return rule.qualifiers[previous] })
}

// matchesCompound reports a word such as "hintergrundbild": a listed modifier
// in front of one of the heads.
func (rule adaptiveGeneratorRule) matchesCompound(text toolIntentText) bool {
	for _, word := range text.words {
		for _, head := range rule.compoundHeads {
			if rest, ok := strings.CutSuffix(word, head); ok && rule.modifiers[rest] {
				return true
			}
		}
	}
	return false
}
