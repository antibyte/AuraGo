package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type instagramUserLookup struct {
	username      string
	usernameOrURL string
}

func instagramUserLookupArg(args map[string]interface{}) instagramUserLookup {
	if value, ok := args["username_or_url"].(string); ok && strings.TrimSpace(value) != "" {
		return instagramUserLookup{
			username:      normalizeInstagramUsername(value),
			usernameOrURL: strings.TrimSpace(value),
		}
	}
	value := stringArgWithFallback(args, "username")
	if strings.TrimSpace(value) == "" {
		return instagramUserLookup{}
	}
	return instagramUserLookup{
		username:      normalizeInstagramUsername(value),
		usernameOrURL: strings.TrimSpace(value),
	}
}

func normalizeInstagramUsername(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "@")
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return ""
	}

	parsedURL, err := url.Parse(trimmed)
	if err == nil && parsedURL.Host != "" {
		host := strings.ToLower(strings.TrimPrefix(parsedURL.Host, "www."))
		if host == "instagram.com" || strings.HasSuffix(host, ".instagram.com") {
			pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
			if len(pathParts) > 0 && pathParts[0] != "" {
				return strings.TrimPrefix(pathParts[0], "@")
			}
		}
	}

	return trimmed
}

// DispatchYepAPIInstagram handles Instagram data operations via YepAPI.
func DispatchYepAPIInstagram(ctx context.Context, client *YepAPIClient, operation string, args map[string]interface{}) (string, error) {
	switch operation {
	case "search":
		query, _ := args["query"].(string)
		if query == "" {
			return yepAPIFormatError("search operation requires a 'query' string"), nil
		}
		endpoint := "/v1/instagram/search"
		payload := map[string]interface{}{"query": query}
		data, finalPayload, err := postInstagramPayloadWithValidationFallback(ctx, client, endpoint, operation, payload, map[string]interface{}{"search_query": query}, "missing search query")
		if err != nil {
			return "", err
		}
		return formatInstagramPayloadSuccess(data, endpoint, operation, finalPayload), nil

	case "user", "userinfo", "user_info", "profile", "user_profile":
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
		endpoint := "/v1/instagram/post"
		payload := map[string]interface{}{"shortcode": shortcode}
		data, err := postInstagramPayload(ctx, client, endpoint, operation, payload)
		if err != nil {
			return "", err
		}
		return formatInstagramPayloadSuccess(data, endpoint, operation, payload), nil

	case "post_comments":
		return dispatchInstagramShortcode(ctx, client, "/v1/instagram/post-comments", operation, args, true)

	case "post_likers":
		return dispatchInstagramShortcode(ctx, client, "/v1/instagram/post-likers", operation, args, true)

	case "hashtag":
		tag := stringArgWithFallback(args, "tag")
		if tag == "" {
			return yepAPIFormatError("hashtag operation requires a 'tag' string (without #)"), nil
		}
		endpoint := "/v1/instagram/hashtag"
		payload := map[string]interface{}{"tag": tag}
		data, err := postInstagramPayload(ctx, client, endpoint, operation, payload)
		if err != nil {
			return "", err
		}
		return formatInstagramPayloadSuccess(data, endpoint, operation, payload), nil

	case "media_id":
		shortcode := stringArgWithFallback(args, "shortcode")
		if shortcode == "" {
			return yepAPIFormatError("media_id operation requires a 'shortcode' string"), nil
		}
		endpoint := "/v1/instagram/media-id"
		payload := map[string]interface{}{"shortcode": shortcode}
		data, err := postInstagramPayload(ctx, client, endpoint, operation, payload)
		if err != nil {
			return "", err
		}
		return formatInstagramPayloadSuccess(data, endpoint, operation, payload), nil

	default:
		return "", fmt.Errorf("unknown yepapi_instagram operation: %s", operation)
	}
}

func postInstagramPayload(ctx context.Context, client *YepAPIClient, endpoint, operation string, payload map[string]interface{}) ([]byte, error) {
	logYepAPIRequestPayload(ctx, "yepapi_instagram", operation, endpoint, payload)
	return client.Post(ctx, endpoint, payload)
}

func postInstagramPayloadWithValidationFallback(ctx context.Context, client *YepAPIClient, endpoint, operation string, payload, fallbackPayload map[string]interface{}, validationNeedle string) ([]byte, map[string]interface{}, error) {
	data, err := postInstagramPayload(ctx, client, endpoint, operation, payload)
	if !instagramValidationErrorContains(data, err, validationNeedle) {
		return data, payload, err
	}
	data, err = postInstagramPayload(ctx, client, endpoint, operation, fallbackPayload)
	return data, fallbackPayload, err
}

func instagramValidationErrorContains(data []byte, err error, validationNeedle string) bool {
	needle := strings.ToLower(validationNeedle)
	if err != nil {
		return strings.Contains(strings.ToLower(err.Error()), needle)
	}

	var response map[string]interface{}
	if json.Unmarshal(data, &response) != nil {
		return false
	}
	message := strings.ToLower(fmt.Sprint(response["error"]))
	return message != "" && strings.Contains(message, needle)
}

func instagramPayloadDiagnostics(endpoint, operation string, payload map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"sent_endpoint":     endpoint,
		"sent_operation":    operation,
		"sent_payload_keys": safeYepAPIPayloadKeys(payload),
	}
}

func formatInstagramPayloadSuccess(data []byte, endpoint, operation string, payload map[string]interface{}) string {
	return yepAPIFormatSuccessWithDiagnostics(data, instagramPayloadDiagnostics(endpoint, operation, payload))
}

func dispatchInstagramUsername(ctx context.Context, client *YepAPIClient, endpoint, operation string, args map[string]interface{}, withLimit bool) (string, error) {
	lookup := instagramUserLookupArg(args)
	if lookup.username == "" {
		return yepAPIFormatError(fmt.Sprintf("%s operation requires a 'username' or 'username_or_url' string", operation)), nil
	}
	payload := map[string]interface{}{"username": lookup.username}
	if withLimit {
		addPositiveIntArg(payload, args, "limit", "limit")
	}
	fallbackPayload := map[string]interface{}{"username_or_url": lookup.usernameOrURL}
	if withLimit {
		addPositiveIntArg(fallbackPayload, args, "limit", "limit")
	}
	data, finalPayload, err := postInstagramPayloadWithValidationFallback(ctx, client, endpoint, operation, payload, fallbackPayload, "username_or_url is required")
	if err != nil {
		return "", err
	}
	return formatInstagramPayloadSuccess(data, endpoint, operation, finalPayload), nil
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
	data, err := postInstagramPayload(ctx, client, endpoint, operation, payload)
	if err != nil {
		return "", err
	}
	return formatInstagramPayloadSuccess(data, endpoint, operation, payload), nil
}
