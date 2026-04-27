package tools

import (
	"context"
	"fmt"
	neturl "net/url"
	"strings"
)

// DispatchYepAPIScrape handles web scraping operations via YepAPI.
func DispatchYepAPIScrape(ctx context.Context, client *YepAPIClient, operation string, args map[string]interface{}) (string, error) {
	url := stringArgWithFallback(args, "url")
	if url == "" {
		return yepAPIFormatError("scrape operations require a 'url' string"), nil
	}
	url = strings.TrimSpace(url)
	parsedURL, err := neturl.ParseRequestURI(url)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return yepAPIFormatError("scrape operations require a valid http:// or https:// URL with a host"), nil
	}

	payload := map[string]interface{}{"url": url}

	switch operation {
	case "scrape":
		if format, ok := args["format"].(string); ok && format != "" {
			payload["format"] = format // "markdown" or "html"
		} else {
			payload["format"] = "markdown"
		}
		data, err := client.Post(ctx, "/v1/scrape", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "js":
		if format, ok := args["format"].(string); ok && format != "" {
			payload["format"] = format
		}
		data, err := client.Post(ctx, "/v1/scrape/js", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "stealth":
		if format, ok := args["format"].(string); ok && format != "" {
			payload["format"] = format
		}
		data, err := client.Post(ctx, "/v1/scrape/stealth", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "screenshot":
		data, err := client.Post(ctx, "/v1/scrape/screenshot", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "ai_extract":
		if prompt, ok := args["prompt"].(string); ok && prompt != "" {
			payload["prompt"] = prompt
		} else {
			return yepAPIFormatError("ai_extract operation requires a 'prompt' string describing what to extract"), nil
		}
		data, err := client.Post(ctx, "/v1/scrape/ai-extract", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	default:
		return "", fmt.Errorf("unknown yepapi_scrape operation: %s", operation)
	}
}
