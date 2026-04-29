# Telnyx Integration Plan — AuraGo

> **Status**: Planning Phase  
> **Author**: AI Agent  
> **Date**: 2026-03-21

---

## 1. Overview

Full Telnyx integration for AuraGo enabling the agent to:

- **Send/receive SMS** — bidirectional messaging with phone numbers
- **Initiate outbound voice calls** — with TTS-generated speech
- **Receive inbound voice calls** — with call routing, IVR, and agent interaction
- **Handle voicemail** — record, transcribe, and process voicemails
- **Act as notification channel** — "telnyx" channel in the existing notification system
- **Conference/transfer calls** — bridge calls between devices from the inventory

### Telnyx API Overview

Telnyx provides a REST API (v2) with webhook-based event delivery:

| API | Endpoint | Purpose |
|-----|----------|---------|
| Messaging | `POST /v2/messages` | Send SMS/MMS |
| Call Control | `POST /v2/calls` | Initiate calls |
| Call Commands | `POST /v2/calls/{id}/actions/*` | Answer, speak, gather, transfer, hangup |
| Phone Numbers | `GET /v2/phone_numbers` | List owned numbers |
| Recordings | `GET /v2/recordings` | Access call recordings |

**Webhooks**: Telnyx pushes events (incoming SMS, call events, DTMF, etc.) to a configured URL.

---

## 2. Architecture

```
┌──────────────────────────────────────────────────────────┐
│                        AuraGo                             │
│                                                           │
│  ┌─────────────────┐    ┌──────────────────────────────┐ │
│  │  internal/telnyx │    │  internal/agent              │ │
│  │                  │    │                              │ │
│  │  client.go       │◄──►│  native_tools.go (schemas)  │ │
│  │  webhook.go      │    │  agent_dispatch_comm.go      │ │
│  │  call.go         │    │  agent.go (ToolCall fields)  │ │
│  │  sms.go          │    │                              │ │
│  │  broker.go       │    └──────────────────────────────┘ │
│  └────────┬─────────┘                                     │
│           │                                               │
│  ┌────────▼─────────┐    ┌──────────────────────────────┐ │
│  │  internal/server  │    │  internal/tools              │ │
│  │                   │    │                              │ │
│  │  server_routes.go │    │  notification.go (+telnyx)   │ │
│  │  (webhook route)  │    │  tts.go (voice synthesis)    │ │
│  └───────────────────┘    └──────────────────────────────┘ │
│                                                           │
└────────────────────┬──────────────────────────────────────┘
                     │ HTTPS
                     ▼
          ┌──────────────────┐
          │    Telnyx API     │
          │  api.telnyx.com   │
          │                   │
          │  SMS ◄──► Webhook │
          │  Call ◄──► Events │
          │  Recording        │
          └──────────────────┘
```

---

## 3. New Files & Modifications

### 3.1 New Package: `internal/telnyx/`

| File | Purpose |
|------|---------|
| `client.go` | HTTP client for Telnyx v2 REST API (auth, retries, rate limiting) |
| `sms.go` | `SendSMS()`, `SendMMS()` — outbound messaging |
| `call.go` | `InitiateCall()`, `AnswerCall()`, `SpeakText()`, `GatherDTMF()`, `TransferCall()`, `HangUp()`, `RecordCall()` |
| `webhook.go` | `HandleWebhook()` — incoming event processing, signature verification |
| `broker.go` | `TelnyxBroker` — implements `agent.FeedbackProvider` for SMS-based agent interaction |
| `types.go` | All Telnyx API request/response types, webhook event types |

### 3.2 Modified Files

| File | Change |
|------|--------|
| `internal/config/config_types.go` | Add `Telnyx` config struct |
| `internal/config/config.go` | Add Telnyx defaults |
| `internal/agent/native_tools.go` | Add `TelnyxEnabled bool` flag + 3 tool schemas |
| `internal/agent/agent.go` | Add Telnyx-specific ToolCall fields |
| `internal/agent/agent_dispatch_comm.go` | Add Telnyx tool dispatch cases |
| `internal/tools/notification.go` | Add `ChannelTelnyx` notification channel |
| `internal/server/server_routes.go` | Register Telnyx webhook endpoint + start bot |
| `config_template.yaml` | Add Telnyx config section |
| `prompts/tools_manuals/telnyx.md` | Agent tool documentation |
| `ui/lang/*.json` | Translation keys for all supported languages |

### 3.3 New Files Outside Package

