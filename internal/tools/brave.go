package tools

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"aurago/internal/security"
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
func ExecuteBraveSearch(apiKey, query string, count int, country, lang string, contexts ...context.Context) string {
	if apiKey == "" {
		return formatError("Brave Search API key is missing. Set it in Settings › Brave Search (the key is stored securely in the vault).")
	}
	if query == "" {
		return formatError("query is required")
	}
	page, err := SearchBrave(requestContext(contexts), apiKey, BraveSearchOptions{Query: query, Count: count, Country: country, Language: lang})
	if err != nil {
		var apiErr *BraveSearchError
		if errors.As(err, &apiErr) && apiErr.nativeMessage != "" {
			return formatError(apiErr.nativeMessage)
		}
		return formatError(err.Error())
	}
	results := make([]map[string]interface{}, 0, len(page.Results))
	for _, r := range page.Results {
		entry := map[string]interface{}{
			"title":       security.IsolateExternalData(r.Title),
			"url":         r.URL,
			"description": security.IsolateExternalData(r.Description),
		}
		if r.Published != "" {
			entry["published"] = r.Published
		}
		results = append(results, entry)
	}
	out := map[string]interface{}{"status": "success", "query": query, "result_count": len(results), "results": results}
	if len(results) == 0 {
		out["message"] = "No results found."
	}
	b, _ := json.Marshal(out)
	return string(b)
}
