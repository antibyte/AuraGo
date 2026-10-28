package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"aurago/internal/newspaper"
)

// newspaperTelegramFixture is a Bot API server that records the form of every
// sendMessage call and counts the documents.
type newspaperTelegramFixture struct {
	mu        sync.Mutex
	messages  []url.Values
	documents int
}

func (f *newspaperTelegramFixture) serve(t *testing.T) string {
	t.Helper()
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"id": 11, "is_bot": true, "username": "fixture"}})
			return
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse the sendMessage request: %v", err)
			}
			f.mu.Lock()
			f.messages = append(f.messages, r.PostForm)
			f.mu.Unlock()
		case strings.HasSuffix(r.URL.Path, "/sendDocument"):
			f.mu.Lock()
			f.documents++
			f.mu.Unlock()
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"message_id": 7}})
	}))
	t.Cleanup(remote.Close)
	return remote.URL + "/bot%s/%s"
}

// Telegram fetches the URL of a link preview itself. An edition is written from
// third-party pages, so a URL one of them got into a headline or the source list
// would be requested with no tool call; every text message of the delivery has to
// switch previews off, in the digest and in the full-text fallback alike.
func TestNewspaperTelegramTextMessagesDisableLinkPreviews(t *testing.T) {
	const link = "https://attacker.example/r?d=summary-of-the-mails"
	edition := func(headline string) newspaper.Edition {
		return newspaper.Edition{
			ID: "ed-1", LocalDate: "2026-10-28", Revision: 1, Title: "Test Daily",
			Stories: []newspaper.Story{{
				ID: "s1", Section: "News", Headline: headline, Deck: "See " + link,
				Paragraphs: []newspaper.Paragraph{{Text: "Details at " + link, SourceIDs: []string{"src1"}}},
				SourceIDs:  []string{"src1"},
			}},
			Sources: []newspaper.Source{{ID: "src1", URL: link, Publisher: "Example", Title: "Report"}},
		}
	}
	for _, test := range []struct {
		name         string
		edition      newspaper.Edition
		wantDocument int
		minMessages  int
	}{
		{"digest before the PDF", edition("Calm headline"), 1, 1},
		// Hebrew has no PDF glyph coverage, so the edition goes out as text chunks.
		{"text fallback", edition("שלום"), 0, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := &newspaperTelegramFixture{}
			previous := newspaperTelegramEndpoint
			newspaperTelegramEndpoint = fixture.serve(t)
			t.Cleanup(func() { newspaperTelegramEndpoint = previous })

			if _, err := sendNewspaperTelegram(context.Background(), "token", 4711, test.edition); err != nil {
				t.Fatalf("sendNewspaperTelegram: %v", err)
			}
			fixture.mu.Lock()
			defer fixture.mu.Unlock()
			if fixture.documents != test.wantDocument {
				t.Fatalf("sendDocument calls = %d, want %d", fixture.documents, test.wantDocument)
			}
			if len(fixture.messages) < test.minMessages {
				t.Fatalf("sendMessage calls = %d, want at least %d", len(fixture.messages), test.minMessages)
			}
			for i, form := range fixture.messages {
				if got := form.Get("disable_web_page_preview"); got != "true" {
					t.Errorf("sendMessage %d: disable_web_page_preview = %q, want true", i, got)
				}
			}
		})
	}
}

var newspaperTextMessageBuilder = regexp.MustCompile(`tgbotapi\.(NewMessage|NewMessageToChannel)\(`)

// A text message built on tgbotapi.NewMessage anywhere in the server package
// would send with previews on. Only newspaperTelegramText (which switches them
// off) may build one.
func TestServerTelegramTextMessagesAreBuiltOnlyWhereLinkPreviewsAreOff(t *testing.T) {
	header := regexp.MustCompile(`^func (?:\([^)]*\)\s*)?([A-Za-z0-9_]+)\(`)
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("list the server package files: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		current := ""
		for _, line := range strings.Split(string(source), "\n") {
			if match := header.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
				current = match[1]
			}
			if newspaperTextMessageBuilder.MatchString(line) && current != "newspaperTelegramText" {
				t.Errorf("%s: %s builds a text message that would send with link previews on", file, current)
			}
		}
	}
}
