package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/i18n"
	"aurago/internal/security"
)

// wikipediaHTTPClient is a shared client with connection pooling.
var wikipediaHTTPClient = &http.Client{Timeout: 10 * time.Second}

var wikipediaBaseURLForLang = func(lang string) string {
	return fmt.Sprintf("https://%s.wikipedia.org", lang)
}

var errWikipediaNotFound = errors.New("wikipedia article not found")

// formatWikipediaError returns a JSON error response.
func formatWikipediaError(msg string) string {
	result := map[string]interface{}{
		"status":  "error",
		"message": msg,
	}
	b, _ := json.Marshal(result)
	return string(b)
}

// ExecuteWikipediaSearch queries the Wikipedia REST API for page summaries
func ExecuteWikipediaSearch(query, lang string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return formatWikipediaError("Wikipedia query must not be empty.")
	}
	lang = wikipediaNormalizeLang(lang)

	data, err := fetchWikipediaSummary(lang, query)
	if errors.Is(err, errWikipediaNotFound) {
		bestTitle, searchErr := searchWikipediaTitle(lang, query)
		if searchErr != nil {
			return formatWikipediaError(fmt.Sprintf("No Wikipedia article found for '%s'.", query))
		}
		data, err = fetchWikipediaSummary(lang, bestTitle)
	}
	if err != nil {
		if errors.Is(err, errWikipediaNotFound) {
			return formatWikipediaError(fmt.Sprintf("No Wikipedia article found for '%s'.", query))
		}
		return formatWikipediaError(err.Error())
	}

	// Check for disambiguation
	if t, ok := data["type"].(string); ok && t == "disambiguation" {
		return formatWikipediaError(fmt.Sprintf("Der Begriff '%s' ist mehrdeutig. Bitte präzisiere deine Suche.", query))
	}

	title, _ := data["title"].(string)
	var pageURL string
	if cu, ok := data["content_urls"].(map[string]interface{}); ok {
		if desktop, ok := cu["desktop"].(map[string]interface{}); ok {
			pageURL, _ = desktop["page"].(string)
		}
	}
	summary, _ := data["extract"].(string)

	result := map[string]interface{}{
		"status":  "success",
		"title":   security.IsolateExternalData(title),
		"url":     pageURL,
		"summary": security.IsolateExternalData(summary),
		"content": security.IsolateExternalData(summary), // Summary acts as content, limit is well within 15k
	}

	b, _ := json.Marshal(result)
	return string(b)
}

func wikipediaNormalizeLang(lang string) string {
	normalized := strings.ToLower(strings.TrimSpace(lang))
	if normalized == "" {
		return "en"
	}
	normalized = strings.ReplaceAll(normalized, "_", "-")
	if idx := strings.Index(normalized, "-"); idx > 0 {
		normalized = normalized[:idx]
	}
	if len(normalized) == 2 {
		return normalized
	}
	return i18n.NormalizeLang(normalized)
}

func fetchWikipediaSummary(lang, query string) (map[string]interface{}, error) {
	apiURL := fmt.Sprintf("%s/api/rest_v1/page/summary/%s", wikipediaBaseURLForLang(lang), url.PathEscape(query))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "AuraGo/1.0 (Integration)")

	resp, err := wikipediaHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Wikipedia request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, errWikipediaNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Wikipedia HTTP Error %d", resp.StatusCode)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to parse Wikipedia response: %w", err)
	}
	return data, nil
}

func searchWikipediaTitle(lang, query string) (string, error) {
	apiURL := fmt.Sprintf("%s/w/api.php?action=query&format=json&list=search&srlimit=1&srsearch=%s",
		wikipediaBaseURLForLang(lang),
		url.QueryEscape(query),
	)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create search request: %w", err)
	}
	req.Header.Set("User-Agent", "AuraGo/1.0 (Integration)")

	resp, err := wikipediaHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Wikipedia search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Wikipedia search HTTP Error %d", resp.StatusCode)
	}

	var data struct {
		Query struct {
			Search []struct {
				Title string `json:"title"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("failed to parse Wikipedia search response: %w", err)
	}
	if len(data.Query.Search) == 0 || strings.TrimSpace(data.Query.Search[0].Title) == "" {
		return "", errWikipediaNotFound
	}
	return data.Query.Search[0].Title, nil
}
