package telnyx

import "time"

// ── Telnyx API v2 request/response types ──────────────────────────────────

// SendMessageRequest is the body for POST /v2/messages.
type SendMessageRequest struct {
	From               string   `json:"from"`
	To                 string   `json:"to"`
	Text               string   `json:"text,omitempty"`
	MediaURLs          []string `json:"media_urls,omitempty"`
	MessagingProfileID string   `json:"messaging_profile_id,omitempty"`
	Type               string   `json:"type,omitempty"` // "SMS" or "MMS"
	WebhookURL         string   `json:"webhook_url,omitempty"`
}

// MessageResponse represents a Telnyx messaging API response.
type MessageResponse struct {
	Data struct {
		ID               string    `json:"id"`
		RecordType       string    `json:"record_type"`
		Direction        string    `json:"direction"`
		Type             string    `json:"type"`
		From             Endpoint  `json:"from"`
		To               []Endpoint `json:"to"`
		Text             string    `json:"text"`
		Status           string    `json:"status"`
		CreatedAt        time.Time `json:"created_at"`
		CompletedAt      time.Time `json:"completed_at"`
		Cost             *Cost     `json:"cost"`
		Parts            int       `json:"parts"`
		MediaURLs        []string  `json:"media"`
		Errors           []APIError `json:"errors"`
	} `json:"data"`
}

// Endpoint represents a phone number endpoint.
type Endpoint struct {
	PhoneNumber string `json:"phone_number"`
	Carrier     string `json:"carrier"`
	LineType    string `json:"line_type"`
	Status      string `json:"status"`
}

// Cost represents the cost of an API operation.
type Cost struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// APIError represents a Telnyx API error.
type APIError struct {
	Code   string `json:"code"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// ErrorResponse is the standard Telnyx error wrapper.
type ErrorResponse struct {
	Errors []APIError `json:"errors"`
}

// ── Call Control Types ────────────────────────────────────────────────────

// CreateCallRequest is the body for POST /v2/calls.
type CreateCallRequest struct {
	ConnectionID        string `json:"connection_id"`
	To                  string `json:"to"`
	From                string `json:"from"`
	WebhookURL          string `json:"webhook_url,omitempty"`
	WebhookURLMethod    string `json:"webhook_url_method,omitempty"`
	AnsweringMachineDetection string `json:"answering_machine_detection,omitempty"`
	TimeoutSecs         int    `json:"timeout_secs,omitempty"`
}

// CallResponse represents a Telnyx call control API response.
type CallResponse struct {
	Data struct {
		CallControlID  string `json:"call_control_id"`
		CallLegID      string `json:"call_leg_id"`
		CallSessionID  string `json:"call_session_id"`
		IsAlive        bool   `json:"is_alive"`
		RecordType     string `json:"record_type"`
		State          string `json:"state"`
	} `json:"data"`
}

// SpeakRequest is the body for call speak action.
type SpeakRequest struct {
	Payload      string `json:"payload"`
	PayloadType  string `json:"payload_type,omitempty"` // "text" or "ssml"
	Voice        string `json:"voice"`
	Language     string `json:"language"`
	ClientState  string `json:"client_state,omitempty"`
	CommandID    string `json:"command_id,omitempty"`
}

// PlaybackStartRequest is the body for call playback action.
type PlaybackStartRequest struct {
	AudioURL    string `json:"audio_url"`
	ClientState string `json:"client_state,omitempty"`
}

// GatherSpeakRequest is the body for gather-using-speak action.
type GatherSpeakRequest struct {
	Payload         string `json:"payload"`
	PayloadType     string `json:"payload_type,omitempty"`
	Voice           string `json:"voice"`
	Language        string `json:"language"`
	MinimumDigits   int    `json:"minimum_digits,omitempty"`
	MaximumDigits   int    `json:"maximum_digits,omitempty"`
	TimeoutMillis   int    `json:"timeout_millis,omitempty"`
	InterDigitTimeout int  `json:"inter_digit_timeout_millis,omitempty"`
	TerminatingDigit string `json:"terminating_digit,omitempty"`
	ValidDigits     string `json:"valid_digits,omitempty"`
	ClientState     string `json:"client_state,omitempty"`
}

// TransferRequest is the body for call transfer action.
type TransferRequest struct {
	To          string `json:"to"`
	From        string `json:"from,omitempty"`
	ClientState string `json:"client_state,omitempty"`
}

// RecordStartRequest is the body for call record-start action.
type RecordStartRequest struct {
	Format      string `json:"format,omitempty"` // "mp3" or "wav"
	Channels    string `json:"channels,omitempty"` // "single" or "dual"
	ClientState string `json:"client_state,omitempty"`
}

// ── Webhook Event Types ──────────────────────────────────────────────────

// WebhookEvent wraps all Telnyx webhook payloads.
type WebhookEvent struct {
	Data struct {
		EventType  string          `json:"event_type"`
		ID         string          `json:"id"`
		OccurredAt time.Time       `json:"occurred_at"`
		RecordType string          `json:"record_type"`
		Payload    WebhookPayload  `json:"payload"`
	} `json:"data"`
	Meta struct {
		Attempt    int    `json:"attempt"`
		DeliveredTo string `json:"delivered_to"`
	} `json:"meta"`
}

// WebhookPayload is the polymorphic payload for webhook events.
type WebhookPayload struct {
	// Common fields
	CallControlID  string `json:"call_control_id,omitempty"`
	CallLegID      string `json:"call_leg_id,omitempty"`
	CallSessionID  string `json:"call_session_id,omitempty"`
	ConnectionID   string `json:"connection_id,omitempty"`
	ClientState    string `json:"client_state,omitempty"`

	// Call event fields
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
	Direction      string `json:"direction,omitempty"`
	State          string `json:"state,omitempty"`
	HangupCause    string `json:"hangup_cause,omitempty"`
	HangupSource   string `json:"hangup_source,omitempty"`

	// DTMF / Gather fields
	Digits         string `json:"digits,omitempty"`
	Result         string `json:"result,omitempty"` // "valid", "invalid", "call_hangup"

	// Recording fields
	RecordingURLs  *RecordingURLs `json:"recording_urls,omitempty"`
	Duration       int            `json:"duration_millis,omitempty"`

	// Speak/Playback events
	Status         string `json:"status,omitempty"`

	// Message fields (SMS)
	MessageID      string            `json:"id,omitempty"`
	Type           string            `json:"type,omitempty"`        // "SMS" or "MMS"
	Text           string            `json:"text,omitempty"`
	MediaURLs      []MediaAttachment `json:"media,omitempty"`
	Parts          int               `json:"parts,omitempty"`

	// Machine detection
	MachineResult  string `json:"result,omitempty"` // "human", "machine", "not_sure"
}

// RecordingURLs holds download links for a recording.
type RecordingURLs struct {
	MP3 string `json:"mp3"`
	WAV string `json:"wav"`
}

// MediaAttachment represents media in an MMS.
type MediaAttachment struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}

// ── Management / Listing Types ───────────────────────────────────────────

// PhoneNumbersResponse is the response for GET /v2/phone_numbers.
type PhoneNumbersResponse struct {
	Data []PhoneNumber `json:"data"`
	Meta PaginationMeta `json:"meta"`
}

// PhoneNumber represents a phone number on the account.
type PhoneNumber struct {
	ID                string `json:"id"`
	PhoneNumber       string `json:"phone_number"`
	Status            string `json:"status"`
	ConnectionID      string `json:"connection_id"`
	MessagingProfileID string `json:"messaging_profile_id"`
	CreatedAt         time.Time `json:"created_at"`
}

// BalanceResponse is the response for GET /v2/balance.
type BalanceResponse struct {
	Data struct {
		Balance         string `json:"balance"`
		Currency        string `json:"currency"`
		CreditLimit     string `json:"credit_limit"`
		AvailableCredit string `json:"available_credit"`
	} `json:"data"`
}

// MessagesListResponse is the response for GET /v2/messages.
type MessagesListResponse struct {
	Data []MessageSummary `json:"data"`
	Meta PaginationMeta   `json:"meta"`
}

// MessageSummary is a compact message entry for history listings.
type MessageSummary struct {
	ID        string    `json:"id"`
	Direction string    `json:"direction"`
	From      Endpoint  `json:"from"`
	To        []Endpoint `json:"to"`
	Text      string    `json:"text"`
	Status    string    `json:"status"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Parts     int       `json:"parts"`
}

