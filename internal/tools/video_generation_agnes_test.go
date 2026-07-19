package tools

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"aurago/internal/config"
	"aurago/internal/testutil"
)

func TestGenerateVideoAgnesFlow(t *testing.T) {
	videoBytes := []byte("fake agnes mp4 data")
	var createPayload map[string]interface{}
	var serverURL string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/download/video.mp4" && r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing Agnes AI bearer token for %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/v1/videos":
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &createPayload); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"task_id":"task-1","video_id":"video-1","status":"queued"}`))
		case "/agnesapi":
			if r.URL.Query().Get("video_id") != "video-1" {
				t.Fatalf("video_id = %q, want video-1", r.URL.Query().Get("video_id"))
			}
			_, _ = w.Write([]byte(`{"id":"task-1","video_id":"video-1","status":"completed","seconds":"6.0","url":"` + serverURL + `/download/video.mp4"}`))
		case "/download/video.mp4":
			_, _ = w.Write(videoBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	serverURL = server.URL
	defer server.Close()

	oldClient := videoGenHTTPClient
	videoGenHTTPClient = server.Client()
	defer func() { videoGenHTTPClient = oldClient }()

	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.VideoGeneration.ProviderType = "agnes"
	cfg.VideoGeneration.BaseURL = server.URL + "/v1"
	cfg.VideoGeneration.APIKey = "test-key"
	cfg.VideoGeneration.ResolvedModel = defaultAgnesVideoModel
	cfg.VideoGeneration.DefaultDurationSeconds = 6
	cfg.VideoGeneration.DefaultResolution = "768P"
	cfg.VideoGeneration.DefaultAspectRatio = "16:9"
	cfg.VideoGeneration.PollIntervalSeconds = -1

	result := GenerateVideoResult(context.Background(), cfg, nil, slog.Default(), VideoGenParams{
		Prompt:         "a firefly forest",
		NegativePrompt: "blurry",
	})
	if result.Status != "ok" {
		t.Fatalf("status = %q, error = %q", result.Status, result.Error)
	}
	if result.Provider != "agnes" || result.Model != defaultAgnesVideoModel {
		t.Fatalf("provider/model = %q/%q", result.Provider, result.Model)
	}
	if createPayload["model"] != defaultAgnesVideoModel {
		t.Fatalf("model = %v", createPayload["model"])
	}
	if createPayload["width"] != float64(1152) || createPayload["height"] != float64(768) {
		t.Fatalf("dimensions = %vx%v, want 1152x768", createPayload["width"], createPayload["height"])
	}
	if createPayload["num_frames"] != float64(145) || createPayload["frame_rate"] != float64(24) {
		t.Fatalf("frames/rate = %v/%v, want 145/24", createPayload["num_frames"], createPayload["frame_rate"])
	}
	if result.TaskID != "task-1" {
		t.Fatalf("task_id = %q, want task-1", result.TaskID)
	}
	if result.FileSize != int64(len(videoBytes)) {
		t.Fatalf("file_size = %d, want %d", result.FileSize, len(videoBytes))
	}
	if result.CostEstimate != 0 {
		t.Fatalf("cost estimate = %f, want current free tier 0", result.CostEstimate)
	}
}

func TestAgnesVideoFrameCountHonorsProviderLimit(t *testing.T) {
	if got := agnesVideoFrameCount(5); got != 121 {
		t.Fatalf("5-second frame count = %d, want 121", got)
	}
	if got := agnesVideoFrameCount(60); got != 441 {
		t.Fatalf("frame limit = %d, want 441", got)
	}
}

func TestAgnesVideoConnectionUsesTextCompatibilityEndpoint(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	oldClient := videoGenHTTPClient
	videoGenHTTPClient = server.Client()
	defer func() { videoGenHTTPClient = oldClient }()

	ok, message := TestVideoConnection(context.Background(), "agnes", "test-key", server.URL+"/v1")
	if !ok {
		t.Fatalf("connection test failed: %s", message)
	}
}
