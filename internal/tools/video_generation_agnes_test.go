package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/testutil"
)

func TestGenerateVideoAgnesFlow(t *testing.T) {
	videoBytes := []byte("fake agnes mp4 data")
	var createPayload map[string]interface{}
	var serverURL string
	pollCount := 0
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
			pollCount++
			if pollCount == 1 {
				_, _ = w.Write([]byte(`{"id":"task-1","video_id":"video-1","status":"in_progress"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"task-1","video_id":"video-1","status":"completed","seconds":"6.5","size":"832x448","metadata":{"size_mapping":{"adjusted":true,"width":832,"height":448},"url":"` + serverURL + `/download/video.mp4"}}`))
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
	if createPayload["width"] != float64(1366) || createPayload["height"] != float64(768) {
		t.Fatalf("dimensions = %vx%v, want 1366x768", createPayload["width"], createPayload["height"])
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
	if result.DurationMs != 6500 || !strings.Contains(result.Message, "(6.5s)") {
		t.Fatalf("duration/message = %d/%q, want API duration 6.5s", result.DurationMs, result.Message)
	}
	if result.Size != "832x448" {
		t.Fatalf("size = %q, want provider size 832x448", result.Size)
	}
}

func TestAgnesVideoModelNormalization(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"":                        defaultAgnesVideoModel,
		"agnes-2.0-flash":         defaultAgnesVideoModel,
		"not-a-video-model":       defaultAgnesVideoModel,
		"agnes-video-v1.5":        "agnes-video-v1.5",
		" Agnes-Video-Future-v1 ": "Agnes-Video-Future-v1",
	} {
		if got := normalizeAgnesVideoModel(input); got != want {
			t.Fatalf("normalizeAgnesVideoModel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAgnesVideoFrameSettingsPreserveRequestedDuration(t *testing.T) {
	frames, frameRate, err := agnesVideoFrameSettings(6)
	if err != nil || frames != 145 || frameRate != 24 {
		t.Fatalf("6 seconds = %d frames at %v fps, error %v", frames, frameRate, err)
	}
	frames, frameRate, err = agnesVideoFrameSettings(30)
	if err != nil || frames != 441 || frameRate != 14.7 {
		t.Fatalf("30 seconds = %d frames at %v fps, error %v", frames, frameRate, err)
	}
	for _, seconds := range []int{0, 31} {
		if _, _, err := agnesVideoFrameSettings(seconds); err == nil {
			t.Fatalf("duration %d unexpectedly accepted", seconds)
		}
	}
}

func TestAgnesVideoDimensions(t *testing.T) {
	tests := []struct {
		resolution string
		aspect     string
		width      int
		height     int
	}{
		{"480p", "16:9", 854, 480}, {"480p", "9:16", 480, 854}, {"480p", "1:1", 480, 480}, {"480p", "4:3", 640, 480}, {"480p", "3:4", 480, 640},
		{"720p", "16:9", 1280, 720}, {"720p", "9:16", 720, 1280}, {"720p", "1:1", 720, 720}, {"720p", "4:3", 960, 720}, {"720p", "3:4", 720, 960},
		{"768P", "16:9", 1366, 768}, {"768P", "9:16", 768, 1366}, {"768P", "1:1", 768, 768}, {"768P", "4:3", 1024, 768}, {"768P", "3:4", 768, 1024},
		{"1080P", "16:9", 1920, 1080}, {"1080P", "9:16", 1080, 1920}, {"1080P", "1:1", 1080, 1080}, {"1080P", "4:3", 1440, 1080}, {"1080P", "3:4", 1080, 1440},
	}
	for _, tt := range tests {
		width, height, err := agnesVideoDimensions(tt.resolution, tt.aspect)
		if err != nil || width != tt.width || height != tt.height {
			t.Fatalf("%s %s = %dx%d, error %v; want %dx%d", tt.resolution, tt.aspect, width, height, err, tt.width, tt.height)
		}
	}
	for _, invalid := range []struct{ resolution, aspect string }{
		{"4k", "16:9"},
		{"720p", "3:2"},
	} {
		if _, _, err := agnesVideoDimensions(invalid.resolution, invalid.aspect); err == nil {
			t.Fatalf("invalid dimensions %+v unexpectedly accepted", invalid)
		}
	}
}

func TestAgnesVideoImageInputsValidateAndOrderKeyframes(t *testing.T) {
	originalValidator := validatePublicImageURL
	defer func() { validatePublicImageURL = originalValidator }()
	validatePublicImageURL = func(rawURL string) error {
		if !strings.HasPrefix(rawURL, "https://public.example/") {
			return fmt.Errorf("not public")
		}
		return nil
	}

	first, keyframes, err := agnesVideoImageInputs(VideoGenParams{
		FirstFrameImage: "https://public.example/first.png",
		LastFrameImage:  "https://public.example/last.png",
		ReferenceImages: []string{
			"https://public.example/ref-1.png",
			"https://public.example/ref-2.png",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first != "https://public.example/first.png" {
		t.Fatalf("first image = %q", first)
	}
	want := []string{
		"https://public.example/first.png",
		"https://public.example/ref-1.png",
		"https://public.example/ref-2.png",
		"https://public.example/last.png",
	}
	if strings.Join(keyframes, "|") != strings.Join(want, "|") {
		t.Fatalf("keyframes = %#v, want %#v", keyframes, want)
	}

	for _, params := range []VideoGenParams{
		{LastFrameImage: "https://public.example/last.png"},
		{ReferenceImages: []string{"https://public.example/only.png"}},
		{FirstFrameImage: "data:image/png;base64,AA=="},
	} {
		if _, _, err := agnesVideoImageInputs(params); err == nil {
			t.Fatalf("invalid image inputs %+v unexpectedly accepted", params)
		}
	}
}

func TestAgnesVideoConnectionUsesFreeResultEndpoint(t *testing.T) {
	tests := []struct {
		status int
		ok     bool
	}{
		{http.StatusNotFound, true},
		{http.StatusUnauthorized, false},
		{http.StatusForbidden, false},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, false},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/agnesapi" {
					t.Fatalf("request = %s %s, want GET /agnesapi", r.Method, r.URL.Path)
				}
				if r.URL.Query().Get("video_id") != "video_aurago_connection_test" {
					t.Fatalf("video_id = %q", r.URL.Query().Get("video_id"))
				}
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(strings.Repeat("bounded-error-", 1000)))
			}))
			defer server.Close()

			oldClient := videoGenHTTPClient
			videoGenHTTPClient = server.Client()
			defer func() { videoGenHTTPClient = oldClient }()

			ok, message := TestVideoConnection(context.Background(), "agnes", "test-key", server.URL+"/v1")
			if ok != tt.ok {
				t.Fatalf("ok = %v, message = %q", ok, message)
			}
			if len(message) > 300 {
				t.Fatalf("connection message was not bounded: %d bytes", len(message))
			}
		})
	}
}

