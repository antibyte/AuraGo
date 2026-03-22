package tools

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TTSConfig holds TTS provider configuration.
type TTSConfig struct {
	Provider   string // "google", "elevenlabs", or "piper"
	Language   string // BCP-47 language code (e.g. "de", "en")
	DataDir    string // base data directory for storing audio files
	ElevenLabs struct {
		APIKey  string
		VoiceID string
		ModelID string
	}
	Piper struct {
		Port      int    // Wyoming TCP port (default 10200)
		Voice     string // e.g. "de_DE-thorsten-high"
		SpeakerID int    // multi-speaker model index
	}
}

var ttsHTTPClient = &http.Client{Timeout: 30 * time.Second}

// TTSSynthesize generates speech audio from text and returns the filename (relative to data/tts/).
// The file is saved as MP3 in {DataDir}/tts/{hash}.mp3.
func TTSSynthesize(cfg TTSConfig, text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	// Enforce 200 character limit
	if len(text) > 200 {
		text = text[:200]
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
		return filename, nil
	}

	var audioData []byte
	var err error

	switch strings.ToLower(cfg.Provider) {
	case "elevenlabs":
		audioData, err = ttsElevenLabs(cfg, text)
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

	return filename, nil
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
		return nil, fmt.Errorf("google TTS returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
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
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ElevenLabs returned status %d: %s", resp.StatusCode, string(errBody))
	}

	data, err := io.ReadAll(resp.Body)
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
