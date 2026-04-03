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
	Provider            string // "google", "elevenlabs", or "piper"
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
	// Enforce 200 character limit (rune-safe to handle multi-byte UTF-8)
	if len([]rune(text)) > 200 {
		text = string([]rune(text)[:200])
	}

	// Ensure output directory exists
	ttsDir := filepath.Join(cfg.DataDir, "tts")
	if err := os.MkdirAll(ttsDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create TTS directory: %w", err)
	}

	// Generate a hash-based filename for caching
	hash := fmt.Sprintf("%x", md5.Sum([]byte(cfg.Provider+cfg.Language+text)))
	ext := ".mp3"
	if strings.ToLower(cfg.Provider) == "piper" {
		ext = ".wav"
	}
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

// ttsMiniMax calls the MiniMax T2A v2 API (non-streaming, hex output) and returns
// the decoded MP3 audio bytes.
func ttsMiniMax(cfg TTSConfig, text string) ([]byte, error) {
	if cfg.MiniMax.APIKey == "" {
		return nil, fmt.Errorf("MiniMax API key not configured")
	}

	model := cfg.MiniMax.ModelID
	if model == "" {
		model = "speech-2.8-hd"
	}
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
