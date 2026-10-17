package mailer

import (
	"context"
	"io"
	"time"
)

type Account struct {
	ID                   string    `json:"id"`
	Source               string    `json:"source"`
	Protocol             string    `json:"protocol,omitempty"`
	Name                 string    `json:"name"`
	Address              string    `json:"address"`
	Capabilities         []string  `json:"capabilities"`
	Disabled             bool      `json:"disabled"`
	Reason               string    `json:"reason,omitempty"`
	AgentAccess          bool      `json:"agent_access"`
	ReadOnly             bool      `json:"read_only"`
	ReauthorizeSupported bool      `json:"reauthorize_supported"`
	Linked               bool      `json:"linked"`
	SyncStatus           string    `json:"sync_status,omitempty"`
	SyncError            string    `json:"sync_error,omitempty"`
	SyncCheckedAt        time.Time `json:"sync_checked_at,omitempty"`
}

type Folder struct {
	ID         string `json:"id"`
	AccountID  string `json:"account_id"`
	ProviderID string `json:"provider_id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	ReadOnly   bool   `json:"read_only"`
}

type Attachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	ContentID   string `json:"content_id,omitempty"`
	Inline      bool   `json:"inline,omitempty"`
	Data        []byte `json:"-"`
}

type Message struct {
	ID              string       `json:"id"`
	AccountID       string       `json:"account_id"`
	FolderID        string       `json:"folder_id"`
	RemoteID        string       `json:"remote_id"`
	ThreadID        string       `json:"thread_id"`
	MessageID       string       `json:"message_id,omitempty"`
	InReplyTo       string       `json:"in_reply_to,omitempty"`
	References      []string     `json:"references,omitempty"`
	Subject         string       `json:"subject"`
	From            string       `json:"from"`
	ReplyTo         []string     `json:"reply_to,omitempty"`
	To              []string     `json:"to"`
	CC              []string     `json:"cc"`
	BCC             []string     `json:"bcc"`
	Date            time.Time    `json:"date"`
	Text            string       `json:"text,omitempty"`
	HTML            string       `json:"html,omitempty"`
	Snippet         string       `json:"snippet,omitempty"`
	AttachmentCount int          `json:"attachment_count,omitempty"`
	HTMLSkipped     bool         `json:"html_skipped,omitempty"`
	ReadError       string       `json:"read_error,omitempty"`
	Unread          bool         `json:"unread"`
	Starred         bool         `json:"starred"`
	Labels          []string     `json:"labels,omitempty"`
	SnoozedUntil    time.Time    `json:"snoozed_until,omitempty"`
	Attachments     []Attachment `json:"attachments,omitempty"`
	Raw             []byte       `json:"-"`
}

type MessageQuery struct {
	AccountID         string   `json:"account_id,omitempty"`
	FolderID          string   `json:"folder_id,omitempty"`
	Query             string   `json:"query,omitempty"`
	Unread            bool     `json:"unread,omitempty"`
	Starred           bool     `json:"starred,omitempty"`
	Snoozed           bool     `json:"snoozed,omitempty"`
	View              string   `json:"view,omitempty"`
	AllowedAccountIDs []string `json:"-"`
	Cursor            string   `json:"cursor,omitempty"`
	Limit             int      `json:"limit,omitempty"`
	Offset            int      `json:"offset,omitempty"`
}

type MessagePage struct {
	Messages   []Message `json:"messages"`
	Total      int       `json:"total"`
	Limit      int       `json:"limit"`
	Offset     int       `json:"offset"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

type ThreadPage struct {
	Threads    []Thread `json:"threads"`
	Total      int      `json:"total"`
	Limit      int      `json:"limit"`
	Offset     int      `json:"offset"`
	NextCursor string   `json:"next_cursor,omitempty"`
}

type Thread struct {
	ID            string    `json:"id"`
	AccountID     string    `json:"account_id"`
	FolderID      string    `json:"folder_id,omitempty"`
	Subject       string    `json:"subject"`
	Participants  []string  `json:"participants"`
	LastMessageAt time.Time `json:"last_message_at"`
	Count         int       `json:"count"`
	UnreadCount   int       `json:"unread_count"`
	Preview       string    `json:"preview,omitempty"`
	Unread        bool      `json:"unread"`
	Starred       bool      `json:"starred"`
	Messages      []Message `json:"messages,omitempty"`
}

type Draft struct {
	ID            string       `json:"id"`
	AccountID     string       `json:"account_id"`
	Revision      int64        `json:"revision"`
	To            []string     `json:"to"`
	CC            []string     `json:"cc"`
	BCC           []string     `json:"bcc"`
	Subject       string       `json:"subject"`
	Text          string       `json:"text"`
	HTML          string       `json:"html"`
	InReplyTo     string       `json:"in_reply_to,omitempty"`
	References    []string     `json:"references,omitempty"`
	ReplyRemoteID string       `json:"reply_remote_id,omitempty"`
	Attachments   []Attachment `json:"attachments,omitempty"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type AccountSettings struct {
	Disabled    bool              `json:"disabled"`
	ReadOnly    bool              `json:"read_only"`
	AgentAccess bool              `json:"agent_access"`
	SignatureID string            `json:"signature_id,omitempty"`
	FolderMap   map[string]string `json:"folder_map,omitempty"`
}

type Settings struct {
	Enabled bool `json:"enabled"`
}

type MutationKind string

const (
	MutationRead    MutationKind = "read"
	MutationUnread  MutationKind = "unread"
	MutationStar    MutationKind = "star"
	MutationUnstar  MutationKind = "unstar"
	MutationMove    MutationKind = "move"
	MutationArchive MutationKind = "archive"
	MutationTrash   MutationKind = "trash"
	MutationRestore MutationKind = "restore"
	MutationDelete  MutationKind = "delete"
)

type Mutation struct {
	Kind                MutationKind `json:"kind"`
	DestinationFolderID string       `json:"destination_folder_id,omitempty"`
	Label               string       `json:"label,omitempty"`
}

const (
	MutationJunk        MutationKind = "junk"
	MutationLabelAdd    MutationKind = "label_add"
	MutationLabelRemove MutationKind = "label_remove"
)

type SyncResult struct {
	Messages               []Message `json:"messages"`
	NextCursor             string    `json:"next_cursor"`
	HasMore                bool      `json:"has_more"`
	UIDValidity            uint32    `json:"uid_validity,omitempty"`
	DeletedRemoteIDs       []string  `json:"deleted_remote_ids,omitempty"`
	RemoteSnapshotComplete bool      `json:"remote_snapshot_complete,omitempty"`
	ResetSnapshot          bool      `json:"reset_snapshot,omitempty"`
}

type Outgoing struct {
	Draft Draft `json:"draft"`
}

type SendResult struct {
	Confirmed bool     `json:"confirmed"`
	Rejected  bool     `json:"rejected,omitempty"`
	RemoteID  string   `json:"remote_id,omitempty"`
	Message   *Message `json:"message,omitempty"`
}

type OutboxItem struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"account_id"`
	DraftID     string    `json:"draft_id"`
	Revision    int64     `json:"revision"`
	Timezone    string    `json:"timezone,omitempty"`
	SendAt      time.Time `json:"send_at"`
	UndoUntil   time.Time `json:"undo_until"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	RemoteID    string    `json:"remote_id,omitempty"`
	AppendError string    `json:"append_error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type OutboxPage struct {
	Items      []OutboxItem `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
	HasMore    bool         `json:"has_more"`
}

type RuleRun struct {
	MessageID    string    `json:"message_id"`
	RuleID       string    `json:"rule_id"`
	RuleRevision int64     `json:"rule_revision"`
	ActionIndex  int       `json:"action_index"`
	ActionCount  int       `json:"action_count"`
	Status       string    `json:"status"`
	Error        string    `json:"error,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SendRequest struct {
	DraftID  string    `json:"draft_id"`
	Revision int64     `json:"revision"`
	SendAt   time.Time `json:"send_at,omitempty"`
	Timezone string    `json:"timezone,omitempty"`
}

type Template struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id,omitempty"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Subject   string    `json:"subject,omitempty"`
	Text      string    `json:"text,omitempty"`
	HTML      string    `json:"html,omitempty"`
	Revision  int64     `json:"revision"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RuleCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type RuleAction struct {
	Kind                MutationKind `json:"kind"`
	DestinationFolderID string       `json:"destination_folder_id,omitempty"`
	Label               string       `json:"label,omitempty"`
	SnoozeUntil         time.Time    `json:"snooze_until,omitempty"`
}

type Rule struct {
	ID         string          `json:"id"`
	AccountID  string          `json:"account_id,omitempty"`
	Revision   int64           `json:"revision"`
	Name       string          `json:"name"`
	Enabled    bool            `json:"enabled"`
	Conditions []RuleCondition `json:"conditions"`
	Actions    []RuleAction    `json:"actions"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type RulePreview struct {
	MessageID string       `json:"message_id"`
	Matches   bool         `json:"matches"`
	Actions   []RuleAction `json:"actions,omitempty"`
}

type Adapter interface {
	Folders(context.Context) ([]Folder, error)
	Sync(context.Context, string, string) (SyncResult, error)
	Read(context.Context, string, string) (Message, error)
	Raw(context.Context, string, string) ([]byte, error)
	Attachment(context.Context, string, string, string) (io.ReadCloser, error)
	Mutate(context.Context, string, string, Mutation) error
	Send(context.Context, Outgoing) (SendResult, error)
	Test(context.Context) error
}

// SyncSession keeps one authenticated provider session for a single account
// sync pass. Adapters that do not implement it open a connection per call.
// stop logs out and closes that session. It is safe to call more than once.
type SyncSession interface {
	StartSyncSession(context.Context) (stop func(), err error)
}

type FolderManager interface {
	CreateFolder(context.Context, string, string) (Folder, error)
}

type AdapterResolver func(context.Context, string) (Adapter, Account, error)
type AccountSourceProvider func(context.Context) ([]Account, error)

type Searcher interface {
	Search(context.Context, string, MessageQuery) (MessagePage, error)
}

type SentAppender interface {
	AppendSent(context.Context, Outgoing, SendResult) error
}

type MappedSentAppender interface {
	AppendSentTo(context.Context, Outgoing, SendResult, Folder) error
}

type RemoteDrafts interface {
	SaveRemoteDraft(context.Context, Draft) error
	DeleteRemoteDraft(context.Context, string) error
}