| File | Purpose |
|------|---------|
| `prompts/tools_manuals/telnyx.md` | RAG-indexed tool manual for the agent |
| `internal/telnyx/client_test.go` | Unit tests for API client |
| `internal/telnyx/webhook_test.go` | Unit tests for webhook handler + signature verification |
| `internal/telnyx/sms_test.go` | Unit tests for SMS sending |
| `internal/telnyx/call_test.go` | Unit tests for call control |

---

## 4. Detailed Component Design

### 4.1 Configuration (`config_types.go`)

```go
Telnyx struct {
    Enabled              bool   `yaml:"enabled"`
    ReadOnly             bool   `yaml:"readonly"`              // true = receive only, no outbound
    APIKey               string `yaml:"-" vault:"api_key"`     // vault-only, never in config
    APISecret            string `yaml:"-" vault:"api_secret"`  // webhook signature verification
    PhoneNumber          string `yaml:"phone_number"`          // Primary Telnyx phone number (E.164)
    MessagingProfileID   string `yaml:"messaging_profile_id"`  // Telnyx messaging profile
    ConnectionID         string `yaml:"connection_id"`         // Telnyx SIP connection ID for calls
    WebhookPath          string `yaml:"webhook_path"`          // Default: "/api/telnyx/webhook"
    AllowedNumbers       []string `yaml:"allowed_numbers"`     // Whitelist of phone numbers (E.164)
    MaxConcurrentCalls   int    `yaml:"max_concurrent_calls"`  // Default: 3
    MaxSMSPerMinute      int    `yaml:"max_sms_per_minute"`    // Rate limit, default: 10
    VoiceLanguage        string `yaml:"voice_language"`        // BCP-47 for TTS in calls, default: "en"
    VoiceGender          string `yaml:"voice_gender"`          // "male" or "female", default: "female"
    RecordCalls          bool   `yaml:"record_calls"`          // Auto-record all calls
    TranscribeVoicemail  bool   `yaml:"transcribe_voicemail"`  // Auto-transcribe voicemails via LLM
    RelayToAgent         bool   `yaml:"relay_to_agent"`        // Forward incoming SMS to agent loop
    CallTimeout          int    `yaml:"call_timeout"`          // Max call duration seconds, default: 300
} `yaml:"telnyx"`
```

**Config Template (`config_template.yaml`)**:
```yaml
telnyx:
    enabled: false
    readonly: false
    phone_number: ""              # Your Telnyx number in E.164 format (+1234567890)
    messaging_profile_id: ""      # From Telnyx portal
    connection_id: ""             # SIP connection ID for voice calls
    webhook_path: "/api/telnyx/webhook"
    allowed_numbers: []           # Empty = deny all, or list E.164 numbers
    max_concurrent_calls: 3
    max_sms_per_minute: 10
    voice_language: "en"
    voice_gender: "female"
    record_calls: false
    transcribe_voicemail: false
    relay_to_agent: true
    call_timeout: 300
```

### 4.2 Telnyx API Client (`client.go`)

```go
package telnyx

import (
    "context"
    "fmt"
    "net/http"
    "time"
)

const (
    baseURL    = "https://api.telnyx.com/v2"
    maxRetries = 3
)

// Client wraps the Telnyx v2 REST API.
type Client struct {
    apiKey     string
    httpClient *http.Client
    baseURL    string
    logger     *slog.Logger
}

// NewClient creates a Telnyx API client.
func NewClient(apiKey string, logger *slog.Logger) *Client {
    return &Client{
        apiKey: apiKey,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
        baseURL: baseURL,
        logger:  logger,
    }
}

// do executes an authenticated API request with retry logic.
func (c *Client) do(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
    // Build request, set Authorization: Bearer {apiKey}
    // Retry with exponential backoff on 429/5xx
    // Return response or wrapped error
}
```

### 4.3 SMS Implementation (`sms.go`)

```go
// SendSMS sends an SMS message via Telnyx.
func (c *Client) SendSMS(ctx context.Context, from, to, text string) (*MessageResponse, error) {
    // POST /v2/messages
    // Body: { "from": from, "to": to, "text": text, "messaging_profile_id": "..." }
    // Returns: MessageResponse with ID, status
}

// SendMMS sends an MMS with media attachment.
func (c *Client) SendMMS(ctx context.Context, from, to, text string, mediaURLs []string) (*MessageResponse, error) {
    // POST /v2/messages
    // Body: includes "media_urls" array
}

// GetMessage retrieves message status by ID.
func (c *Client) GetMessage(ctx context.Context, messageID string) (*MessageResponse, error) {
    // GET /v2/messages/{messageID}
}
```

**Agent Tool**: `telnyx_sms`
```
Operations:
  - send        → Send SMS to a phone number
  - send_mms    → Send MMS with media
  - status      → Check message delivery status
```

