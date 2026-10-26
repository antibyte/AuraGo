package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"aurago/internal/tools"
)

type youtubeVideoRef struct {
	VideoID      string
	URL          string
	EmbedURL     string
	StartSeconds int
}

func handleSendYouTubeVideo(req youtubeVideoArgs, logger *slog.Logger) string {
	encode := func(r map[string]interface{}) string {
		b, err := json.Marshal(r)
		if err != nil {
			fallback, fallbackErr := json.Marshal(map[string]interface{}{
				"status":  "error",
				"message": fmt.Sprintf("encode send_youtube_video result: %v", err),
			})
			if fallbackErr != nil {
				return `Tool Output: {"status":"error","message":"encode send_youtube_video result failed"}`
			}
			return "Tool Output: " + string(fallback)
		}
		return "Tool Output: " + string(b)
	}

	ref, err := parseYouTubeVideoURL(req.URL)
	if err != nil {
		return encode(map[string]interface{}{"status": "error", "message": err.Error()})
	}
	if req.StartSeconds > tools.MaxYouTubeStartSeconds {
		return encode(map[string]interface{}{
			"status":  "error",
			"message": "start time must be at most 48 hours",
		})
	}
	if req.StartSeconds > 0 {
		ref.StartSeconds = req.StartSeconds
		ref.URL, ref.EmbedURL = tools.BuildYouTubeURLs(ref.VideoID, ref.StartSeconds)
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "YouTube video"
	} else if utf8.RuneCountInString(title) > 120 {
		return encode(map[string]interface{}{
			"status":  "error",
			"message": "title must be at most 120 characters",
		})
	}
	markdown := fmt.Sprintf("[%s](%s)", escapeMarkdownLinkText(title), ref.URL)

	logger.Info("[send_youtube_video] YouTube video ready", "video_id", ref.VideoID, "url", ref.URL)
	return encode(map[string]interface{}{
		"status":        "success",
		"provider":      "youtube",
		"video_id":      ref.VideoID,
		"url":           ref.URL,
		"embed_url":     ref.EmbedURL,
		"title":         title,
		"start_seconds": ref.StartSeconds,
		"markdown":      markdown,
	})
}

func parseYouTubeVideoURL(raw string) (youtubeVideoRef, error) {
	ref, err := tools.ParseYouTubeVideoURL(raw)
	if err != nil {
		return youtubeVideoRef{}, err
	}
	return youtubeVideoRef{
		VideoID:      ref.VideoID,
		URL:          ref.URL,
		EmbedURL:     ref.EmbedURL,
		StartSeconds: ref.StartSeconds,
	}, nil
}

func parseYouTubeTimeValue(raw string) int {
	return tools.ParseYouTubeTimeValue(raw)
}

func escapeMarkdownLinkText(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`)
	return replacer.Replace(text)
}
