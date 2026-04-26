package tools

import (
	"context"
	"fmt"
)

// serpEngines maps friendly operation names to YepAPI endpoint paths.
var serpEngines = map[string]string{
	"google":            "/v1/serp/google",
	"google_images":     "/v1/serp/google-images",
	"google_news":       "/v1/serp/google-news",
	"google_maps":       "/v1/serp/google-maps",
	"google_datasets":   "/v1/serp/google-datasets",
	"google_autocomplete": "/v1/serp/google-autocomplete",
	"google_ads":        "/v1/serp/google-ads",
	"google_ai_mode":    "/v1/serp/google-ai-mode",
	"google_finance":    "/v1/serp/google-finance",
	"yahoo":             "/v1/serp/yahoo",
	"bing":              "/v1/serp/bing",
	"baidu":             "/v1/serp/baidu",
	"youtube":           "/v1/serp/youtube",
}

// DispatchYepAPISERP handles SERP tool operations via YepAPI.
func DispatchYepAPISERP(ctx context.Context, client *YepAPIClient, operation string, args map[string]interface{}) (string, error) {
	endpoint, ok := serpEngines[operation]
	if !ok {
		return "", fmt.Errorf("unknown yepapi_serp operation: %s", operation)
	}

	query, _ := args["query"].(string)
	if query == "" {
		return yepAPIFormatError("serp operations require a 'query' string"), nil
	}

	payload := map[string]interface{}{"query": query}

	if depth, ok := args["depth"].(float64); ok && depth > 0 {
		payload["depth"] = int(depth)
	}
	if location, ok := args["location"].(string); ok && location != "" {
		payload["location"] = location
	}
	if language, ok := args["language"].(string); ok && language != "" {
		payload["language"] = language
	}
	// google-maps specific parameters
	if operation == "google_maps" {
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["limit"] = int(limit)
		}
		if openNow, ok := args["open_now"].(bool); ok {
			payload["open_now"] = openNow
		}
	}

	data, err := client.Post(ctx, endpoint, payload)
	if err != nil {
		return "", err
	}
	return yepAPIFormatSuccess(data), nil
}
