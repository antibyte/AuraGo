package agent

import (
	"slices"
	"testing"
)

func TestToolSearchTextNormalization(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Enzyklopädie", "enzyklopaedie"},
		{"Größe ÜBER Öl", "groesse ueber oel"},
		{"Café naïve", "cafe naive"},
		{"cpu_temp", "cpu_temp"},
	} {
		if got := foldToolSearchText(tc.in); got != tc.want {
			t.Errorf("fold(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	for _, tc := range []struct{ word, want string }{
		{"notizen", "notiz"}, {"notiz", "notiz"}, {"notes", "not"}, {"note", "not"},
		{"scanning", "scan"}, {"stopped", "stop"}, {"pulling", "pull"}, {"settings", "set"},
		{"directories", "directory"}, {"processes", "process"}, {"class", "class"}, {"status", "status"},
		{"analysis", "analysis"}, {"uses", "use"}, {"thing", "thing"}, {"docs", "doc"},
		{"dateien", "datei"}, {"verwaltung", "verwalt"}, {"einstellungen", "einstell"},
		{"aendern", "aender"}, {"speichern", "speicher"}, {"wandeln", "wandel"},
		{"spiele", "spiel"}, {"lights", "light"}, {"api", "api"}, {"nutzer", "nutzer"},
	} {
		if got := stemToolSearchWord(tc.word); got != tc.want {
			t.Errorf("stem(%q) = %q, want %q", tc.word, got, tc.want)
		}
	}

	for _, tc := range []struct {
		text  string
		index bool
		want  []string
	}{
		{"Bitte zeige mir die Notizen", false, []string{"zeig", "notiz"}},
		{"Please save the notes for me", false, []string{"sav", "not"}},
		{"Retro-Spiele", true, []string{"retro", "spiel", "retrospiel"}},
		{"Retro-Spiele", false, []string{"retro", "spiel"}},
		{"disk_io, a b3 DNS-Lookup", false, []string{"disk", "io", "b3", "dns", "lookup"}},
		{"die und der", false, nil},
	} {
		if got := toolSearchTermsOf(tc.text, tc.index); !slices.Equal(got, tc.want) {
			t.Errorf("toolSearchTermsOf(%q, %v) = %q, want %q", tc.text, tc.index, got, tc.want)
		}
	}
}