### 4.4 Voice Call Implementation (`call.go`)

```go
// InitiateCall starts an outbound call.
func (c *Client) InitiateCall(ctx context.Context, from, to, connectionID string) (*CallResponse, error) {
    // POST /v2/calls
    // Body: { "connection_id": connectionID, "to": to, "from": from, "webhook_url": "..." }
}

// AnswerCall answers an inbound call.
func (c *Client) AnswerCall(ctx context.Context, callControlID string) error {
    // POST /v2/calls/{callControlID}/actions/answer
}

// SpeakText uses Telnyx TTS to speak text during a call.
func (c *Client) SpeakText(ctx context.Context, callControlID, text, language, voice string) error {
    // POST /v2/calls/{callControlID}/actions/speak
    // Body: { "payload": text, "language": language, "voice": voice }
}

// PlayAudio plays a pre-recorded audio file during a call.
func (c *Client) PlayAudio(ctx context.Context, callControlID, audioURL string) error {
    // POST /v2/calls/{callControlID}/actions/playback_start
}

// GatherDTMF collects DTMF input from the caller.
func (c *Client) GatherDTMF(ctx context.Context, callControlID string, opts GatherOptions) error {
    // POST /v2/calls/{callControlID}/actions/gather_using_speak
    // Speak a prompt, then collect digits
}

// TransferCall transfers to another number.
func (c *Client) TransferCall(ctx context.Context, callControlID, targetNumber string) error {
    // POST /v2/calls/{callControlID}/actions/transfer
}

// RecordCall starts recording.
func (c *Client) RecordCall(ctx context.Context, callControlID string) error {
    // POST /v2/calls/{callControlID}/actions/record_start
}

// HangUp ends a call.
func (c *Client) HangUp(ctx context.Context, callControlID string) error {
    // POST /v2/calls/{callControlID}/actions/hangup
}
```

**Agent Tool**: `telnyx_call`
```
Operations:
  - initiate      → Start outbound call
  - answer        → Answer inbound call (from webhook context)
  - speak         → TTS speak during active call
  - play_audio    → Play audio file URL during call
  - gather_dtmf   → Collect DTMF digits with prompt
  - transfer      → Transfer call to another number
  - record_start  → Start recording
  - record_stop   → Stop recording
  - hangup        → End call
  - list_active   → List currently active calls
```

### 4.5 Webhook Handler (`webhook.go`)

Telnyx sends all events (incoming SMS, call events, DTMF, recordings) via webhook POST.

```go
// Event types from Telnyx
const (
    EventMessageReceived    = "message.received"
    EventMessageSent        = "message.sent"
    EventMessageFailed      = "message.finalized"
    EventCallInitiated      = "call.initiated"
    EventCallAnswered       = "call.answered"
    EventCallHangup         = "call.hangup"
    EventCallSpeakEnded     = "call.speak.ended"
    EventCallGatherEnded    = "call.gather.ended"
    EventCallRecordingSaved = "call.recording.saved"
    EventCallDTMF           = "call.dtmf.received"
    EventCallMachineDetect  = "call.machine.detection.ended"
)

// WebhookHandler processes incoming Telnyx events.
type WebhookHandler struct {
    cfg           *config.Config
    logger        *slog.Logger
    client        *Client
    llmClient     llm.ChatClient
    shortTermMem  *memory.SQLiteMemory
    longTermMem   memory.VectorDB
    vault         *security.Vault
    // ... other agent dependencies
    
    activeCalls   map[string]*CallSession // callControlID → session
    mu            sync.RWMutex
}

// HandleWebhook is the HTTP handler for POST /api/telnyx/webhook
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
    // 1. Verify signature (HMAC with API secret)
    // 2. Parse event JSON
    // 3. Route by event type:
    //    - message.received → handleIncomingSMS()
    //    - call.initiated   → handleCallInitiated()
    //    - call.answered    → handleCallAnswered()
    //    - call.hangup      → handleCallHangup()
    //    - call.gather.ended → handleDTMFResult()
    //    - call.recording.saved → handleRecording()
    // 4. Return 200 OK immediately (async processing)
}

// handleIncomingSMS processes an incoming SMS and optionally routes to agent.
func (h *WebhookHandler) handleIncomingSMS(event *WebhookEvent) {
    // 1. Validate sender against allowed_numbers whitelist
    // 2. Extract message text, sender, media URLs
    // 3. Wrap content in <external_data> tags (prompt injection protection)
    // 4. If relay_to_agent: send as user message via loopback
    // 5. Log to webhook logger
}

// handleCallInitiated processes an incoming call.
func (h *WebhookHandler) handleCallInitiated(event *WebhookEvent) {
    // 1. Validate caller against allowed_numbers
    // 2. Answer call
    // 3. Create CallSession with state machine
    // 4. Greet caller with TTS
    // 5. Gather DTMF or relay voice to agent
}
```

