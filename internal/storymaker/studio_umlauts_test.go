package storymaker

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode"
)

func TestRepairUmlautsRestoresWholeWordsAndKeepsTheCase(t *testing.T) {
	text := "Eine {1}Firewall pruft jeden Strom. Pruft sie? PRUFT! Sie durfen wahrend der Zeit personlichen Schutz."
	got, fixes := RepairUmlauts(text, "de")
	want := "Eine {1}Firewall prüft jeden Strom. Prüft sie? PRÜFT! Sie dürfen während der Zeit persönlichen Schutz."
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	counts := map[string]int{}
	for _, f := range fixes {
		counts[f.From+">"+f.To] += f.Count
	}
	if counts["pruft>prüft"] != 1 || counts["Pruft>Prüft"] != 1 || counts["PRUFT>PRÜFT"] != 1 || counts["durfen>dürfen"] != 1 || counts["wahrend>während"] != 1 || counts["personlichen>persönlichen"] != 1 {
		t.Fatalf("fixes = %+v", fixes)
	}
}

func TestRepairUmlautsTouchesOnlyWholeWordsOfGerman(t *testing.T) {
	for _, tc := range []struct{ text, language string }{
		{"Eine Firewall prueft und schuetzt das Netz.", "de"},       // the ue spelling is a spelling, not a loss
		{"Unpruft und Pruftest sind keine Wörter der Liste.", "de"}, // whole words only
		{"Die Firewall pruft jedes Paket.", "en"},                   // another language is never touched
		{"Die Firewall pruft jedes Paket.", ""},                     // no language, no repair
		{"Furcht und Uberfluss bleiben.", "de"},                     // fur and uber inside longer words
		{"Das war schon, wurde aber fuhren und losen.", "de"},       // real German words stay
	} {
		if got, fixes := RepairUmlauts(tc.text, tc.language); got != tc.text || len(fixes) != 0 {
			t.Errorf("RepairUmlauts(%q, %q) = %q, %v: must be unchanged", tc.text, tc.language, got, fixes)
		}
	}
	if got, _ := RepairUmlauts("Er pruft das.", "de-AT"); got != "Er prüft das." {
		t.Errorf("a regional tag has the base language de, got %q", got)
	}
	if got, fixes := RepairUmlauts("", "de"); got != "" || fixes != nil {
		t.Errorf("empty text = %q, %v", got, fixes)
	}
}

// Every entry of the map is checked: a lower-case ASCII key, a lower-case value that differs from it only by umlauts, no
// real German word as key.
func TestUmlautMapEntriesAreSound(t *testing.T) {
	var entries map[string]string
	if err := json.Unmarshal(deUmlautsJSON, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) < 30 {
		t.Fatalf("the map has only %d entries", len(entries))
	}
	replacer := strings.NewReplacer("ä", "a", "ö", "o", "ü", "u")
	// ASCII spellings that are German words of their own: they must never be keys.
	realWords := []string{"wurde", "wurden", "fuhren", "losen", "losung", "losungen", "wahlen", "andern", "hohe", "schon"}
	for key, value := range entries {
		if key != strings.ToLower(key) || value != strings.ToLower(value) {
			t.Errorf("%q: keys and values are lower case", key)
		}
		for _, r := range key {
			if r > unicode.MaxASCII || !unicode.IsLetter(r) {
				t.Errorf("%q: a key is plain ASCII letters", key)
				break
			}
		}
		if replacer.Replace(value) != key {
			t.Errorf("%q -> %q: the value must differ from the key only by umlauts", key, value)
		}
		if !strings.ContainsAny(value, "äöü") {
			t.Errorf("%q -> %q: the value holds no umlaut", key, value)
		}
	}
	for _, word := range realWords {
		if _, bad := entries[word]; bad {
			t.Errorf("%q is a real German word and must not be a key", word)
		}
	}
	for _, key := range []string{"pruft", "durfen", "wahrend", "personlichen", "stromen"} {
		if entries[key] == "" {
			t.Errorf("the words of the real run need an entry: %q", key)
		}
	}
}

// The real run of 2026-11-08 had five such words in its narration. The repair fixes them and leaves the rest alone.
func TestRepairUmlautsOnTheRealFirewallRun(t *testing.T) {
	raw, err := os.ReadFile("testdata/stories/agent-firewall-run.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Scenes []struct{ ID, Narration string }
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, sc := range doc.Scenes {
		fixed, fixes := RepairUmlauts(sc.Narration, "de")
		for _, f := range fixes {
			total += f.Count
		}
		if strings.Contains(fixed, "pruft") || strings.Contains(fixed, "durfen") || strings.Contains(fixed, "wahrend") {
			t.Errorf("scene %s still has a missing umlaut: %q", sc.ID, fixed)
		}
	}
	if total < 5 {
		t.Errorf("the run has at least 5 missing umlauts, repaired %d", total)
	}
}
