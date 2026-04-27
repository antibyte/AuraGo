package tools

import (
	"context"
	"fmt"
)

// DispatchYepAPIYouTube handles YouTube data operations via YepAPI.
func DispatchYepAPIYouTube(ctx context.Context, client *YepAPIClient, operation string, args map[string]interface{}) (string, error) {
	switch operation {
	case "search":
		query, _ := args["query"].(string)
		if query == "" {
			return yepAPIFormatError("search operation requires a 'query' string"), nil
		}
		payload := map[string]interface{}{"query": query}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["limit"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/youtube/search", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "video":
		return dispatchYouTubeVideoID(ctx, client, "/v1/youtube/video", operation, args)

	case "video_info":
		return dispatchYouTubeVideoID(ctx, client, "/v1/youtube/video-info", operation, args)

	case "metadata":
		return dispatchYouTubeVideoID(ctx, client, "/v1/youtube/metadata", operation, args)

	case "transcript":
		return dispatchYouTubeVideoID(ctx, client, "/v1/youtube/transcript", operation, args)

	case "subtitles":
		return dispatchYouTubeVideoID(ctx, client, "/v1/youtube/subtitles", operation, args)

	case "comments":
		videoID := stringArgWithFallback(args, "video_id")
		if videoID == "" {
			return yepAPIFormatError("comments operation requires a 'video_id' string"), nil
		}
		payload := map[string]interface{}{"videoId": videoID}
		addPositiveIntArg(payload, args, "limit", "limit")
		data, err := client.Post(ctx, "/v1/youtube/comments", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "channel":
		return dispatchYouTubeChannelID(ctx, client, "/v1/youtube/channel", operation, args, false)

	case "channel_videos":
		return dispatchYouTubeChannelID(ctx, client, "/v1/youtube/channel-videos", operation, args, true)

	case "channel_shorts":
		return dispatchYouTubeChannelID(ctx, client, "/v1/youtube/channel-shorts", operation, args, true)

	case "channel_livestreams", "channel_live":
		return dispatchYouTubeChannelID(ctx, client, "/v1/youtube/channel-livestreams", operation, args, false)

	case "channel_playlists":
		return dispatchYouTubeChannelID(ctx, client, "/v1/youtube/channel-playlists", operation, args, true)

	case "channel_community":
		return dispatchYouTubeChannelID(ctx, client, "/v1/youtube/channel-community", operation, args, true)

	case "channel_about":
		return dispatchYouTubeChannelID(ctx, client, "/v1/youtube/channel-about", operation, args, false)

	case "channel_channels":
		return dispatchYouTubeChannelID(ctx, client, "/v1/youtube/channel-channels", operation, args, true)

	case "channel_store":
		return dispatchYouTubeChannelID(ctx, client, "/v1/youtube/channel-store", operation, args, false)

	case "channel_search":
		channelID := stringArgWithFallback(args, "channel_id")
		query, _ := args["query"].(string)
		if channelID == "" {
			return yepAPIFormatError("channel_search operation requires a 'channel_id' string"), nil
		}
		if query == "" {
			return yepAPIFormatError("channel_search operation requires a 'query' string"), nil
		}
		payload := map[string]interface{}{"channelId": channelID, "query": query}
		addPositiveIntArg(payload, args, "limit", "limit")
		data, err := client.Post(ctx, "/v1/youtube/channel-search", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "playlist":
		return dispatchYouTubePlaylistID(ctx, client, "/v1/youtube/playlist", operation, args, true)

	case "playlist_info":
		return dispatchYouTubePlaylistID(ctx, client, "/v1/youtube/playlist-info", operation, args, false)

	case "trending":
		payload := youtubeAmbientPayload(args)
		addPositiveIntArg(payload, args, "limit", "limit")
		data, err := client.Post(ctx, "/v1/youtube/trending", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "related":
		videoID := stringArgWithFallback(args, "video_id")
		if videoID == "" {
			return yepAPIFormatError("related operation requires a 'video_id' string"), nil
		}
		payload := map[string]interface{}{"videoId": videoID}
		addPositiveIntArg(payload, args, "limit", "limit")
		data, err := client.Post(ctx, "/v1/youtube/related", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "screenshot":
		return dispatchYouTubeVideoID(ctx, client, "/v1/youtube/screenshot", operation, args)

	case "shorts":
		query, _ := args["query"].(string)
		payload := youtubeAmbientPayload(args)
		if query != "" {
			payload["query"] = query
		}
		addPositiveIntArg(payload, args, "limit", "limit")
		data, err := client.Post(ctx, "/v1/youtube/shorts", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "shorts_info":
		return dispatchYouTubeVideoID(ctx, client, "/v1/youtube/shorts-info", operation, args)

	case "suggest":
		query, _ := args["query"].(string)
		if query == "" {
			return yepAPIFormatError("suggest operation requires a 'query' string"), nil
		}
		data, err := client.Post(ctx, "/v1/youtube/suggest", map[string]interface{}{"query": query})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "hashtag":
		tag := stringArgWithFallback(args, "tag")
		if tag == "" {
			return yepAPIFormatError("hashtag operation requires a 'tag' string"), nil
		}
		payload := map[string]interface{}{"tag": tag}
		addPositiveIntArg(payload, args, "limit", "limit")
		data, err := client.Post(ctx, "/v1/youtube/hashtag", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "post":
		postID := stringArgWithFallback(args, "post_id")
		if postID == "" {
			return yepAPIFormatError("post operation requires a 'post_id' string"), nil
		}
		data, err := client.Post(ctx, "/v1/youtube/post", map[string]interface{}{"postId": postID})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "post_comments":
		postID := stringArgWithFallback(args, "post_id")
		if postID == "" {
			return yepAPIFormatError("post_comments operation requires a 'post_id' string"), nil
		}
		payload := map[string]interface{}{"postId": postID}
		addPositiveIntArg(payload, args, "limit", "limit")
		data, err := client.Post(ctx, "/v1/youtube/post-comments", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "home":
		payload := youtubeAmbientPayload(args)
		addPositiveIntArg(payload, args, "limit", "limit")
		data, err := client.Post(ctx, "/v1/youtube/home", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "hype":
		payload := youtubeAmbientPayload(args)
		addPositiveIntArg(payload, args, "limit", "limit")
		data, err := client.Post(ctx, "/v1/youtube/hype", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "resolve":
		url := stringArgWithFallback(args, "url")
		if url == "" {
			return yepAPIFormatError("resolve operation requires a 'url' string"), nil
		}
		data, err := client.Post(ctx, "/v1/youtube/resolve", map[string]interface{}{"url": url})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	default:
		return "", fmt.Errorf("unknown yepapi_youtube operation: %s", operation)
	}
}

func dispatchYouTubeVideoID(ctx context.Context, client *YepAPIClient, endpoint, operation string, args map[string]interface{}) (string, error) {
	videoID := stringArgWithFallback(args, "video_id")
	if videoID == "" {
		return yepAPIFormatError(fmt.Sprintf("%s operation requires a 'video_id' string", operation)), nil
	}
	data, err := client.Post(ctx, endpoint, map[string]interface{}{"videoId": videoID})
	if err != nil {
		return "", err
	}
	return yepAPIFormatSuccess(data), nil
}

func dispatchYouTubeChannelID(ctx context.Context, client *YepAPIClient, endpoint, operation string, args map[string]interface{}, withLimit bool) (string, error) {
	channelID := stringArgWithFallback(args, "channel_id")
	if channelID == "" {
		return yepAPIFormatError(fmt.Sprintf("%s operation requires a 'channel_id' string", operation)), nil
	}
	payload := map[string]interface{}{"channelId": channelID}
	if withLimit {
		addPositiveIntArg(payload, args, "limit", "limit")
	}
	data, err := client.Post(ctx, endpoint, payload)
	if err != nil {
		return "", err
	}
	return yepAPIFormatSuccess(data), nil
}

func dispatchYouTubePlaylistID(ctx context.Context, client *YepAPIClient, endpoint, operation string, args map[string]interface{}, withLimit bool) (string, error) {
	playlistID := stringArgWithFallback(args, "playlist_id")
	if playlistID == "" {
		return yepAPIFormatError(fmt.Sprintf("%s operation requires a 'playlist_id' string", operation)), nil
	}
	payload := map[string]interface{}{"playlistId": playlistID}
	if withLimit {
		addPositiveIntArg(payload, args, "limit", "limit")
	}
	data, err := client.Post(ctx, endpoint, payload)
	if err != nil {
		return "", err
	}
	return yepAPIFormatSuccess(data), nil
}

func youtubeAmbientPayload(args map[string]interface{}) map[string]interface{} {
	payload := map[string]interface{}{}
	addOptionalStringArg(payload, args, "country", "country")
	addOptionalStringArg(payload, args, "language", "language")
	return payload
}