// PaginationMeta contains pagination info.
type PaginationMeta struct {
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
	PageNumber   int `json:"page_number"`
	PageSize     int `json:"page_size"`
}

// ── Webhook Signature Constants ──────────────────────────────────────────

const (
	// EventMessageReceived is fired when an SMS/MMS arrives.
	EventMessageReceived = "message.received"
	// EventMessageSent is fired when an outbound message is sent.
	EventMessageSent = "message.sent"
	// EventMessageFinalized is fired when a message reaches final state.
	EventMessageFinalized = "message.finalized"
	// EventCallInitiated is fired when a call starts.
	EventCallInitiated = "call.initiated"
	// EventCallAnswered is fired when a call is answered.
	EventCallAnswered = "call.answered"
	// EventCallHangup is fired when a call ends.
	EventCallHangup = "call.hangup"
	// EventCallSpeakEnded is fired when TTS finishes.
	EventCallSpeakEnded = "call.speak.ended"
	// EventCallGatherEnded is fired when DTMF gather completes.
	EventCallGatherEnded = "call.gather.ended"
	// EventCallRecordingSaved is fired when a recording is available.
	EventCallRecordingSaved = "call.recording.saved"
	// EventCallPlaybackEnded is fired when audio playback finishes.
	EventCallPlaybackEnded = "call.playback.ended"
	// EventCallMachineDetect is fired after answering machine detection.
	EventCallMachineDetect = "call.machine.detection.ended"

	// SignatureHeader is the Ed25519 signature header.
	SignatureHeader = "telnyx-signature-ed25519"
	// TimestampHeader is the timestamp header for replay protection.
	TimestampHeader = "telnyx-timestamp"
)
