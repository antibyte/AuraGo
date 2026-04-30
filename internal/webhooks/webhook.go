package webhooks

import "time"

// MaxWebhooks is the maximum number of webhooks that can be created.
const MaxWebhooks = 10

// Webhook represents a configured incoming webhook endpoint.
type Webhook struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Enabled     bool           `json:"enabled"`
	Slug        string         `json:"slug"`
	TokenID     string         `json:"token_id"`
	Format      WebhookFormat  `json:"format"`
	Delivery    DeliveryConfig `json:"delivery"`
	CreatedAt   time.Time      `json:"created_at"`
	LastFiredAt *time.Time     `json:"last_fired_at,omitempty"`
	FireCount   int64          `json:"fire_count"`
}

// WebhookFormat defines the expected data format and validation rules.
type WebhookFormat struct {
	// AcceptedContentTypes limits which Content-Type headers are accepted.
	AcceptedContentTypes []string `json:"accepted_content_types"`

	// Fields extracts specific fields from the payload. Empty = forward entire body.
	Fields []FieldMapping `json:"fields,omitempty"`

	// Description is a human-readable note about the expected payload schema.
	Description string `json:"description,omitempty"`

	// HMAC signature validation (e.g. GitHub X-Hub-Signature-256)
	SignatureHeader string `json:"signature_header,omitempty"`
	SignatureAlgo   string `json:"signature_algo,omitempty"`   // "sha256", "sha1", "plain"
	SignatureSecret string `json:"signature_secret,omitempty"` // stored in vault if set
}

// FieldMapping maps a dot-path source field to an alias.
type FieldMapping struct {
	Source string `json:"source"`          // e.g. "repository.full_name"
	Alias  string `json:"alias,omitempty"` // e.g. "repo" (defaults to Source if empty)
}

// DeliveryConfig controls how webhook data is forwarded to the agent.
type DeliveryConfig struct {
	Mode           DeliveryMode `json:"mode"`
	PromptTemplate string       `json:"prompt_template,omitempty"`
	Priority       string       `json:"priority,omitempty"` // "immediate" or "queue" (default)
}

// DeliveryMode defines how the webhook payload reaches the agent.
type DeliveryMode string

const (
	// DeliveryModeMessage sends the payload as a user message to the agent via loopback.
	DeliveryModeMessage DeliveryMode = "message"
	// DeliveryModeNotify sends an SSE notification + UI badge, no agent invocation.
	DeliveryModeNotify DeliveryMode = "notify"
	// DeliveryModeSilent only logs the event, no SSE or agent.
	DeliveryModeSilent DeliveryMode = "silent"
)

// DefaultPromptTemplate is the safe placeholder-based template used when none is configured.
const DefaultPromptTemplate = `[Webhook: {{webhook_name}}]
Zeitpunkt: {{timestamp}}
Slug: {{slug}}
Payload:
{{payload}}`

// PromptData holds the variables available inside the prompt template.
// Allowed placeholders are:
//   - {{webhook_name}}
//   - {{slug}}
//   - {{payload}}
//   - {{timestamp}}
//   - {{field.<name>}}
//   - {{header.<name>}}
type PromptData struct {
	WebhookName string
	Slug        string
	Payload     string
	Fields      map[string]interface{}
	Headers     map[string]string
	Timestamp   string
}
