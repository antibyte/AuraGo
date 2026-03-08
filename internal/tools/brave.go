package tools

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var braveHTTPClient = &http.Client{Timeout: 15 * time.Second}

// braveWebResult is a single search result from the Brave API.
type braveWebResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Published   string `json:"page_age,omitempty"`
}

// braveResponse mirrors the top-level Brave Search API response.
type braveResponse struct {
	Web struct {
		Results []braveWebResult `json:"results"`
	} `json:"web"`
	Mixed struct {
		Main []struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
		} `json:"main"`
	} `json:"mixed"`
}

// ExecuteBraveSearch queries the Brave Search API and returns structured results.
// apiKey is the Brave API subscription token.
// query is the search query.
// count is the number of results (1-20; 0 defaults to 10).
// country is the two-letter country code for localised results (e.g. "DE", "US"; empty = global).
// lang is the search language code (e.g. "de", "en"; empty = default).
func ExecuteBraveSearch(apiKey, query string, count int, country, lang string) string {
	if apiKey == "" {
		return formatError("Brave Search API key is missing. Configure it under brave_search.api_key in the settings.")
	}
	if query == "" {
		return formatError("query is required")
	}
	if count <= 0 {
		count = 10
	}
	if count > 20 {
		count = 20
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("count", fmt.Sprintf("%d", count))
	if country != "" {
		params.Set("country", strings.ToUpper(country))
	}
	if lang != "" {
		params.Set("search_lang", strings.ToLower(lang))
		params.Set("ui_lang", strings.ToLower(lang))
	}

	endpoint := "https://api.search.brave.com/res/v1/web/search?" + params.Encode()

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return formatError(fmt.Sprintf("failed to build request: %v", err))
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Subscription-Token", apiKey)

	resp, err := braveHTTPClient.Do(req)
	if err != nil {
		return formatError(fmt.Sprintf("Brave Search request failed: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return formatError("Brave Search API key is invalid or expired. Check your subscription.")
	}
	if resp.StatusCode == 429 {
		return formatError("Brave Search rate limit exceeded. Try again later.")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return formatError(fmt.Sprintf("Brave Search HTTP error %d", resp.StatusCode))
	}

	// Handle optional gzip encoding
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return formatError(fmt.Sprintf("failed to decompress response: %v", err))
		}
		defer gz.Close()
		reader = gz
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(reader, 256*1024))
	if err != nil {
		return formatError(fmt.Sprintf("failed to read response: %v", err))
	}

	var apiResp braveResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return formatError(fmt.Sprintf("failed to parse Brave response: %v", err))
	}

	if len(apiResp.Web.Results) == 0 {
		b, _ := json.Marshal(map[string]interface{}{
			"status":  "success",
			"results": []interface{}{},
			"message": "No results found.",
		})
		return string(b)
	}

	results := make([]map[string]interface{}, 0, len(apiResp.Web.Results))
	for _, r := range apiResp.Web.Results {
		entry := map[string]interface{}{
			"title":       fmt.Sprintf("<external_data>%s</external_data>", r.Title),
			"url":         r.URL,
			"description": fmt.Sprintf("<external_data>%s</external_data>", r.Description),
		}
		if r.Published != "" {
			entry["published"] = r.Published
		}
		results = append(results, entry)
	}

	out := map[string]interface{}{
		"status":       "success",
		"query":        query,
		"result_count": len(results),
		"results":      results,
	}
	b, _ := json.Marshal(out)
	return string(b)
}
