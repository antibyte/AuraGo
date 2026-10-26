package tools

import (
	"context"
	"fmt"
)

// DispatchYepAPISEO handles SEO tool operations via YepAPI.
func DispatchYepAPISEO(ctx context.Context, client *YepAPIClient, operation string, args map[string]interface{}) (string, error) {
	ctx = WithYepAPIRequestContext(ctx, "yepapi_seo", operation)
	switch operation {
	case "keywords":
		keywords := stringSliceForOperation(args, "keywords", "keywords")
		if len(keywords) == 0 {
			return yepAPIFormatError("keywords operation requires a 'keywords' array"), nil
		}
		data, err := client.Post(ctx, "/v1/seo/keywords", map[string]interface{}{"keywords": keywords})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "keyword_ideas":
		seed := stringArgForOperation(args, "keyword_ideas", "seed")
		if seed == "" {
			return yepAPIFormatError("keyword_ideas operation requires a 'seed' string"), nil
		}
		data, err := client.Post(ctx, "/v1/seo/keywords/ideas", map[string]interface{}{"keyword": seed})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "domain_overview":
		domain, argErr := domainArgForOperation(args, "domain_overview", "domain")
		if argErr != nil {
			return yepAPIFormatError(argErr.Error()), nil
		}
		if domain == "" {
			return yepAPIFormatError("domain_overview operation requires a 'domain' string"), nil
		}
		data, err := client.Post(ctx, "/v1/seo/domain/overview", map[string]interface{}{"domain": domain})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "domain_keywords":
		domain, argErr := domainArgForOperation(args, "domain_keywords", "domain")
		if argErr != nil {
			return yepAPIFormatError(argErr.Error()), nil
		}
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
		if keywords := stringSliceForOperation(args, "competitors", "keywords"); len(keywords) > 0 {
			payload["keywords"] = keywords
		}
		data, err := client.Post(ctx, "/v1/seo/competitors/serp", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "backlinks":
		target, argErr := targetArgForOperation(args, "backlinks", "target")
		if argErr != nil {
			return yepAPIFormatError(argErr.Error()), nil
		}
		if target == "" {
			return yepAPIFormatError("backlinks operation requires a 'target' string (domain or URL)"), nil
		}
		data, err := client.Post(ctx, "/v1/seo/backlinks/summary", map[string]interface{}{"target": target})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "onpage":
		url, argErr := urlArgForOperation(args, "onpage", "url")
		if argErr != nil {
			return yepAPIFormatError(argErr.Error()), nil
		}
		if url == "" {
			return yepAPIFormatError("onpage operation requires a 'url' string"), nil
		}
		data, err := client.Post(ctx, "/v1/seo/onpage/instant", map[string]interface{}{"url": url})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "trends":
		keywords := stringSliceForOperation(args, "trends", "keywords")
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