### 4.6 Call Session State Machine

For inbound calls, we need an interactive session:

```
┌──────────┐     answer     ┌───────────┐    speak/gather   ┌──────────────┐
│ RINGING  │───────────────►│  GREETING │──────────────────►│  LISTENING   │
└──────────┘                └───────────┘                    └──────┬───────┘
                                                                   │
                                                    DTMF / timeout │
                                                                   ▼
                                                          ┌──────────────┐
                                                          │  PROCESSING  │
                                                          │  (agent loop)│
                                                          └──────┬───────┘
                                                                 │
                                                      speak response│
                                                                 ▼
                                                          ┌──────────────┐
                                                          │  RESPONDING  │
                                                          └──────┬───────┘
                                                                 │
                                                    loop / hangup│
                                                                 ▼
                                                          ┌──────────────┐
                                                          │  ENDED       │
                                                          └──────────────┘
```

```go
type CallState int

const (
    CallStateRinging    CallState = iota
    CallStateGreeting
    CallStateListening
    CallStateProcessing
    CallStateResponding
    CallStateEnded
)

// CallSession tracks state for an active call.
type CallSession struct {
    CallControlID string
    CallerNumber  string
    State         CallState
    StartedAt     time.Time
    LastActivity  time.Time
    Context       []string  // Conversation context for this call
    mu            sync.Mutex
}
```

### 4.7 TelnyxBroker — Agent Feedback Provider (`broker.go`)

When a call triggers an agent loop, the broker sends responses back via the call:

```go
// TelnyxSMSBroker sends agent feedback via SMS.
type TelnyxSMSBroker struct {
    client      *Client
    fromNumber  string
    toNumber    string
    logger      *slog.Logger
}

func (b *TelnyxSMSBroker) Send(event, message string) {
    // Filter events: only send "final_response" and "error_recovery"
    // Truncate long messages for SMS (160 char segments)
    // Call client.SendSMS()
}

// TelnyxCallBroker sends agent feedback via TTS during an active call.
type TelnyxCallBroker struct {
    client        *Client
    callControlID string
    language      string
    voice         string
    logger        *slog.Logger
}

func (b *TelnyxCallBroker) Send(event, message string) {
    // On "progress": speak intermediate results
    // On "final_response": speak final answer
    // On "error_recovery": speak error message
    // Uses client.SpeakText()
}
```

### 4.8 Notification Channel Integration

In `internal/tools/notification.go`:

```go
const (
    ChannelNtfy     NotificationChannel = "ntfy"
    ChannelPushover NotificationChannel = "pushover"
    ChannelTelegram NotificationChannel = "telegram"
    ChannelDiscord  NotificationChannel = "discord"
    ChannelPush     NotificationChannel = "push"
    ChannelTelnyx   NotificationChannel = "telnyx"   // NEW
    ChannelAll      NotificationChannel = "all"
)
```

New `sendTelnyxNotification()` function:
```go
func sendTelnyxNotification(cfg *config.Config, title, message string) error {
    if cfg.Telnyx.APIKey == "" || cfg.Telnyx.PhoneNumber == "" {
        return fmt.Errorf("telnyx not configured")
    }
    if len(cfg.Telnyx.AllowedNumbers) == 0 {
        return fmt.Errorf("no allowed numbers configured for notifications")
    }
    client := NewClient(cfg.Telnyx.APIKey, nil)
    text := fmt.Sprintf("[%s] %s", title, message)
    // Send to first allowed number (primary contact)
    _, err := client.SendSMS(context.Background(), cfg.Telnyx.PhoneNumber, cfg.Telnyx.AllowedNumbers[0], text)
    return err
}
```

---

## 5. Tool Schemas (native_tools.go)

### 5.1 Feature Flag

```go
// In ToolFeatureFlags struct:
TelnyxSMSEnabled  bool
TelnyxCallEnabled bool
```

### 5.2 Tool: `telnyx_sms`

