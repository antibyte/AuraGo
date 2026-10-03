package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// BraveSearchOptions extends server-owned research without changing the native
// brave_search tool schema or its existing callers.
type BraveSearchOptions struct {
	Query, Country, Language string
	Kind, Freshness          string
	Count, Offset            int
}

// BraveSearchHit contains untrusted source metadata, never instructions.
type BraveSearchHit struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Published   string `json:"page_age"`
}

type BraveSearchPage struct {
	Results          []BraveSearchHit
	More             bool
	NextRequestAfter time.Duration
}

// BraveSearchError exposes only bounded, locally selected diagnostic codes.
type BraveSearchError struct {
	StatusCode int
	Code       string
	Temporary  bool
	RetryAfter time.Duration
	// Retain the native tool's established diagnostic text without exposing
	// provider response bodies through the typed research error.
	nativeMessage string
}

func (e *BraveSearchError) Error() string {
	return fmt.Sprintf("Brave Search %s (HTTP %d)", e.Code, e.StatusCode)
}

// SearchBrave performs exactly one request. Callers own retry and query budgets.
func SearchBrave(ctx context.Context, apiKey string, opts BraveSearchOptions) (BraveSearchPage, error) {
	page := BraveSearchPage{}
	if apiKey == "" {
		return page, &BraveSearchError{Code: "key_missing"}
	}
	if strings.TrimSpace(opts.Query) == "" {
		return page, &BraveSearchError{Code: "query_missing"}
	}
	kind := opts.Kind
	if kind == "" {
		kind = "web"
	}
	if kind != "web" && kind != "news" {
		return page, &BraveSearchError{Code: "invalid_kind"}
	}
	if opts.Offset < 0 || opts.Offset > 9 {
		return page, &BraveSearchError{Code: "invalid_offset"}
	}
	if opts.Freshness != "" && opts.Freshness != "pd" && opts.Freshness != "pw" {
		return page, &BraveSearchError{Code: "invalid_freshness"}
	}
	if opts.Count <= 0 {
		opts.Count = 10
	}
	limit := 20
	if kind == "news" {
		limit = 50
	}
	opts.Count = min(opts.Count, limit)
	params := url.Values{"q": {opts.Query}, "count": {strconv.Itoa(opts.Count)}}
	if opts.Country != "" {
		params.Set("country", strings.ToUpper(opts.Country))
	}
	if value := braveNormalizeSearchLang(opts.Language); value != "" {
		params.Set("search_lang", value)
	}
	if value := braveNormalizeUILang(opts.Language); value != "" {
		params.Set("ui_lang", value)
	}
	if opts.Freshness != "" {
		params.Set("freshness", opts.Freshness)
	}
	if opts.Offset != 0 {
		params.Set("offset", strconv.Itoa(opts.Offset))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.search.brave.com/res/v1/"+kind+"/search?"+params.Encode(), nil)
	if err != nil {
		return page, fmt.Errorf("build Brave search: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Subscription-Token", apiKey)
	resp, err := braveHTTPClient.Do(req)
	if err != nil {
		return page, fmt.Errorf("request Brave search: %w", err)
	}
	defer resp.Body.Close()
	page.NextRequestAfter = braveNextRequestAfter(resp.Header)
	body, err := braveReadBody(resp)
	if err != nil {
		return page, fmt.Errorf("read Brave search: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := "request_failed"
		temporary := resp.StatusCode == 429 || resp.StatusCode >= 500
		nativeMessage := braveFormatAPIError(resp.StatusCode, body)
		var apiErr braveErrorResponse
		_ = json.Unmarshal(body, &apiErr)
		if apiErr.Error.Code == "SUBSCRIPTION_TOKEN_INVALID" {
			code, temporary = "access_denied", false
		}
		switch resp.StatusCode {
		case 401, 403:
			code = "access_denied"
			nativeMessage = "Brave Search API key is invalid or expired. Check your subscription."
		case 429:
			code = "rate_limited"
			nativeMessage = "Brave Search rate limit exceeded. Try again later."
			if apiErr.Error.Code == "QUOTA_LIMITED" {
				code, temporary = "quota_exhausted", false
			}
		}
		return page, &BraveSearchError{StatusCode: resp.StatusCode, Code: code, Temporary: temporary, RetryAfter: page.NextRequestAfter, nativeMessage: nativeMessage}
	}
	var response struct {
		Results []BraveSearchHit `json:"results"`
		Web     struct {
			Results []BraveSearchHit `json:"results"`
		} `json:"web"`
		Query struct {
			More bool `json:"more_results_available"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return page, fmt.Errorf("decode Brave search: %w", err)
	}
	page.Results, page.More = response.Web.Results, response.Query.More
	if kind == "news" {
		page.Results = response.Results
	}
	if len(page.Results) > opts.Count {
		page.Results = page.Results[:opts.Count]
	}
	for i := range page.Results {
		page.Results[i].Title = braveStripHTML(page.Results[i].Title)
		page.Results[i].Description = braveStripHTML(page.Results[i].Description)
	}
	return page, nil
}

func braveNextRequestAfter(header http.Header) time.Duration {
	// A conservative default also works with one-request-per-second plans.
	delay := time.Second
	remaining := strings.Split(header.Get("X-RateLimit-Remaining"), ",")
	reset := strings.Split(header.Get("X-RateLimit-Reset"), ",")
	for i, value := range remaining {
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n > 0 || i >= len(reset) {
			continue
		}
		if seconds, err := strconv.ParseFloat(strings.TrimSpace(reset[i]), 64); err == nil && seconds > 0 {
			delay = max(delay, time.Duration(min(seconds, 86400))*time.Second)
		}
	}
	if seconds, err := strconv.Atoi(header.Get("Retry-After")); err == nil && seconds > 0 {
		delay = max(delay, time.Duration(min(seconds, 86400))*time.Second)
	} else if at, err := http.ParseTime(header.Get("Retry-After")); err == nil {
		delay = max(delay, min(time.Until(at), 24*time.Hour))
	}
	return delay
}
