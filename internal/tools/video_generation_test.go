package tools

import (
	"context"
	"encoding/base64"
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

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/video_generation":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method for create: %s", r.Method)
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
	cfg.VideoGeneration.ResolvedModel = "Hailuo-2.3-768P"
	cfg.VideoGeneration.DefaultDurationSeconds = 6
	cfg.VideoGeneration.DefaultResolution = "768P"
	cfg.VideoGeneration.PollIntervalSeconds = -1

	result := GenerateVideoResult(context.Background(), cfg, db, slog.Default(), VideoGenParams{Prompt: "a calm lake at sunrise"})
	if result.Status != "ok" {
		t.Fatalf("status = %q, error = %q", result.Status, result.Error)
	}
	if result.Provider != "minimax" || result.Model != "Hailuo-2.3-768P" {
		t.Fatalf("provider/model = %s/%s", result.Provider, result.Model)
	}
	if result.WebPath == "" || result.MediaID == 0 {
		t.Fatalf("expected web path and media registry id, got web=%q id=%d", result.WebPath, result.MediaID)
	}
	if got := fileSizeOrZero(result.FilePath); got != int64(len(videoBytes)) {
		t.Fatalf("file size = %d, want %d", got, len(videoBytes))
	}
}

func TestGenerateVideoGoogleFlow(t *testing.T) {
	tmpDir := t.TempDir()
	videoBytes := []byte("fake google mp4 data")
	encoded := base64.StdEncoding.EncodeToString(videoBytes)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