```go
if ff.TelnyxSMSEnabled {
    tools = append(tools, tool("telnyx_sms",
        "Send and manage SMS/MMS messages via Telnyx. Can send text messages and multimedia messages to phone numbers.",
        schema(map[string]interface{}{
            "operation": map[string]interface{}{
                "type":        "string",
                "description": "Operation to perform",
                "enum":        []string{"send", "send_mms", "status"},
            },
            "to": prop("string",
                "Recipient phone number in E.164 format (e.g. +49151xxxxxxxx). Required for send/send_mms."),
            "message": prop("string",
                "Text message content. Required for send/send_mms. Max 1600 chars for SMS."),
            "media_urls": map[string]interface{}{
                "type":        "array",
                "items":       map[string]interface{}{"type": "string"},
                "description": "URLs of media files to attach (for send_mms only). Max 10 items.",
            },
            "message_id": prop("string",
                "Message ID to check status for (for status operation)."),
        }, "operation"),
    ))
}
```

### 5.3 Tool: `telnyx_call`

```go
if ff.TelnyxCallEnabled {
    tools = append(tools, tool("telnyx_call",
        "Initiate and control voice calls via Telnyx. Can make calls, speak text (TTS), gather DTMF input, transfer, and record.",
        schema(map[string]interface{}{
            "operation": map[string]interface{}{
                "type":        "string",
                "description": "Call operation to perform",
                "enum":        []string{"initiate", "speak", "play_audio", "gather_dtmf",
                                        "transfer", "record_start", "record_stop",
                                        "hangup", "list_active"},
            },
            "to": prop("string",
                "Phone number to call in E.164 format. Required for initiate/transfer."),
            "call_control_id": prop("string",
                "Call control ID of active call. Required for speak/play_audio/gather_dtmf/transfer/record_*/hangup."),
            "text": prop("string",
                "Text to speak via TTS during the call. Required for speak/gather_dtmf."),
            "audio_url": prop("string",
                "URL of audio file to play. Required for play_audio."),
            "max_digits": prop("integer",
                "Maximum DTMF digits to collect (for gather_dtmf). Default: 1."),
            "timeout_secs": prop("integer",
                "Timeout in seconds for DTMF gathering. Default: 10."),
        }, "operation"),
    ))
}
```

### 5.4 Tool: `telnyx_manage` (Admin/Config)

```go
if ff.TelnyxSMSEnabled || ff.TelnyxCallEnabled {
    tools = append(tools, tool("telnyx_manage",
        "Manage Telnyx phone resources: list phone numbers, check balance, view call/message history.",
        schema(map[string]interface{}{
            "operation": map[string]interface{}{
                "type":        "string",
                "description": "Management operation",
                "enum":        []string{"list_numbers", "check_balance", "message_history", "call_history"},
            },
            "limit": prop("integer",
                "Max results to return for history queries. Default: 20."),
            "page": prop("integer",
                "Page number for pagination. Default: 1."),
        }, "operation"),
    ))
}
```

---

## 6. Tool Dispatch (`agent_dispatch_comm.go`)

Add to the `dispatchComm()` switch:

```go
case "telnyx_sms":
    if !cfg.Telnyx.Enabled {
        return `Tool Output: {"status":"error","message":"Telnyx integration is disabled"}`
    }
    if cfg.Telnyx.ReadOnly {
        return `Tool Output: {"status":"error","message":"Telnyx is in read-only mode"}`
    }
    return telnyx.DispatchSMS(ctx, tc, cfg, logger)

case "telnyx_call":
    if !cfg.Telnyx.Enabled {
        return `Tool Output: {"status":"error","message":"Telnyx integration is disabled"}`
    }
    if cfg.Telnyx.ReadOnly && tc.Operation != "list_active" {
        return `Tool Output: {"status":"error","message":"Telnyx is in read-only mode"}`
    }
    return telnyx.DispatchCall(ctx, tc, cfg, logger)

case "telnyx_manage":
    if !cfg.Telnyx.Enabled {
        return `Tool Output: {"status":"error","message":"Telnyx integration is disabled"}`
    }
    return telnyx.DispatchManage(ctx, tc, cfg, logger)
```

---

## 7. ToolCall Fields (`agent.go`)

Add Telnyx-specific fields to the `ToolCall` struct:

```go
// Telnyx
CallControlID string   `json:"call_control_id,omitempty"` // Active call identifier
MaxDigits     int      `json:"max_digits,omitempty"`       // DTMF max digits
TimeoutSecs   int      `json:"timeout_secs,omitempty"`     // Gather timeout
AudioURL      string   `json:"audio_url,omitempty"`        // Audio playback URL
MediaURLs     []string `json:"media_urls,omitempty"`       // MMS attachments
MessageID     string   `json:"message_id,omitempty"`       // SMS message ID for status
```

> **Note**: Fields like `To`, `Message`, `Text`, `Operation` already exist in the ToolCall struct and will be reused.

---

