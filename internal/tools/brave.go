package tools

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var braveHTTPClient = &http.Client{Timeout: 15 * time.Second}

var braveUILangMap = map[string]string{
	"de": "de-DE",
	"en": "en-US",
	"es": "es-ES",
	"fr": "fr-FR",
	"it": "it-IT",
	"ja": "ja-JP",
	"ko": "ko-KR",
	"nl": "nl-NL",
	"no": "no-NO",
	"pl": "pl-PL",
	"pt": "pt-BR",
	"sv": "sv-SE",
	"ru": "ru-RU",
	"zh": "zh-CN",
	"da": "da-DK",
	"fi": "fi-FI",
	"el": "el-GR",
}

// braveStripHTML removes HTML tags from a string.
// The Brave Search API returns descriptions with <strong> etc. markup.
var braveHTMLTag = regexp.MustCompile(`<[^>]+>`)

func braveStripHTML(s string) string {
	return strings.TrimSpace(braveHTMLTag.ReplaceAllString(s, ""))
}

func braveNormalizeSearchLang(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if lang == "" {
		return ""
	}

	switch lang {
	case "zh", "zh-cn", "zh-sg", "zh-hans":
		return "zh-hans"
	case "zh-tw", "zh-hk", "zh-mo", "zh-hant":
		return "zh-hant"
	case "ja", "no", "nb", "nn", "pt", "pt-br", "pt-pt":
		// Brave currently rejects these as search_lang values. We still send a
		// compatible ui_lang so the request stays localised where possible.
		return ""
	}

	if i := strings.Index(lang, "-"); i > 0 {
		lang = lang[:i]
	}
	return lang
}

func braveNormalizeUILang(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if lang == "" {
		return ""
	}
	if mapped, ok := braveUILangMap[lang]; ok {
		return mapped
	}
	if len(lang) == 5 && lang[2] == '-' {
		return lang[:2] + "-" + strings.ToUpper(lang[3:])
	}
	return ""
}

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

type braveErrorResponse struct {
	Error struct {
		Code   string `json:"code"`
		Detail string `json:"detail"`
		Status int    `json:"status"`
	} `json:"error"`
	Type string `json:"type"`
}

func braveReadBody(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress response: %v", err)
		}
		defer gz.Close()
		reader = gz
	}
	return io.ReadAll(io.LimitReader(reader, 256*1024))
}

func braveFormatAPIError(statusCode int, body []byte) string {
	var apiErr braveErrorResponse
	if err := json.Unmarshal(body, &apiErr); err == nil {
		code := strings.TrimSpace(apiErr.Error.Code)
		detail := strings.TrimSpace(apiErr.Error.Detail)
		switch code {
		case "SUBSCRIPTION_TOKEN_INVALID":
			return "Brave Search API key is invalid. Update the Brave Search subscription token in the vault/settings."
		}
		if detail != "" {
			if code != "" {
				return fmt.Sprintf("Brave Search error %s: %s", code, detail)
			}
			return fmt.Sprintf("Brave Search HTTP error %d: %s", statusCode, detail)
		}
		if code != "" {
			return fmt.Sprintf("Brave Search error %s (HTTP %d)", code, statusCode)
		}
	}
	return fmt.Sprintf("Brave Search HTTP error %d", statusCode)
}

// ExecuteBraveSearch queries the Brave Search API and returns structured results.
// apiKey is the Brave API subscription token.
// query is the search query.
// count is the number of results (1-20; 0 defaults to 10).
// country is the two-letter country code for localised results (e.g. "DE", "US"; empty = global).
// lang is the search language code (e.g. "de", "en"; empty = default).
func ExecuteBraveSearch(apiKey, query string, count int, country, lang string) string {
	if apiKey == "" {
		return formatError("Brave Search API key is missing. Set it in Settings › Brave Search (the key is stored securely in the vault).")
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
		if searchLang := braveNormalizeSearchLang(lang); searchLang != "" {
			params.Set("search_lang", searchLang)
		}
		if uiLang := braveNormalizeUILang(lang); uiLang != "" {
			params.Set("ui_lang", uiLang)
		}
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

	bodyBytes, err := braveReadBody(resp)
	if err != nil {
		return formatError(err.Error())
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return formatError("Brave Search API key is invalid or expired. Check your subscription.")
	}
	if resp.StatusCode == 429 {
		return formatError("Brave Search rate limit exceeded. Try again later.")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return formatError(braveFormatAPIError(resp.StatusCode, bodyBytes))
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
			"title":       fmt.Sprintf("<external_data>%s</external_data>", braveStripHTML(r.Title)),
			"url":         r.URL,
			"description": fmt.Sprintf("<external_data>%s</external_data>", braveStripHTML(r.Description)),
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
