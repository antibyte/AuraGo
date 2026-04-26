package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var youtubeVideoIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

type youtubeVideoRef struct {
	VideoID      string
	URL          string
	EmbedURL     string
	StartSeconds int
}

func handleSendYouTubeVideo(req youtubeVideoArgs, logger *slog.Logger) string {
	encode := func(r map[string]interface{}) string {
		b, _ := json.Marshal(r)
		return "Tool Output: " + string(b)
	}

	ref, err := parseYouTubeVideoURL(req.URL)
	if err != nil {
		return encode(map[string]interface{}{"status": "error", "message": err.Error()})
	}
	if req.StartSeconds > 0 {
		ref.StartSeconds = req.StartSeconds
		ref.URL, ref.EmbedURL = buildYouTubeURLs(ref.VideoID, ref.StartSeconds)
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "YouTube video"
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
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return youtubeVideoRef{}, fmt.Errorf("url is required")
	}
	if !strings.Contains(clean, "://") && (strings.Contains(clean, "youtube.") || strings.Contains(clean, "youtu.be")) {
		clean = "https://" + clean
	}

	parsed, err := url.Parse(clean)
	if err != nil || parsed.Hostname() == "" {
		return youtubeVideoRef{}, fmt.Errorf("invalid YouTube URL")
	}

	host := normalizeYouTubeHost(parsed.Hostname())
	path := strings.Trim(parsed.EscapedPath(), "/")
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if decoded, decErr := url.PathUnescape(segment); decErr == nil {
			segments[i] = decoded
		}
	}

	videoID := ""
	switch host {
	case "youtu.be":
		if len(segments) > 0 {
			videoID = segments[0]
		}
	case "youtube.com", "m.youtube.com", "music.youtube.com":
		videoID = youtubeVideoIDFromPathAndQuery(segments, parsed.Query())
	case "youtube-nocookie.com":
		if len(segments) >= 2 && segments[0] == "embed" {
			videoID = segments[1]
		}
	default:
		return youtubeVideoRef{}, fmt.Errorf("unsupported YouTube host: %s", parsed.Hostname())
	}

	if !youtubeVideoIDRe.MatchString(videoID) {
		return youtubeVideoRef{}, fmt.Errorf("invalid YouTube video id")
	}

	startSeconds := parseYouTubeStartSeconds(parsed.Query())
	canonicalURL, embedURL := buildYouTubeURLs(videoID, startSeconds)
	return youtubeVideoRef{
		VideoID:      videoID,
		URL:          canonicalURL,
		EmbedURL:     embedURL,
		StartSeconds: startSeconds,
	}, nil
}

func youtubeVideoIDFromPathAndQuery(segments []string, query url.Values) string {
	if len(segments) == 0 || segments[0] == "" || segments[0] == "watch" {
		return query.Get("v")
	}
	switch segments[0] {
	case "shorts", "embed", "live":
		if len(segments) >= 2 {
			return segments[1]
		}
	}
	return ""
}

func normalizeYouTubeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimPrefix(host, "www.")
	return host
}

func parseYouTubeStartSeconds(query url.Values) int {
	for _, key := range []string{"start", "t"} {
		if seconds := parseYouTubeTimeValue(query.Get(key)); seconds > 0 {
			return seconds
		}
	}
	return 0
}

func parseYouTubeTimeValue(raw string) int {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return 0
	}
	value = strings.TrimPrefix(value, "?t=")
	value = strings.TrimPrefix(value, "#t=")
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return seconds
	}
	if strings.HasSuffix(value, "s") {
		if seconds, err := strconv.Atoi(strings.TrimSuffix(value, "s")); err == nil && seconds > 0 {
			return seconds
		}
	}
	if strings.Contains(value, ":") {
		parts := strings.Split(value, ":")
		total := 0
		for _, part := range parts {
			n, err := strconv.Atoi(part)
			if err != nil || n < 0 {
				return 0
			}
			total = total*60 + n
		}
		return total
	}

	matches := regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`).FindStringSubmatch(value)
	if matches == nil {
		return 0
	}
	total := 0
	units := []int{3600, 60, 1}
	for i, unit := range units {
		if matches[i+1] == "" {
			continue
		}
		n, err := strconv.Atoi(matches[i+1])
		if err != nil {
			return 0
		}
		total += n * unit
	}
	return total
}

func buildYouTubeURLs(videoID string, startSeconds int) (string, string) {
	canonicalURL := "https://www.youtube.com/watch?v=" + videoID
	embedURL := "https://www.youtube-nocookie.com/embed/" + videoID
	if startSeconds > 0 {
		canonicalURL += "&t=" + strconv.Itoa(startSeconds) + "s"
		embedURL += "?start=" + strconv.Itoa(startSeconds)
	}
	return canonicalURL, embedURL
}

func escapeMarkdownLinkText(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`)
	return replacer.Replace(text)
}