## 8. Webhook Endpoint & Security

### Route Registration (`server_routes.go`)

```go
// Phase 35.x: Telnyx webhook + bot initialization
if s.Cfg.Telnyx.Enabled {
    telnyxHandler := telnyx.NewWebhookHandler(s.Cfg, s.Logger, s.LLMClient,
        s.ShortTermMem, s.LongTermMem, s.Vault, s.Registry,
        s.CronManager, s.HistoryManager, s.KG, s.InventoryDB, s.MissionManagerV2)
    
    webhookPath := s.Cfg.Telnyx.WebhookPath
    if webhookPath == "" {
        webhookPath = "/api/telnyx/webhook"
    }
    mux.HandleFunc(webhookPath, telnyxHandler.HandleWebhook)
    s.Logger.Info("Telnyx webhook registered", "path", webhookPath)
}
```

### Webhook Signature Verification

```go
func verifyTelnyxSignature(r *http.Request, publicKey string) bool {
    // Telnyx uses Ed25519 signatures
    // Header: "telnyx-signature-ed25519"
    // Header: "telnyx-timestamp"
    // Verify: ed25519.Verify(pubKey, timestamp + "|" + body, signature)
    // Reject if timestamp older than 5 minutes (replay protection)
}
```

### Security Measures

1. **Signature verification** — Ed25519 on every webhook (reject unsigned/tampered)
2. **Allowed numbers whitelist** — Only process SMS/calls from whitelisted E.164 numbers
3. **Rate limiting** — Per-number rate limiter, configurable max SMS/min
4. **Call timeout** — Automatic hangup after `call_timeout` seconds
5. **Max concurrent calls** — Prevent resource exhaustion
6. **External data wrapping** — All incoming content wrapped in `<external_data>` tags
7. **API key in vault** — Never in config file, never exposed to agent
8. **Read-only mode** — Disable outbound operations while keeping inbound processing

---

## 9. Inbound Call Flow (IVR)

When a call comes in, the agent can act as an interactive voice assistant:

### Default Flow

```
1. RING → Auto-answer (if caller in allowed_numbers)
2. GREET → TTS: "Hello, this is {agent_name}. How can I help you?"
3. LISTEN → Gather speech/DTMF (Telnyx speech-to-text or DTMF menu)
4. PROCESS → Route to agent loop with TelnyxCallBroker
5. RESPOND → TTS speaks agent response
6. LOOP → Return to step 3 (or hangup on "goodbye"/timeout)
```

### DTMF Menu (Optional IVR)

```
Press 1: Talk to the agent (speech mode)
Press 2: Leave a voicemail
Press 3: Get system status
Press 9: Hang up
```

### Speech-to-Text Integration

For voice mode, use Telnyx's built-in `gather_using_audio` with speech recognition, or:
1. Record the caller's speech segment
2. Download recording
3. Transcribe using existing `TranscribeMultimodal()` from `internal/telegram/voice.go`
4. Feed transcription into agent loop

---

## 10. Voicemail System

```go
// StartVoicemail initiates voicemail recording for a call.
func (h *WebhookHandler) startVoicemail(callControlID string) {
    // 1. Speak: "Please leave a message after the tone."
    // 2. Play beep audio
    // 3. Start recording (max 120 seconds)
    // 4. On silence/hangup → save recording
}

// processVoicemail handles a completed voicemail recording.
func (h *WebhookHandler) processVoicemail(event *WebhookEvent) {
    // 1. Download recording from Telnyx
    // 2. Save to data/telnyx/voicemail/{timestamp}_{caller}.mp3
    // 3. If transcribe_voicemail enabled:
    //    a. Transcribe via Whisper/multimodal
    //    b. Store transcription alongside audio
    // 4. Notify agent: "Voicemail from {number}: {transcription}"
    // 5. Send push notification
}
```

---

## 11. Integration with Existing Systems

### 11.1 Notification System

```
Channel "telnyx" → Sends SMS notification to primary allowed number
Channel "all"    → Includes telnyx if enabled + has allowed numbers
```

### 11.2 Inventory Integration

The SSH device inventory (`internal/inventory/`) contains devices with optional phone numbers. The agent can:
- Look up a device owner's phone number from inventory
- Send SMS alerts for device status changes
- Initiate calls based on inventory contacts

### 11.3 Cron/Scheduler Integration

Scheduled tasks can trigger Telnyx actions:
- "Send daily SMS report at 8am"
- "Call me if server goes down"
- Cron callbacks via existing tool dispatch

### 11.4 Fritz!Box Telephony Bridge

