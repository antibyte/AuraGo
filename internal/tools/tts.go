package tools

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TTSConfig holds TTS provider configuration.
type TTSConfig struct {
	Provider            string // "google", "elevenlabs", "minimax", "piper", or "supertonic"
	Language            string // BCP-47 language code (e.g. "de", "en")
	DataDir             string // base data directory for storing audio files
	CacheRetentionHours int    // remove cached files older than this many hours; 0 disables age-based cleanup
	CacheMaxFiles       int    // keep at most this many cached files; 0 disables count-based cleanup
	ElevenLabs          struct {
		APIKey  string
		VoiceID string
		ModelID string
	}
	Piper struct {
		Port      int    // Wyoming TCP port (default 10200)
		Voice     string // e.g. "de_DE-thorsten-high"
		SpeakerID int    // multi-speaker model index
	}
	Supertonic struct {
		URL            string
		Model          string
		Voice          string
		Speed          float64
		Steps          int
		ResponseFormat string
	}
	MiniMax struct {
		APIKey  string
		VoiceID string  // e.g. "English_expressive_narrator"
		ModelID string  // "speech-2.8-hd" or "speech-2.8-turbo"
		Speed   float64 // 0.5–2.0; 0 means default (1.0)
	}
}

var ttsHTTPClient = &http.Client{Timeout: 30 * time.Second}

// TTSSynthesize generates speech audio from text and returns the filename (relative to data/tts/).
// The file is saved as MP3 in {DataDir}/tts/{hash}.mp3.
func TTSSynthesize(cfg TTSConfig, text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	// Enforce 500 character limit (rune-safe to handle multi-byte UTF-8)
	if len([]rune(text)) > 500 {
		text = string([]rune(text)[:500])
	}

	// Ensure output directory exists
	ttsDir := filepath.Join(cfg.DataDir, "tts")
	if err := os.MkdirAll(ttsDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create TTS directory: %w", err)
	}

	// Generate a hash-based filename for caching. Provider-specific settings are part
	// of the key so changing voices, models, or audio formats never returns stale audio.
	hash := fmt.Sprintf("%x", md5.Sum([]byte(ttsCacheKey(cfg, text))))
	ext := ttsAudioExtension(cfg)
	filename := hash + ext
	filePath := filepath.Join(ttsDir, filename)

	// Return cached file if it exists
	if _, err := os.Stat(filePath); err == nil {
		now := time.Now()
		_ = os.Chtimes(filePath, now, now)
		_ = cleanupTTSCache(cfg, filename, now)
		return filename, nil
	}

	var audioData []byte
	var err error

	switch strings.ToLower(cfg.Provider) {
	case "elevenlabs":
		audioData, err = ttsElevenLabs(cfg, text)
	case "minimax":
		audioData, err = ttsMiniMax(cfg, text)
	case "piper":
		audioData, err = ttsPiper(cfg, text)
	case "supertonic":
		audioData, err = ttsSupertonic(cfg, text)
	default: // "google" or fallback
		audioData, err = ttsGoogle(text, cfg.Language)
	}

	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filePath, audioData, 0o644); err != nil {
		return "", fmt.Errorf("failed to write audio file: %w", err)
	}

	_ = cleanupTTSCache(cfg, filename, time.Now())

	return filename, nil
}

func ttsCacheKey(cfg TTSConfig, text string) string {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "elevenlabs":
		return strings.Join([]string{
			provider,
			cfg.Language,
			cfg.ElevenLabs.VoiceID,
			cfg.ElevenLabs.ModelID,
			text,
		}, "\x00")
	case "minimax":
		return strings.Join([]string{
			provider,
			cfg.Language,
			cfg.MiniMax.VoiceID,
			cfg.MiniMax.ModelID,
			fmt.Sprintf("%.3f", cfg.MiniMax.Speed),
			text,
		}, "\x00")
	case "piper":
		return strings.Join([]string{
			provider,
			cfg.Language,
			fmt.Sprintf("%d", cfg.Piper.Port),
			cfg.Piper.Voice,
			fmt.Sprintf("%d", cfg.Piper.SpeakerID),
			text,
		}, "\x00")
	case "supertonic":
		return strings.Join([]string{
			provider,
			cfg.Language,
			cfg.Supertonic.URL,
			cfg.Supertonic.Model,
			cfg.Supertonic.Voice,
			fmt.Sprintf("%.3f", cfg.Supertonic.Speed),
			fmt.Sprintf("%d", cfg.Supertonic.Steps),
			supertonicResponseFormat(cfg.Supertonic.ResponseFormat),
			text,
		}, "\x00")
	default:
		return cfg.Provider + cfg.Language + text
	}
}

func ttsAudioExtension(cfg TTSConfig) string {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "piper":
		return ".wav"
	case "supertonic":
		return "." + supertonicResponseFormat(cfg.Supertonic.ResponseFormat)
	default:
		return ".mp3"
	}
}

