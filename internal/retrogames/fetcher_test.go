package retrogames

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func plainClient(string, time.Duration) (*http.Client, error) {
	return &http.Client{Timeout: 5 * time.Second}, nil
}

func TestHTTPFetcherBuildsScrapeRequest(t *testing.T) {
	var gotQuery, gotUA, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"identifier":"a","title":"A","emulator":"dosbox","year":1990,"downloads":12},{"identifier":5}],"count":2,"total":7,"cursor":"next"}`))
	}))
	defer srv.Close()
	f := &HTTPFetcher{BaseURL: srv.URL + "/services/search/v1/scrape", ClientFor: plainClient}
	page, err := f.FetchPage(context.Background(), "collection:(x) AND emulator:*", "c1")
	if err != nil {
		t.Fatalf("FetchPage: %v", err)
	}
	if gotPath != "/services/search/v1/scrape" || !strings.Contains(gotUA, "AuraGo-RetroGames") {
		t.Fatalf("path=%q ua=%q", gotPath, gotUA)
	}
	for _, want := range []string{"count=10000", "cursor=c1", "fields=identifier%2Ctitle%2Cyear%2Ccreator%2Cdownloads%2Cemulator%2Csubject", "q=collection%3A%28x%29+AND+emulator%3A%2A"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q lacks %q", gotQuery, want)
		}
	}
	if len(page.Items) != 1 || page.Items[0].Identifier != "a" || page.Invalid != 1 || page.Cursor != "next" || page.Total != 7 {
		t.Fatalf("page = %+v", page)
	}
}

func TestHTTPFetcherStatusErrors(t *testing.T) {
	cases := []struct {
		status    int
		header    string
		retry     bool
		retryWait time.Duration
	}{
		{http.StatusTooManyRequests, "7", true, 7 * time.Second},
		{http.StatusServiceUnavailable, "", true, 0},
		{http.StatusBadRequest, "", false, 0},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tc.header != "" {
				w.Header().Set("Retry-After", tc.header)
			}
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(`{"error":"bad count","errorType":"x"}`))
		}))
		_, err := (&HTTPFetcher{BaseURL: srv.URL, ClientFor: plainClient}).FetchPage(context.Background(), "q", "")
		srv.Close()
		var se *StatusError
		if !errors.As(err, &se) || se.Status != tc.status {
			t.Fatalf("status %d: err = %v", tc.status, err)
		}
		if tc.status == http.StatusBadRequest && !strings.Contains(se.Error(), "bad count") {
			t.Errorf("error message lost: %v", se)
		}
		retry, wait := retryable(err)
		if retry != tc.retry || wait != tc.retryWait {
			t.Errorf("status %d: retryable = %v %v", tc.status, retry, wait)
		}
	}
}

func TestHTTPFetcherRejectsBadBodies(t *testing.T) {
	bodies := map[string]string{
		"error payload": `{"error":"broken"}`,
		"no items":      `{"count":0}`,
		"not json":      `<html>`,
		"too large":     `{"items":[` + strings.Repeat(" ", 2048) + `]}`,
	}
	for name, body := range bodies {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		_, err := (&HTTPFetcher{BaseURL: srv.URL, ClientFor: plainClient, MaxBytes: 1024}).FetchPage(context.Background(), "q", "")
		srv.Close()
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
		if retry, _ := retryable(err); retry {
			t.Errorf("%s: must not be retried", name)
		}
	}
}

func TestHTTPFetcherNetworkErrorIsRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()
	_, err := (&HTTPFetcher{BaseURL: url, ClientFor: plainClient}).FetchPage(context.Background(), "q", "")
	if retry, _ := retryable(err); err == nil || !retry {
		t.Fatalf("err = %v retry=%v", err, retry)
	}
}

func TestNewHTTPFetcherDefaults(t *testing.T) {
	f := NewHTTPFetcher()
	if f.BaseURL != "https://archive.org/services/search/v1/scrape" || f.ClientFor == nil || f.MaxBytes != 32<<20 {
		t.Fatalf("defaults = %+v", f)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	if got := parseRetryAfter("12", now); got != 12*time.Second {
		t.Fatalf("seconds = %v", got)
	}
	if got := parseRetryAfter(now.Add(30*time.Second).Format(http.TimeFormat), now); got != 30*time.Second {
		t.Fatalf("date = %v", got)
	}
	if got := parseRetryAfter("soon", now); got != 0 {
		t.Fatalf("garbage = %v", got)
	}
}
