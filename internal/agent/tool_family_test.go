package agent

import (
	"testing"
	"unicode/utf8"
)

func TestInferToolFamilyFromQueryMatchesWholeWordsInEnglishAndGerman(t *testing.T) {
	for _, tc := range []struct {
		query, want string
	}{
		// deployment
		{"deploy the homepage to netlify", "deployment"},
		{"publish the website", "deployment"},
		{"Veröffentliche die Webseite", "deployment"},
		{"bring the landing page online", "deployment"},
		{"lade das Paket hoch, es ist schon hochgeladen", "deployment"},
		// files
		{"read the file and show me the folder", "files"},
		{"edit the yaml", "files"},
		{"Zeig mir den Ordner", "files"},
		{"Öffne die Datei", "files"},
		{"Prüfe die Konfigurationsdateien", "files"},
		{"Lege einen Unterordner an", "files"},
		{"Wie heißt das Verzeichnis?", "files"},
		{"Der Dateiname fehlt", "files"},
		{"Lege ein Unterverzeichnis an", "files"},
		{"Wechsle ins Arbeitsverzeichnis", "files"},
		// shell
		{"run this command in the terminal", "shell"},
		{"open a bash shell", "shell"},
		{"Führe den Befehl aus", "shell"},
		{"Starte die Konsole", "shell"},
		{"Führe das Shellskript aus", "shell"},
		{"Schreibe ein Startskript", "shell"},
		{"Die Skriptsprache ist Bash", "shell"},
		{"Der Shellbefehl schlägt fehl", "shell"},
		{"Die Konsolenausgabe ist leer", "shell"},
		{"Öffne die Kommandozeile", "shell"},
		{"wie viel RAM ist frei?", "shell"},
		{"Zeig die CPU-Auslastung", "shell"},
		// coding
		{"write python code", "coding"},
		{"Schreibe ein Programm in Python", "coding"},
		{"Programmiere eine Funktion", "coding"},
		{"Refaktoriere den Quellcode", "coding"},
		// memory
		{"remember this for later", "memory"},
		{"write a note in the journal", "memory"},
		{"Merk dir das bitte", "memory"},
		{"Mach eine Notiz", "memory"},
		{"Was steht in deinem Gedächtnis?", "memory"},
		// web
		{"search the web for golang news", "web"},
		{"scrape this page", "web"},
		{"call the REST api endpoint", "web"},
		{"Recherchiere im Internet", "web"},
		{"Suche im Web nach Rezepten", "web"},
		// network
		{"ping the router", "network"},
		{"which ports are open on the host", "network"},
		{"Scanne das Netzwerk", "network"},
		{"Prüfe mein Heimnetzwerk", "network"},
		{"wake on lan for the pc", "network"},
		// infra
		{"restart the docker container", "infra"},
		{"check proxmox", "infra"},
		{"Starte den Server neu", "infra"},
		{"ssh into the nas", "infra"},
		// communication
		{"send an email", "communication"},
		{"write an sms to my contact", "communication"},
		{"Ruf bitte zurück, das war ein Anruf", "communication"},
		{"Schick eine E-Mail", "communication"},
		{"Lege einen Kontakt an", "communication"},
		// automation
		{"schedule a daily job", "automation"},
		{"Erstelle einen Zeitplan", "automation"},
		{"Lege einen Cron-Job an", "automation"},
		{"Starte die Mission jeden Morgen", "automation"},
		// media
		{"generate an image", "media"},
		{"make a video with music", "media"},
		{"Erstelle ein Bild", "media"},
		{"Spiel Musik ab", "media"},
		{"Zeig das Foto", "media"},
		{"Aktiviere die Sprachausgabe", "media"},
		// none
		{"", ""},
		{"hello there", ""},
	} {
		if got := inferToolFamilyFromQuery(tc.query); got != tc.want {
			t.Errorf("inferToolFamilyFromQuery(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

// A message that names several families goes to the one with the most terms; a
// tie keeps the long-standing precedence of the families.
func TestInferToolFamilyFromQueryMostTermsThenPrecedence(t *testing.T) {
	for _, tc := range []struct {
		query, want string
	}{
		{"search the web and call an api endpoint", "web"},
		{"read the file and the folder, then search", "files"},
		{"read the file and search", "files"},
		{"search my memory", "memory"},
		{"ping the wikipedia server", "network"},
		{"wikipedia proxmox", "infra"},
		{"memory usage of the container", "shell"},
		{"deploy the file", "deployment"},
	} {
		if got := inferToolFamilyFromQuery(tc.query); got != tc.want {
			t.Errorf("inferToolFamilyFromQuery(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

// Substrings of other words are no terms: each of these used to name a family
// because "file", "port", "form", "ram", "call" or "media" sit inside them.
func TestInferToolFamilyFromQueryIgnoresSubstrings(t *testing.T) {
	for _, query := range []string{
		"show my profile",
		"the important report about the export and the import",
		"customer support",
		"the capital of France",
		"platform information",
		"a short description",
		"do you recall that",
		"come immediately",
		"the portal is open",
		"Vorbild und Abbildung",
		"das Programmheft",
		// words that only end or start like a German head
		"Bitte das Manuskript korrigieren",
		"Das Transkript liegt vor",
		"Inhaltsverzeichnis erstellen",
		"Das Literaturverzeichnis fehlt",
		"Spielkonsole einschalten",
		"Haftbefehl erlassen",
		"Das Einsatzkommando rückt aus",
		// words that sound like media but are not
		"Ich stimme zu",
		"die Werte stimmen nicht",
		"Das stimmt nicht",
		"you lied to me about the backup",
		"draw conclusions from the logs",
	} {
		if got := inferToolFamilyFromQuery(query); got != "" {
			t.Errorf("inferToolFamilyFromQuery(%q) = %q, want none", query, got)
		}
	}
}

func TestIntentHelpersMatchWholeWords(t *testing.T) {
	for _, tc := range []struct {
		name  string
		match func(toolIntentText) bool
		yes   []string
		no    []string
	}{
		{
			name:  "resource usage",
			match: isResourceUsageIntent,
			yes: []string{
				"wieviel ram verbraucht aurago", "the cpu is busy", "show the memory usage", "Zeig die Auslastung",
				"Speicherverbrauch prüfen", "wie viel Speicher ist frei", "Arbeitsspeicher", "resource usage please",
				"Ressourcen anzeigen", "the resources", "der Verbrauch", "Verbrauchswerte", "Systemverbrauch",
			},
			no: []string{
				"das Programm", "the parameter", "a frame", "a diagram", "gram", "ramp up", "the memory leak",
				"Verbraucherzentrale anrufen", "den Stromverbrauch senken",
			},
		},
		{
			name:  "container",
			match: isContainerIntent,
			yes:   []string{"docker ps", "restart the container", "docker-compose up", "bau das Dockerfile", "Container neu starten"},
			no:    []string{"containment", "the dock", "composer install"},
		},
		{
			name:  "mcp",
			match: isMCPIntent,
			yes:   []string{"use the MCP server", "model context protocol"},
			no:    []string{"mcpherson", "the context"},
		},
		{
			name:  "composio",
			match: isComposioIntent,
			yes:   []string{"use composio", "list the integrations", "prüfe gmail", "google mail"},
			no:    []string{"disintegrate it"},
		},
		{
			name:  "manus",
			match: isManusIntent,
			yes:   []string{"delegate this to Manus", "an manus delegieren", "manus"},
			no:    []string{"the manuscript", "read the manual", "Bitte das Manuskript korrigieren"},
		},
		{
			name:  "network camera",
			match: isNetworkCameraIntent,
			yes: []string{
				"mache einen snapshot der netzwerkkamera", "show the ONVIF camera live stream", "öffne das livebild der IP-Kamera",
				"Ist die Personenerkennung aktiv?", "Schalte die Alarmanlage scharf", "Kameras unscharf schalten", "disarm the cameras",
				"Zeig die Netzwerkkameras",
			},
			no: []string{"erstelle einen snapshot der webseite", "Mein Arm tut weh", "set an alarm for 7", "das Messer ist scharf", "Mein Kamerad ist scharf auf Kuchen"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, query := range tc.yes {
				if !tc.match(newToolIntentText(query)) {
					t.Errorf("%q should match", query)
				}
			}
			for _, query := range tc.no {
				if tc.match(newToolIntentText(query)) {
					t.Errorf("%q must not match", query)
				}
			}
		})
	}
}

// The family tables rely on a few rules that a typo would break silently.
func TestToolFamilyTermSetsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for i, set := range toolFamilyTermSets {
		if seen[set.family] {
			t.Errorf("family %q listed twice", set.family)
		}
		seen[set.family] = true
		if len(set.terms) == 0 || toolFamilyTerms[i] == nil || len(toolFamilyTerms[i].terms) < len(set.terms) {
			t.Errorf("family %q: table not compiled", set.family)
		}
		heads := make([]toolIntentHead, 0, len(set.heads))
		for _, spec := range set.heads {
			head := parseToolIntentHead(spec)
			heads = append(heads, head)
			if utf8.RuneCountInString(head.word) < toolIntentMinHeadRunes {
				t.Errorf("family %q: head %q is shorter than %d runes", set.family, spec, toolIntentMinHeadRunes)
			}
			if _, stop := toolSearchStopwords[head.word]; stop {
				t.Errorf("family %q: head %q is a stopword", set.family, spec)
			}
		}
		// A term that a head already finds would count a word twice.
		for _, term := range set.terms {
			text := newToolIntentText(term)
			for _, head := range heads {
				if len(text.words) == 1 && text.hasHead(head) {
					t.Errorf("family %q: term %q is covered by head %q", set.family, term, head.word)
				}
			}
		}
	}
	for _, want := range []string{"deployment", "files", "shell", "coding", "memory", "web", "network", "infra", "communication", "automation", "media"} {
		if !seen[want] {
			t.Errorf("family %q missing", want)
		}
	}
}

func TestToolIntentTermsStopwordStemsAreNoTerms(t *testing.T) {
	// "note" and "not" share a stem, "such" and "suche" too.
	notes := newToolIntentTerms([]string{"note", "suche"}, nil)
	for query, want := range map[string]int{
		"I do not want this": 0, "such a thing": 0, "take notes": 1, "a note": 1, "Suche etwas": 1, "note and notes": 1,
	} {
		if got := notes.count(newToolIntentText(query)); got != want {
			t.Errorf("count(%q) = %d, want %d", query, got, want)
		}
	}
}

// The head notation decides where a German noun may sit inside a compound.
func TestToolIntentHeadNotation(t *testing.T) {
	terms := newToolIntentTerms(nil, []string{"datei", "^kommando", "$verbrauch:cpu,speicher", "verzeichnis:unter,arbeits"})
	for query, want := range map[string]int{
		"Konfigurationsdateien":     1, // either end, plural
		"Dateiname":                 1,
		"Kommandozeile":             1, // start
		"Einsatzkommando":           0, // not at the end
		"Speicherverbrauch":         1, // end after a listed modifier
		"Stromverbrauch":            0,
		"Verbrauchswerte":           0, // $ never matches at the start
		"Verbraucher":               0,
		"Unterverzeichnisse":        1,
		"Inhaltsverzeichnis":        0,
		"Verzeichnis und Dateiname": 2,
	} {
		if got := terms.count(newToolIntentText(query)); got != want {
			t.Errorf("count(%q) = %d, want %d", query, got, want)
		}
	}
	exact := newToolIntentTerms([]string{"=malen"}, nil)
	for query, want := range map[string]bool{"Bilder malen": true, "the male users": false, "mal sehen": false} {
		if got := exact.matches(newToolIntentText(query)); got != want {
			t.Errorf("exact term on %q = %v, want %v", query, got, want)
		}
	}
}
