package telnyx

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"aurago/internal/config"
)

// InitiateCall places a new outbound call.
func (c *Client) InitiateCall(ctx context.Context, connectionID, from, to string, webhookURL string, timeoutSecs int) (*CallResponse, error) {
	if err := ValidateE164(to); err != nil {
		return nil, fmt.Errorf("invalid 'to' number: %w", err)
	}
	if err := ValidateE164(from); err != nil {
		return nil, fmt.Errorf("invalid 'from' number: %w", err)
	}
	if connectionID == "" {
		return nil, fmt.Errorf("connection_id is required for initiating calls")
	}

	req := CreateCallRequest{
		ConnectionID: connectionID,
		To:           to,
		From:         from,
		TimeoutSecs:  timeoutSecs,
	}
	if webhookURL != "" {
		req.WebhookURL = webhookURL
		req.WebhookURLMethod = "POST"
	}

	data, _, err := c.post(ctx, "/v2/calls", req)
	if err != nil {
		return nil, fmt.Errorf("initiate call: %w", err)
	}
	var resp CallResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode call response: %w", err)
	}
	return &resp, nil
}

// AnswerCall answers an incoming call.
func (c *Client) AnswerCall(ctx context.Context, callControlID string) error {
	path := fmt.Sprintf("/v2/calls/%s/actions/answer", callControlID)
	_, _, err := c.post(ctx, path, map[string]string{})
	return err
}

// SpeakText speaks text on an active call using TTS.
func (c *Client) SpeakText(ctx context.Context, callControlID, text, language, voice string) error {
	if callControlID == "" {
		return fmt.Errorf("call_control_id is required")
	}
	if text == "" {
		return fmt.Errorf("text is required for speak")
	}
	if language == "" {
		language = "en-US"
	}
	if voice == "" {
		voice = "female"
	}

	path := fmt.Sprintf("/v2/calls/%s/actions/speak", callControlID)
	req := SpeakRequest{
		Payload:     text,
		PayloadType: "text",
		Voice:       voice,
		Language:    language,
	}
	_, _, err := c.post(ctx, path, req)
	return err
}

// PlayAudio plays an audio file on an active call.
func (c *Client) PlayAudio(ctx context.Context, callControlID, audioURL string) error {
	if callControlID == "" {
		return fmt.Errorf("call_control_id is required")
	}
	if audioURL == "" {
		return fmt.Errorf("audio_url is required for play_audio")
	}

	path := fmt.Sprintf("/v2/calls/%s/actions/playback_start", callControlID)
	req := PlaybackStartRequest{
		AudioURL: audioURL,
	}
	_, _, err := c.post(ctx, path, req)
	return err
}

// GatherDTMF gathers DTMF digits with a TTS prompt.
func (c *Client) GatherDTMF(ctx context.Context, callControlID, prompt, language, voice string, maxDigits, timeoutSecs int) error {
	if callControlID == "" {
		return fmt.Errorf("call_control_id is required")
	}
	if prompt == "" {
		return fmt.Errorf("text prompt is required for gather_dtmf")
	}
	if language == "" {
		language = "en-US"
	}
	if voice == "" {
		voice = "female"
	}
	if maxDigits <= 0 {
		maxDigits = 1
	}
	if timeoutSecs <= 0 {
		timeoutSecs = 10
	}

	path := fmt.Sprintf("/v2/calls/%s/actions/gather_using_speak", callControlID)
	req := GatherSpeakRequest{
		Payload:       prompt,
		PayloadType:   "text",
		Voice:         voice,
		Language:      language,
		MaximumDigits: maxDigits,
		TimeoutMillis: timeoutSecs * 1000,
	}
	_, _, err := c.post(ctx, path, req)
	return err
}

// TransferCall transfers an active call to another number.
func (c *Client) TransferCall(ctx context.Context, callControlID, to, from string) error {
	if callControlID == "" {
		return fmt.Errorf("call_control_id is required")
	}
	if err := ValidateE164(to); err != nil {
		return fmt.Errorf("invalid transfer 'to' number: %w", err)
	}

	path := fmt.Sprintf("/v2/calls/%s/actions/transfer", callControlID)
	req := TransferRequest{
		To:   to,
		From: from,
	}
	_, _, err := c.post(ctx, path, req)
	return err
}

// RecordStart starts recording an active call.
func (c *Client) RecordStart(ctx context.Context, callControlID, format, channels string) error {
	if callControlID == "" {
		return fmt.Errorf("call_control_id is required")
	}
	if format == "" {
		format = "mp3"
	}
	if channels == "" {
		channels = "single"
	}

	path := fmt.Sprintf("/v2/calls/%s/actions/record_start", callControlID)
	req := RecordStartRequest{
		Format:   format,
		Channels: channels,
	}
	_, _, err := c.post(ctx, path, req)
	return err
}

// RecordStop stops recording an active call.
func (c *Client) RecordStop(ctx context.Context, callControlID string) error {
	if callControlID == "" {
		return fmt.Errorf("call_control_id is required")
	}

	path := fmt.Sprintf("/v2/calls/%s/actions/record_stop", callControlID)
	_, _, err := c.post(ctx, path, map[string]string{})
	return err
}

