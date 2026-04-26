package tools

import (
	"context"
	"fmt"
)

// DispatchYepAPITikTok handles TikTok data operations via YepAPI.
func DispatchYepAPITikTok(ctx context.Context, client *YepAPIClient, operation string, args map[string]interface{}) (string, error) {
	switch operation {
	case "search":
		query, _ := args["query"].(string)
		if query == "" {
			return yepAPIFormatError("search operation requires a 'query' string"), nil
		}
		payload := map[string]interface{}{"keywords": query}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["count"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/tiktok/search", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "search_user":
		query, _ := args["query"].(string)
		if query == "" {
			return yepAPIFormatError("search_user operation requires a 'query' string"), nil
		}
		payload := map[string]interface{}{"keyword": query}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["count"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/tiktok/search-user", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "video":
		videoURL, _ := args["url"].(string)
		if videoURL == "" {
			return yepAPIFormatError("video operation requires a 'url' string (TikTok video URL)"), nil
		}
		data, err := client.Post(ctx, "/v1/tiktok/video", map[string]interface{}{"url": videoURL})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "user":
		username, _ := args["username"].(string)
		if username == "" {
			return yepAPIFormatError("user operation requires a 'username' string"), nil
		}
		data, err := client.Post(ctx, "/v1/tiktok/user", map[string]interface{}{"unique_id": username})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "user_posts":
		username, _ := args["username"].(string)
		if username == "" {
			return yepAPIFormatError("user_posts operation requires a 'username' string"), nil
		}
		payload := map[string]interface{}{"unique_id": username}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["count"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/tiktok/user-posts", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "comments":
		videoURL, _ := args["url"].(string)
		if videoURL == "" {
			return yepAPIFormatError("comments operation requires a 'url' string (TikTok video URL)"), nil
		}
		payload := map[string]interface{}{"url": videoURL}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["count"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/tiktok/comments", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "music":
		musicURL, _ := args["url"].(string)
		if musicURL == "" {
			return yepAPIFormatError("music operation requires a 'url' string (TikTok music URL)"), nil
		}
		data, err := client.Post(ctx, "/v1/tiktok/music", map[string]interface{}{"url": musicURL})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "challenge":
		name, _ := args["name"].(string)
		if name == "" {
			return yepAPIFormatError("challenge operation requires a 'name' string"), nil
		}
		data, err := client.Post(ctx, "/v1/tiktok/challenge", map[string]interface{}{"name": name})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	default:
		return "", fmt.Errorf("unknown yepapi_tiktok operation: %s", operation)
	}
}
