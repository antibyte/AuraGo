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
		{"Bad\x00Title Line", "Bad Title Line", 0},
		{"", "", 0},
	}
	for _, tc := range cases {
		title, year := CleanTitle(tc.raw)
		if title != tc.title || year != tc.year {
			t.Errorf("CleanTitle(%q) = %q, %d; want %q, %d", tc.raw, title, year, tc.title, tc.year)
		}
	}
}

func TestParseYear(t *testing.T) {
	cases := map[string]int{"1990": 1990, "1990-05-01": 1990, "c. 1987": 1987, "19xx": 0, "12345": 0, "2031": 0, "1949": 0, "": 0, "2030": 2030}
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
}
