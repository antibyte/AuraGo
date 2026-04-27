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
		addPositiveIntArg(payload, args, "limit", "count")
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
		addPositiveIntArg(payload, args, "limit", "count")
		data, err := client.Post(ctx, "/v1/tiktok/search-user", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "search_challenge":
		query, _ := args["query"].(string)
		if query == "" {
			return yepAPIFormatError("search_challenge operation requires a 'query' string"), nil
		}
		payload := map[string]interface{}{"keyword": query}
		addPositiveIntArg(payload, args, "limit", "count")
		data, err := client.Post(ctx, "/v1/tiktok/search-challenge", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "search_photo":
		query, _ := args["query"].(string)
		if query == "" {
			return yepAPIFormatError("search_photo operation requires a 'query' string"), nil
		}
		payload := map[string]interface{}{"keywords": query}
		addPositiveIntArg(payload, args, "limit", "count")
		data, err := client.Post(ctx, "/v1/tiktok/search-photo", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "video":
		videoURL := stringArgWithFallback(args, "url")
		if videoURL == "" {
			return yepAPIFormatError("video operation requires a 'url' string (TikTok video URL)"), nil
		}
		data, err := client.Post(ctx, "/v1/tiktok/video", map[string]interface{}{"url": videoURL})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "user":
		username := stringArgWithFallback(args, "username")
		if username == "" {
			return yepAPIFormatError("user operation requires a 'username' string"), nil
		}
		data, err := client.Post(ctx, "/v1/tiktok/user", map[string]interface{}{"unique_id": username})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "user_posts":
		return dispatchTikTokUsername(ctx, client, "/v1/tiktok/user-posts", operation, args, true)

	case "user_followers":
		return dispatchTikTokUsername(ctx, client, "/v1/tiktok/user-followers", operation, args, true)

	case "user_following":
		return dispatchTikTokUsername(ctx, client, "/v1/tiktok/user-following", operation, args, true)

	case "user_favorites":
		return dispatchTikTokUsername(ctx, client, "/v1/tiktok/user-favorites", operation, args, true)

	case "user_reposts":
		return dispatchTikTokUsername(ctx, client, "/v1/tiktok/user-reposts", operation, args, true)

	case "user_story":
		return dispatchTikTokUsername(ctx, client, "/v1/tiktok/user-story", operation, args, false)

	case "comments":
		videoURL := stringArgWithFallback(args, "url")
		if videoURL == "" {
			return yepAPIFormatError("comments operation requires a 'url' string (TikTok video URL)"), nil
		}
		payload := map[string]interface{}{"url": videoURL}
		addPositiveIntArg(payload, args, "limit", "count")
		data, err := client.Post(ctx, "/v1/tiktok/comments", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "comment_replies":
		commentID := stringArgWithFallback(args, "comment_id")
		if commentID == "" {
			return yepAPIFormatError("comment_replies operation requires a 'comment_id' string"), nil
		}
		payload := map[string]interface{}{"comment_id": commentID}
		addOptionalStringArg(payload, args, "url", "url")
		addPositiveIntArg(payload, args, "limit", "count")
		data, err := client.Post(ctx, "/v1/tiktok/comment-replies", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "music":
		musicURL := stringArgWithFallback(args, "url")
		if musicURL == "" {
			return yepAPIFormatError("music operation requires a 'url' string (TikTok music URL)"), nil
		}
		data, err := client.Post(ctx, "/v1/tiktok/music", map[string]interface{}{"url": musicURL})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "music_videos":
		musicURL := stringArgWithFallback(args, "url")
		if musicURL == "" {
			return yepAPIFormatError("music_videos operation requires a 'url' string (TikTok music URL)"), nil
		}
		payload := map[string]interface{}{"url": musicURL}
		addPositiveIntArg(payload, args, "limit", "count")
		data, err := client.Post(ctx, "/v1/tiktok/music-videos", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "challenge":
		name := stringArgWithFallback(args, "name")
		if name == "" {
			return yepAPIFormatError("challenge operation requires a 'name' string"), nil
		}
		data, err := client.Post(ctx, "/v1/tiktok/challenge", map[string]interface{}{"name": name})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "challenge_videos":
		name := stringArgWithFallback(args, "name")
		if name == "" {
			return yepAPIFormatError("challenge_videos operation requires a 'name' string"), nil
		}
		payload := map[string]interface{}{"name": name}
		addPositiveIntArg(payload, args, "limit", "count")
		data, err := client.Post(ctx, "/v1/tiktok/challenge-videos", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	default:
		return "", fmt.Errorf("unknown yepapi_tiktok operation: %s", operation)
	}
}

func dispatchTikTokUsername(ctx context.Context, client *YepAPIClient, endpoint, operation string, args map[string]interface{}, withLimit bool) (string, error) {
	username := stringArgWithFallback(args, "username")
	if username == "" {
		return yepAPIFormatError(fmt.Sprintf("%s operation requires a 'username' string", operation)), nil
	}
	payload := map[string]interface{}{"unique_id": username}
	if withLimit {
		addPositiveIntArg(payload, args, "limit", "count")
	}
	data, err := client.Post(ctx, endpoint, payload)
	if err != nil {
		return "", err
	}
	return yepAPIFormatSuccess(data), nil
}
