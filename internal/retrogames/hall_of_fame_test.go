package retrogames

import (
	"strings"
	"testing"
)

func TestHallOfFameIntegrity(t *testing.T) {
	all := HallOfFame("")
	if len(all) < 40 {
		t.Fatalf("hall of fame has %d entries, want >= 40", len(all))
	}
	perPlatform := map[string]int{}
	seen := map[string]bool{}
	consoleNames := map[string]bool{}
	for _, name := range consoleSystems {
		consoleNames[name] = true
	}
	for _, g := range all {
		if seen[g.Identifier] {
			t.Fatalf("duplicate %s", g.Identifier)
		}
		seen[g.Identifier] = true
		perPlatform[g.Platform]++
		if g.Platform == PlatformConsole {
			if !consoleNames[g.System] {
				t.Errorf("%s: unknown console system %q", g.Identifier, g.System)
			}
		} else if g.System != SystemFor(g.Platform, "") {
			t.Errorf("%s: system %q, want %q", g.Identifier, g.System, SystemFor(g.Platform, ""))
		}
	}
	for _, src := range Sources() {
		if perPlatform[src.Key] < 4 {
			t.Errorf("platform %s has %d entries, want >= 4", src.Key, perPlatform[src.Key])
		}
	}
}

func TestHallOfFameFilterAndLookup(t *testing.T) {
	zx := HallOfFame(PlatformZX)
	if len(zx) == 0 {
		t.Fatal("no zx entries")
	}
	for _, g := range zx {
		if g.Platform != PlatformZX {
			t.Fatalf("filter leaked %+v", g)
		}
	}
	g, ok := hallOfFameGame("msdos_Prince_of_Persia_1990")
	if !ok || g.Title != "Prince of Persia" || g.Year != 1990 {
		t.Fatalf("lookup = %+v ok=%v", g, ok)
	}
	zx[0].Title = "mutated"
	if HallOfFame(PlatformZX)[0].Title == "mutated" {
		t.Fatal("HallOfFame exposes internal slice")
	}
}

func TestParseHallOfFameRejectsBadData(t *testing.T) {
	entry := func(id, platform, system string) string {
		return `{"identifier":"` + id + `","title":"T","platform":"` + platform + `","system":"` + system + `","year":1990,"why":"w"}`
	}
	cases := map[string]string{
		"duplicate":        "[" + entry("a", "msdos", "MS-DOS") + "," + entry("a", "msdos", "MS-DOS") + "]",
		"bad id":           "[" + entry("../x", "msdos", "MS-DOS") + "]",
		"unknown platform": "[" + entry("a", "nes", "NES") + "]",
		"empty system":     "[" + entry("a", "msdos", "") + "]",
		"unknown field":    `[{"identifier":"a","title":"T","platform":"msdos","system":"MS-DOS","year":1990,"why":"w","extra":1}]`,
		"bad year":         `[{"identifier":"a","title":"T","platform":"msdos","system":"MS-DOS","year":1800,"why":"w"}]`,
		"no why":           `[{"identifier":"a","title":"T","platform":"msdos","system":"MS-DOS","year":1990,"why":" "}]`,
	}
	for name, data := range cases {
		if _, err := parseHallOfFame([]byte(data)); err == nil {
			t.Errorf("%s: accepted", name)
		} else if !strings.Contains(err.Error(), "hall of fame") {
			t.Errorf("%s: error %v lacks context", name, err)
		}
	}
}
