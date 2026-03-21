package telnyx

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
)

// e164Pattern validates E.164 phone number format.
var e164Pattern = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// ValidateE164 checks if a phone number is in valid E.164 format.
func ValidateE164(number string) error {
	if !e164Pattern.MatchString(number) {
		return fmt.Errorf("invalid E.164 phone number: %q (must be +<country><number>, e.g. +14155551234)", number)
	}
	return nil
}

// SendSMS sends an SMS message via Telnyx.
func (c *Client) SendSMS(ctx context.Context, from, to, text, messagingProfileID string) (*MessageResponse, error) {
	if err := ValidateE164(from); err != nil {
		return nil, fmt.Errorf("from number: %w", err)
	}
	if err := ValidateE164(to); err != nil {
		return nil, fmt.Errorf("to number: %w", err)
	}
	if text == "" {
		return nil, fmt.Errorf("message text is required")
	}

	req := SendMessageRequest{
		From:               from,
		To:                 to,
		Text:               text,
		Type:               "SMS",
		MessagingProfileID: messagingProfileID,
	}

	data, _, err := c.post(ctx, "/messages", req)
	if err != nil {
		return nil, fmt.Errorf("send SMS: %w", err)
	}

	var resp MessageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode SMS response: %w", err)
	}
	return &resp, nil
}

// SendMMS sends an MMS message with media attachments.
func (c *Client) SendMMS(ctx context.Context, from, to, text string, mediaURLs []string, messagingProfileID string) (*MessageResponse, error) {
	if err := ValidateE164(from); err != nil {
		return nil, fmt.Errorf("from number: %w", err)
	}
	if err := ValidateE164(to); err != nil {
		return nil, fmt.Errorf("to number: %w", err)
	}
	if len(mediaURLs) == 0 {
		return nil, fmt.Errorf("at least one media URL is required for MMS")
	}
	if len(mediaURLs) > 10 {
		return nil, fmt.Errorf("maximum 10 media URLs allowed for MMS")
	}

	req := SendMessageRequest{
		From:               from,
		To:                 to,
		Text:               text,
		Type:               "MMS",
		MediaURLs:          mediaURLs,
		MessagingProfileID: messagingProfileID,
	}

	data, _, err := c.post(ctx, "/messages", req)
	if err != nil {
		return nil, fmt.Errorf("send MMS: %w", err)
	}

	var resp MessageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode MMS response: %w", err)
	}
	return &resp, nil
}

// GetMessage retrieves the status of a sent message.
func (c *Client) GetMessage(ctx context.Context, messageID string) (*MessageResponse, error) {
	if messageID == "" {
		return nil, fmt.Errorf("message ID is required")
	}

	data, _, err := c.get(ctx, "/messages/"+messageID)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}

	var resp MessageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode message response: %w", err)
	}
	return &resp, nil
}
