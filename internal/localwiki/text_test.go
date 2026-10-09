package localwiki

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
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
