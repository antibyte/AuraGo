package retrogames

import "testing"

func TestCleanTitle(t *testing.T) {
	cases := []struct {
		raw   string
		title string
		year  int
	}{
		{"Oregon Trail, The", "The Oregon Trail", 0},
		{"Secret of Monkey Island, The ", "The Secret of Monkey Island", 0},
		{"Internet Arcade: Centipede", "Centipede", 0},
		{"Atari 2600: Pitfall II - Lost Caverns", "Pitfall II - Lost Caverns", 0},
		{"Commodore 64: Impossible Mission (Clean)", "Impossible Mission", 0},
		{" Turrican II   The Final Fight (1991)(Rainbow Arts)[cr Defjam][h Apex][t +6 Apex]", "Turrican II The Final Fight", 1991},
		{" Speedball 2   Brutal Deluxe v1.00 (1991 01 07)(Image Works)[cr CLS   RZR]", "Speedball 2 Brutal Deluxe v1.00", 1991},
		{" Great Giana Sisters, The (1987 Go!) [5569]", "The Great Giana Sisters", 1987},
		{"Last Ninja 2 Ripp (19xx)(Black Mail)", "Last Ninja 2 Ripp", 0},
		{"Jetpac [16K]", "Jetpac", 0},
		{"1942 (Revision B)", "1942 (Revision B)", 0},
		{"Wolfenstein 3D (1992) MS-DOS", "Wolfenstein 3D", 1992},
		{"[Hack] Foo", "[Hack] Foo", 0},
		{"Bad\x00Title\U00002028Line", "Bad Title Line", 0},
		{"", "", 0},
	}
	for _, tc := range cases {
		title, year := CleanTitle(tc.raw)
		if title != tc.title || year != tc.year {
			t.Errorf("CleanTitle(%q) = %q, %d; want %q, %d", tc.raw, title, year, tc.title, tc.year)
		}
	}
}

func TestCleanTitleStripsInvisibleAndBidiCharacters(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"bidi override and isolate", "\U0000202Eevil\U00002066", "evil"},
		{"isolates", "a\U00002067b\U00002068c\U00002069d", "abcd"},
		{"tag characters", "A\U000E0041B\U000E007F", "AB"},
		{"zero width space", "Foo\U0000200BBar", "FooBar"},
		{"byte order mark", "\U0000FEFFFoo", "Foo"},
		{"zero width joiner kept", "A\U0000200DB", "A\U0000200DB"},
		{"zero width non-joiner kept", "A\U0000200CB", "A\U0000200CB"},
		{"nbsp run collapses", "Foo\U000000A0\U000000A0\U000000A0Bar", "Foo Bar"},
		{"mixed unicode spaces collapse", "Foo\U00002003\U000000A0\U00003000Bar", "Foo Bar"},
		{"line separators", "Foo\U00002028\U00002029Bar", "Foo Bar"},
	}
	for _, tc := range cases {
		if got, _ := CleanTitle(tc.raw); got != tc.want {
			t.Errorf("%s: CleanTitle(%q) = %q, want %q", tc.name, tc.raw, got, tc.want)
		}
	}
}

func TestParseYear(t *testing.T) {
	cases := map[string]int{"1990": 1990, "1990-05-01": 1990, "c. 1987": 1987, "19xx": 0, "12345": 0, "2031": 0, "1949": 0, "": 0, "2030": 2030, "2050": 0, "2050 (1993)": 1993}
	for in, want := range cases {
		if got := ParseYear(in); got != want {
			t.Errorf("ParseYear(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("äöüß", 2); got != "äö" {
		t.Fatalf("truncateRunes = %q", got)
	}
	if got := truncateRunes("abc", 5); got != "abc" {
		t.Fatalf("truncateRunes = %q", got)
	}
	if got := truncateRunes("abc", 0); got != "" {
		t.Fatalf("truncateRunes(limit 0) = %q", got)
	}
	if got := truncateRunes("abc", -1); got != "" {
		t.Fatalf("truncateRunes(limit -1) = %q", got)
	}
}