type ttsCacheFile struct {
	name    string
	path    string
	modTime time.Time
}

func cleanupTTSCache(cfg TTSConfig, keepFilename string, now time.Time) error {
	if cfg.CacheRetentionHours <= 0 && cfg.CacheMaxFiles <= 0 {
		return nil
	}

	ttsDir := TTSAudioDir(cfg.DataDir)
	entries, err := os.ReadDir(ttsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read TTS cache directory: %w", err)
	}

	var retained []ttsCacheFile
	var cutoff time.Time
	if cfg.CacheRetentionHours > 0 {
		cutoff = now.Add(-time.Duration(cfg.CacheRetentionHours) * time.Hour)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(ttsDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if cfg.CacheRetentionHours > 0 && name != keepFilename && info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
			continue
		}
		retained = append(retained, ttsCacheFile{name: name, path: path, modTime: info.ModTime()})
	}

	if cfg.CacheMaxFiles <= 0 || len(retained) <= cfg.CacheMaxFiles {
		return nil
	}

	sort.Slice(retained, func(i, j int) bool {
		return retained[i].modTime.Before(retained[j].modTime)
	})

	removeCount := len(retained) - cfg.CacheMaxFiles
	for _, file := range retained {
		if removeCount <= 0 {
			break
		}
		if file.name == keepFilename {
			continue
		}
		_ = os.Remove(file.path)
		removeCount--
	}

	return nil
}

// ttsGoogle uses Google Translate's TTS endpoint (free, max ~200 chars).
func ttsGoogle(text, lang string) ([]byte, error) {
	if lang == "" {
		lang = "en"
	}

	u := fmt.Sprintf("https://translate.google.com/translate_tts?ie=UTF-8&tl=%s&client=tw-ob&q=%s",
		url.QueryEscape(lang),
		url.QueryEscape(text),
	)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := ttsHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
		if err != nil {
			return nil, fmt.Errorf("google TTS returned status %d and failed to read response body: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("google TTS returned status %d: %s", resp.StatusCode, string(body))
	}

	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return data, nil
}

// ttsElevenLabs uses the ElevenLabs API for high-quality TTS.
func ttsElevenLabs(cfg TTSConfig, text string) ([]byte, error) {
	if cfg.ElevenLabs.APIKey == "" {
		return nil, fmt.Errorf("ElevenLabs API key is required")
	}
	voiceID := cfg.ElevenLabs.VoiceID
	if voiceID == "" {
		voiceID = "21m00Tcm4TlvDq8ikWAM" // Default: Rachel
	}
	modelID := cfg.ElevenLabs.ModelID
	if modelID == "" {
		modelID = "eleven_multilingual_v2"
	}

	apiURL := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s", voiceID)

	body := fmt.Sprintf(`{"text":%q,"model_id":%q,"voice_settings":{"stability":0.5,"similarity_boost":0.75}}`,
		text, modelID)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "audio/mpeg")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", cfg.ElevenLabs.APIKey)

	resp, err := ttsHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ElevenLabs request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
		if err != nil {
			return nil, fmt.Errorf("ElevenLabs returned status %d and failed to read response body: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("ElevenLabs returned status %d: %s", resp.StatusCode, string(errBody))
	}

	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return data, nil
}

// TTSAudioDir returns the path to the TTS audio directory.
func TTSAudioDir(dataDir string) string {
	return filepath.Join(dataDir, "tts")
}

// CastMediaDir returns the path to local media published for Chromecast playback.
func CastMediaDir(dataDir string) string {
	return filepath.Join(dataDir, "cast_media")
}

// CastMediaMIMEType returns a supported Chromecast media MIME type by extension.
func CastMediaMIMEType(filename string) (string, bool) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp3":
		return "audio/mpeg", true
	case ".wav":
		return "audio/wav", true
	case ".ogg":
		return "audio/ogg", true
	case ".flac":
		return "audio/flac", true
	case ".m4a", ".aac":
		return "audio/mp4", true
	case ".opus":
		return "audio/opus", true
	case ".mp4":
		return "video/mp4", true
	case ".webm":
		return "video/webm", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".png":
		return "image/png", true
	case ".gif":
		return "image/gif", true
	case ".webp":
		return "image/webp", true
	default:
		return "", false
	}
}