Bridge Telnyx VoIP with Fritz!Box landline:
- Use Telnyx for outbound calls (cheaper than landline)
- Transfer Telnyx calls to Fritz!Box internal phones
- Unified call history across both systems

### 11.5 TTS Integration

Voice calls can use AuraGo's existing TTS system:
1. Generate speech via Google/ElevenLabs TTS (`tts.go`)
2. Host MP3 temporarily via AuraGo HTTP server
3. Play via `PlayAudio()` for higher quality than Telnyx built-in TTS
4. Fallback to Telnyx native TTS for low latency

---

## 12. Dashboard & Web UI

### New Dashboard Card

```
┌─────────────────────────────────────┐
│  📞 Telnyx                          │
│                                     │
│  Status: ● Connected               │
│  Number: +49 151 xxxx xxxx         │
│  Active Calls: 0/3                  │
│  SMS Today: 12 sent / 8 received    │
│  Balance: €23.45                    │
│                                     │
│  [Recent Activity]                  │
│  • SMS from +49... "Hey, ..." 2m    │
│  • Call to +49... 5:23 duration 14m │
│  • Voicemail from +49... 1h        │
└─────────────────────────────────────┘
```

### API Endpoints for UI

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `GET /api/telnyx/status` | GET | Connection status, balance, active calls |
| `GET /api/telnyx/history` | GET | Recent SMS and call history |
| `GET /api/telnyx/calls` | GET | Active call list |
| `POST /api/telnyx/sms` | POST | Send SMS from UI |
| `GET /api/telnyx/voicemails` | GET | List voicemails with transcriptions |
| `GET /api/telnyx/voicemail/{id}` | GET | Download voicemail audio |

### Settings Page

- Enable/disable toggle
- Phone number display
- Allowed numbers management (add/remove)
- Read-only toggle
- Voice settings (language, gender)
- Recording & transcription toggles
- Webhook URL display (for Telnyx portal configuration)
- Connection test button

---

## 13. Agent Tool Manual (`prompts/tools_manuals/telnyx.md`)

```markdown
# Telnyx SMS & Voice Tools

## telnyx_sms
Send and receive SMS/MMS messages via Telnyx.

### Operations
- **send**: Send a text message to a phone number
  - `to`: Phone number in E.164 format (e.g., +491511234567)
  - `message`: Text content (max 1600 characters)
- **send_mms**: Send multimedia message
  - `to`, `message`: Same as above
  - `media_urls`: Array of URLs to attach (images, audio, video)
- **status**: Check delivery status of a sent message
  - `message_id`: The ID returned from send operation

### Notes
- All phone numbers must be in E.164 format (+countrycode number)
- SMS is limited to 1600 characters (will be split into segments)
- MMS supports up to 10 media attachments
- Only numbers in the allowed_numbers whitelist can receive messages

## telnyx_call
Make and control voice calls via Telnyx.

### Operations
- **initiate**: Start an outbound call
  - `to`: Phone number to call (E.164 format)
- **speak**: Speak text during active call (TTS)
  - `call_control_id`: Active call ID
  - `text`: Text to speak
- **play_audio**: Play audio file during call
  - `call_control_id`, `audio_url`
- **gather_dtmf**: Collect keypad input
  - `call_control_id`, `text` (prompt to speak)
  - `max_digits`, `timeout_secs`
- **transfer**: Transfer call to another number
  - `call_control_id`, `to`
- **record_start**/**record_stop**: Control call recording
  - `call_control_id`
- **hangup**: End a call
  - `call_control_id`
- **list_active**: List all currently active calls

### Notes
- Maximum concurrent calls: configurable (default 3)
- Call timeout: configurable (default 300 seconds)
- Calls can be recorded if enabled in config
- Active call IDs are returned by initiate and provided in webhook events

## telnyx_manage
Administrative operations for Telnyx.

### Operations
- **list_numbers**: List all phone numbers on the account
- **check_balance**: Check account balance
- **message_history**: View recent SMS history
- **call_history**: View recent call history
```

---

## 14. Translation Keys

Add to all `ui/lang/*.json` files:

```json
{
    "telnyx_title": "Telnyx Phone",
    "telnyx_status": "Status",
    "telnyx_connected": "Connected",
    "telnyx_disconnected": "Not configured",
    "telnyx_phone_number": "Phone Number",
    "telnyx_active_calls": "Active Calls",
    "telnyx_sms_today": "SMS Today",
    "telnyx_balance": "Balance",
    "telnyx_enabled": "Enable Telnyx",
    "telnyx_readonly": "Read-Only Mode",
    "telnyx_allowed_numbers": "Allowed Numbers",
    "telnyx_voice_language": "Voice Language",
    "telnyx_voice_gender": "Voice Gender",
    "telnyx_record_calls": "Record Calls",
    "telnyx_transcribe": "Auto-Transcribe Voicemails",
    "telnyx_webhook_url": "Webhook URL",
    "telnyx_test_connection": "Test Connection",
    "telnyx_send_sms": "Send SMS",
    "telnyx_voicemails": "Voicemails",
    "telnyx_call_history": "Call History",
    "telnyx_relay_to_agent": "Forward SMS to Agent"
}
```

