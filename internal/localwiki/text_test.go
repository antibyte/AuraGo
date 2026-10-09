package localwiki

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/zim/xapian"
)

func TestFoldTextIgnoresCaseAndAccents(t *testing.T) {
	for in, want := range map[string]string{"Münster": "munster", "ÉCOLE": "ecole", "Ærø": "ærø", "東京": "東京", "हिंदी": "हद"} {
		if got := foldText(in); got != want {
			t.Fatalf("foldText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeQueryEnforcesLimits(t *testing.T) {
	got, err := normalizeQuery("  Brandenburger \t Tor ")
	if err != nil || got != "Brandenburger Tor" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := normalizeQuery(strings.Repeat("a ", 16)); err != nil {
		t.Fatalf("16 words rejected: %v", err)
	}
	if _, err := normalizeQuery(strings.Repeat("a ", 17)); !errors.Is(err, ErrQueryTooLong) {
		t.Fatalf("17 words: %v", err)
	}
	if _, err := normalizeQuery(strings.Repeat("ä", 201)); !errors.Is(err, ErrQueryTooLong) {
		t.Fatalf("201 runes: %v", err)
	}
	if _, err := normalizeQuery(""); !errors.Is(err, ErrQueryEmpty) {
		t.Fatalf("empty: %v", err)
	}
}

func TestTitleKeyMatchesNearExactTitles(t *testing.T) {
	for _, pair := range [][2]string{
		{"Berlin (Stadt)", "berlin"},
		{"Albert_Einstein", "albert einstein"},
		{"Café de Flore", "cafe de flore"},
		{"Rock 'n' Roll", "rock n roll"},
	} {
		if titleKey(pair[0]) != titleKey(pair[1]) {
			t.Fatalf("titleKey(%q) = %q, titleKey(%q) = %q", pair[0], titleKey(pair[0]), pair[1], titleKey(pair[1]))
		}
	}
	if titleKey("Berlin") == titleKey("Bern") {
		t.Fatal("different titles share a key")
	}
}

func TestPathCandidates(t *testing.T) {
	if got := pathCandidates(" albert  einstein "); !reflect.DeepEqual(got, []string{"albert_einstein", "Albert_einstein"}) {
		t.Fatalf("got %v", got)
	}
	if got := pathCandidates("Berlin"); !reflect.DeepEqual(got, []string{"Berlin"}) {
		t.Fatalf("got %v", got)
	}
	if got := pathCandidates("  "); got != nil {
		t.Fatalf("got %v", got)
	}
}

func TestContainsQueryWord(t *testing.T) {
	words := matchWords("Hauptstadt Ärzte a")
	if !reflect.DeepEqual(words, []string{"hauptstadt", "arzte"}) {
		t.Fatalf("matchWords = %v", words)
	}
	if !containsQueryWord("Berlin ist die Hauptstadt.", words) || !containsQueryWord("Die ÄRZTEKAMMER tagt.", words) {
		t.Fatal("expected word-prefix matches")
	}
	if containsQueryWord("Die Landeshauptstadt ist Potsdam.", words) {
		t.Fatal("matched inside a word")
	}
	if !containsQueryWord("東京は日本の首都です。", matchWords("首都")) {
		t.Fatal("CJK substring not matched")
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("kurz", 10); got != "kurz" {
		t.Fatalf("got %q", got)
	}
	got := truncateRunes("Berlin ist die Hauptstadt der Bundesrepublik Deutschland", 20)
	if got != "Berlin ist die…" || utf8.RuneCountInString(got) > 20 {
		t.Fatalf("got %q", got)
	}
	if got := truncateRunes(strings.Repeat("ü", 30), 10); got != strings.Repeat("ü", 9)+"…" {
		t.Fatalf("got %q", got)
	}
}

func TestFoldTextMatchesLibzimNormalisation(t *testing.T) {
	// Greek final sigma: libzim (ICU Lower) maps a word-final capital sigma
	// to ς, so folded query and indexed text must agree on it.
	if got := foldText("ΣΩΚΡΆΤΗΣ"); got != "σωκρατης" {
		t.Fatalf("foldText(Greek) = %q, want %q", got, "σωκρατης")
	}
	for _, s := range []string{"ΣΩΚΡΆΤΗΣ", "Σωκράτης", "Münster", "İstanbul", "ÉCOLE", "हिंदी", "東京", "Ærø", "a\xffb"} {
		if got, want := foldText(s), xapian.Normalize(s); got != want {
			t.Fatalf("foldText(%q) = %q, xapian.Normalize = %q", s, got, want)
		}
		if again := foldText(foldText(s)); again != foldText(s) {
			t.Fatalf("foldText is not idempotent for %q: %q then %q", s, foldText(s), again)
		}
	}
	if titleKey("ΣΩΚΡΆΤΗΣ") != titleKey("Σωκράτης") || titleKey("Σωκράτης (φιλόσοφος)") != titleKey("σωκρατης") {
		t.Fatal("Greek titles with final sigma do not share a key")
	}
	if !containsQueryWord("Ο ΣΩΚΡΆΤΗΣ γεννήθηκε στην Αθήνα.", matchWords("Σωκράτης")) {
		t.Fatal("Greek query word not found in upper-case text")
	}
}

func TestNormalizeQueryRejectsHugeInputBeforeSplitting(t *testing.T) {
	huge := strings.Repeat("wort ", 400<<10) // 2 MiB
	if _, err := normalizeQuery(huge); !errors.Is(err, ErrQueryTooLong) {
		t.Fatalf("2 MiB query: %v", err)
	}
	if allocs := testing.AllocsPerRun(3, func() { _, _ = normalizeQuery(huge) }); allocs != 0 {
		t.Fatalf("rejecting a huge query allocated %v times", allocs)
	}
	// 200 four-byte runes in 16 words are within the limits.
	words := make([]string, 16)
	for i := range words {
		words[i] = strings.Repeat("𝔘", 11)
	}
	words[0] += strings.Repeat("𝔘", 9)
	if got, err := normalizeQuery("  " + strings.Join(words, "  ") + "\n"); err != nil || len(got) < 700 {
		t.Fatalf("200-rune query of four-byte runes: %d bytes, %v", len(got), err)
	}
}

func TestTruncateRunesEdgeCases(t *testing.T) {
	for _, max := range []int{0, -1, -100} {
		if got := truncateRunes("irgendein Text", max); got != "" {
			t.Fatalf("truncateRunes(max=%d) = %q, want empty", max, got)
		}
	}
	if got := truncateRunes("irgendein Text", 1); got != "…" {
		t.Fatalf("max=1: %q", got)
	}
	mark := string(rune(0x301))
	vs16 := string(rune(0xFE0F))
	zwj := string(zeroWidthJoiner)
	for _, tc := range []struct {
		name, in string
		max      int
		want     string
	}{
		{"combining mark", "abcde" + mark + "fghijkl", 6, "abcd…"},
		{"variation selector", "ab" + "☎" + vs16 + "cdefgh", 4, "ab…"},
		{"zero width joiner", "xx" + "👨" + zwj + "👩" + "yyyyyyyy", 5, "xx…"},
		{"skin tone", "xx" + "👍" + string(rune(0x1F3FD)) + "yyyyyyyy", 4, "xx…"},
		{"flag pair kept", "ab" + "🇩🇪" + "🇫🇷" + "zzzz", 5, "ab🇩🇪…"},
		{"flag pair split", "ab" + "🇩🇪" + "🇫🇷" + "zzzz", 4, "ab…"},
		{"plain cut", "abcdefghij", 6, "abcde…"},
	} {
		got := truncateRunes(tc.in, tc.max)
		if got != tc.want || utf8.RuneCountInString(got) > tc.max {
			t.Fatalf("%s: truncateRunes(%q, %d) = %q, want %q", tc.name, tc.in, tc.max, got, tc.want)
		}
	}
}
