package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

type noisemakerFakeChatClient struct {
	content string
	err     error
}

func (f noisemakerFakeChatClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if f.err != nil {
		return openai.ChatCompletionResponse{}, f.err
	}
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Content: f.content}},
		},
	}, nil
}

func (f noisemakerFakeChatClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, fmt.Errorf("not implemented")
}

func noisemakerTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	return cfg
}

func TestNoisemakerStateDisabled(t *testing.T) {
	s := &Server{Cfg: noisemakerTestConfig(t), Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/noisemaker/state", nil)
	rec := httptest.NewRecorder()
	handleNoisemakerState(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d", rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["enabled"] != false {
		t.Fatalf("enabled = %#v, want false", payload["enabled"])
	}
	if payload["llm_available"] != false {
		t.Fatalf("llm_available = %#v, want false", payload["llm_available"])
	}
}

func TestNoisemakerStateEnabled(t *testing.T) {
	cfg := noisemakerTestConfig(t)
	cfg.MusicGeneration.Enabled = true
	cfg.MusicGeneration.Provider = "main"
	cfg.MusicGeneration.ProviderType = "minimax"
	cfg.MusicGeneration.APIKey = "test-key"
	cfg.MusicGeneration.ResolvedModel = "music-2.5+"
	cfg.MusicGeneration.MaxDaily = 10
	cfg.ImageGeneration.Enabled = true
	cfg.ImageGeneration.ProviderType = "openai"
	cfg.ImageGeneration.APIKey = "img-key"
	s := &Server{Cfg: cfg, Logger: slog.Default(), LLMClient: noisemakerFakeChatClient{content: "hi"}}

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/noisemaker/state", nil)
	rec := httptest.NewRecorder()
	handleNoisemakerState(s).ServeHTTP(rec, req)

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["enabled"] != true {
		t.Fatalf("enabled = %#v, want true", payload["enabled"])
	}
	if payload["supports_lyrics"] != true {
		t.Fatalf("supports_lyrics = %#v, want true for minimax", payload["supports_lyrics"])
	}
	if payload["covers_enabled"] != true {
		t.Fatalf("covers_enabled = %#v, want true", payload["covers_enabled"])
	}
	if payload["llm_available"] != true {
		t.Fatalf("llm_available = %#v, want true", payload["llm_available"])
	}
	if payload["daily_max"].(float64) != 10 {
		t.Fatalf("daily_max = %#v, want 10", payload["daily_max"])
	}
}

func TestNoisemakerEnhanceRequiresLLM(t *testing.T) {
	s := &Server{Cfg: noisemakerTestConfig(t), Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodPost, "/api/desktop/noisemaker/enhance", strings.NewReader(`{"kind":"idea","text":"test"}`))
	rec := httptest.NewRecorder()
	handleNoisemakerEnhance(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want 503", rec.Code)
	}
}

func TestNoisemakerEnhanceInvalidKind(t *testing.T) {
	s := &Server{Cfg: noisemakerTestConfig(t), Logger: slog.Default(), LLMClient: noisemakerFakeChatClient{content: "x"}}

	req := httptest.NewRequest(http.MethodPost, "/api/desktop/noisemaker/enhance", strings.NewReader(`{"kind":"bogus","text":"test"}`))
	rec := httptest.NewRecorder()
	handleNoisemakerEnhance(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want 400", rec.Code)
	}
}

func TestNoisemakerEnhanceIdeaReturnsText(t *testing.T) {
	s := &Server{Cfg: noisemakerTestConfig(t), Logger: slog.Default(), LLMClient: noisemakerFakeChatClient{content: "  expanded idea  "}}

	req := httptest.NewRequest(http.MethodPost, "/api/desktop/noisemaker/enhance", strings.NewReader(`{"kind":"idea","text":"a song about rain","lang":"de"}`))
	rec := httptest.NewRecorder()
	handleNoisemakerEnhance(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["text"] != "expanded idea" {
		t.Fatalf("text = %#v, want trimmed fake content", payload["text"])
	}
}

func TestNoisemakerGenerateRequiresPrompt(t *testing.T) {
	s := &Server{Cfg: noisemakerTestConfig(t), Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodPost, "/api/desktop/noisemaker/generate", strings.NewReader(`{"prompt":""}`))
	rec := httptest.NewRecorder()
	handleNoisemakerGenerate(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want 400", rec.Code)
	}
}

func TestNoisemakerGenerateDisabled(t *testing.T) {
	s := &Server{Cfg: noisemakerTestConfig(t), Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodPost, "/api/desktop/noisemaker/generate", strings.NewReader(`{"prompt":"epic song"}`))
	rec := httptest.NewRecorder()
	handleNoisemakerGenerate(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want 400", rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("status = %#v, want error", payload["status"])
	}
}

func TestNoisemakerGenerateLyricsRequiredWithoutLLM(t *testing.T) {
	cfg := noisemakerTestConfig(t)
	cfg.MusicGeneration.Enabled = true
	cfg.MusicGeneration.ProviderType = "minimax"
	cfg.MusicGeneration.APIKey = "test-key"
	s := &Server{Cfg: cfg, Logger: slog.Default()} // no LLMClient

	req := httptest.NewRequest(http.MethodPost, "/api/desktop/noisemaker/generate", strings.NewReader(`{"prompt":"epic song","instrumental":false}`))
	rec := httptest.NewRecorder()
	handleNoisemakerGenerate(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want 400", rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != "lyrics_required" {
		t.Fatalf("code = %#v, want lyrics_required", payload["code"])
	}
}

func TestNoisemakerGenerateInstrumentalSkipsAutoLyrics(t *testing.T) {
	cfg := noisemakerTestConfig(t)
	cfg.MusicGeneration.Enabled = true
	cfg.MusicGeneration.ProviderType = "minimax"
	cfg.MusicGeneration.APIKey = "test-key"
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	// Instrumental tracks must not hit the lyrics_required gate; they proceed
	// to the provider call (which then fails without a reachable API).
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/noisemaker/generate", strings.NewReader(`{"prompt":"epic song","instrumental":true}`))
	rec := httptest.NewRecorder()
	handleNoisemakerGenerate(s).ServeHTTP(rec, req)

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] == "lyrics_required" {
		t.Fatalf("instrumental generation must not require lyrics")
	}
}

func noisemakerSetupRegistry(t *testing.T, cfg *config.Config) *Server {
	t.Helper()
	db, err := tools.InitMediaRegistryDB(filepath.Join(cfg.Directories.DataDir, "media_registry.db"))
	if err != nil {
		t.Fatalf("InitMediaRegistryDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &Server{Cfg: cfg, Logger: slog.Default(), MediaRegistryDB: db}
}

func noisemakerRegisterTrack(t *testing.T, s *Server, cfg *config.Config, filename string) int64 {
	t.Helper()
	audioDir := filepath.Join(cfg.Directories.DataDir, "audio")
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	filePath := filepath.Join(audioDir, filename)
	if err := os.WriteFile(filePath, []byte("fake-mp3"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	id, _, err := tools.RegisterMedia(s.MediaRegistryDB, tools.MediaItem{
		MediaType:   "music",
		SourceTool:  "generate_music",
		Filename:    filename,
		FilePath:    filePath,
		WebPath:     "/files/audio/" + filename,
		Format:      "mp3",
		Prompt:      "epic orchestral rain song",
		Description: "Rain Anthem",
		DurationMs:  120000,
		SourceImage: "/files/generated_images/cover.png",
		Tags:        []string{"auto-generated", "music"},
	})
	if err != nil {
		t.Fatalf("RegisterMedia: %v", err)
	}
	return id
}

func TestNoisemakerTracksEmpty(t *testing.T) {
	cfg := noisemakerTestConfig(t)
	s := noisemakerSetupRegistry(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/noisemaker/tracks", nil)
	rec := httptest.NewRecorder()
	handleNoisemakerTracks(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d", rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["total"].(float64) != 0 {
		t.Fatalf("total = %#v, want 0", payload["total"])
	}
}

func TestNoisemakerTracksListsMusic(t *testing.T) {
	cfg := noisemakerTestConfig(t)
	s := noisemakerSetupRegistry(t, cfg)
	noisemakerRegisterTrack(t, s, cfg, "music_test1.mp3")

	// An image item must not appear in the music library.
	if _, _, err := tools.RegisterMedia(s.MediaRegistryDB, tools.MediaItem{
		MediaType: "image", SourceTool: "generate_image", Filename: "img_x.png", Format: "png",
	}); err != nil {
		t.Fatalf("RegisterMedia image: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/noisemaker/tracks", nil)
	rec := httptest.NewRecorder()
	handleNoisemakerTracks(s).ServeHTTP(rec, req)

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items, ok := payload["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("items = %#v, want exactly 1 music track", payload["items"])
	}
	track := items[0].(map[string]interface{})
	if track["title"] != "Rain Anthem" {
		t.Fatalf("title = %#v, want Rain Anthem", track["title"])
	}
	if track["cover_url"] != "/files/generated_images/cover.png" {
		t.Fatalf("cover_url = %#v", track["cover_url"])
	}
	if track["duration_ms"].(float64) != 120000 {
		t.Fatalf("duration_ms = %#v", track["duration_ms"])
	}
}

func noisemakerTrackIDs(t *testing.T, payload map[string]interface{}) []int64 {
	t.Helper()
	items, ok := payload["items"].([]interface{})
	if !ok {
		t.Fatalf("items missing or wrong type: %#v", payload["items"])
	}
	ids := make([]int64, 0, len(items))
	for _, raw := range items {
		ids = append(ids, int64(raw.(map[string]interface{})["id"].(float64)))
	}
	return ids
}

func TestNoisemakerTracksPaginationNewestFirst(t *testing.T) {
	cfg := noisemakerTestConfig(t)
	s := noisemakerSetupRegistry(t, cfg)

	id1 := noisemakerRegisterTrack(t, s, cfg, "music_page1.mp3")
	id2 := noisemakerRegisterTrack(t, s, cfg, "music_page2.mp3")
	id3 := noisemakerRegisterTrack(t, s, cfg, "music_page3.mp3")

	// Deterministic timestamps: id1 oldest, id3 newest.
	for id, ts := range map[int64]string{id1: "2026-01-01 10:00:00", id2: "2026-01-02 10:00:00", id3: "2026-01-03 10:00:00"} {
		if _, err := s.MediaRegistryDB.Exec("UPDATE media_items SET created_at = ? WHERE id = ?", ts, id); err != nil {
			t.Fatalf("update created_at: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/noisemaker/tracks?limit=2&offset=0", nil)
	rec := httptest.NewRecorder()
	handleNoisemakerTracks(s).ServeHTTP(rec, req)
	var page1 map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &page1); err != nil {
		t.Fatalf("decode page 1: %v", err)
	}
	if page1["total"].(float64) != 3 {
		t.Fatalf("total = %#v, want 3", page1["total"])
	}
	if page1["limit"].(float64) != 2 || page1["offset"].(float64) != 0 {
		t.Fatalf("limit/offset = %#v/%#v, want 2/0", page1["limit"], page1["offset"])
	}
	ids := noisemakerTrackIDs(t, page1)
	if len(ids) != 2 || ids[0] != id3 || ids[1] != id2 {
		t.Fatalf("page 1 order = %#v, want [%d %d] (newest first)", ids, id3, id2)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/desktop/noisemaker/tracks?limit=2&offset=2", nil)
	rec = httptest.NewRecorder()
	handleNoisemakerTracks(s).ServeHTTP(rec, req)
	var page2 map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &page2); err != nil {
		t.Fatalf("decode page 2: %v", err)
	}
	ids = noisemakerTrackIDs(t, page2)
	if len(ids) != 1 || ids[0] != id1 {
		t.Fatalf("page 2 = %#v, want [%d] (oldest last)", ids, id1)
	}
}

func TestNoisemakerTracksSearchFilters(t *testing.T) {
	cfg := noisemakerTestConfig(t)
	s := noisemakerSetupRegistry(t, cfg)
	noisemakerRegisterTrack(t, s, cfg, "music_rain.mp3") // prompt "epic orchestral rain song"

	audioDir := filepath.Join(cfg.Directories.DataDir, "audio")
	lofiPath := filepath.Join(audioDir, "music_lofi.mp3")
	if err := os.WriteFile(lofiPath, []byte("fake-mp3"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, _, err := tools.RegisterMedia(s.MediaRegistryDB, tools.MediaItem{
		MediaType: "music", SourceTool: "generate_music", Filename: "music_lofi.mp3",
		FilePath: lofiPath, WebPath: "/files/audio/music_lofi.mp3", Format: "mp3",
		Prompt: "calm lofi beats", Description: "Lofi Dreams",
	}); err != nil {
		t.Fatalf("RegisterMedia: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/noisemaker/tracks?q=rain", nil)
	rec := httptest.NewRecorder()
	handleNoisemakerTracks(s).ServeHTTP(rec, req)
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["total"].(float64) != 1 {
		t.Fatalf("total = %#v, want 1 for q=rain", payload["total"])
	}
	ids := noisemakerTrackIDs(t, payload)
	if len(ids) != 1 {
		t.Fatalf("items = %#v, want exactly the rain track", ids)
	}
}

func TestNoisemakerDeleteNotFound(t *testing.T) {
	cfg := noisemakerTestConfig(t)
	s := noisemakerSetupRegistry(t, cfg)

	req := httptest.NewRequest(http.MethodDelete, "/api/desktop/noisemaker/tracks/999", nil)
	rec := httptest.NewRecorder()
	handleNoisemakerTrackDelete(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want 404", rec.Code)
	}
}

func TestNoisemakerDeleteRejectsNonMusic(t *testing.T) {
	cfg := noisemakerTestConfig(t)
	s := noisemakerSetupRegistry(t, cfg)
	imgID, _, err := tools.RegisterMedia(s.MediaRegistryDB, tools.MediaItem{
		MediaType: "image", SourceTool: "generate_image", Filename: "img_y.png", Format: "png",
	})
	if err != nil {
		t.Fatalf("RegisterMedia: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/desktop/noisemaker/tracks/%d", imgID), nil)
	rec := httptest.NewRecorder()
	handleNoisemakerTrackDelete(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status code = %d, want 403", rec.Code)
	}
}

func TestNoisemakerDeleteMusic(t *testing.T) {
	cfg := noisemakerTestConfig(t)
	s := noisemakerSetupRegistry(t, cfg)
	id := noisemakerRegisterTrack(t, s, cfg, "music_delete.mp3")
	filePath := filepath.Join(cfg.Directories.DataDir, "audio", "music_delete.mp3")

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/desktop/noisemaker/tracks/%d", id), nil)
	rec := httptest.NewRecorder()
	handleNoisemakerTrackDelete(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("audio file still exists after delete")
	}
}

func TestNoisemakerDeleteReadonly(t *testing.T) {
	cfg := noisemakerTestConfig(t)
	cfg.VirtualDesktop.ReadOnly = true
	s := noisemakerSetupRegistry(t, cfg)
	id := noisemakerRegisterTrack(t, s, cfg, "music_readonly.mp3")

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/desktop/noisemaker/tracks/%d", id), nil)
	rec := httptest.NewRecorder()
	handleNoisemakerTrackDelete(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status code = %d, want 403", rec.Code)
	}
}
