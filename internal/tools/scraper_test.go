package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func withScraperTestClient(t *testing.T, contentType, body string) {
	t.Helper()
	originalClient := scraperHTTPClient
	scraperHTTPClient = &http.Client{
		Transport: ddgRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{contentType}},
			}, nil
		}),
	}
	t.Cleanup(func() {
		scraperHTTPClient = originalClient
	})
}

func TestExecuteWebScraperRSSModeReturnsStructuredItems(t *testing.T) {
	withScraperTestClient(t, "application/rss+xml", `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>KI News</title>
    <item>
      <title>Agent pipeline recovered</title>
      <link>https://example.com/news/agent-pipeline</link>
      <description>Build and scrape flows are healthy again.</description>
      <pubDate>Sun, 28 Jun 2026 08:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`)

	result := ExecuteWebScraperWithOptions("https://example.com/feed.xml", WebScraperOptions{Mode: "rss"})
	var payload struct {
		Status string `json:"status"`
		Mode   string `json:"mode"`
		Title  string `json:"title"`
		Items  []struct {
			Title       string `json:"title"`
			Link        string `json:"link"`
			Description string `json:"description"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("scraper result is not JSON: %s", result)
	}
	if payload.Status != "success" || payload.Mode != "rss" {
		t.Fatalf("unexpected payload status/mode: %+v", payload)
	}
	if payload.Title != "KI News" {
		t.Fatalf("feed title = %q, want KI News", payload.Title)
	}
	if len(payload.Items) != 1 || payload.Items[0].Title != "Agent pipeline recovered" {
		t.Fatalf("unexpected rss items: %+v", payload.Items)
	}
}

func TestExecuteWebScraperAutoDetectsRSSContent(t *testing.T) {
	withScraperTestClient(t, "application/atom+xml", `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>AI Updates</title>
  <entry>
    <title>RSS auto mode works</title>
    <link href="https://example.com/atom/rss-auto"/>
    <summary>Auto mode should parse Atom feeds as structured items.</summary>
    <updated>2026-06-28T08:00:00Z</updated>
  </entry>
</feed>`)

	result := ExecuteWebScraperWithOptions("https://example.com/atom.xml", WebScraperOptions{Mode: "auto"})
	if !strings.Contains(result, `"mode":"rss"`) {
		t.Fatalf("expected auto mode to return rss mode payload, got: %s", result)
	}
	if !strings.Contains(result, "RSS auto mode works") {
		t.Fatalf("expected Atom item title in result, got: %s", result)
	}
}
