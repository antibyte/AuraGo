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
			payload["depth"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/youtube/search", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "video":
		videoID, _ := args["video_id"].(string)
		if videoID == "" {
			return yepAPIFormatError("video operation requires a 'video_id' string"), nil
		}
		data, err := client.Post(ctx, "/v1/youtube/video", map[string]interface{}{"videoId": videoID})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "transcript":
		videoID, _ := args["video_id"].(string)
		if videoID == "" {
			return yepAPIFormatError("transcript operation requires a 'video_id' string"), nil
		}
		data, err := client.Post(ctx, "/v1/youtube/transcript", map[string]interface{}{"videoId": videoID})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "comments":
		videoID, _ := args["video_id"].(string)
		if videoID == "" {
			return yepAPIFormatError("comments operation requires a 'video_id' string"), nil
		}
		payload := map[string]interface{}{"videoId": videoID}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["limit"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/youtube/comments", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "channel":
		channelID, _ := args["channel_id"].(string)
		if channelID == "" {
			return yepAPIFormatError("channel operation requires a 'channel_id' string"), nil
		}
		data, err := client.Post(ctx, "/v1/youtube/channel", map[string]interface{}{"channelId": channelID})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "channel_videos":
		channelID, _ := args["channel_id"].(string)
		if channelID == "" {
			return yepAPIFormatError("channel_videos operation requires a 'channel_id' string"), nil
		}
		payload := map[string]interface{}{"channelId": channelID}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["limit"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/youtube/channel-videos", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "playlist":
		playlistID, _ := args["playlist_id"].(string)
		if playlistID == "" {
			return yepAPIFormatError("playlist operation requires a 'playlist_id' string"), nil
		}
		data, err := client.Post(ctx, "/v1/youtube/playlist", map[string]interface{}{"playlistId": playlistID})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "trending":
		data, err := client.Post(ctx, "/v1/youtube/trending", map[string]interface{}{})
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

	case "shorts":
		query, _ := args["query"].(string)
		payload := map[string]interface{}{}
		if query != "" {
			payload["query"] = query
		}
		if limit, ok := args["limit"].(float64); ok && limit > 0 {
			payload["limit"] = int(limit)
		}
		data, err := client.Post(ctx, "/v1/youtube/shorts", payload)
		if err != nil {
			return "", err
		}
		return yepAPIFormatSuccess(data), nil

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

	default:
		return "", fmt.Errorf("unknown yepapi_youtube operation: %s", operation)
	}
}
