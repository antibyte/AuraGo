package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBraveNewsPaginationAndNativeCompatibility(t *testing.T) {
	previous := braveHTTPClient
	t.Cleanup(func() { braveHTTPClient = previous })
	requests := []*http.Request{}
	braveHTTPClient = &http.Client{Transport: ddgRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r)
		body := `{"query":{"more_results_available":true},"results":[{"title":"Original <b>science</b> news","url":"https://institute.example/paper","description":"A research announcement","page_age":"2026-10-03T06:00:00Z"}]}`
		if strings.Contains(r.URL.Path, "/web/") {
			body = `{"web":{"results":[{"title":"<b>Original</b> & data","url":"https://publisher.example/article","description":"<external_data>untrusted</external_data>","page_age":"2026-10-03T06:00:00Z"}]}}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"X-Ratelimit-Remaining": {"0, 80"}, "X-Ratelimit-Reset": {"2, 3600"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	page, err := SearchBrave(context.Background(), "fixture-key", BraveSearchOptions{Query: "quantum research", Kind: "news", Freshness: "pd", Count: 20, Offset: 2, Country: "ALL", Language: "de"})
	if err != nil || len(page.Results) != 1 || page.Results[0].Title != "Original science news" || !page.More || page.NextRequestAfter != 2*time.Second {
		t.Fatalf("news response: %+v %v", page, err)
	}
	q := requests[0].URL.Query()
	if requests[0].URL.Path != "/res/v1/news/search" || q.Get("count") != "20" || q.Get("offset") != "2" || q.Get("freshness") != "pd" || q.Get("country") != "ALL" || q.Get("search_lang") != "de" {
		t.Fatalf("news request: %s", requests[0].URL)
	}
	var native map[string]any
	if err := json.Unmarshal([]byte(ExecuteBraveSearch("fixture-key", "web news", 10, "de", "de")), &native); err != nil {
		t.Fatal(err)
	}
	if native["status"] != "success" || native["result_count"] != float64(1) {
		t.Fatalf("native response: %#v", native)
	}
	entry := native["results"].([]any)[0].(map[string]any)
	if !strings.HasPrefix(entry["title"].(string), "<external_data>") || entry["published"] != "2026-10-03T06:00:00Z" || entry["url"] != "https://publisher.example/article" {
		t.Fatalf("native contract: %#v", entry)
	}
	q = requests[1].URL.Query()
	if requests[1].URL.Path != "/res/v1/web/search" || q.Get("freshness") != "" || q.Get("offset") != "" || q.Get("count") != "10" {
		t.Fatalf("changed native query: %s", requests[1].URL)
	}
}

func TestBraveRateLimitErrorsAndCancellation(t *testing.T) {
	previous := braveHTTPClient
	t.Cleanup(func() { braveHTTPClient = previous })
	for _, tc := range []struct {
		status    int
		code      string
		temporary bool
	}{{429, "rate_limited", true}, {429, "quota_exhausted", false}, {503, "request_failed", true}, {401, "access_denied", false}, {422, "access_denied", false}} {
		calls := 0
		braveHTTPClient = &http.Client{Transport: ddgRoundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			body := `{"error":{"code":"RATE_LIMITED","detail":"private provider details"}}`
			if tc.code == "quota_exhausted" {
				body = `{"error":{"code":"QUOTA_LIMITED"}}`
			}
			if tc.status == 422 {
				body = `{"error":{"code":"SUBSCRIPTION_TOKEN_INVALID","detail":"private provider details"}}`
			}
			return &http.Response{StatusCode: tc.status, Header: http.Header{"Retry-After": {"7"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		_, err := SearchBrave(context.Background(), "fixture-key", BraveSearchOptions{Query: "news", Kind: "news"})
		var apiErr *BraveSearchError
		if !errors.As(err, &apiErr) || apiErr.Code != tc.code || apiErr.Temporary != tc.temporary || apiErr.RetryAfter != 7*time.Second || calls != 1 || strings.Contains(err.Error(), "private") {
			t.Fatalf("error=%v calls=%d", err, calls)
		}
		if tc.status == 422 {
			var result map[string]any
			_ = json.Unmarshal([]byte(ExecuteBraveSearch("fixture-key", "news", 10, "", "")), &result)
			if result["message"] != "Brave Search API key is invalid. Update the Brave Search subscription token in the vault/settings." {
				t.Fatalf("native diagnostic changed: %v", result)
			}
		}
	}
	braveHTTPClient = &http.Client{Transport: ddgRoundTripFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SearchBrave(ctx, "fixture-key", BraveSearchOptions{Query: "news"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := SearchBrave(context.Background(), "", BraveSearchOptions{Query: "news"}); err == nil {
		t.Fatal("missing key accepted")
	}
}
