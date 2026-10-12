package retrogames

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func plainClient(string, time.Duration) (*http.Client, error) {
	return &http.Client{Timeout: 5 * time.Second}, nil
}

// noRedirectClient mirrors the strict public client: redirects are not followed.
func noRedirectClient(string, time.Duration) (*http.Client, error) {
	return &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, nil
}

// fetchServedBody serves body with status 200 and fetches one page from it.
func fetchServedBody(t *testing.T, body string, maxBytes int64) (Page, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	return (&HTTPFetcher{BaseURL: srv.URL, ClientFor: plainClient, MaxBytes: maxBytes}).FetchPage(context.Background(), "q", "")
}

func itemsBody(n int) string {
	return `{"items":[` + strings.TrimSuffix(strings.Repeat(`{"identifier":"x"},`, n), ",") + `],"total":1}`
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

func TestParseRetryAfterIsClampedToOneHour(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	cases := map[string]time.Duration{
		"10000000000":          time.Hour, // would overflow a time.Duration if multiplied first
		"99999999999999999999": time.Hour, // does not even fit into an int64
		"3600":                 time.Hour,
		"3601":                 time.Hour,
		"-5":                   0,
		"0":                    0,
		now.Add(48 * time.Hour).Format(http.TimeFormat): time.Hour,
		now.Add(-time.Minute).Format(http.TimeFormat):   0,
	}
	for in, want := range cases {
		if got := parseRetryAfter(in, now); got != want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestHTTPFetcherEmptyAndNullItems(t *testing.T) {
	page, err := fetchServedBody(t, `{"items":[]}`, 0)
	if err != nil || len(page.Items) != 0 || page.Invalid != 0 {
		t.Fatalf("empty items: page=%+v err=%v", page, err)
	}
	for name, body := range map[string]string{
		"null items":   `{"items":null}`,
		"scalar items": `{"items":5}`,
		"object items": `{"items":{"a":1}}`,
	} {
		if _, err := fetchServedBody(t, body, 0); err == nil || !strings.Contains(err.Error(), "no items") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestHTTPFetcherRejectsTrailingData(t *testing.T) {
	if _, err := fetchServedBody(t, `{"items":[]} {"items":[]}`, 0); err == nil {
		t.Fatal("trailing data accepted")
	}
	if _, err := fetchServedBody(t, `[]`, 0); err == nil {
		t.Fatal("top-level array accepted")
	}
}

func TestHTTPFetcherCapsItemsPerPage(t *testing.T) {
	page, err := fetchServedBody(t, itemsBody(scrapePageSize), 0)
	if err != nil || len(page.Items) != scrapePageSize || page.Total != 1 {
		t.Fatalf("exactly %d items: len=%d total=%d err=%v", scrapePageSize, len(page.Items), page.Total, err)
	}
	_, err = fetchServedBody(t, itemsBody(scrapePageSize+1), 0)
	if err == nil || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("%d items: err = %v", scrapePageSize+1, err)
	}
	if retry, _ := retryable(err); retry {
		t.Fatal("an oversized page must not be retried")
	}
}

func TestHTTPFetcherHostileItemArrayStaysCheap(t *testing.T) {
	body := `{"items":[` + strings.Repeat("1,", 1<<20) + `1]}` // ~2 MiB of non-object elements
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := fetchServedBody(t, body, 0)
	runtime.ReadMemStats(&after)
	if err == nil || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("err = %v", err)
	}
	if delta := after.TotalAlloc - before.TotalAlloc; delta > 64<<20 {
		t.Fatalf("decoding allocated %d MiB", delta>>20)
	}
}

func TestHTTPFetcherCountsInvalidElementsTowardsTheCap(t *testing.T) {
	body := `{"items":[` + strings.Repeat("1,", scrapePageSize) + `{"identifier":"x"}]}`
	if _, err := fetchServedBody(t, body, 0); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("err = %v", err)
	}
}

func TestHTTPFetcherRedirectIsNotRetried(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/elsewhere")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	_, err := (&HTTPFetcher{BaseURL: srv.URL, ClientFor: noRedirectClient}).FetchPage(context.Background(), "q", "")
	var se *StatusError
	if !errors.As(err, &se) || se.Status != http.StatusFound {
		t.Fatalf("err = %v", err)
	}
	if retry, _ := retryable(err); retry {
		t.Fatal("a redirect must not be retried")
	}
}

func TestHTTPFetcherCancelledContextIsNotRetried(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	defer close(release)
	f := &HTTPFetcher{BaseURL: srv.URL, ClientFor: plainClient}

	t.Run("cancelled before the call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := f.FetchPage(ctx, "q", "")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
		if retry, _ := retryable(err); retry {
			t.Fatal("cancellation must not be retried")
		}
	})

	t.Run("cancelled while the request is in flight", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			<-started
			cancel()
		}()
		_, err := f.FetchPage(ctx, "q", "")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
		if retry, _ := retryable(err); retry {
			t.Fatal("cancellation must not be retried")
		}
	})
}

func TestHTTPFetcherSanitizesAPIErrorOn200(t *testing.T) {
	body, err := json.Marshal(map[string]string{"error": "x\x1b[2Jclear\U0000202e  and\ttabs"})
	if err != nil {
		t.Fatal(err)
	}
	_, fetchErr := fetchServedBody(t, string(body), 0)
	if fetchErr == nil {
		t.Fatal("error payload accepted")
	}
	msg := fetchErr.Error()
	if strings.ContainsRune(msg, 0x1b) || strings.ContainsRune(msg, '\U0000202e') || strings.Contains(msg, "\t") {
		t.Fatalf("unsanitized error text: %q", msg)
	}
	if !strings.Contains(msg, "clear") {
		t.Fatalf("error text lost: %q", msg)
	}
}

func TestHTTPFetcherLargeErrorPageStaysAStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("<html>" + strings.Repeat("x", 1<<20) + "</html>"))
	}))
	defer srv.Close()
	// MaxBytes is far below the error page: the status must win over the size limit.
	_, err := (&HTTPFetcher{BaseURL: srv.URL, ClientFor: plainClient, MaxBytes: 1024}).FetchPage(context.Background(), "q", "")
	var se *StatusError
	if !errors.As(err, &se) || se.Status != http.StatusServiceUnavailable {
		t.Fatalf("err = %v", err)
	}
	if retry, _ := retryable(err); !retry {
		t.Fatal("a 503 must stay retryable")
	}
}
