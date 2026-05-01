package telnyx

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"
)

func truncateSMSMessage(message string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(message)
	if len(runes) <= maxRunes {
		return message
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

// TelnyxSMSBroker implements agent.FeedbackProvider for SMS-based interaction.
type TelnyxSMSBroker struct {
	client     *Client
	fromNumber string
	toNumber   string
	logger     *slog.Logger
}

// NewSMSBroker creates a broker that sends agent feedback via SMS.
func NewSMSBroker(cfg *config.Config, toNumber string, logger *slog.Logger) *TelnyxSMSBroker {
	return &TelnyxSMSBroker{
		client:     NewClient(cfg.Telnyx.APIKey, logger),
		fromNumber: cfg.Telnyx.PhoneNumber,
		toNumber:   toNumber,
		logger:     logger,
	}
}

// Send sends a feedback event via SMS. Only sends final responses and errors.
func (b *TelnyxSMSBroker) Send(event, message string) {
	// Only send significant events via SMS to avoid spamming
	switch event {
	case "final_response", "error_recovery", "question_user":
		// Truncate long messages for SMS (max ~1600 chars for long SMS)
		message = truncateSMSMessage(message, 1500)
		_, err := b.client.SendSMS(context.Background(), b.fromNumber, b.toNumber, message, "")
		if err != nil {
			b.logger.Warn("Telnyx SMS broker: failed to send", "error", err)
		}
	default:
		// Skip tool_start, progress, budget_warning etc. via SMS
		b.logger.Debug("Telnyx SMS broker: skipping event", "event", event)
	}
}

// SendJSON is a no-op for SMS broker (token usage data not relevant).
func (b *TelnyxSMSBroker) SendJSON(jsonStr string) {
	// No-op: token usage not useful via SMS
}

func (b *TelnyxSMSBroker) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
}

func (b *TelnyxSMSBroker) SendLLMStreamDone(finishReason string) {}

func (b *TelnyxSMSBroker) SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal int, isEstimated, isFinal bool, source string) {
}

func (b *TelnyxSMSBroker) SendThinkingBlock(provider, content, state string) {
}

// TelnyxCallBroker implements agent.FeedbackProvider for active voice calls.
type TelnyxCallBroker struct {
	client        *Client
	callControlID string
	language      string
	voice         string
	logger        *slog.Logger
}

// NewCallBroker creates a broker that sends agent feedback via TTS during a call.
func NewCallBroker(cfg *config.Config, callControlID string, logger *slog.Logger) *TelnyxCallBroker {
	voice := "female"
	if cfg.Telnyx.VoiceGender == "male" {
		voice = "male"
	}
	return &TelnyxCallBroker{
		client:        NewClient(cfg.Telnyx.APIKey, logger),
		callControlID: callControlID,
		language:      cfg.Telnyx.VoiceLanguage,
		voice:         voice,
		logger:        logger,
	}
}

// Send speaks agent feedback during the active call.
func (b *TelnyxCallBroker) Send(event, message string) {
	switch event {
	case "final_response":
		// Speak the final response
		if err := b.client.SpeakText(context.Background(), b.callControlID, message, b.language, b.voice); err != nil {
			b.logger.Warn("Telnyx call broker: failed to speak", "error", err)
		}
	case "progress":
		// Speak short progress updates if they're meaningful
		if len(message) > 0 && len(message) < 200 {
			if err := b.client.SpeakText(context.Background(), b.callControlID, message, b.language, b.voice); err != nil {
				b.logger.Warn("Telnyx call broker: failed to speak progress", "error", err)
			}
		}
	case "error_recovery":
		errMsg := fmt.Sprintf("An error occurred: %s", message)
		if err := b.client.SpeakText(context.Background(), b.callControlID, errMsg, b.language, b.voice); err != nil {
			b.logger.Warn("Telnyx call broker: failed to speak error", "error", err)
		}
	}
}

// SendJSON is a no-op for call broker.
func (b *TelnyxCallBroker) SendJSON(jsonStr string) {
	// No-op
}

func (b *TelnyxCallBroker) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
}

func (b *TelnyxCallBroker) SendLLMStreamDone(finishReason string) {}

func (b *TelnyxCallBroker) SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal int, isEstimated, isFinal bool, source string) {
}

func (b *TelnyxCallBroker) SendThinkingBlock(provider, content, state string) {
}

// FormatSMSForAgent wraps incoming SMS content for the agent with external data protection.
func FormatSMSForAgent(from, text string, mediaURLs []string) string {
	var content strings.Builder
	content.WriteString(text)
	if len(mediaURLs) > 0 {
		content.WriteString("\n\nAttachments:\n")
		for _, u := range mediaURLs {
			content.WriteString("- " + u + "\n")
		}
	}
	return fmt.Sprintf("[Incoming SMS from %s]\n%s", from, security.IsolateExternalData(content.String()))
}
