package agent

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

func searchNames(catalog *ToolCatalog, query string) []string {
	var names []string
	for _, entry := range catalog.Search(query) {
		names = append(names, entry.Name)
	}
	return names
}

func testSearchEntry(name, description string, status ToolStatus) *ToolCatalogEntry {
	entry := &ToolCatalogEntry{Name: name, Kind: ToolKindNative, Description: description, Status: status, Enabled: true}
	switch status {
	case ToolStatusActive:
		entry.Active = true
	case ToolStatusDisabled:
		entry.Enabled = false
	}
	return entry
}

func TestToolSearchOrdersByStatusClass(t *testing.T) {
	catalog := newToolCatalog(nil, nil, "", false)
	for _, entry := range []*ToolCatalogEntry{
		testSearchEntry("a_disabled", "Frobnicate widgets", ToolStatusDisabled),
		testSearchEntry("b_setup", "Frobnicate widgets", ToolStatusNeedsSetup),
		testSearchEntry("d_active", "Frobnicate widgets", ToolStatusActive),
		testSearchEntry("c_hidden", "Frobnicate widgets", ToolStatusHidden),
		testSearchEntry("e_exact", "Something else entirely", ToolStatusHidden),
	} {
		catalog.add(entry)
	}
	// Active and hidden share a class, so the order survives an adaptive-filter pass.
	if got, want := searchNames(catalog, "frobnicate widgets"), []string{"c_hidden", "d_active", "b_setup", "a_disabled"}; !slices.Equal(got, want) {
		t.Fatalf("lexical order = %v, want %v", got, want)
	}
	// An exact name wins whatever its status.
	if got := searchNames(catalog, "a_disabled"); len(got) != 1 || got[0] != "a_disabled" {
		t.Fatalf("exact name = %v, want [a_disabled]", got)
	}
}

func TestToolSearchIdentifierGroups(t *testing.T) {
	catalog := newToolCatalog(nil, nil, "", false)
	for _, entry := range []*ToolCatalogEntry{
		testSearchEntry("yepapi_instagram", "Instagram data with the capability to look up users", ToolStatusHidden),
		testSearchEntry("yepapi_youtube", "YouTube data with the capability to look up videos", ToolStatusHidden),
		testSearchEntry("monitor_cpu_usage", "Watch processor load", ToolStatusHidden),
		testSearchEntry("monitor_site", "Watch a website for changes", ToolStatusHidden),
		testSearchEntry("cpu_load", "Read the processor usage", ToolStatusHidden),
	} {
		catalog.add(entry)
	}
	for _, tc := range []struct {
		query string
		want  []string
	}{
		// All parts must match: yepapi_youtube shares only "yepapi".
		{"yepapi_instagram", []string{"yepapi_instagram"}},
		// Two of three parts are enough; one is not.
		{"monitor_cpu_usage", []string{"monitor_cpu_usage", "cpu_load"}},
		// One part out of four is no identifier match.
		{"zzzz_nonexistent_capability_987654321", nil},
		// Plain words still match by any of them.
		{"capability users", []string{"yepapi_instagram", "yepapi_youtube"}},
	} {
		if got := searchNames(catalog, tc.query); !slices.Equal(got, tc.want) {
			t.Errorf("Search(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestToolSearchIndexFollowsAddedEntries(t *testing.T) {
	catalog := newToolCatalog(nil, nil, "", false)
	catalog.add(testSearchEntry("first_tool", "Handle quarks", ToolStatusHidden))
	if got := searchNames(catalog, "quarks"); !slices.Equal(got, []string{"first_tool"}) {
		t.Fatalf("before add = %v", got)
	}
	catalog.add(testSearchEntry("second_tool", "Also handle quarks", ToolStatusHidden))
	if got := searchNames(catalog, "quarks"); !slices.Equal(got, []string{"first_tool", "second_tool"}) {
		t.Fatalf("after add = %v, want the new entry in the index", got)
	}
}

func TestToolSearchConcurrentFirstUse(t *testing.T) {
	catalog := BuildToolCatalog(BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil), nil, "")
	var wg sync.WaitGroup
	results := make([]string, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = fmt.Sprint(searchNames(catalog, "send an email"))
		}()
	}
	wg.Wait()
	for _, got := range results {
		if got != results[0] {
			t.Fatalf("concurrent searches disagree: %q vs %q", got, results[0])
		}
	}
}

// TestToolSearchLongGermanSentence guards the sentence-shaped queries a model
// copies from the user: a few stopword-heavy German lines must find the tool
// among the first five without flooding the page.
func TestToolSearchLongGermanSentence(t *testing.T) {
	catalog := BuildToolCatalog(BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil), nil, "")
	for _, tc := range []struct{ query, want string }{
		{"Bitte lege für mich morgen früh um neun Uhr einen neuen Termin im Kalender an und erinnere mich rechtzeitig daran", "manage_appointments"},
		{"Kannst du mir bitte sagen, wie hoch die Temperatur des Prozessors gerade ist und ob alles im grünen Bereich liegt?", "system_metrics"},
		{"Schalte bitte das Licht im Wohnzimmer aus und mach die Heizung etwas wärmer", "home_assistant"},
	} {
		names := searchNames(catalog, tc.query)
		if len(names) > 30 || !slices.Contains(names[:min(5, len(names))], tc.want) {
			t.Errorf("Search(%q): %d hits, first five %v, want %s among them", tc.query, len(names), names[:min(5, len(names))], tc.want)
		}
	}
}

