package agentmail

import (
	"encoding/json"
	"strings"

	"aurago/internal/config"
)

const (
	DefaultBaseURL      = "https://api.agentmail.to"
	DefaultWebSocketURL = "wss://ws.agentmail.to/v0"
	DefaultPollSeconds  = 120
	DefaultMaxAttachMB  = 10
)

// Config is the AgentMail runtime configuration used by this package.
type Config struct {
	Enabled             bool
	ReadOnly            bool
	APIKey              string
	InboxID             string
	AutoCreateInbox     bool
	Username            string
	Domain              string
	DisplayName         string
	UseWebSocket        bool
	PollIntervalSeconds int
	RelayToAgent        bool
	MaxAttachmentMB     int
	BaseURL             string
	WebSocketURL        string
}

// ConfigFromAppConfig converts AuraGo's YAML config type into the package-local
// runtime config. Keeping the internal client free of YAML tags makes tests and
// service wiring easier to reason about.
func ConfigFromAppConfig(src config.AgentMailConfig) Config {
	cfg := Config{
		Enabled:             src.Enabled,
		ReadOnly:            src.ReadOnly,
		APIKey:              strings.TrimSpace(src.APIKey),
		InboxID:             strings.TrimSpace(src.InboxID),
		AutoCreateInbox:     src.AutoCreateInbox,
		Username:            strings.TrimSpace(src.Username),
		Domain:              strings.TrimSpace(src.Domain),
		DisplayName:         strings.TrimSpace(src.DisplayName),
		UseWebSocket:        src.UseWebSocket,
		PollIntervalSeconds: src.PollIntervalSeconds,
		RelayToAgent:        src.RelayToAgent,
		MaxAttachmentMB:     src.MaxAttachmentMB,
		BaseURL:             strings.TrimSpace(src.BaseURL),
		WebSocketURL:        strings.TrimSpace(src.WebSocketURL),
	}
	return normalizeConfig(cfg)
}

func normalizeConfig(cfg Config) Config {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if strings.TrimSpace(cfg.WebSocketURL) == "" {
		cfg.WebSocketURL = DefaultWebSocketURL
	}
	if cfg.PollIntervalSeconds <= 0 {
		cfg.PollIntervalSeconds = DefaultPollSeconds
	}
	if cfg.MaxAttachmentMB <= 0 {
		cfg.MaxAttachmentMB = DefaultMaxAttachMB
	}
	return cfg
}

// Address represents an email address in AgentMail responses and requests.
type Address struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// UnmarshalJSON accepts both structured address objects and plain email strings.
func (a *Address) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		a.Email = s
		return nil
	}
	type alias Address
	var obj alias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*a = Address(obj)
	return nil
}

