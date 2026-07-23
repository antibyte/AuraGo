package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

var multimodalTranscribeHTTPClient = &http.Client{Timeout: 60 * time.Second}

const maxTranscriptionAudioBytes = 100 * 1024 * 1024

// TranscribeAudioFile sends an audio file to the configured Whisper/STT service for transcription.
// It tries the native OpenAI Whisper API first, and falls back to multimodal transcription
// if the provider is set to "multimodal".
func TranscribeAudioFile(filePath string, cfg *config.Config) (string, float64, error) {
	resolvedPath, err := resolveToolInputPath(filePath, cfg)
	if err != nil {
		return "", 0.0, fmt.Errorf("invalid audio file path: %w", err)
	}

	if transcriptionMode(cfg) == "multimodal" {
		return transcribeMultimodal(resolvedPath, cfg)
	}
	return transcribeWhisper(resolvedPath, cfg)
}

// TranscribeAudio transcribes an in-memory audio payload without creating a
// temporary file. fileName is used only as multipart format metadata.
func TranscribeAudio(ctx context.Context, fileName string, audioData []byte, cfg *config.Config) (string, float64, error) {
	if cfg == nil {
		return "", 0, fmt.Errorf("transcription config is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if len(audioData) == 0 {
		return "", 0, fmt.Errorf("audio data is required")
	}
	if len(audioData) > maxTranscriptionAudioBytes {
		return "", 0, fmt.Errorf("audio data too large (%d bytes, max 100 MB)", len(audioData))
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = "audio.wav"
	}
	if transcriptionMode(cfg) == "multimodal" {
		return transcribeMultimodalBytes(ctx, audioData, audioFormatFromName(fileName), cfg)
	}
	return transcribeWhisperBytes(ctx, fileName, audioData, cfg)
}

func transcriptionMode(cfg *config.Config) string {
	mode := strings.ToLower(cfg.Whisper.Mode)
	// OpenRouter does not support OpenAI's /v1/audio/transcriptions endpoint.
	if mode == "" && strings.EqualFold(cfg.Whisper.ProviderType, "openrouter") {
		mode = "multimodal"
	}
	// Non-Whisper models do not implement /v1/audio/transcriptions.
	if mode != "multimodal" && mode != "local" {
		model := strings.ToLower(cfg.Whisper.Model)
		if model != "" && !strings.Contains(model, "whisper") {
			mode = "multimodal"
		}
	}
	return mode
}

// transcribeWhisper uses the standard OpenAI Whisper API.
func transcribeWhisper(filePath string, cfg *config.Config) (string, float64, error) {
	return transcribeWhisperRequest(context.Background(), openai.AudioRequest{
		FilePath: filePath,
	}, cfg)
}

func transcribeWhisperBytes(ctx context.Context, fileName string, audioData []byte, cfg *config.Config) (string, float64, error) {
	return transcribeWhisperRequest(ctx, openai.AudioRequest{
		FilePath: fileName,
		Reader:   bytes.NewReader(audioData),
	}, cfg)
}

func transcribeWhisperRequest(parent context.Context, request openai.AudioRequest, cfg *config.Config) (string, float64, error) {
	// Resolved by config.ResolveProviders (falls back to main LLM)
	apiKey := cfg.Whisper.APIKey
	baseURL := cfg.Whisper.BaseURL

	client := openai.NewClient(apiKey)
	if baseURL != "" {
		c := openai.DefaultConfig(apiKey)
		c.BaseURL = baseURL
		client = openai.NewClientWithConfig(c)
	}

	model := cfg.Whisper.Model
	if model == "" {
		model = openai.Whisper1
	}
	request.Model = model

	timeout := time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	resp, err := client.CreateTranscription(ctx, request)
	if err != nil {
		return "", 0.0, fmt.Errorf("whisper transcription failed: %w", err)
	}

	return security.IsolateExternalData(resp.Text), 0.0, nil
}

// transcribeMultimodal uses a multimodal LLM (e.g. Gemini) via OpenRouter for transcription.
func transcribeMultimodal(filePath string, cfg *config.Config) (string, float64, error) {
	audioInfo, err := os.Stat(filePath)
	if err != nil {
		return "", 0.0, fmt.Errorf("failed to stat audio file: %w", err)
	}
	if audioInfo.Size() > maxTranscriptionAudioBytes {
		return "", 0.0, fmt.Errorf("audio file too large (%d bytes, max 100 MB)", audioInfo.Size())
	}
	audioData, err := os.ReadFile(filePath)
	if err != nil {
		return "", 0.0, fmt.Errorf("failed to read audio file: %w", err)
	}
	return transcribeMultimodalBytes(context.Background(), audioData, audioFormatFromName(filePath), cfg)
}

func transcribeMultimodalBytes(ctx context.Context, audioData []byte, format string, cfg *config.Config) (string, float64, error) {
	// Resolved by config.ResolveProviders (falls back to main LLM)
	apiKey := cfg.Whisper.APIKey
	baseURL := cfg.Whisper.BaseURL

	model := cfg.Whisper.Model
	if model == "" {
		model = "google/gemini-2.5-flash-lite-preview-09-2025"
	}

	encodedAudio := base64.StdEncoding.EncodeToString(audioData)

	type AudioPart struct {
		Data   string `json:"data"`
		Format string `json:"format"`
	}
	type ContentPart struct {
		Type       string     `json:"type"`
		Text       string     `json:"text,omitempty"`
		InputAudio *AudioPart `json:"input_audio,omitempty"`
	}
	type Message struct {
		Role    string        `json:"role"`
		Content []ContentPart `json:"content"`
	}
	type RequestBody struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
	}

	payload := RequestBody{
		Model: model,
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentPart{
					{
						Type: "text",
						Text: "Transcribe the following voice message accurately in the language it is spoken. Output ONLY the raw transcribed text, without any introductory or conversational filler.",
					},
					{
						Type: "input_audio",
						InputAudio: &AudioPart{
							Data:   encodedAudio,
							Format: format,
						},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", 0.0, fmt.Errorf("failed to marshal transcription payload: %w", err)
	}

	reqURL := baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0.0, fmt.Errorf("failed to create transcription request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/antibyte/AuraGo")
	req.Header.Set("X-Title", "AuraGo")

	resp, err := multimodalTranscribeHTTPClient.Do(req)
	if err != nil {
		return "", 0.0, fmt.Errorf("transcription request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
		if err != nil {
			return "", 0.0, fmt.Errorf("transcription API error (status %d) and failed to read response body: %w", resp.StatusCode, err)
		}
		return "", 0.0, fmt.Errorf("transcription API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0.0, fmt.Errorf("failed to decode transcription response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", 0.0, fmt.Errorf("no transcription received in response")
	}

	return security.IsolateExternalData(result.Choices[0].Message.Content), 0.0, nil
}

func audioFormatFromName(fileName string) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".wav":
		return "wav"
	case ".ogg":
		return "ogg"
	case ".flac":
		return "flac"
	case ".m4a":
		return "m4a"
	case ".webm":
		return "webm"
	default:
		return "mp3"
	}
}
