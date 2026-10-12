package retrogames

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/security"
)

const (
	scrapeURL           = "https://archive.org/services/search/v1/scrape"
	scrapeFields        = "identifier,title,year,creator,downloads,emulator,subject"
	scrapePageSize      = 10000
	defaultMaxPageBytes = 32 << 20
	maxErrorBodyBytes   = 64 << 10
	maxRetryAfter       = time.Hour
	requestTimeout      = 30 * time.Second
	userAgent           = "AuraGo-RetroGames/1 (+https://github.com/antibyte/AuraGo)"
)

// Page is one page of scrape results.
type Page struct {
	Items   []Item
	Cursor  string
	Total   int
	Invalid int
}

// Fetcher loads one page of archive.org scrape results.
type Fetcher interface {
	FetchPage(ctx context.Context, query, cursor string) (Page, error)
}

// HTTPFetcher talks to the archive.org scrape API with AuraGo's strict
// public-internet client (DNS pinned, no private addresses, no redirects).
type HTTPFetcher struct {
	BaseURL   string
	ClientFor func(rawURL string, timeout time.Duration) (*http.Client, error)
	MaxBytes  int64
}

// NewHTTPFetcher returns the production fetcher.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{BaseURL: scrapeURL, ClientFor: security.NewStrictPublicHTTPClientForURL, MaxBytes: defaultMaxPageBytes}
}

// StatusError is a non-200 answer of the scrape API.
type StatusError struct {
	Status     int
	RetryAfter time.Duration
	Message    string
}

func (e *StatusError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("retrogames: archive.org answered HTTP %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("retrogames: archive.org answered HTTP %d", e.Status)
}

type transientError struct{ err error }

func (e *transientError) Error() string { return "retrogames: request failed: " + e.err.Error() }
func (e *transientError) Unwrap() error { return e.err }

// FetchPage loads one scrape page for query, continuing at cursor.
func (f *HTTPFetcher) FetchPage(ctx context.Context, query, cursor string) (Page, error) {
	values := url.Values{}
	values.Set("q", query)
	values.Set("fields", scrapeFields)
	values.Set("count", strconv.Itoa(scrapePageSize))
	if cursor != "" {
		values.Set("cursor", cursor)
	}
	rawURL := f.BaseURL + "?" + values.Encode()
	clientFor := f.ClientFor
	if clientFor == nil {
		clientFor = security.NewStrictPublicHTTPClientForURL
	}
	client, err := clientFor(rawURL, requestTimeout)
	if err != nil {
		return Page{}, &transientError{err: err}
	}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Page{}, fmt.Errorf("retrogames: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Page{}, ctxErr
		}
		return Page{}, &transientError{err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Judge by status first: a huge error page must stay a (retryable)
		// StatusError instead of tripping the success-body size limit.
		errBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		if readErr != nil && ctx.Err() != nil {
			return Page{}, ctx.Err()
		}
		return Page{}, &StatusError{Status: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()), Message: scrapeErrorMessage(errBody)}
	}
	limit := f.MaxBytes
	if limit <= 0 {
		limit = defaultMaxPageBytes
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Page{}, ctxErr
		}
		return Page{}, &transientError{err: err}
	}
	if int64(len(body)) > limit {
		return Page{}, fmt.Errorf("retrogames: scrape page exceeds %d bytes", limit)
	}
	return decodePage(body)
}

// decodePage streams the scrape JSON so a hostile body cannot make the
// decoder materialise millions of elements: at most scrapePageSize items are
// looked at, each one decoded on its own.
func decodePage(body []byte) (Page, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	fail := func(err error) (Page, error) {
		return Page{}, fmt.Errorf("retrogames: decode scrape page: %w", err)
	}
	tok, err := dec.Token()
	if err != nil {
		return fail(err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fail(errors.New("expected a JSON object"))
	}
	page := Page{}
	var (
		hasItems bool
		apiError string
		raw      json.RawMessage
	)
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return fail(err)
		}
		key, _ := keyTok.(string)
		switch key {
		case "items":
			tok, err := dec.Token()
			if err != nil {
				return fail(err)
			}
			if d, ok := tok.(json.Delim); ok && d == '{' {
				// An object where the item array belongs: the page is unusable.
				return Page{}, errors.New("retrogames: scrape page has no items")
			}
			if d, ok := tok.(json.Delim); !ok || d != '[' {
				continue // null or a scalar: treated as missing
			}
			hasItems = true
			page.Items = make([]Item, 0, 64)
			count := 0
			for dec.More() {
				if count == scrapePageSize {
					return Page{}, fmt.Errorf("retrogames: scrape page has more than %d items", scrapePageSize)
				}
				count++
				if err := dec.Decode(&raw); err != nil {
					return fail(err)
				}
				var it Item
				if err := json.Unmarshal(raw, &it); err != nil {
					page.Invalid++
					continue
				}
				page.Items = append(page.Items, it)
			}
			if _, err := dec.Token(); err != nil { // the closing ']'
				return fail(err)
			}
		case "cursor":
			if err := dec.Decode(&page.Cursor); err != nil {
				return fail(err)
			}
		case "total":
			if err := dec.Decode(&page.Total); err != nil {
				return fail(err)
			}
		case "error":
			if err := dec.Decode(&apiError); err != nil {
				return fail(err)
			}
		default:
			if err := dec.Decode(&raw); err != nil {
				return fail(err)
			}
		}
	}
	if _, err := dec.Token(); err != nil { // the closing '}'
		return fail(err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return fail(errors.New("unexpected data after the JSON object"))
	}
	if apiError != "" {
		return Page{}, fmt.Errorf("retrogames: archive.org error: %s", truncateRunes(collapseSpace(stripControl(apiError)), 200))
	}
	if !hasItems {
		return Page{}, errors.New("retrogames: scrape page has no items")
	}
	return page, nil
}

func scrapeErrorMessage(body []byte) string {
	var v struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &v) != nil {
		return ""
	}
	return truncateRunes(collapseSpace(stripControl(v.Error)), 200)
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	// Clamp before converting to a Duration so a huge value cannot overflow.
	secs, err := strconv.ParseInt(value, 10, 64)
	if err == nil || errors.Is(err, strconv.ErrRange) {
		if secs <= 0 {
			return 0
		}
		if secs >= int64(maxRetryAfter/time.Second) {
			return maxRetryAfter
		}
		return time.Duration(secs) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		if wait := at.Sub(now); wait < maxRetryAfter {
			return wait
		}
		return maxRetryAfter
	}
	return 0
}

// retryable reports whether err is worth another attempt and how long the
// server asked to wait (0 = use the default backoff).
func retryable(err error) (bool, time.Duration) {
	var se *StatusError
	if errors.As(err, &se) {
		switch se.Status {
		case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true, se.RetryAfter
		}
		return false, 0
	}
	var te *transientError
	if errors.As(err, &te) {
		return true, 0
	}
	return false, 0
}
