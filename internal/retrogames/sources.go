// Package retrogames keeps a local index of the browser-playable games on
// archive.org, the user's favorites and play history, and a curated Hall of
// Fame for the Retro Games desktop app and the retro_games agent tool.
package retrogames

import (
	"regexp"
	"strings"
)

// Platform keys. Every source feeds exactly one platform, so they double as source keys.
const (
	PlatformMSDOS   = "msdos"
	PlatformArcade  = "arcade"
	PlatformConsole = "console"
	PlatformC64     = "c64"
	PlatformAmiga   = "amiga"
	PlatformApple2  = "apple2"
	PlatformZX      = "zx"
	PlatformWin3    = "win3"
)

// SystemOtherConsole labels console items whose emulator has no mapping.
const SystemOtherConsole = "Other console"

// Source is one archive.org query that feeds one platform.
type Source struct {
	Key         string
	Label       string
	Collections []string
}

var sources = []Source{
	{Key: PlatformMSDOS, Label: "MS-DOS", Collections: []string{"softwarelibrary_msdos_games", "softwarelibrary_msdos_showcase"}},
	{Key: PlatformArcade, Label: "Arcade", Collections: []string{"internetarcade"}},
	{Key: PlatformConsole, Label: "Console", Collections: []string{"consolelivingroom"}},
	{Key: PlatformC64, Label: "Commodore 64", Collections: []string{"softwarelibrary_c64_games", "softwarelibrary_c64_showcase"}},
	{Key: PlatformAmiga, Label: "Amiga", Collections: []string{"softwarelibrary_amiga_games", "softwarelibrary_amiga_showcase"}},
	{Key: PlatformApple2, Label: "Apple II", Collections: []string{"softwarelibrary_apple_games"}},
	{Key: PlatformZX, Label: "ZX Spectrum", Collections: []string{"zx_spectrum_library_games"}},
	{Key: PlatformWin3, Label: "Windows 3.x", Collections: []string{"softwarelibrary_win3_games"}},
}

// copySource returns s with its own Collections slice, so callers cannot
// mutate the internal table.
func copySource(s Source) Source {
	return Source{Key: s.Key, Label: s.Label, Collections: append([]string(nil), s.Collections...)}
}

// Sources returns a copy of the source table in display order.
func Sources() []Source {
	out := make([]Source, len(sources))
	for i, src := range sources {
		out[i] = copySource(src)
	}
	return out
}

// SourceByKey looks up a source by its platform key. Only the match is copied.
func SourceByKey(key string) (Source, bool) {
	for _, src := range sources {
		if src.Key == key {
			return copySource(src), true
		}
	}
	return Source{}, false
}

// ValidPlatform reports whether key names one of the eight platforms.
func ValidPlatform(key string) bool {
	_, ok := SourceByKey(key)
	return ok
}

// Query is the archive.org search query for the source.
func (s Source) Query() string {
	return "collection:(" + strings.Join(s.Collections, " OR ") + ") AND mediatype:software AND emulator:*"
}

var consoleSystems = map[string]string{
	"genesis":  "Mega Drive / Genesis",
	"megadriv": "Mega Drive / Genesis",
	"megadrij": "Mega Drive / Genesis",
	"a2600":    "Atari 2600",
	"a7800":    "Atari 7800",
	"a5200":    "Atari 5200",
	"tg16":     "TurboGrafx-16",
	"coleco":   "ColecoVision",
	"odyssey2": "Odyssey²",
	"sg1000":   "SG-1000",
	"gameboy":  "Game Boy",
	"gbcolor":  "Game Boy Color",
	"gba":      "Game Boy Advance",
	"lynx":     "Atari Lynx",
	"channelf": "Channel F",
	"nes":      "NES",
	"scv":      "Super Cassette Vision",
	"bally":    "Bally Astrocade",
	"svision":  "Supervision",
	"crvision": "CreatiVision",
	"snes":     "SNES",
	"sms":      "Master System",
	"gamegear": "Game Gear",
	"psx":      "PlayStation",
}

// SystemFor derives the display system of a game. Non-console platforms use
// their platform label; console items are mapped by emulator.
func SystemFor(platform, emulator string) string {
	if platform == PlatformConsole {
		if name, ok := consoleSystems[strings.ToLower(strings.TrimSpace(emulator))]; ok {
			return name
		}
		return SystemOtherConsole
	}
	if src, ok := SourceByKey(platform); ok {
		return src.Label
	}
	return ""
}

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// ValidIdentifier reports whether id is a safe archive.org item identifier.
func ValidIdentifier(id string) bool {
	return identifierPattern.MatchString(id)
}
