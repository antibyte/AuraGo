package server

import (
	"context"
	"strings"
	"testing"
)

func TestNewspaperFeedParsingAndPublicURLGate(t *testing.T) {
	rss := `<rss><channel><item><title>Regional library opens</title><link>https://news.example/library?utm_source=feed</link><pubDate>Fri, 25 Sep 2026 06:00:00 GMT</pubDate></item></channel></rss>`
	hits, err := parseNewspaperFeed("https://news.example/feed.xml", []byte(rss))
	if err != nil || len(hits) != 1 || hits[0].Title != "Regional library opens" || hits[0].URL != "https://news.example/library" {
		t.Fatalf("RSS parsing: %+v, %v", hits, err)
	}
	atom := `<feed xmlns="http://www.w3.org/2005/Atom"><entry><title>Science update</title><link rel="alternate" href="/science"/><updated>2026-09-25T05:00:00Z</updated></entry></feed>`
	hits, err = parseNewspaperFeed("https://science.example/atom.xml", []byte(atom))
	if err != nil || len(hits) != 1 || hits[0].URL != "https://science.example/science" || hits[0].Published != "2026-09-25T05:00:00Z" {
		t.Fatalf("Atom parsing: %+v, %v", hits, err)
	}
	if _, err := fetchNewspaperFeed(context.Background(), "http://127.0.0.1/private.xml"); err == nil || !strings.Contains(err.Error(), "not public") {
		t.Fatalf("private feed was accepted: %v", err)
	}
}
