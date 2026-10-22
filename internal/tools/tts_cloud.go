package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"aurago/internal/security"
)

func ttsCloud(cfg TTSConfig, text string) ([]byte, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	var endpoint, key, voice, model string
	payload := map[string]any{"input": text, "response_format": "mp3"}
	switch provider {
	case "openai", "stepfun":
		settings := cfg.OpenAI
		endpoint, voice, model = "https://api.openai.com/v1/audio/speech", "coral", "gpt-4o-mini-tts"
		if provider == "stepfun" {
			settings = cfg.StepFun
			endpoint, voice, model = "https://api.stepfun.com/v1/audio/speech", "jingdiannvsheng", "step-tts-mini"
		}
		key = settings.APIKey
		if strings.TrimSpace(settings.VoiceID) != "" {
			voice = strings.TrimSpace(settings.VoiceID)
		}
		if strings.TrimSpace(settings.ModelID) != "" {
			model = strings.TrimSpace(settings.ModelID)
		}
		payload["model"], payload["voice"] = model, voice
	case "xai":
		endpoint, key, voice = "https://api.x.ai/v1/tts", cfg.XAI.APIKey, strings.TrimSpace(cfg.XAI.VoiceID)
		if voice == "" {
			voice = "eve"
		}
		language := strings.TrimSpace(cfg.Language)
		if language == "" {
			language = "auto"
		}
		payload = map[string]any{"text": text, "voice_id": voice, "language": language, "output_format": map[string]string{"codec": "mp3"}}
	case "resemble":
		endpoint, key, voice = "https://f.cluster.resemble.ai/synthesize", cfg.Resemble.APIKey, strings.TrimSpace(cfg.Resemble.VoiceID)
		if voice == "" {
			return nil, fmt.Errorf("Resemble voice ID is required")
		}
		payload = map[string]any{"data": text, "voice_uuid": voice, "output_format": "mp3"}
	default:
		return nil, fmt.Errorf("unsupported cloud TTS provider %q", provider)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("%s API key is required", provider)
	}
	security.RegisterSensitive(key)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode %s speech request: %w", provider, err)
	}
	req, err := http.NewRequestWithContext(ttsRequestContext(cfg), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create %s speech request: %w", provider, err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ttsHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s speech request failed: %w", provider, err)
	}
	defer resp.Body.Close()
	audio, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, fmt.Errorf("read %s speech response: %w", provider, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s speech API returned HTTP %d", provider, resp.StatusCode)
	}
	if provider == "resemble" {
		var result struct {
			Success      bool   `json:"success"`
			AudioContent string `json:"audio_content"`
		}
		if err := json.Unmarshal(audio, &result); err != nil {
			return nil, fmt.Errorf("decode Resemble response: %w", err)
		}
		if !result.Success {
			return nil, fmt.Errorf("Resemble speech synthesis failed")
		}
		audio, err = base64.StdEncoding.DecodeString(result.AudioContent)
		if err != nil {
			return nil, fmt.Errorf("decode Resemble audio: %w", err)
		}
	} else if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "json") {
		return nil, fmt.Errorf("%s returned JSON instead of speech audio", provider)
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("%s returned empty speech audio", provider)
	}
	return audio, nil
}
