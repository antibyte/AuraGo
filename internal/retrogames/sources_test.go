package retrogames

import (
	"strings"
	"testing"
)

func TestSourcesCoverEightPlatformsInOrder(t *testing.T) {
	want := []string{PlatformMSDOS, PlatformArcade, PlatformConsole, PlatformC64, PlatformAmiga, PlatformApple2, PlatformZX, PlatformWin3}
	got := Sources()
	if len(got) != len(want) {
		t.Fatalf("len(Sources()) = %d, want %d", len(got), len(want))
	}
	for i, src := range got {
		if src.Key != want[i] {
			t.Fatalf("source %d = %q, want %q", i, src.Key, want[i])
		}
		if src.Label == "" || len(src.Collections) == 0 {
			t.Fatalf("source %q is incomplete: %+v", src.Key, src)
		}
	}
}

func TestSourceQueryCombinesCollections(t *testing.T) {
	src, ok := SourceByKey(PlatformAmiga)
	if !ok {
		t.Fatal("amiga source missing")
	}
	want := "collection:(softwarelibrary_amiga_games OR softwarelibrary_amiga_showcase) AND mediatype:software AND emulator:*"
	if got := src.Query(); got != want {
		t.Fatalf("Query() = %q, want %q", got, want)
	}
	win, _ := SourceByKey(PlatformWin3)
	if got := win.Query(); got != "collection:(softwarelibrary_win3_games) AND mediatype:software AND emulator:*" {
		t.Fatalf("win3 Query() = %q", got)
	}
}

func TestSourcesReturnsCopies(t *testing.T) {
	first := Sources()
	first[0].Collections[0] = "mutated"
	if Sources()[0].Collections[0] == "mutated" {
		t.Fatal("Sources() exposes the internal table")
	}
}

func TestSourceByKeyReturnsCopies(t *testing.T) {
	src, ok := SourceByKey(PlatformAmiga)
	if !ok {
		t.Fatal("amiga source missing")
	}
	want := src.Collections[0]
	src.Collections[0] = "mutated"
	again, ok := SourceByKey(PlatformAmiga)
	if !ok {
		t.Fatal("amiga source missing on second lookup")
	}
	if again.Collections[0] != want {
		t.Fatalf("SourceByKey exposes the internal table: Collections[0] = %q, want %q", again.Collections[0], want)
	}
	if _, ok := SourceByKey("nope"); ok {
		t.Fatal("SourceByKey(\"nope\") ok = true, want false")
	}
}

func TestSystemFor(t *testing.T) {
	cases := []struct{ platform, emulator, want string }{
		{PlatformConsole, "genesis", "Mega Drive / Genesis"},
		{PlatformConsole, " MEGADRIV ", "Mega Drive / Genesis"},
		{PlatformConsole, "a2600", "Atari 2600"},
		{PlatformConsole, "snes", "SNES"},
		{PlatformConsole, "dosbox", SystemOtherConsole},
		{PlatformMSDOS, "dosbox", "MS-DOS"},
		{PlatformZX, "spectrum", "ZX Spectrum"},
		{"bogus", "dosbox", ""},
	}
	for _, tc := range cases {
		if got := SystemFor(tc.platform, tc.emulator); got != tc.want {
			t.Errorf("SystemFor(%q, %q) = %q, want %q", tc.platform, tc.emulator, got, tc.want)
		}
	}
}

func TestValidIdentifier(t *testing.T) {
	valid := []string{"msdos_Prince_of_Persia_1990", "Gunstar_Heroes_USA.md", "chips_challenge_windows_3.x", "Speedball_2_Brutal_Deluxe_v1.00_1991-01-07_Image_Works_cr_CLS_RZR", "a", strings.Repeat("a", 100)}
	for _, id := range valid {
		if !ValidIdentifier(id) {
			t.Errorf("ValidIdentifier(%q) = false", id)
		}
	}
	invalid := []string{"", "..", ".hidden", "-x", "a/b", "a b", "a?b", "ä", strings.Repeat("a", 101), "a\n", "a%2e", "a\\b", "a\x00"}
	for _, id := range invalid {
		if ValidIdentifier(id) {
			t.Errorf("ValidIdentifier(%q) = true", id)
		}
	}
}

func TestValidPlatform(t *testing.T) {
	if !ValidPlatform(PlatformC64) || ValidPlatform("") || ValidPlatform("nes") {
		t.Fatal("ValidPlatform mismatch")
	}
}