// ttsPiper uses a local Piper container via the Wyoming protocol.
func ttsPiper(cfg TTSConfig, text string) ([]byte, error) {
	port := cfg.Piper.Port
	if port <= 0 {
		port = 10200
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	conn, err := WyomingConnect(addr)
	if err != nil {
		return nil, fmt.Errorf("piper connect: %w", err)
	}
	defer conn.Close()

	// If voice is "piper" (TTS system name, not a real voice), clear it so Piper uses its default.
	voice := cfg.Piper.Voice
	if voice == "piper" {
		voice = ""
	}

	pcm, rate, width, channels, err := WyomingSynthesize(conn, text, voice, cfg.Piper.SpeakerID)
	if err != nil {
		return nil, fmt.Errorf("piper synthesize: %w", err)
	}
	if len(pcm) == 0 {
		return nil, fmt.Errorf("piper returned empty audio")
	}

	return PCMToWAV(pcm, rate, width, channels), nil
}

// ttsSupertonic calls the native Supertonic HTTP API exposed by the managed sidecar.
func ttsSupertonic(cfg TTSConfig, text string) ([]byte, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.Supertonic.URL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("Supertonic URL is required")
	}

	voice := strings.TrimSpace(cfg.Supertonic.Voice)
	if voice == "" {
		voice = "M1"
	}
	lang := strings.TrimSpace(cfg.Language)
	if lang == "" {
		lang = "na"
	}
	speed := cfg.Supertonic.Speed
	if speed <= 0 {
		speed = 1.0
	}
	steps := cfg.Supertonic.Steps
	if steps <= 0 {
		steps = 8
	}
	format := supertonicResponseFormat(cfg.Supertonic.ResponseFormat)

	payload := map[string]interface{}{
		"text":            text,
		"voice":           voice,
		"lang":            lang,
		"speed":           speed,
		"steps":           steps,
		"response_format": format,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode Supertonic request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/tts", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create Supertonic request: %w", err)
	}
	req.Header.Set("Accept", "audio/"+format)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ttsHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Supertonic request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("Supertonic returned status %d and failed to read response body: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("failed to read Supertonic response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if msg := supertonicErrorMessage(respBody); msg != "" {
			return nil, fmt.Errorf("Supertonic API error %d: %s", resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("Supertonic API error %d: %s", resp.StatusCode, string(respBody))
	}
	if len(respBody) == 0 {
		return nil, fmt.Errorf("Supertonic returned empty audio data")
	}
	return respBody, nil
}

func supertonicResponseFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "flac":
		return "flac"
	case "ogg":
		return "ogg"
	default:
		return "wav"
	}
}

func supertonicErrorMessage(body []byte) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
		Detail interface{} `json:"detail"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return ""
	}
	if envelope.Error.Message != "" {
		parts := []string{envelope.Error.Message}
		if envelope.Error.Code != "" {
			parts = append(parts, "code="+envelope.Error.Code)
		}
		if envelope.Error.Type != "" {
			parts = append(parts, "type="+envelope.Error.Type)
		}
		return strings.Join(parts, ", ")
	}
	if envelope.Detail != nil {
		return fmt.Sprintf("%v", envelope.Detail)
	}
	return ""
}

// ttsMiniMax calls the MiniMax T2A v2 API (non-streaming, hex output) and returns
// the decoded MP3 audio bytes.
func ttsMiniMax(cfg TTSConfig, text string) ([]byte, error) {
	if cfg.MiniMax.APIKey == "" {
		return nil, fmt.Errorf("MiniMax API key not configured")
	}

	model := cfg.MiniMax.ModelID
	model = miniMaxTTSModelForAPI(model)
	voiceID := cfg.MiniMax.VoiceID
	if voiceID == "" {
		voiceID = "English_expressive_narrator"
	}
	speed := cfg.MiniMax.Speed
	if speed == 0 {
		speed = 1.0
	}

	payload := map[string]interface{}{
		"model":  model,
		"text":   text,
		"stream": false,
		"voice_setting": map[string]interface{}{
			"voice_id": voiceID,
			"speed":    speed,
			"vol":      1,
			"pitch":    0,
		},
		"audio_setting": map[string]interface{}{
			"format":      "mp3",
			"sample_rate": 32000,
			"bitrate":     128000,
			"channel":     1,
		},
		"output_format": "hex",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode MiniMax request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.minimax.io/v1/t2a_v2", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create MiniMax request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.MiniMax.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ttsHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MiniMax request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read MiniMax response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MiniMax API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			Audio string `json:"audio"`
		} `json:"data"`
		BaseResp struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse MiniMax response: %w", err)
	}
	if result.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("MiniMax API error %d: %s", result.BaseResp.StatusCode, result.BaseResp.StatusMsg)
	}
	if result.Data.Audio == "" {
		return nil, fmt.Errorf("MiniMax returned empty audio data")
	}

	audioData, err := hex.DecodeString(result.Data.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to decode MiniMax audio hex: %w", err)
	}
	return audioData, nil
}

func miniMaxTTSModelForAPI(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "":
		return "speech-2.8-hd"
	case "speech-02-hd":
		return "speech-2.8-hd"
	case "speech-02-turbo":
		return "speech-2.8-turbo"
	default:
		return strings.TrimSpace(model)
	}
}
