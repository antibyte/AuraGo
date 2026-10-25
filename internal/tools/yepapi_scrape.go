package tools

import (
	"context"
	"fmt"
)

// DispatchYepAPIScrape handles web scraping operations via YepAPI.
func DispatchYepAPIScrape(ctx context.Context, client *YepAPIClient, operation string, args map[string]interface{}) (string, error) {
	ctx = WithYepAPIRequestContext(ctx, "yepapi_scrape", operation)
	switch operation {
	case "scrape":
		payload, ok, msg := scrapeURLPayload(args, operation)
		if !ok {
			return yepAPIFormatError(msg), nil
		}
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
		payload, ok, msg := scrapeURLPayload(args, operation)
		if !ok {
			return yepAPIFormatError(msg), nil
		}
		if format, ok := args["format"].(string); ok && format != "" {
			payload["format"] = format
		}
		data, err := client.Post(ctx, "/v1/scrape/js", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "stealth":
		payload, ok, msg := scrapeURLPayload(args, operation)
		if !ok {
			return yepAPIFormatError(msg), nil
		}
		if format, ok := args["format"].(string); ok && format != "" {
			payload["format"] = format
		}
		data, err := client.Post(ctx, "/v1/scrape/stealth", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "screenshot":
		payload, ok, msg := scrapeURLPayload(args, operation)
		if !ok {
			return yepAPIFormatError(msg), nil
		}
		data, err := client.Post(ctx, "/v1/scrape/screenshot", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "extract", "ai_extract":
		payload, ok, msg := scrapeURLPayload(args, operation)
		if !ok {
			return yepAPIFormatError(msg), nil
		}
		endpoint := "/v1/scrape/extract"
		if operation == "ai_extract" {
			endpoint = "/v1/scrape/ai-extract"
			if prompt, ok := args["prompt"].(string); ok && prompt != "" {
				payload["prompt"] = prompt
			} else {
				return yepAPIFormatError("ai_extract operation requires a 'prompt' string describing what to extract"), nil
			}
		} else {
			addOptionalStringArg(payload, args, "selector", "selector")
			addOptionalStringArg(payload, args, "xpath", "xpath")
			if _, hasSelector := payload["selector"]; !hasSelector {
				if _, hasXPath := payload["xpath"]; !hasXPath {
					return yepAPIFormatError("extract operation requires a 'selector' or 'xpath' string"), nil
				}
			}
		}
		data, err := client.Post(ctx, endpoint, payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "search_google":
		query, _ := args["query"].(string)
		if query == "" {
			return yepAPIFormatError("search_google operation requires a 'query' string"), nil
		}
		payload := map[string]interface{}{"query": query}
		addPositiveIntArg(payload, args, "limit", "limit")
		data, err := client.Post(ctx, "/v1/search/google", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	default:
		return "", fmt.Errorf("unknown yepapi_scrape operation: %s", operation)
	}
}

func scrapeURLPayload(args map[string]interface{}, operation string) (map[string]interface{}, bool, string) {
	url, err := urlArgForOperation(args, operation, "url")
	if err != nil {
		return nil, false, err.Error()
	}
	if url == "" {
		return nil, false, "scrape operations require a 'url' string"
	}
	return map[string]interface{}{"url": url}, true, ""
}