---

## 15. Testing Strategy

### Unit Tests

| Test File | Covers |
|-----------|--------|
| `client_test.go` | API client auth headers, retry logic, error handling |
| `sms_test.go` | SMS sending, E.164 validation, message truncation |
| `call_test.go` | Call initiation, state machine transitions, timeout handling |
| `webhook_test.go` | Signature verification, event parsing, allowed number filtering |
| `broker_test.go` | SMS/Call broker message formatting, event filtering |

### Integration Tests

```go
func TestWebhookSignatureVerification(t *testing.T) {
    // Test valid signature passes
    // Test invalid signature rejects
    // Test expired timestamp rejects
    // Test missing headers rejects
}

func TestAllowedNumberFiltering(t *testing.T) {
    // Test whitelisted number passes
    // Test non-whitelisted number rejects
    // Test empty whitelist denies all
}

func TestCallSessionStateMachine(t *testing.T) {
    // Test state transitions: Ringing→Greeting→Listening→Processing→Responding
    // Test timeout transitions
    // Test concurrent call limits
}

func TestSMSNotificationChannel(t *testing.T) {
    // Test SendNotification with channel="telnyx"
    // Test channel="all" includes telnyx when enabled
}
```

---

## 16. Implementation Order

### Phase 1: Foundation (Core)
1. Config types + defaults + template yaml
2. `internal/telnyx/types.go` — All API types
3. `internal/telnyx/client.go` — HTTP client with auth
4. `internal/telnyx/client_test.go` — Client tests

### Phase 2: SMS
5. `internal/telnyx/sms.go` — Send/receive SMS
6. `internal/telnyx/sms_test.go` — SMS tests
7. Tool schema `telnyx_sms` in `native_tools.go`
8. ToolCall fields in `agent.go`
9. Dispatch in `agent_dispatch_comm.go`

### Phase 3: Webhooks & Inbound
10. `internal/telnyx/webhook.go` — Event handling + signature verification
11. `internal/telnyx/webhook_test.go` — Webhook tests
12. Webhook route in `server_routes.go`
13. Inbound SMS → agent loop (with TelnyxSMSBroker)

### Phase 4: Voice Calls
14. `internal/telnyx/call.go` — Call control
15. `internal/telnyx/call_test.go` — Call tests
16. Call session state machine
17. Tool schema `telnyx_call` in `native_tools.go`
18. Dispatch in `agent_dispatch_comm.go`
19. TelnyxCallBroker for voice responses

### Phase 5: Voicemail & Advanced
20. Voicemail recording + transcription
21. TTS integration (AuraGo TTS → Telnyx PlayAudio)
22. IVR/DTMF menu system

### Phase 6: Integration & UI
23. Notification channel integration
24. `telnyx_manage` tool
25. Dashboard card + API endpoints
26. Settings page
27. Translations for all languages
28. Tool manual for RAG

### Phase 7: Testing & Polish
29. Full test suite
30. Security audit (OWASP, input validation, rate limiting)
31. Documentation update

---

## 17. Dependencies

**No new Go modules required.** The integration uses:
- Standard library `net/http` for REST API calls
- Standard library `crypto/ed25519` for webhook signature verification
- Standard library `encoding/json` for JSON marshaling
- Existing `internal/media/` for audio file handling
- Existing TTS system for voice synthesis

This keeps Telnyx as a **zero-dependency integration**, consistent with AuraGo's minimal dependency philosophy.

---

## 18. Cost Considerations

| Operation | Telnyx Cost (approx.) |
|-----------|----------------------|
| Outbound SMS (US) | ~$0.004/message |
| Outbound SMS (DE) | ~$0.07/message |
| Inbound SMS | ~$0.003/message |
| Outbound Call (US) | ~$0.012/min |
| Outbound Call (DE) | ~$0.02/min |
| Inbound Call | ~$0.01/min |
| Phone Number | ~$1.00/month |
| Recording Storage | ~$0.003/min |

The `check_balance` operation in `telnyx_manage` lets the agent monitor costs. Budget tracking can integrate with `internal/budget/` for cost awareness.