// HangUp ends an active call.
func (c *Client) HangUp(ctx context.Context, callControlID string) error {
	if callControlID == "" {
		return fmt.Errorf("call_control_id is required")
	}

	path := fmt.Sprintf("/v2/calls/%s/actions/hangup", callControlID)
	_, _, err := c.post(ctx, path, map[string]string{})
	return err
}

// DispatchCall handles telnyx_call tool calls from the agent.
func DispatchCall(ctx context.Context, operation, to, callControlID, text, audioURL string, maxDigits, timeoutSecs int, cfg *config.Config, logger *slog.Logger) string {
	if cfg.Telnyx.ReadOnly && operation != "list_active" {
		return encodeResult("error", "Telnyx is in read-only mode")
	}
	client := NewClient(cfg.Telnyx.APIKey, logger)
	language := cfg.Telnyx.VoiceLanguage
	voice := cfg.Telnyx.VoiceGender

	switch operation {
	case "initiate":
		if to == "" {
			return encodeResult("error", "parameter 'to' is required for initiate")
		}
		if cfg.Telnyx.ConnectionID == "" {
			return encodeResult("error", "telnyx connection_id not configured — required for voice calls")
		}
		webhookURL := ""
		if cfg.Telnyx.WebhookPath != "" {
			webhookURL = cfg.Telnyx.WebhookPath
		}
		timeout := timeoutSecs
		if timeout <= 0 {
			timeout = cfg.Telnyx.CallTimeout
		}
		resp, err := client.InitiateCall(ctx, cfg.Telnyx.ConnectionID, cfg.Telnyx.PhoneNumber, to, webhookURL, timeout)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to initiate call: %v", err))
		}
		return encodeJSON(map[string]interface{}{
			"status":          "success",
			"call_control_id": resp.Data.CallControlID,
			"call_session_id": resp.Data.CallSessionID,
			"state":           resp.Data.State,
			"message":         fmt.Sprintf("Call initiated to %s", to),
		})

	case "speak":
		if callControlID == "" {
			return encodeResult("error", "parameter 'call_control_id' is required for speak")
		}
		if text == "" {
			return encodeResult("error", "parameter 'text' is required for speak")
		}
		err := client.SpeakText(ctx, callControlID, text, language, voice)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to speak: %v", err))
		}
		return encodeJSON(map[string]interface{}{
			"status":  "success",
			"message": "TTS playback started",
		})

	case "play_audio":
		if callControlID == "" {
			return encodeResult("error", "parameter 'call_control_id' is required for play_audio")
		}
		if audioURL == "" {
			return encodeResult("error", "parameter 'audio_url' is required for play_audio")
		}
		err := client.PlayAudio(ctx, callControlID, audioURL)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to play audio: %v", err))
		}
		return encodeJSON(map[string]interface{}{
			"status":  "success",
			"message": "Audio playback started",
		})

	case "gather_dtmf":
		if callControlID == "" {
			return encodeResult("error", "parameter 'call_control_id' is required for gather_dtmf")
		}
		if text == "" {
			return encodeResult("error", "parameter 'text' is required for gather_dtmf (TTS prompt)")
		}
		err := client.GatherDTMF(ctx, callControlID, text, language, voice, maxDigits, timeoutSecs)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to gather DTMF: %v", err))
		}
		return encodeJSON(map[string]interface{}{
			"status":     "success",
			"message":    "DTMF gathering started — results will arrive via webhook",
			"max_digits": maxDigits,
		})

	case "transfer":
		if callControlID == "" {
			return encodeResult("error", "parameter 'call_control_id' is required for transfer")
		}
		if to == "" {
			return encodeResult("error", "parameter 'to' is required for transfer")
		}
		err := client.TransferCall(ctx, callControlID, to, cfg.Telnyx.PhoneNumber)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to transfer: %v", err))
		}
		return encodeJSON(map[string]interface{}{
			"status":  "success",
			"message": fmt.Sprintf("Call transferred to %s", to),
		})

	case "record_start":
		if callControlID == "" {
			return encodeResult("error", "parameter 'call_control_id' is required for record_start")
		}
		err := client.RecordStart(ctx, callControlID, "mp3", "single")
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to start recording: %v", err))
		}
		return encodeJSON(map[string]interface{}{
			"status":  "success",
			"message": "Recording started",
		})

	case "record_stop":
		if callControlID == "" {
			return encodeResult("error", "parameter 'call_control_id' is required for record_stop")
		}
		err := client.RecordStop(ctx, callControlID)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to stop recording: %v", err))
		}
		return encodeJSON(map[string]interface{}{
			"status":  "success",
			"message": "Recording stopped",
		})

	case "hangup":
		if callControlID == "" {
			return encodeResult("error", "parameter 'call_control_id' is required for hangup")
		}
		err := client.HangUp(ctx, callControlID)
		if err != nil {
			return encodeResult("error", fmt.Sprintf("failed to hang up: %v", err))
		}
		return encodeJSON(map[string]interface{}{
			"status":  "success",
			"message": "Call ended",
		})

	case "list_active":
		logger.Info("Listing active calls from webhook handler")
		return encodeJSON(map[string]interface{}{
			"status":  "success",
			"message": "Active call listing requires access to the webhook handler's active calls map. Use the dashboard or check server logs.",
		})

	default:
		return encodeResult("error", fmt.Sprintf("unknown telnyx_call operation: %s. Use initiate, speak, play_audio, gather_dtmf, transfer, record_start, record_stop, hangup, or list_active", operation))
	}
}
