package telegram

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

// ConvertOggToWav uses ffmpeg to convert a Telegram voice message (ogg/opus) to a wav file.
func ConvertOggToWav(inputPath, outputPath string) error {
	// Check if ffmpeg is on the path
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found on system path: %w", err)
	}

	// ffmpeg -i <input> <output> with 60s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", inputPath, outputPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// TranscribeVoice sends an audio file to the OpenAI Whisper API for transcription.
func TranscribeVoice(filePath string, cfg *config.Config) (string, error) {
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
		return "", fmt.Errorf("transcription failed: %w", err)
	}

	return resp.Text, nil
}

// ConvertOggToMp3 uses ffmpeg to convert a Telegram voice message (ogg/opus) to an mp3 file.
func ConvertOggToMp3(inputPath, outputPath string) error {
	// Check if ffmpeg is on the path
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found on system path: %w", err)
	}

	// ffmpeg -y -i <input> -vn -ar 44100 -ac 2 -b:a 192k <output> with 60s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", inputPath, "-vn", "-ar", "44100", "-ac", "2", "-b:a", "192k", outputPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg conversion to mp3 failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// TranscribeMultimodal sends an audio file to OpenRouter using a multimodal LLM for transcription.
func TranscribeMultimodal(filePath string, cfg *config.Config) (string, error) {
	apiKey := cfg.Whisper.APIKey
	if apiKey == "" {
		apiKey = cfg.LLM.APIKey
	}

	baseURL := cfg.Whisper.BaseURL
	if baseURL == "" {
		baseURL = cfg.LLM.BaseURL
	}

	model := cfg.Whisper.Model
	if model == "" {
		model = "google/gemini-2.5-flash-lite-preview-09-2025"
	}

	// 1. Read and Base64 encode the audio file
	audioData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read audio file: %w", err)
	}
	encodedAudio := base64.StdEncoding.EncodeToString(audioData)

	// 2. Construct Custom OpenRouter Payload
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
							Format: "mp3",
						},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 3. Send Request to OpenRouter
	reqURL := baseURL + "/chat/completions"
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	// OpenRouter specific headers (good practice)
	req.Header.Set("HTTP-Referer", "https://github.com/andre/aurago")
	req.Header.Set("X-Title", "AuraGo")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openrouter API error (status %d): %s", resp.StatusCode, string(body))
	}

	// 4. Parse Response
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no transcription received in response")
	}

	return result.Choices[0].Message.Content, nil
}
