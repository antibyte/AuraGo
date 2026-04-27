package tools

import (
	"context"
	"fmt"
)

// DispatchYepAPIAmazon handles Amazon data operations via YepAPI.
func DispatchYepAPIAmazon(ctx context.Context, client *YepAPIClient, operation string, args map[string]interface{}) (string, error) {
	switch operation {
	case "search":
		query, _ := args["query"].(string)
		if query == "" {
			return yepAPIFormatError("search operation requires a 'query' string"), nil
		}
		payload := map[string]interface{}{"query": query}
		if country, ok := args["country"].(string); ok && country != "" {
			payload["country"] = country
		} else {
			payload["country"] = "US"
		}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["page"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/amazon/search", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "product":
		asin := stringArgWithFallback(args, "asin")
		if asin == "" {
			return yepAPIFormatError("product operation requires an 'asin' string"), nil
		}
		payload := map[string]interface{}{"asin": asin}
		if country, ok := args["country"].(string); ok && country != "" {
			payload["country"] = country
		} else {
			payload["country"] = "US"
		}
		data, err := client.Post(ctx, "/v1/amazon/product", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "reviews":
		asin := stringArgWithFallback(args, "asin")
		if asin == "" {
			return yepAPIFormatError("reviews operation requires an 'asin' string"), nil
		}
		payload := map[string]interface{}{"asin": asin}
		if country, ok := args["country"].(string); ok && country != "" {
			payload["country"] = country
		} else {
			payload["country"] = "US"
		}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["page"] = int(limit)
		}
		if sortBy, ok := args["sort_by"].(string); ok && sortBy != "" {
			payload["sort_by"] = sortBy
		}
		data, err := client.Post(ctx, "/v1/amazon/product-reviews", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "deals":
		payload := map[string]interface{}{}
		if country, ok := args["country"].(string); ok && country != "" {
			payload["country"] = country
		} else {
			payload["country"] = "US"
		}
		if category, ok := args["category"].(string); ok && category != "" {
			payload["category"] = category
		}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["page"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/amazon/deals", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "best_sellers":
		payload := map[string]interface{}{}
		if country, ok := args["country"].(string); ok && country != "" {
			payload["country"] = country
		} else {
			payload["country"] = "US"
		}
		if category, ok := args["category"].(string); ok && category != "" {
			payload["category"] = category
		}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["page"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/amazon/best-sellers", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	default:
		return "", fmt.Errorf("unknown yepapi_amazon operation: %s", operation)
	}
}
