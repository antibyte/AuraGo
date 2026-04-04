package tools

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"

	"github.com/google/uuid"
)

// MusicGenParams holds the parameters for the generate_music tool call.
type MusicGenParams struct {
	Prompt       string `json:"prompt"`
	Lyrics       string `json:"lyrics"`
	Instrumental bool   `json:"instrumental"`
	Title        string `json:"title"`
}

// MusicGenResult holds the result of a music generation.
type MusicGenResult struct {
	Status     string `json:"status"`
	Title      string `json:"title,omitempty"`
	Filename   string `json:"filename,omitempty"`
	FilePath   string `json:"file_path,omitempty"`
	WebPath    string `json:"web_path,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	Format     string `json:"format,omitempty"`
	FileSize   int64  `json:"file_size,omitempty"`
	MediaID    int64  `json:"media_id,omitempty"`
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
}

// musicDailyCounter tracks daily usage for music generation.
type musicDailyCounter struct {
	mu    sync.Mutex
	date  string
	count int
}

var musicCounter = &musicDailyCounter{}

// musicCounterIncrement checks and increments the daily counter.
// Returns (current_count, allowed).
func musicCounterIncrement(maxDaily int) (int, bool) {
	musicCounter.mu.Lock()
	defer musicCounter.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if musicCounter.date != today {
		musicCounter.date = today
		musicCounter.count = 0
	}
	if maxDaily > 0 && musicCounter.count >= maxDaily {
		return musicCounter.count, false
	}
	musicCounter.count++
	return musicCounter.count, true
}

// MusicCounterGet returns the current daily count (for display).
func MusicCounterGet() int {
	musicCounter.mu.Lock()
	defer musicCounter.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if musicCounter.date != today {
		return 0
	}
	return musicCounter.count
}

// GenerateMusic is the main entry point for the generate_music tool.
func GenerateMusic(ctx context.Context, cfg *config.Config, mediaDB *sql.DB, logger *slog.Logger, params MusicGenParams) string {
	if params.Prompt == "" {
		return mustJSON(MusicGenResult{Status: "error", Error: "'prompt' is required for music generation."})
	}

	providerType := cfg.MusicGeneration.ProviderType
	apiKey := cfg.MusicGeneration.APIKey
	model := cfg.MusicGeneration.ResolvedModel

	logger.Info("Music generation requested", "provider_type", providerType, "prompt_len", len(params.Prompt), "instrumental", params.Instrumental)

	if apiKey == "" {
		return mustJSON(MusicGenResult{Status: "error", Error: "Music generation provider not configured. Set a provider in Settings > Music Generation."})
	}

	// Check daily limit
	if cfg.MusicGeneration.MaxDaily > 0 {
		count, allowed := musicCounterIncrement(cfg.MusicGeneration.MaxDaily)
		if !allowed {
			return mustJSON(MusicGenResult{
				Status: "error",
				Error:  fmt.Sprintf("Daily music generation limit reached (%d/%d). Try again tomorrow or increase the limit in settings.", count, cfg.MusicGeneration.MaxDaily),
			})
		}
	} else {
		musicCounterIncrement(0)
	}

	// Ensure audio output directory exists
	audioDir := filepath.Join(cfg.Directories.DataDir, "audio")
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		return mustJSON(MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to create audio directory: %v", err)})
	}

	var result MusicGenResult
	switch strings.ToLower(providerType) {
	case "minimax":
		result = generateMusicMiniMax(ctx, apiKey, model, params, audioDir, logger)
	case "google", "google_lyria":
		result = generateMusicGoogleLyria(ctx, apiKey, model, params, audioDir, logger)
	default:
		return mustJSON(MusicGenResult{Status: "error", Error: fmt.Sprintf("Unknown music generation provider type: %q. Supported: minimax, google", providerType)})
	}

	// Register in media registry
	if result.Status == "ok" && mediaDB != nil {
		tags := []string{"auto-generated", "music"}
		if params.Instrumental {
			tags = append(tags, "instrumental")
		}
		title := params.Title
		if title == "" {
			title = truncateString(params.Prompt, 100)
		}
		regID, _, regErr := RegisterMedia(mediaDB, MediaItem{
			MediaType:   "music",
			SourceTool:  "generate_music",
			Filename:    result.Filename,
			FilePath:    result.FilePath,
			WebPath:     result.WebPath,
			FileSize:    result.FileSize,
			Format:      result.Format,
			Provider:    result.Provider,
			Model:       result.Model,
			Prompt:      params.Prompt,
			Description: title,
			DurationMs:  result.DurationMs,
			Tags:        tags,
		})
		if regErr != nil {
			logger.Warn("Failed to register music in media registry", "error", regErr)
		} else {
			result.MediaID = regID
		}
	}

	return mustJSON(result)
}

// --- MiniMax Music API ---

type miniMaxMusicRequest struct {
	Model          string              `json:"model"`
	Prompt         string              `json:"prompt,omitempty"`
	Lyrics         string              `json:"lyrics,omitempty"`
	OutputFormat   string              `json:"output_format"`
	AudioSetting   miniMaxAudioSetting `json:"audio_setting"`
	LyricsOptimize bool                `json:"lyrics_optimizer,omitempty"`
	IsInstrumental bool                `json:"is_instrumental,omitempty"`
}

type miniMaxAudioSetting struct {
	SampleRate int    `json:"sample_rate"`
	Bitrate    int    `json:"bitrate"`
	Format     string `json:"format"`
}

type miniMaxMusicResponse struct {
	Data struct {
		Audio  string `json:"audio"`
		Status int    `json:"status"`
	} `json:"data"`
	ExtraInfo struct {
		MusicDuration   int64 `json:"music_duration"`
		MusicSampleRate int   `json:"music_sample_rate"`
		MusicChannel    int   `json:"music_channel"`
		Bitrate         int   `json:"bitrate"`
		MusicSize       int64 `json:"music_size"`
	} `json:"extra_info"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

func generateMusicMiniMax(ctx context.Context, apiKey, model string, params MusicGenParams, audioDir string, logger *slog.Logger) MusicGenResult {
	if model == "" {
		model = "music-2.5+"
	}

	reqBody := miniMaxMusicRequest{
		Model:        model,
		Prompt:       params.Prompt,
		OutputFormat: "url",
		AudioSetting: miniMaxAudioSetting{
			SampleRate: 44100,
			Bitrate:    256000,
			Format:     "mp3",
		},
	}

	if params.Instrumental {
		reqBody.IsInstrumental = true
	} else if params.Lyrics != "" {
		reqBody.Lyrics = params.Lyrics
	} else {
		// Auto-generate lyrics from prompt
		reqBody.LyricsOptimize = true
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to build request: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.minimax.io/v1/music_generation", bytes.NewReader(bodyBytes))
	if err != nil {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to create request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("MiniMax API request failed: %v", err)}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to read response: %v", err)}
	}

	if resp.StatusCode != http.StatusOK {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("MiniMax API returned status %d: %s", resp.StatusCode, truncateString(string(respBody), 500))}
	}

	var mmResp miniMaxMusicResponse
	if err := json.Unmarshal(respBody, &mmResp); err != nil {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to parse MiniMax response: %v", err)}
	}

	if mmResp.BaseResp.StatusCode != 0 {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("MiniMax API error: %s (code %d)", mmResp.BaseResp.StatusMsg, mmResp.BaseResp.StatusCode)}
	}

	audioData := mmResp.Data.Audio
	if audioData == "" {
		return MusicGenResult{Status: "error", Error: "MiniMax returned empty audio data"}
	}

	// Generate filename
	filename := fmt.Sprintf("music_%s.mp3", uuid.New().String()[:8])
	filePath := filepath.Join(audioDir, filename)

	// Handle URL or hex output
	if strings.HasPrefix(audioData, "http") {
		// Download from URL
		if err := downloadFile(ctx, audioData, filePath); err != nil {
			return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to download audio: %v", err)}
		}
	} else {
		// Hex-encoded data
		decoded, err := hex.DecodeString(audioData)
		if err != nil {
			return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to decode hex audio: %v", err)}
		}
		if err := os.WriteFile(filePath, decoded, 0644); err != nil {
			return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to write audio file: %v", err)}
		}
	}

	// Get file size
	stat, _ := os.Stat(filePath)
	fileSize := int64(0)
	if stat != nil {
		fileSize = stat.Size()
	}

	title := params.Title
	if title == "" {
		title = truncateString(params.Prompt, 100)
	}

	return MusicGenResult{
		Status:     "ok",
		Title:      title,
		Filename:   filename,
		FilePath:   filePath,
		WebPath:    "/api/media/audio/" + filename,
		DurationMs: mmResp.ExtraInfo.MusicDuration,
		Provider:   "minimax",
		Model:      model,
		Format:     "mp3",
		FileSize:   fileSize,
		Message:    fmt.Sprintf("Music generated successfully: %s (%.1fs)", title, float64(mmResp.ExtraInfo.MusicDuration)/1000.0),
	}
}

