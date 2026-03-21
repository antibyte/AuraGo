package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

// TranscribeAudioFile sends an audio file to the configured Whisper/STT service for transcription.
// It tries the native OpenAI Whisper API first, and falls back to multimodal transcription
// if the provider is set to "multimodal".
func TranscribeAudioFile(filePath string, cfg *config.Config) (string, error) {
	mode := strings.ToLower(cfg.Whisper.Mode)

	// OpenRouter does not support OpenAI's /v1/audio/transcriptions endpoint.
	// If the user hasn't explicitly specified a mode and uses OpenRouter,
	// default to multimodal (embedding base64 audio into a chat completion).
	if mode == "" && strings.EqualFold(cfg.Whisper.ProviderType, "openrouter") {
		mode = "multimodal"
	}

	// If mode is still not "multimodal" or "local", check whether the configured model is
	// actually a Whisper-family model. Non-Whisper models (e.g. google/gemini-*, meta-llama/*)
	// do not implement the /v1/audio/transcriptions endpoint – use multimodal instead.
	if mode != "multimodal" && mode != "local" {
		model := strings.ToLower(cfg.Whisper.Model)
		if model != "" && !strings.Contains(model, "whisper") {
			mode = "multimodal"
		}
	}

	if mode == "multimodal" {
		return transcribeMultimodal(filePath, cfg)
	}
	return transcribeWhisper(filePath, cfg)
}

// transcribeWhisper uses the standard OpenAI Whisper API.
func transcribeWhisper(filePath string, cfg *config.Config) (string, error) {
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

	resp, err := client.CreateTranscription(
		context.Background(),
		openai.AudioRequest{
			Model:    model,
			FilePath: filePath,
		},
	)
	if err != nil {
		return "", fmt.Errorf("whisper transcription failed: %w", err)
	}

	return resp.Text, nil
}

// transcribeMultimodal uses a multimodal LLM (e.g. Gemini) via OpenRouter for transcription.
func transcribeMultimodal(filePath string, cfg *config.Config) (string, error) {
	// Resolved by config.ResolveProviders (falls back to main LLM)
	apiKey := cfg.Whisper.APIKey
	baseURL := cfg.Whisper.BaseURL

	model := cfg.Whisper.Model
	if model == "" {
		model = "google/gemini-2.5-flash-lite-preview-09-2025"
	}

	audioData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read audio file: %w", err)
	}
	encodedAudio := base64.StdEncoding.EncodeToString(audioData)

	// Detect format from file extension
	format := "mp3"
	lower := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lower, ".wav"):
		format = "wav"
	case strings.HasSuffix(lower, ".ogg"):
		format = "ogg"
	case strings.HasSuffix(lower, ".flac"):
		format = "flac"
	case strings.HasSuffix(lower, ".m4a"):
		format = "m4a"
	case strings.HasSuffix(lower, ".webm"):
		format = "webm"
	}

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
		return "", fmt.Errorf("failed to marshal transcription payload: %w", err)
	}

	reqURL := baseURL + "/chat/completions"
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create transcription request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/andre/aurago")
	req.Header.Set("X-Title", "AuraGo")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("transcription request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("transcription API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode transcription response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no transcription received in response")
	}

	return result.Choices[0].Message.Content, nil
}
