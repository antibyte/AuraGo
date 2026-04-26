package tools

import (
	"context"
	"fmt"
)

// DispatchYepAPIInstagram handles Instagram data operations via YepAPI.
func DispatchYepAPIInstagram(ctx context.Context, client *YepAPIClient, operation string, args map[string]interface{}) (string, error) {
	switch operation {
	case "search":
		query, _ := args["query"].(string)
		if query == "" {
			return yepAPIFormatError("search operation requires a 'query' string"), nil
		}
		data, err := client.Post(ctx, "/v1/instagram/search", map[string]interface{}{"query": query})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "user":
		username := stringArgWithFallback(args, "username")
		if username == "" {
			return yepAPIFormatError("user operation requires a 'username' string"), nil
		}
		data, err := client.Post(ctx, "/v1/instagram/user", map[string]interface{}{"username_or_url": username})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "user_posts":
		username := stringArgWithFallback(args, "username")
		if username == "" {
			return yepAPIFormatError("user_posts operation requires a 'username' string"), nil
		}
		payload := map[string]interface{}{"username_or_url": username}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["limit"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/instagram/user-posts", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "user_reels":
		username := stringArgWithFallback(args, "username")
		if username == "" {
			return yepAPIFormatError("user_reels operation requires a 'username' string"), nil
		}
		payload := map[string]interface{}{"username_or_url": username}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["limit"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/instagram/user-reels", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "post":
		shortcode := stringArgWithFallback(args, "shortcode")
		if shortcode == "" {
			return yepAPIFormatError("post operation requires a 'shortcode' string"), nil
		}
		data, err := client.Post(ctx, "/v1/instagram/post", map[string]interface{}{"shortcode": shortcode})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "post_comments":
		shortcode := stringArgWithFallback(args, "shortcode")
		if shortcode == "" {
			return yepAPIFormatError("post_comments operation requires a 'shortcode' string"), nil
		}
		payload := map[string]interface{}{"shortcode": shortcode}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["limit"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/instagram/post-comments", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "hashtag":
		tag := stringArgWithFallback(args, "tag")
		if tag == "" {
			return yepAPIFormatError("hashtag operation requires a 'tag' string (without #)"), nil
		}
		data, err := client.Post(ctx, "/v1/instagram/hashtag", map[string]interface{}{"tag": tag})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	default:
		return "", fmt.Errorf("unknown yepapi_instagram operation: %s", operation)
	}
}
