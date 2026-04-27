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
		addCountryArg(payload, args)
		addPositiveIntArg(payload, args, "limit", "limit")
		addPositiveIntArg(payload, args, "page", "page")
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
		addCountryArg(payload, args)
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
		addCountryArg(payload, args)
		addPositiveIntArg(payload, args, "limit", "limit")
		addPositiveIntArg(payload, args, "page", "page")
		if sortBy, ok := args["sort_by"].(string); ok && sortBy != "" {
			payload["sort_by"] = sortBy
		}
		data, err := client.Post(ctx, "/v1/amazon/product-reviews", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "product_offers":
		asin := stringArgWithFallback(args, "asin")
		if asin == "" {
			return yepAPIFormatError("product_offers operation requires an 'asin' string"), nil
		}
		payload := map[string]interface{}{"asin": asin}
		addCountryArg(payload, args)
		addPositiveIntArg(payload, args, "limit", "limit")
		addPositiveIntArg(payload, args, "page", "page")
		data, err := client.Post(ctx, "/v1/amazon/product-offers", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "products_by_category":
		category := stringArgWithFallback(args, "category")
		if category == "" {
			return yepAPIFormatError("products_by_category operation requires a 'category' string"), nil
		}
		payload := map[string]interface{}{"category": category}
		addCountryArg(payload, args)
		addPositiveIntArg(payload, args, "limit", "limit")
		addPositiveIntArg(payload, args, "page", "page")
		data, err := client.Post(ctx, "/v1/amazon/products-by-category", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "categories":
		payload := map[string]interface{}{}
		addCountryArg(payload, args)
		data, err := client.Post(ctx, "/v1/amazon/categories", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "deals":
		payload := map[string]interface{}{}
		addCountryArg(payload, args)
		if category, ok := args["category"].(string); ok && category != "" {
			payload["category"] = category
		}
		addPositiveIntArg(payload, args, "limit", "limit")
		addPositiveIntArg(payload, args, "page", "page")
		data, err := client.Post(ctx, "/v1/amazon/deals", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "best_sellers":
		payload := map[string]interface{}{}
		addCountryArg(payload, args)
		if category, ok := args["category"].(string); ok && category != "" {
			payload["category"] = category
		}
		addPositiveIntArg(payload, args, "limit", "limit")
		addPositiveIntArg(payload, args, "page", "page")
		data, err := client.Post(ctx, "/v1/amazon/best-sellers", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "influencer":
		handle := stringArgWithFallback(args, "handle")
		if handle == "" {
			return yepAPIFormatError("influencer operation requires a 'handle' string"), nil
		}
		payload := map[string]interface{}{"handle": handle}
		addCountryArg(payload, args)
		data, err := client.Post(ctx, "/v1/amazon/influencer", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "seller":
		sellerID := stringArgWithFallback(args, "seller_id")
		if sellerID == "" {
			return yepAPIFormatError("seller operation requires a 'seller_id' string"), nil
		}
		payload := map[string]interface{}{"seller_id": sellerID}
		addCountryArg(payload, args)
		data, err := client.Post(ctx, "/v1/amazon/seller", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "seller_reviews":
		sellerID := stringArgWithFallback(args, "seller_id")
		if sellerID == "" {
			return yepAPIFormatError("seller_reviews operation requires a 'seller_id' string"), nil
		}
		payload := map[string]interface{}{"seller_id": sellerID}
		addCountryArg(payload, args)
		addPositiveIntArg(payload, args, "limit", "limit")
		addPositiveIntArg(payload, args, "page", "page")
		data, err := client.Post(ctx, "/v1/amazon/seller-reviews", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	default:
		return "", fmt.Errorf("unknown yepapi_amazon operation: %s", operation)
	}
}