type Inbox struct {
	ID          string `json:"inbox_id,omitempty"`
	Email       string `json:"email_address,omitempty"`
	Username    string `json:"username,omitempty"`
	Domain      string `json:"domain,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type CreateInboxRequest struct {
	Username    string `json:"username,omitempty"`
	Domain      string `json:"domain,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

type UpdateInboxRequest struct {
	DisplayName string `json:"display_name,omitempty"`
}

type ListInboxesOptions struct {
	Limit  int
	Cursor string
}

type ListInboxesResponse struct {
	Inboxes    []Inbox `json:"inboxes,omitempty"`
	Data       []Inbox `json:"data,omitempty"`
	NextCursor string  `json:"next_cursor,omitempty"`
	HasMore    bool    `json:"has_more,omitempty"`
}

func (r *ListInboxesResponse) normalize() {
	if len(r.Inboxes) == 0 && len(r.Data) > 0 {
		r.Inboxes = r.Data
	}
}

type Message struct {
	ID          string       `json:"message_id,omitempty"`
	InboxID     string       `json:"inbox_id,omitempty"`
	ThreadID    string       `json:"thread_id,omitempty"`
	From        Address      `json:"from,omitempty"`
	FromAlt     Address      `json:"from_,omitempty"`
	To          []Address    `json:"to,omitempty"`
	CC          []Address    `json:"cc,omitempty"`
	BCC         []Address    `json:"bcc,omitempty"`
	Subject     string       `json:"subject,omitempty"`
	Text        string       `json:"text,omitempty"`
	HTML        string       `json:"html,omitempty"`
	Snippet     string       `json:"snippet,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
	Labels      []string     `json:"labels,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

func (m *Message) UnmarshalJSON(data []byte) error {
	type alias Message
	var aux struct {
		alias
		ID      string  `json:"id,omitempty"`
		FromRaw Address `json:"from_,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*m = Message(aux.alias)
	if m.ID == "" {
		m.ID = aux.ID
	}
	if m.From.Email == "" && aux.FromRaw.Email != "" {
		m.From = aux.FromRaw
	}
	return nil
}

type ListMessagesOptions struct {
	Limit  int
	Cursor string
	After  string
	Labels []string
	Thread string
}

type ListMessagesResponse struct {
	Messages   []Message `json:"messages,omitempty"`
	Data       []Message `json:"data,omitempty"`
	NextCursor string    `json:"next_cursor,omitempty"`
	HasMore    bool      `json:"has_more,omitempty"`
}

func (r *ListMessagesResponse) normalize() {
	if len(r.Messages) == 0 && len(r.Data) > 0 {
		r.Messages = r.Data
	}
}

type UpdateMessageRequest struct {
	AddLabels    []string `json:"add_labels,omitempty"`
	RemoveLabels []string `json:"remove_labels,omitempty"`
}

type Attachment struct {
	ID          string `json:"attachment_id,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
}

type AttachmentInput struct {
	Path        string `json:"path,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Base64      string `json:"base64,omitempty"`
}

type OutgoingAttachment struct {
	Filename      string `json:"filename,omitempty"`
	ContentType   string `json:"content_type,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
}

type SendMessageRequest struct {
	To          []string             `json:"to,omitempty"`
	CC          []string             `json:"cc,omitempty"`
	BCC         []string             `json:"bcc,omitempty"`
	Subject     string               `json:"subject,omitempty"`
	Text        string               `json:"text,omitempty"`
	HTML        string               `json:"html,omitempty"`
	Attachments []OutgoingAttachment `json:"attachments,omitempty"`
}

type ReplyMessageRequest struct {
	Text        string               `json:"text,omitempty"`
	HTML        string               `json:"html,omitempty"`
	Attachments []OutgoingAttachment `json:"attachments,omitempty"`
}

type ForwardMessageRequest struct {
	To          []string             `json:"to,omitempty"`
	CC          []string             `json:"cc,omitempty"`
	BCC         []string             `json:"bcc,omitempty"`
	Text        string               `json:"text,omitempty"`
	HTML        string               `json:"html,omitempty"`
	Attachments []OutgoingAttachment `json:"attachments,omitempty"`
}

type Thread struct {
	ID        string    `json:"thread_id,omitempty"`
	InboxID   string    `json:"inbox_id,omitempty"`
	Subject   string    `json:"subject,omitempty"`
	Messages  []Message `json:"messages,omitempty"`
	CreatedAt string    `json:"created_at,omitempty"`
	UpdatedAt string    `json:"updated_at,omitempty"`
}

type ListThreadsOptions struct {
	Limit  int
	Cursor string
}

type ListThreadsResponse struct {
	Threads    []Thread `json:"threads,omitempty"`
	Data       []Thread `json:"data,omitempty"`
	NextCursor string   `json:"next_cursor,omitempty"`
	HasMore    bool     `json:"has_more,omitempty"`
}

func (r *ListThreadsResponse) normalize() {
	if len(r.Threads) == 0 && len(r.Data) > 0 {
		r.Threads = r.Data
	}
}

type Draft struct {
	ID          string               `json:"draft_id,omitempty"`
	InboxID     string               `json:"inbox_id,omitempty"`
	ThreadID    string               `json:"thread_id,omitempty"`
	To          []string             `json:"to,omitempty"`
	CC          []string             `json:"cc,omitempty"`
	BCC         []string             `json:"bcc,omitempty"`
	Subject     string               `json:"subject,omitempty"`
	Text        string               `json:"text,omitempty"`
	HTML        string               `json:"html,omitempty"`
	Attachments []OutgoingAttachment `json:"attachments,omitempty"`
	CreatedAt   string               `json:"created_at,omitempty"`
	UpdatedAt   string               `json:"updated_at,omitempty"`
}

func (d *Draft) UnmarshalJSON(data []byte) error {
	type alias Draft
	var aux struct {
		alias
		ID string `json:"id,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*d = Draft(aux.alias)
	if d.ID == "" {
		d.ID = aux.ID
	}
	return nil
}

type ListDraftsOptions struct {
	Limit  int
	Cursor string
}

type ListDraftsResponse struct {
	Drafts     []Draft `json:"drafts,omitempty"`
	Data       []Draft `json:"data,omitempty"`
	NextCursor string  `json:"next_cursor,omitempty"`
	HasMore    bool    `json:"has_more,omitempty"`
}

func (r *ListDraftsResponse) normalize() {
	if len(r.Drafts) == 0 && len(r.Data) > 0 {
		r.Drafts = r.Data
	}
}
