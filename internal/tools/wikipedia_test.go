package tools

import (
	"aurago/internal/testutil"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestExecuteWikipediaSearchFallsBackToSearchResult(t *testing.T) {
	summaryRequests := 0
	requests := make([]string, 0, 3)
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path+"?"+r.URL.RawQuery)
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/rest_v1/page/summary/"):
			summaryRequests++
			if summaryRequests == 1 {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"title":"Künstliche Intelligenz","extract":"Deutsche Zusammenfassung","content_urls":{"desktop":{"page":"https://de.wikipedia.org/wiki/Künstliche_Intelligenz"}}}`))
			return
		case strings.Contains(r.URL.Path, "/w/api.php") || strings.Contains(r.URL.RawQuery, "list=search"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"query":{"search":[{"title":"Künstliche Intelligenz"}]}}`))
			return
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	origBaseURL := wikipediaBaseURLForLang
	wikipediaBaseURLForLang = func(lang string) string { return server.URL }
	defer func() { wikipediaBaseURLForLang = origBaseURL }()

	result := ExecuteWikipediaSearch("Künstliche Intelligenz", "de")

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if payload["status"] != "success" {
		t.Fatalf("expected success result, got %s; requests=%v", result, requests)
	}
	title, _ := payload["title"].(string)
	if !strings.Contains(title, "Künstliche Intelligenz") {
		t.Fatalf("title = %v, want wrapped Künstliche Intelligenz", payload["title"])
	}
}

func TestExecuteWikipediaSearchNormalizesLanguage(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"title":"Artificial intelligence","extract":"Summary","content_urls":{"desktop":{"page":"https://en.wikipedia.org/wiki/Artificial_intelligence"}}}`))
	}))
	defer server.Close()

	origBaseURL := wikipediaBaseURLForLang
	seenLangs := make([]string, 0, 1)
	wikipediaBaseURLForLang = func(lang string) string {
		seenLangs = append(seenLangs, lang)
		return server.URL
	}
	defer func() { wikipediaBaseURLForLang = origBaseURL }()

	result := ExecuteWikipediaSearch("Artificial intelligence", "Deutsch")
	if !strings.Contains(result, `"status":"success"`) {
		t.Fatalf("expected success result, got %s", result)
	}
	if len(seenLangs) == 0 || seenLangs[0] != "de" {
		t.Fatalf("seen langs = %v, want first request in de", seenLangs)
	}
}