func TestPollAgnesVideoFallsBackToLegacyTaskIDAndReadsSizeMapping(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/videos/task-legacy" {
			t.Fatalf("path = %q, want legacy task endpoint", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"task_id":"task-legacy",
			"status":"completed",
			"seconds":"4",
			"metadata":{
				"url":"https://public.example/video.mp4",
				"size_mapping":{"width":1280,"height":720}
			}
		}`))
	}))
	defer server.Close()

	oldClient := videoGenHTTPClient
	videoGenHTTPClient = server.Client()
	defer func() { videoGenHTTPClient = oldClient }()

	state, err := pollAgnesVideo(
		context.Background(),
		server.URL+"/v1",
		server.URL+"/agnesapi",
		"test-key",
		agnesVideoState{TaskID: "task-legacy", Status: "queued"},
		-1,
		10,
	)
	if err != nil {
		t.Fatalf("pollAgnesVideo failed: %v", err)
	}
	if state.URL != "https://public.example/video.mp4" || state.Size != "1280x720" || state.Seconds != 4 {
		t.Fatalf("legacy final state = %+v", state)
	}
}

func TestPollAgnesVideoReturnsStructuredFailure(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"video_id":"video-failed","status":"failed","error":{"code":"generation_failed","message":"provider rejected the prompt"}}`))
	}))
	defer server.Close()

	oldClient := videoGenHTTPClient
	videoGenHTTPClient = server.Client()
	defer func() { videoGenHTTPClient = oldClient }()

	_, err := pollAgnesVideo(
		context.Background(),
		server.URL+"/v1",
		server.URL+"/agnesapi",
		"test-key",
		agnesVideoState{VideoID: "video-failed", Status: "queued"},
		-1,
		10,
	)
	if err == nil || !strings.Contains(err.Error(), "provider rejected the prompt") {
		t.Fatalf("structured provider failure = %v", err)
	}
}

func TestPollAgnesVideoHonorsCancellationAndTimeout(t *testing.T) {
	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := pollAgnesVideo(
			ctx,
			"http://127.0.0.1/v1",
			"http://127.0.0.1/agnesapi",
			"test-key",
			agnesVideoState{TaskID: "task-cancel", Status: "queued"},
			1,
			10,
		)
		if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
			t.Fatalf("cancellation error = %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"video_id":"video-timeout","status":"queued"}`))
		}))
		defer server.Close()

		oldClient := videoGenHTTPClient
		videoGenHTTPClient = server.Client()
		defer func() { videoGenHTTPClient = oldClient }()

		_, err := pollAgnesVideo(
			context.Background(),
			server.URL+"/v1",
			server.URL+"/agnesapi",
			"test-key",
			agnesVideoState{VideoID: "video-timeout", Status: "queued"},
			1,
			1,
		)
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("timeout error = %v", err)
		}
	})
}

func TestDownloadFileWithHeadersLimitRemovesOversizedPartialFile(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte(strings.Repeat("v", 32)))
	}))
	defer server.Close()

	oldClient := videoGenHTTPClient
	videoGenHTTPClient = server.Client()
	defer func() { videoGenHTTPClient = oldClient }()

	dest := filepath.Join(t.TempDir(), "oversized.mp4")
	err := downloadFileWithHeadersLimit(context.Background(), server.URL, dest, nil, 16)
	if err == nil {
		t.Fatal("oversized video download unexpectedly succeeded")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("partial video file still exists after failure: %v", statErr)
	}
}
