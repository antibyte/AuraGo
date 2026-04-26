package tools

import (
	"context"
	"fmt"
)

// DispatchYepAPISEO handles SEO tool operations via YepAPI.
func DispatchYepAPISEO(ctx context.Context, client *YepAPIClient, operation string, args map[string]interface{}) (string, error) {
	switch operation {
	case "keywords":
		keywords := stringSliceFromArgs(args, "keywords")
		if len(keywords) == 0 {
			return yepAPIFormatError("keywords operation requires a 'keywords' array"), nil
		}
		data, err := client.Post(ctx, "/v1/seo/keywords", map[string]interface{}{"keywords": keywords})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "keyword_ideas":
		seed, _ := args["seed"].(string)
		if seed == "" {
			return yepAPIFormatError("keyword_ideas operation requires a 'seed' string"), nil
		}
		data, err := client.Post(ctx, "/v1/seo/keywords/ideas", map[string]interface{}{"keyword": seed})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "domain_overview":
		domain, _ := args["domain"].(string)
		if domain == "" {
			return yepAPIFormatError("domain_overview operation requires a 'domain' string"), nil
		}
		data, err := client.Post(ctx, "/v1/seo/domain/overview", map[string]interface{}{"domain": domain})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "domain_keywords":
		domain, _ := args["domain"].(string)
		if domain == "" {
			return yepAPIFormatError("domain_keywords operation requires a 'domain' string"), nil
		}
		data, err := client.Post(ctx, "/v1/seo/domain/keywords", map[string]interface{}{"domain": domain})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "competitors":
		payload := map[string]interface{}{}
		if domain, ok := args["domain"].(string); ok && domain != "" {
			payload["domain"] = domain
		}
		if keywords := stringSliceFromArgs(args, "keywords"); len(keywords) > 0 {
			payload["keywords"] = keywords
		}
		data, err := client.Post(ctx, "/v1/seo/competitors/serp", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "backlinks":
		target, _ := args["target"].(string)
		if target == "" {
			return yepAPIFormatError("backlinks operation requires a 'target' string (domain or URL)"), nil
		}
		data, err := client.Post(ctx, "/v1/seo/backlinks/summary", map[string]interface{}{"target": target})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "onpage":
		url, _ := args["url"].(string)
		if url == "" {
			return yepAPIFormatError("onpage operation requires a 'url' string"), nil
		}
		data, err := client.Post(ctx, "/v1/seo/onpage/instant", map[string]interface{}{"url": url})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "trends":
		keywords := stringSliceFromArgs(args, "keywords")
		if len(keywords) == 0 {
			return yepAPIFormatError("trends operation requires a 'keywords' array (up to 5)"), nil
		}
		data, err := client.Post(ctx, "/v1/seo/trends", map[string]interface{}{"keywords": keywords})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	default:
		return "", fmt.Errorf("unknown yepapi_seo operation: %s", operation)
	}
}
