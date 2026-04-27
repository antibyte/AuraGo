package agent

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestParseYouTubeVideoURL(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantID    string
		wantStart int
	}{
		{name: "watch url", raw: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ"},
		{name: "short url", raw: "https://youtu.be/dQw4w9WgXcQ?t=1m30s", wantID: "dQw4w9WgXcQ", wantStart: 90},
		{name: "fragment start url", raw: "https://youtu.be/dQw4w9WgXcQ#t=43", wantID: "dQw4w9WgXcQ", wantStart: 43},
		{name: "shorts url", raw: "https://youtube.com/shorts/dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ"},
		{name: "embed url", raw: "https://www.youtube.com/embed/dQw4w9WgXcQ?start=42", wantID: "dQw4w9WgXcQ", wantStart: 42},
		{name: "nocookie embed", raw: "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ"},
		{name: "live url", raw: "https://www.youtube.com/live/dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := parseYouTubeVideoURL(tt.raw)
			if err != nil {
				t.Fatalf("parseYouTubeVideoURL() error = %v", err)
			}
			if ref.VideoID != tt.wantID || ref.StartSeconds != tt.wantStart {
				t.Fatalf("ref = %+v, want id %q start %d", ref, tt.wantID, tt.wantStart)
			}
			if !strings.Contains(ref.URL, "youtube.com/watch?v="+tt.wantID) {
				t.Fatalf("canonical URL = %q, want watch URL with id", ref.URL)
			}
			if !strings.Contains(ref.EmbedURL, "youtube-nocookie.com/embed/"+tt.wantID) {
				t.Fatalf("embed URL = %q, want nocookie embed URL with id", ref.EmbedURL)
			}
		})
	}
}

func TestParseYouTubeVideoURLRejectsInvalidInput(t *testing.T) {
	for _, raw := range []string{
		"",
		"https://example.com/watch?v=dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=not-valid",
		"https://www.youtube.com/playlist?list=PL123",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := parseYouTubeVideoURL(raw); err == nil {
				t.Fatalf("parseYouTubeVideoURL(%q) succeeded, want error", raw)
			}
		})
	}
}

func TestParseYouTubeTimeValueDurationForms(t *testing.T) {
	tests := map[string]int{
		"1h2m3s": 3723,
		"2m3s":   123,
		"90s":    90,
		"1:30":   90,
	}
	for raw, want := range tests {
		if got := parseYouTubeTimeValue(raw); got != want {
			t.Fatalf("parseYouTubeTimeValue(%q) = %d, want %d", raw, got, want)
		}
	}
}

func TestHandleSendYouTubeVideoReturnsEmbedPayload(t *testing.T) {
	raw := handleSendYouTubeVideo(youtubeVideoArgs{
		URL:          "https://youtu.be/dQw4w9WgXcQ",
		Title:        "Demo",
		StartSeconds: 12,
	}, slog.Default())
	raw = stringsTrimToolOutput(raw)

	var result struct {
		Status       string `json:"status"`
		Provider     string `json:"provider"`
		VideoID      string `json:"video_id"`
		URL          string `json:"url"`
		EmbedURL     string `json:"embed_url"`
		Title        string `json:"title"`
		StartSeconds int    `json:"start_seconds"`
		Markdown     string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v\n%s", err, raw)
	}
	if result.Status != "success" || result.Provider != "youtube" || result.VideoID != "dQw4w9WgXcQ" {
		t.Fatalf("unexpected result identity: %+v", result)
	}
	if result.URL != "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=12s" {
		t.Fatalf("URL = %q", result.URL)
	}
	if result.EmbedURL != "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?start=12" {
		t.Fatalf("EmbedURL = %q", result.EmbedURL)
	}
	if result.Title != "Demo" || result.Markdown != "[Demo](https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=12s)" {
		t.Fatalf("unexpected display fields: %+v", result)
	}
}

func TestEmitMediaSSEEventsSendsYouTubeVideoEvent(t *testing.T) {
	broker := &captureBroker{}
	emitMediaSSEEvents(broker, "send_youtube_video", `Tool Output: {"status":"success","url":"https://www.youtube.com/watch?v=dQw4w9WgXcQ","embed_url":"https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ","video_id":"dQw4w9WgXcQ","title":"Demo","start_seconds":0}`, t.TempDir())

	if len(broker.events) != 1 || broker.events[0].event != "youtube_video" {
		t.Fatalf("events = %+v, want one youtube_video event", broker.events)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(broker.events[0].message), &payload); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}
	if payload["video_id"] != "dQw4w9WgXcQ" || payload["embed_url"] == "" {
		t.Fatalf("unexpected youtube event payload: %+v", payload)
	}
}
