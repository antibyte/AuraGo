package tools

import (
	"context"
	"fmt"
)

func instagramUsernameOrURLArg(args map[string]interface{}) string {
	if v, ok := args["username_or_url"].(string); ok && v != "" {
		return v
	}
	return stringArgWithFallback(args, "username")
}

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
		return dispatchInstagramUsername(ctx, client, "/v1/instagram/user", operation, args, false)

	case "user_about":
		return dispatchInstagramUsername(ctx, client, "/v1/instagram/user-about", operation, args, false)

	case "user_posts":
		return dispatchInstagramUsername(ctx, client, "/v1/instagram/user-posts", operation, args, true)

	case "user_reels":
		return dispatchInstagramUsername(ctx, client, "/v1/instagram/user-reels", operation, args, true)

	case "user_stories":
		return dispatchInstagramUsername(ctx, client, "/v1/instagram/user-stories", operation, args, false)

	case "user_highlights":
		return dispatchInstagramUsername(ctx, client, "/v1/instagram/user-highlights", operation, args, false)

	case "user_tagged":
		return dispatchInstagramUsername(ctx, client, "/v1/instagram/user-tagged", operation, args, true)

	case "user_followers":
		return dispatchInstagramUsername(ctx, client, "/v1/instagram/user-followers", operation, args, true)

	case "user_similar":
		return dispatchInstagramUsername(ctx, client, "/v1/instagram/user-similar", operation, args, true)

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
		return dispatchInstagramShortcode(ctx, client, "/v1/instagram/post-comments", operation, args, true)

	case "post_likers":
		return dispatchInstagramShortcode(ctx, client, "/v1/instagram/post-likers", operation, args, true)

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

	case "media_id":
		shortcode := stringArgWithFallback(args, "shortcode")
		if shortcode == "" {
			return yepAPIFormatError("media_id operation requires a 'shortcode' string"), nil
		}
		data, err := client.Post(ctx, "/v1/instagram/media-id", map[string]interface{}{"shortcode": shortcode})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	default:
		return "", fmt.Errorf("unknown yepapi_instagram operation: %s", operation)
	}
}

func dispatchInstagramUsername(ctx context.Context, client *YepAPIClient, endpoint, operation string, args map[string]interface{}, withLimit bool) (string, error) {
	username := instagramUsernameOrURLArg(args)
	if username == "" {
		return yepAPIFormatError(fmt.Sprintf("%s operation requires a 'username' or 'username_or_url' string", operation)), nil
	}
	payload := map[string]interface{}{"username": username}
	if withLimit {
		addPositiveIntArg(payload, args, "limit", "limit")
	}
	data, err := client.Post(ctx, endpoint, payload)
	if err != nil {
		return "", err
	}
	return yepAPIFormatSuccess(data), nil
}

func dispatchInstagramShortcode(ctx context.Context, client *YepAPIClient, endpoint, operation string, args map[string]interface{}, withLimit bool) (string, error) {
	shortcode := stringArgWithFallback(args, "shortcode")
	if shortcode == "" {
		return yepAPIFormatError(fmt.Sprintf("%s operation requires a 'shortcode' string", operation)), nil
	}
	payload := map[string]interface{}{"shortcode": shortcode}
	if withLimit {
		addPositiveIntArg(payload, args, "limit", "limit")
	}
	data, err := client.Post(ctx, endpoint, payload)
	if err != nil {
		return "", err
	}
	return yepAPIFormatSuccess(data), nil
}
