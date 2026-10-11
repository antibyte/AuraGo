package cydcreator

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// Message roles (overview "Message" JSON).
const (
	MessageRoleUser   = "user"
	MessageRoleAgent  = "agent"
	MessageRoleSystem = "system"

	maxListedMessages = 200
)

// Message is one entry of the project's agent chat.
type Message struct {
	ID          string       `json:"id"`
	JobID       string       `json:"job_id"`
	Role        string       `json:"role"`
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments"`
	CreatedAt   string       `json:"created_at"`
}

// AddUserMessage stores a user chat message (with optional log attachments)
// and emits job_message. A message needs text or a non-blank attachment.
func (s *Service) AddUserMessage(ctx context.Context, projectID, jobID, text string, attachments []Attachment) (Message, error) {
	text, err := validPrompt(text)
	if err != nil {
		return Message{}, err
	}
	attachments, err = validAttachments(attachments)
	if err != nil {
		return Message{}, err
	}
	if text == "" && len(attachments) == 0 {
		return Message{}, NewError(CodeBadRequest, "the message is empty", nil)
	}
	if _, err := s.loadProjectRow(ctx, projectID); err != nil {
		return Message{}, s.internalError("add message", err)
	}
	return s.addMessage(ctx, projectID, jobID, MessageRoleUser, text, attachments)
}

// addMessage stores a message of any role and emits job_message. Agent and
// system texts are capped like prompts.
func (s *Service) addMessage(ctx context.Context, projectID, jobID, role, text string, attachments []Attachment) (Message, error) {
	if utf8.RuneCountInString(text) > MaxPromptRunes {
		text = string([]rune(text)[:MaxPromptRunes-1]) + "…"
	}
	if attachments == nil {
		attachments = []Attachment{}
	}
	m := Message{ID: s.ids.NewID("ccm"), JobID: jobID, Role: role, Text: text, Attachments: attachments,
		CreatedAt: formatTime(s.now())}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO cc_messages(id,project_id,job_id,role,text,attachments,created_at)
		VALUES(?,?,?,?,?,?,?)`, m.ID, projectID, jobID, role, text, mustJSON(attachments), m.CreatedAt); err != nil {
		return Message{}, s.internalError("add message", err)
	}
	s.Emit(projectID, EventJobMessage, jobID, "", map[string]any{"message": m})
	return m, nil
}

// postMessage stores an agent or system chat line; failures are logged.
func (s *Service) postMessage(projectID, jobID, role, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if _, err := s.addMessage(context.Background(), projectID, jobID, role, text, nil); err != nil {
		s.logger.Warn("CYD Creator chat message could not be stored", "job_id", jobID, "error", err)
	}
}

// ListMessages returns the newest 200 chat messages of a project, oldest
// first. Messages of the same millisecond keep their insertion order (rowid),
// not the order of their random ids.
func (s *Service) ListMessages(ctx context.Context, projectID string) ([]Message, error) {
	if _, err := s.loadProjectRow(ctx, projectID); err != nil {
		return nil, s.internalError("list messages", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,job_id,role,text,attachments,created_at FROM (
		SELECT rowid AS rid, * FROM cc_messages WHERE project_id=? ORDER BY created_at DESC, rowid DESC LIMIT ?) ORDER BY created_at, rid`,
		projectID, maxListedMessages)
	if err != nil {
		return nil, s.internalError("list messages", err)
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var attachments string
		if err := rows.Scan(&m.ID, &m.JobID, &m.Role, &m.Text, &attachments, &m.CreatedAt); err != nil {
			return nil, s.internalError("list messages", err)
		}
		m.Attachments = []Attachment{}
		_ = json.Unmarshal([]byte(attachments), &m.Attachments)
		if m.Attachments == nil {
			m.Attachments = []Attachment{}
		}
		out = append(out, m)
	}
	return out, s.internalError("list messages", rows.Err())
}
