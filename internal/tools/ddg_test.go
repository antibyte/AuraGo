package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type ddgRoundTripFunc func(*http.Request) (*http.Response, error)

func (f ddgRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func withDDGTestClient(t *testing.T, body string) {
	t.Helper()
	originalClient := scraperHTTPClient
	scraperHTTPClient = &http.Client{
		Transport: ddgRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	t.Cleanup(func() {
		scraperHTTPClient = originalClient
	})
}

func decodeDDGResult(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("DDG result is not valid JSON: %v\nraw: %s", err, raw)
	}
	return out
}

func TestExecuteDDGSearchParsesCurrentLiteMarkup(t *testing.T) {
	html := `
<html><body>
<a rel="nofollow" href="https://www.tagesschau.de/" class='result-link'>tagesschau.de - Nachrichten</a>
<td class='result-snippet'>Die wichtigsten <b>News</b> des Tages</td>
</body></html>`
	withDDGTestClient(t, html)

	out := decodeDDGResult(t, ExecuteDDGSearch("tagesschau news", 5))
	if out["status"] != "success" {
		t.Fatalf("status = %v, want success; output: %#v", out["status"], out)
	}
	results, ok := out["results"].([]interface{})
	if !ok || len(results) != 1 {
		t.Fatalf("results = %#v, want one parsed result", out["results"])
	}
	first, _ := results[0].(map[string]interface{})
	if first["link"] != "https://www.tagesschau.de/" {
		t.Fatalf("link = %v, want tagesschau URL", first["link"])
	}
	if !strings.Contains(first["title"].(string), "tagesschau.de") {
		t.Fatalf("title = %v, want parsed title", first["title"])
	}
	if !strings.Contains(first["snippet"].(string), "wichtigsten News") {
		t.Fatalf("snippet = %v, want stripped snippet text", first["snippet"])
	}
}

func TestExecuteDDGSearchReturnsErrorForUnparseableSuccessPage(t *testing.T) {
	withDDGTestClient(t, `<html><title>tagesschau news at DuckDuckGo</title><body>No result markup here.</body></html>`)

	out := decodeDDGResult(t, ExecuteDDGSearch("tagesschau news", 5))
	if out["status"] != "error" {
		t.Fatalf("status = %v, want error for unparseable DDG page; output: %#v", out["status"], out)
	}
	if !strings.Contains(out["message"].(string), "No parseable DDG results") {
		t.Fatalf("message = %v, want parse failure context", out["message"])
	}
}