// --- Google Lyria API ---

type lyriaRequest struct {
	Contents []lyriaContent `json:"contents"`
	Config   lyriaConfig    `json:"config"`
}

type lyriaContent struct {
	Role  string      `json:"role"`
	Parts []lyriaPart `json:"parts"`
}

type lyriaPart struct {
	Text string `json:"text,omitempty"`
}

type lyriaConfig struct {
	ResponseModalities []string `json:"response_modalities"`
	ResponseMimeType   string   `json:"response_mime_type"`
}

type lyriaResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text       string `json:"text,omitempty"`
				InlineData *struct {
					MimeType string `json:"mime_type"`
					Data     string `json:"data"`
				} `json:"inline_data,omitempty"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func generateMusicGoogleLyria(ctx context.Context, apiKey, model string, params MusicGenParams, audioDir string, logger *slog.Logger) MusicGenResult {
	if model == "" {
		model = "lyria-3-clip-preview"
	}

	// Build prompt text
	promptText := params.Prompt
	if params.Lyrics != "" && !params.Instrumental {
		promptText += "\n\nLyrics:\n" + params.Lyrics
	}
	if params.Instrumental {
		promptText += "\n\nInstrumental only, no vocals."
	}

	reqBody := lyriaRequest{
		Contents: []lyriaContent{
			{
				Role:  "user",
				Parts: []lyriaPart{{Text: promptText}},
			},
		},
		Config: lyriaConfig{
			ResponseModalities: []string{"AUDIO", "TEXT"},
			ResponseMimeType:   "audio/mp3",
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to build request: %v", err)}
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to create request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("Google Lyria API request failed: %v", err)}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to read response: %v", err)}
	}

	if resp.StatusCode != http.StatusOK {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("Google Lyria API returned status %d: %s", resp.StatusCode, truncateString(string(respBody), 500))}
	}

	var lResp lyriaResponse
	if err := json.Unmarshal(respBody, &lResp); err != nil {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to parse Lyria response: %v", err)}
	}

	if lResp.Error != nil {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("Google Lyria API error: %s (code %d)", lResp.Error.Message, lResp.Error.Code)}
	}

	if len(lResp.Candidates) == 0 {
		return MusicGenResult{Status: "error", Error: "Google Lyria returned no candidates"}
	}

	// Extract audio data from response
	var audioBytes []byte
	for _, part := range lResp.Candidates[0].Content.Parts {
		if part.InlineData != nil {
			decoded, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
			if err != nil {
				return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to decode base64 audio: %v", err)}
			}
			audioBytes = decoded
			break
		}
	}

	if len(audioBytes) == 0 {
		return MusicGenResult{Status: "error", Error: "Google Lyria returned no audio data"}
	}

	filename := fmt.Sprintf("music_%s.mp3", uuid.New().String()[:8])
	filePath := filepath.Join(audioDir, filename)

	if err := os.WriteFile(filePath, audioBytes, 0644); err != nil {
		return MusicGenResult{Status: "error", Error: fmt.Sprintf("Failed to write audio file: %v", err)}
	}

	title := params.Title
	if title == "" {
		title = truncateString(params.Prompt, 100)
	}

	return MusicGenResult{
		Status:   "ok",
		Title:    title,
		Filename: filename,
		FilePath: filePath,
		WebPath:  "/api/media/audio/" + filename,
		Provider: "google_lyria",
		Model:    model,
		Format:   "mp3",
		FileSize: int64(len(audioBytes)),
		Message:  fmt.Sprintf("Music generated successfully: %s", title),
	}
}

// --- Helpers ---

// downloadFile downloads a URL to a local file path.
func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// mustJSON marshals v to JSON string, returning error JSON on failure.
func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"status":"error","error":"JSON marshal failed"}`
	}
	return string(b)
}