// longMessagePreamble is a sixty-five word German report that ends in no tool
// request of its own.
const longMessagePreamble = "Gestern Abend habe ich nach dem Update bemerkt, dass die Webseite sehr langsam lädt, außerdem kamen mehrere Fehlermeldungen im Browser, der Speicherplatz wird knapp, die Lüfter laufen ständig laut, mein Kollege meinte, es könnte an der Konfiguration liegen, wir haben schon vieles probiert, leider ohne Erfolg, das Monitoring zeigt rote Werte, der Router blinkt komisch, das Backup lief nicht durch. "

// TestToolSearchLongMessageKeepsTheAskAtTheEnd guards the selection of the query
// tokens: the request at the end of a long message must be scored, not cut
// off behind the first words.
func TestToolSearchLongMessageKeepsTheAskAtTheEnd(t *testing.T) {
	query := longMessagePreamble + "Bitte starte den Docker Container nginx neu."
	var stems []string
	for _, token := range parseToolSearchQuery(query, "").tokens {
		stems = append(stems, token.stem)
	}
	if len(stems) > toolSearchMaxTokens {
		t.Fatalf("%d tokens, want at most %d", len(stems), toolSearchMaxTokens)
	}
	for _, want := range []string{"docker", "container"} {
		if !slices.Contains(stems, want) {
			t.Errorf("tokens %v lost %q from the end of the message", stems, want)
		}
	}
	catalog := BuildToolCatalog(BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil), nil, "")
	if names := searchNames(catalog, query); !slices.Contains(names[:min(10, len(names))], "docker") {
		t.Errorf("docker is not among the first ten of %d hits: %v", len(names), names[:min(10, len(names))])
	}
}

// TestToolSearchBoundsALongMessage guards the cost of a search with the whole
// user message as the query: it is ranked from its head and tail only.
func TestToolSearchBoundsALongMessage(t *testing.T) {
	catalog := BuildToolCatalog(BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil), nil, "")
	catalog.Search("warm up") // build the index and the statistics first
	query := "docker " + strings.Repeat("send file email server müde ", 40000) + " restart"
	start := time.Now()
	names := searchNames(catalog, query)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("searching %d bytes took %s", len(query), elapsed)
	}
	if !slices.Contains(names, "docker") {
		t.Errorf("the head of the message was not searched: %v", names)
	}
	schemas := BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil)
	start = time.Now()
	priority := buildAdaptiveToolPriority(schemas, nil, query, nil, nil)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("the adaptive selection for %d bytes took %s", len(query), elapsed)
	}
	if !slices.Contains(priority, "docker") {
		t.Errorf("the adaptive selection lost the head of the message: %v", priority)
	}
	bounded := boundToolSearchQuery(query)
	if len(bounded) > toolSearchQueryLimit || !utf8.ValidString(bounded) || !strings.HasPrefix(bounded, "docker send") || !strings.HasSuffix(bounded, "müde  restart") {
		t.Errorf("bounded query = %d bytes, valid=%v, %.20q...%.20q", len(bounded), utf8.ValidString(bounded), bounded, bounded[len(bounded)-20:])
	}
}
