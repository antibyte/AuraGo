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

// Message is one entry of the project's agent chat. System lines carry the
// code of their text (a messageTexts key, which the client translates) and
// its vars; user and agent lines carry neither.
type Message struct {
	ID          string         `json:"id"`
	JobID       string         `json:"job_id"`
	Role        string         `json:"role"`
	Text        string         `json:"text"`
	Code        string         `json:"code,omitempty"`
	Vars        map[string]any `json:"vars,omitempty"`
	Attachments []Attachment   `json:"attachments"`
	CreatedAt   string         `json:"created_at"`
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
	return s.addMessage(ctx, projectID, jobID, MessageRoleUser, text, attachments, "", nil)
}

// addMessage stores a message of any role and emits job_message. Agent and
// system texts are capped like prompts. code and vars (system lines only)
// are stored as given; vars as JSON, "" when there are none.
func (s *Service) addMessage(ctx context.Context, projectID, jobID, role, text string, attachments []Attachment, code string, vars map[string]any) (Message, error) {
	if utf8.RuneCountInString(text) > MaxPromptRunes {
		text = string([]rune(text)[:MaxPromptRunes-1]) + "…"
	}
	if attachments == nil {
		attachments = []Attachment{}
	}
	storedVars := ""
	if len(vars) > 0 {
		storedVars = mustJSON(vars)
	} else {
		vars = nil
	}
	m := Message{ID: s.ids.NewID("ccm"), JobID: jobID, Role: role, Text: text, Code: code, Vars: vars, Attachments: attachments,
		CreatedAt: formatTime(s.now())}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO cc_messages(id,project_id,job_id,role,text,attachments,code,vars,created_at)
		VALUES(?,?,?,?,?,?,?,?,?)`, m.ID, projectID, jobID, role, text, mustJSON(attachments), code, storedVars, m.CreatedAt); err != nil {
		return Message{}, s.internalError("add message", err)
	}
	s.Emit(projectID, EventJobMessage, jobID, "", map[string]any{"message": m})
	return m, nil
}

// postMessage stores an agent chat line; failures are logged. System lines
// go through postSystemMessage.
func (s *Service) postMessage(projectID, jobID, role, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if _, err := s.addMessage(context.Background(), projectID, jobID, role, text, nil, "", nil); err != nil {
		s.logger.Warn("CYD Creator chat message could not be stored", "job_id", jobID, "error", err)
	}
}

// postSystemMessage stores a system chat line whose English text is rendered
// from its code's template (messageTexts) with vars; the line keeps code and
// vars for the client's translation. An unknown code is a programming error:
// it is logged and nothing is posted. Failures are logged.
func (s *Service) postSystemMessage(projectID, jobID, code string, vars map[string]any) {
	template, ok := messageTexts[code]
	if !ok {
		s.logger.Error("CYD Creator system message without a text for its code", "code", code, "job_id", jobID)
		return
	}
	vars = textVars(vars, maxMessageVarRunes)
	text := strings.TrimSpace(renderText(template, vars))
	if _, err := s.addMessage(context.Background(), projectID, jobID, MessageRoleSystem, text, nil, code, vars); err != nil {
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
	rows, err := s.db.QueryContext(ctx, `SELECT id,job_id,role,text,attachments,code,vars,created_at FROM (
		SELECT rowid AS rid, * FROM cc_messages WHERE project_id=? ORDER BY created_at DESC, rowid DESC LIMIT ?) ORDER BY created_at, rid`,
		projectID, maxListedMessages)
	if err != nil {
		return nil, s.internalError("list messages", err)
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var attachments, vars string
		if err := rows.Scan(&m.ID, &m.JobID, &m.Role, &m.Text, &attachments, &m.Code, &vars, &m.CreatedAt); err != nil {
			return nil, s.internalError("list messages", err)
		}
		m.Attachments = []Attachment{}
		_ = json.Unmarshal([]byte(attachments), &m.Attachments)
		if m.Attachments == nil {
			m.Attachments = []Attachment{}
		}
		if vars != "" {
			// Numbers stay exact (json.Number). Unreadable vars drop the code
			// too: the client then shows the stored English text.
			dec := json.NewDecoder(strings.NewReader(vars))
			dec.UseNumber()
			if err := dec.Decode(&m.Vars); err != nil {
				m.Code, m.Vars = "", nil
			}
		}
		out = append(out, m)
	}
	return out, s.internalError("list messages", rows.Err())
}
