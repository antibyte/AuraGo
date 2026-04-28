package tools

import (
	"aurago/internal/testutil"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func TestGenerateVideoMiniMaxFlow(t *testing.T) {
	tmpDir := t.TempDir()
	videoBytes := []byte("fake mp4 data")
	var createPayload map[string]interface{}

	var server *httptest.Server
	server = testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/video_generation":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method for create: %s", r.Method)
			}
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read create body: %v", err)
			}
			if err := json.Unmarshal(raw, &createPayload); err != nil {
				t.Fatalf("unmarshal create body: %v", err)
			}
			w.Write([]byte(`{"task_id":"task-1","base_resp":{"status_code":0}}`))
		case "/v1/query/video_generation":
			if got := r.URL.Query().Get("task_id"); got != "task-1" {
				t.Fatalf("task_id = %q, want task-1", got)
			}
			w.Write([]byte(`{"status":"Success","file_id":"file-1","base_resp":{"status_code":0}}`))
		case "/v1/files/retrieve":
			if got := r.URL.Query().Get("file_id"); got != "file-1" {
				t.Fatalf("file_id = %q, want file-1", got)
			}
			w.Write([]byte(`{"file":{"download_url":"` + server.URL + `/download/video.mp4"},"base_resp":{"status_code":0}}`))
		case "/download/video.mp4":
			w.Write(videoBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldClient := videoGenHTTPClient
	videoGenHTTPClient = server.Client()
	defer func() { videoGenHTTPClient = oldClient }()

	db, err := InitMediaRegistryDB(filepath.Join(tmpDir, "media.db"))
	if err != nil {
		t.Fatalf("InitMediaRegistryDB failed: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Directories.DataDir = tmpDir
	cfg.VideoGeneration.ProviderType = "minimax"
	cfg.VideoGeneration.BaseURL = server.URL + "/v1"
	cfg.VideoGeneration.APIKey = "test-key"
	cfg.VideoGeneration.ResolvedModel = "MiniMax-M2.7-highspeed"
	cfg.VideoGeneration.DefaultDurationSeconds = 6
	cfg.VideoGeneration.DefaultResolution = "768P"
	cfg.VideoGeneration.PollIntervalSeconds = -1

	result := GenerateVideoResult(context.Background(), cfg, db, slog.Default(), VideoGenParams{
		Prompt:         "a calm lake at sunrise",
		AspectRatio:    "16:9",
		NegativePrompt: "blurry",
	})
	if result.Status != "ok" {
		t.Fatalf("status = %q, error = %q", result.Status, result.Error)
	}
	if result.Provider != "minimax" || result.Model != "MiniMax-Hailuo-2.3" {
		t.Fatalf("provider/model = %s/%s", result.Provider, result.Model)
	}
	if createPayload["model"] != "MiniMax-Hailuo-2.3" {
		t.Fatalf("MiniMax request model = %v, want MiniMax-Hailuo-2.3", createPayload["model"])
	}
	if createPayload["resolution"] != "768P" {
		t.Fatalf("MiniMax request resolution = %v, want 768P", createPayload["resolution"])
	}
	if _, ok := createPayload["aspect_ratio"]; ok {
		t.Fatalf("MiniMax request should not include unsupported aspect_ratio: %v", createPayload)
	}
	if _, ok := createPayload["negative_prompt"]; ok {
		t.Fatalf("MiniMax request should not include unsupported negative_prompt: %v", createPayload)
	}
	if result.WebPath == "" || result.MediaID == 0 {
		t.Fatalf("expected web path and media registry id, got web=%q id=%d", result.WebPath, result.MediaID)
	}
	if got := fileSizeOrZero(result.FilePath); got != int64(len(videoBytes)) {
		t.Fatalf("file size = %d, want %d", got, len(videoBytes))
	}
}

func TestMiniMaxVideoModelForAPI(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantAPI     string
		wantDisplay string
	}{
		{name: "empty default", input: "", wantAPI: "MiniMax-Hailuo-2.3", wantDisplay: "MiniMax-Hailuo-2.3"},
		{name: "legacy preset", input: "Hailuo-2.3-768P", wantAPI: "MiniMax-Hailuo-2.3", wantDisplay: "Hailuo-2.3-768P"},
		{name: "official model", input: "MiniMax-Hailuo-2.3", wantAPI: "MiniMax-Hailuo-2.3", wantDisplay: "MiniMax-Hailuo-2.3"},
		{name: "chat model fallback", input: "MiniMax-M2.7-highspeed", wantAPI: "MiniMax-Hailuo-2.3", wantDisplay: "MiniMax-Hailuo-2.3"},
		{name: "google model fallback on minimax provider", input: "veo-3.1-generate-preview", wantAPI: "MiniMax-Hailuo-2.3", wantDisplay: "MiniMax-Hailuo-2.3"},
		{name: "subject reference model", input: "S2V-01", wantAPI: "S2V-01", wantDisplay: "S2V-01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAPI, gotDisplay := miniMaxVideoModelForAPI(tt.input)
			if gotAPI != tt.wantAPI || gotDisplay != tt.wantDisplay {
				t.Fatalf("miniMaxVideoModelForAPI(%q) = (%q, %q), want (%q, %q)", tt.input, gotAPI, gotDisplay, tt.wantAPI, tt.wantDisplay)
			}
		})
	}
}

func TestGenerateVideoGoogleFlow(t *testing.T) {
	tmpDir := t.TempDir()
	videoBytes := []byte("fake google mp4 data")
	encoded := base64.StdEncoding.EncodeToString(videoBytes)

	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1beta/models/veo-test:predictLongRunning":
			if r.Header.Get("x-goog-api-key") != "test-key" {
				t.Fatalf("missing Google API key header")
			}
			w.Write([]byte(`{"name":"operations/op-1"}`))
		case "/v1beta/operations/op-1":
			w.Write([]byte(`{"name":"operations/op-1","done":true,"response":{"generatedVideos":[{"video":{"bytesBase64Encoded":"` + encoded + `"}}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldClient := videoGenHTTPClient
	videoGenHTTPClient = server.Client()
	defer func() { videoGenHTTPClient = oldClient }()

	cfg := &config.Config{}
	cfg.Directories.DataDir = tmpDir
	cfg.VideoGeneration.ProviderType = "google"
	cfg.VideoGeneration.BaseURL = server.URL + "/v1beta"
	cfg.VideoGeneration.APIKey = "test-key"
	cfg.VideoGeneration.ResolvedModel = "veo-test"
	cfg.VideoGeneration.DefaultDurationSeconds = 8
	cfg.VideoGeneration.DefaultAspectRatio = "16:9"
	cfg.VideoGeneration.PollIntervalSeconds = -1

	result := GenerateVideoResult(context.Background(), cfg, nil, slog.Default(), VideoGenParams{Prompt: "cinematic city lights"})
	if result.Status != "ok" {
		t.Fatalf("status = %q, error = %q", result.Status, result.Error)
	}
	if result.Provider != "google" || result.Model != "veo-test" {
		t.Fatalf("provider/model = %s/%s", result.Provider, result.Model)
	}
	if got := fileSizeOrZero(result.FilePath); got != int64(len(videoBytes)) {
		t.Fatalf("file size = %d, want %d", got, len(videoBytes))
	}
}