// TestMusicConnection performs a lightweight API connectivity test.
func TestMusicConnection(ctx context.Context, provider, apiKey string) (bool, string) {
	switch provider {
	case "minimax":
		// Send a minimal request with intentionally short lyrics to verify API key
		reqBody := miniMaxMusicRequest{
			Model:  "music-2.5+",
			Prompt: "test",
			Lyrics: "[Verse]\nTest connection\n",
			AudioSetting: miniMaxAudioSetting{
				SampleRate: 44100,
				Bitrate:    128000,
				Format:     "mp3",
			},
			OutputFormat: "url",
		}
		bodyBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.minimax.io/v1/music_generation", bytes.NewReader(bodyBytes))
		if err != nil {
			return false, fmt.Sprintf("Request creation failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return false, fmt.Sprintf("Connection failed: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return false, "Authentication failed — check your API key"
		}
		// Any non-error response (even 200 with processing) means key works
		var mmResp miniMaxMusicResponse
		if err := json.Unmarshal(body, &mmResp); err == nil {
			if mmResp.BaseResp.StatusCode == 0 || mmResp.BaseResp.StatusCode == 1 {
				return true, "Connection successful"
			}
			if mmResp.BaseResp.StatusMsg != "" {
				return false, mmResp.BaseResp.StatusMsg
			}
		}
		// Status 200 is still a good sign
		if resp.StatusCode == 200 {
			return true, "Connection successful"
		}
		return false, fmt.Sprintf("API returned status %d: %s", resp.StatusCode, truncateString(string(body), 200))

	case "google_lyria":
		// Verify API key by sending a minimal generate request
		reqBody := lyriaRequest{
			Contents: []lyriaContent{{
				Role:  "user",
				Parts: []lyriaPart{{Text: "Generate a short 2-second test beep"}},
			}},
			Config: lyriaConfig{
				ResponseModalities: []string{"AUDIO", "TEXT"},
				ResponseMimeType:   "audio/mp3",
			},
		}
		bodyBytes, _ := json.Marshal(reqBody)
		url := "https://generativelanguage.googleapis.com/v1beta/models/lyria-3-clip-preview:generateContent"
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return false, fmt.Sprintf("Request creation failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", apiKey)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return false, fmt.Sprintf("Connection failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return false, "Authentication failed — check your API key"
		}
		if resp.StatusCode == 200 {
			return true, "Connection successful"
		}
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Sprintf("API returned status %d: %s", resp.StatusCode, truncateString(string(body), 200))

	default:
		return false, fmt.Sprintf("Unknown provider: %s", provider)
	}
}
