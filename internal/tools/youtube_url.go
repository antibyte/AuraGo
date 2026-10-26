package tools

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// MaxYouTubeStartSeconds is 48 hours. Longer offsets are rejected.
const MaxYouTubeStartSeconds = 48 * 60 * 60

var (
	youtubeVideoIDRe     = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	youtubeTimeRe        = regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`)
	youtubeCountryHostRe = regexp.MustCompile(`^youtube\.(?:[a-z]{2}|co\.[a-z]{2}|com\.[a-z]{2})$`)
)

// YouTubeVideoRef is a checked video the player and YepAPI resolve can share.
type YouTubeVideoRef struct {
	VideoID      string
	URL          string
	EmbedURL     string
	StartSeconds int
}

// ParseYouTubeVideoURL accepts watch, shorts, live, embed, /v/, /e/, youtu.be,
// YouTube subdomains, country hosts, and one attribution_link hop.
// Clip and playlist URLs are rejected. The result is always an https nocookie embed.
func ParseYouTubeVideoURL(raw string) (YouTubeVideoRef, error) {
	return parseYouTubeVideoURL(raw, 0)
}

func parseYouTubeVideoURL(raw string, depth int) (YouTubeVideoRef, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return YouTubeVideoRef{}, fmt.Errorf("url is required")
	}
	if !strings.Contains(clean, "://") && (strings.Contains(clean, "youtube.") || strings.Contains(clean, "youtu.be")) {
		clean = "https://" + clean
	}

	parsed, err := url.Parse(clean)
	if err != nil || parsed.Hostname() == "" {
		return YouTubeVideoRef{}, fmt.Errorf("invalid YouTube URL")
	}
	if parsed.User != nil {
		return YouTubeVideoRef{}, fmt.Errorf("invalid YouTube URL")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return YouTubeVideoRef{}, fmt.Errorf("invalid YouTube URL")
	}

	host := normalizeYouTubeHost(parsed.Hostname())
	kind := youtubeHostKind(host)
	if kind == "" {
		return YouTubeVideoRef{}, fmt.Errorf("unsupported YouTube host: %s", parsed.Hostname())
	}
	segments := youtubePathSegments(parsed)
	if kind == "watch" && len(segments) > 0 && segments[0] == "attribution_link" {
		if depth >= 1 {
			return YouTubeVideoRef{}, fmt.Errorf("invalid YouTube URL")
		}
		inner := strings.TrimSpace(parsed.Query().Get("u"))
		if inner == "" {
			return YouTubeVideoRef{}, fmt.Errorf("invalid YouTube URL")
		}
		if !strings.Contains(inner, "://") {
			if !strings.HasPrefix(inner, "/") {
				inner = "/" + inner
			}
			inner = "https://" + parsed.Hostname() + inner
		}
		return parseYouTubeVideoURL(inner, depth+1)
	}

	videoID := youtubeVideoID(kind, segments, parsed.Query())
	if !youtubeVideoIDRe.MatchString(videoID) {
		return YouTubeVideoRef{}, fmt.Errorf("invalid YouTube video id")
	}
	startSeconds := parseYouTubeStartSeconds(parsed.Query(), parsed.Fragment)
	if startSeconds > MaxYouTubeStartSeconds {
		return YouTubeVideoRef{}, fmt.Errorf("start time must be at most 48 hours")
	}
	canonicalURL, embedURL := BuildYouTubeURLs(videoID, startSeconds)
	return YouTubeVideoRef{
		VideoID:      videoID,
		URL:          canonicalURL,
		EmbedURL:     embedURL,
		StartSeconds: startSeconds,
	}, nil
}

func youtubePathSegments(parsed *url.URL) []string {
	path := strings.Trim(parsed.EscapedPath(), "/")
	if path == "" {
		return nil
	}
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if decoded, decErr := url.PathUnescape(segment); decErr == nil {
			segments[i] = decoded
		}
	}
	return segments
}

func youtubeHostKind(host string) string {
	switch {
	case host == "youtu.be":
		return "short"
	case host == "youtube-nocookie.com" || strings.HasSuffix(host, ".youtube-nocookie.com"):
		return "nocookie"
	case host == "youtube.com" || strings.HasSuffix(host, ".youtube.com") || youtubeCountryHostRe.MatchString(host):
		return "watch"
	default:
		return ""
	}
}

func youtubeVideoID(kind string, segments []string, query url.Values) string {
	switch kind {
	case "short":
		if len(segments) > 0 {
			return segments[0]
		}
	case "nocookie":
		if len(segments) >= 2 && segments[0] == "embed" {
			return segments[1]
		}
	case "watch":
		if len(segments) == 0 || segments[0] == "" || segments[0] == "watch" {
			return query.Get("v")
		}
		switch segments[0] {
		case "shorts", "embed", "live", "v", "e":
			if len(segments) >= 2 {
				return segments[1]
			}
		}
	}
	return ""
}

func normalizeYouTubeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	return strings.TrimPrefix(host, "www.")
}

func parseYouTubeStartSeconds(query url.Values, fragment string) int {
	for _, key := range []string{"start", "t"} {
		if seconds := ParseYouTubeTimeValue(query.Get(key)); seconds > 0 {
			return seconds
		}
	}
	if seconds := ParseYouTubeTimeValue(fragment); seconds > 0 {
		return seconds
	}
	return 0
}

// ParseYouTubeTimeValue reads a start offset. Values above MaxYouTubeStartSeconds
// are returned so the caller can reject them.
func ParseYouTubeTimeValue(raw string) int {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return 0
	}
	value = strings.TrimPrefix(value, "?t=")
	value = strings.TrimPrefix(value, "#t=")
	value = strings.TrimPrefix(value, "t=")
	value = strings.TrimPrefix(value, "start=")
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
			if n > MaxYouTubeStartSeconds || total > MaxYouTubeStartSeconds/60 {
				return MaxYouTubeStartSeconds + 1
			}
			total = total*60 + n
		}
		return total
	}

	matches := youtubeTimeRe.FindStringSubmatch(value)
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
		if err != nil || n < 0 {
			return 0
		}
		if n > MaxYouTubeStartSeconds/unit || total > MaxYouTubeStartSeconds-n*unit {
			return MaxYouTubeStartSeconds + 1
		}
		total += n * unit
	}
	return total
}

// BuildYouTubeURLs returns the canonical watch URL and the nocookie embed.
func BuildYouTubeURLs(videoID string, startSeconds int) (string, string) {
	canonicalURL := "https://www.youtube.com/watch?v=" + videoID
	embedURL := "https://www.youtube-nocookie.com/embed/" + videoID
	if startSeconds > 0 {
		canonicalURL += "&t=" + strconv.Itoa(startSeconds) + "s"
		embedURL += "?start=" + strconv.Itoa(startSeconds)
	}
	return canonicalURL, embedURL
}
