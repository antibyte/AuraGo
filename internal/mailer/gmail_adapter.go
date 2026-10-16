package mailer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const gmailAPI = "https://gmail.googleapis.com/gmail/v1/users/me"
const maxGmailResponseBytes = 32 << 20
const maxGmailSyncMessages = 100
const maxGmailHistoryIDs = 10000

type gmailAdapter struct{ cfg TransportConfig }

func (a *gmailAdapter) TestSend(ctx context.Context) error {
	if a.cfg.Token == nil {
		return fmt.Errorf("Google send token source unavailable")
	}
	_, err := a.cfg.Token(ctx, "send", true)
	return err
}

func (a *gmailAdapter) Test(ctx context.Context) error {
	var profile struct {
		EmailAddress string `json:"emailAddress"`
	}
	return a.requestLimit(ctx, "read", http.MethodGet, "/profile", nil, nil, &profile, 64<<10)
}
func (a *gmailAdapter) Profile(ctx context.Context) (string, error) {
	var profile struct {
		EmailAddress string `json:"emailAddress"`
	}
	if err := a.request(ctx, "read", http.MethodGet, "/profile", nil, nil, &profile); err != nil {
		return "", err
	}
	address := strings.TrimSpace(profile.EmailAddress)
	if address == "" {
		return "", fmt.Errorf("Gmail profile did not include an email address")
	}
	if _, err := mailboxAddress(address); err != nil {
		return "", fmt.Errorf("Gmail profile returned an invalid email address")
	}
	return address, nil
}
func (a *gmailAdapter) request(ctx context.Context, grant, method, path string, q url.Values, body any, out any) error {
	return a.requestLimit(ctx, grant, method, path, q, body, out, maxGmailResponseBytes)
}

func (a *gmailAdapter) requestLimit(ctx context.Context, grant, method, path string, q url.Values, body any, out any, limit int64) error {
	if a.cfg.Token == nil {
		return fmt.Errorf("Google token source unavailable")
	}
	token, err := a.cfg.Token(ctx, grant, grant != "read")
	if err != nil {
		return err
	}
	var rd io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return e
		}
		rd = bytes.NewReader(b)
	}
	u := gmailAPI + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := a.cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 25 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if limit <= 0 || limit > maxGmailResponseBytes {
		limit = maxGmailResponseBytes
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return fmt.Errorf("Gmail response exceeds size limit")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var envelope struct {
			Error struct {
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &envelope)
		return &gmailAPIError{StatusCode: resp.StatusCode, Message: envelope.Error.Status}
	}
	if out != nil && len(data) > 0 {
		if err = json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode Gmail response: %w", err)
		}
	}
	return nil
}
func (a *gmailAdapter) get(ctx context.Context, grant, path string, q url.Values, out any) ([]byte, error) {
	if out == nil {
		return nil, fmt.Errorf("response target required")
	}
	if grant == "" {
		grant = "read"
	}
	err := a.request(ctx, grant, http.MethodGet, path, q, nil, out)
	return nil, err
}

type gmailAPIError struct {
	StatusCode int
	Message    string
}

func (e *gmailAPIError) Error() string { return "Gmail API request rejected" }
func (a *gmailAdapter) Folders(ctx context.Context) ([]Folder, error) {
	var out struct {
		Labels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"labels"`
	}
	if err := a.request(ctx, "read", http.MethodGet, "/labels", nil, nil, &out); err != nil {
		return nil, err
	}
	folders := make([]Folder, 0, len(out.Labels))
	for _, l := range out.Labels {
		kind := "label"
		switch l.ID {
		case "INBOX":
			kind = "inbox"
		case "SENT":
			kind = "sent"
		case "TRASH":
			kind = "trash"
		case "SPAM":
			kind = "junk"
		case "STARRED":
			kind = "starred"
		case "DRAFT":
			kind = "drafts"
		}
		folders = append(folders, Folder{ID: gmailFolderID(a.accountID(), l.ID), AccountID: a.accountID(), ProviderID: l.ID, Name: l.Name, Kind: kind, ReadOnly: a.cfg.ReadOnly})
	}
	return folders, nil
}

// Search uses Gmail's server-side query syntax and returns metadata only. The
// text is passed as a URL query value, never interpreted as a path or header.
func (a *gmailAdapter) Search(ctx context.Context, folder string, query MessageQuery) (MessagePage, error) {
	text := strings.TrimSpace(query.Query)
	if text == "" || len(text) > 2048 {
		return MessagePage{}, fmt.Errorf("invalid Gmail search query")
	}
	limit := query.Limit
	if limit < 1 || limit > 100 {
		limit = 100
	}
	q := url.Values{"q": {text}, "maxResults": {strconv.Itoa(limit)}}
	if query.Unread {
		q.Set("q", strings.TrimSpace(q.Get("q"))+" is:unread")
	}
	if query.Starred {
		q.Set("q", strings.TrimSpace(q.Get("q"))+" is:starred")
	}
	addGmailFolderFilter(q, a.accountID(), folder)
	if query.Cursor != "" {
		if len(query.Cursor) > 8192 {
			return MessagePage{}, fmt.Errorf("invalid Gmail search cursor")
		}
		token, err := base64.RawURLEncoding.DecodeString(query.Cursor)
		if err != nil || len(token) == 0 || len(token) > 6144 {
			return MessagePage{}, fmt.Errorf("invalid Gmail search cursor")
		}
		q.Set("pageToken", string(token))
	}
	var listed struct {
		Messages      []gmailMessageRef `json:"messages"`
		NextPageToken string            `json:"nextPageToken"`
		ResultSize    int               `json:"resultSizeEstimate"`
	}
	if err := a.requestLimit(ctx, "read", http.MethodGet, "/messages", q, nil, &listed, 4<<20); err != nil {
		return MessagePage{}, err
	}
	page := MessagePage{Messages: make([]Message, 0, len(listed.Messages)), Total: listed.ResultSize, Limit: limit}
	if len(listed.Messages) > limit {
		listed.Messages = listed.Messages[:limit]
	}
	for _, ref := range listed.Messages {
		m, err := a.readMetadata(ctx, ref.ID, ref.ThreadID, folder)
		if err != nil {
			return MessagePage{}, err
		}
		page.Messages = append(page.Messages, m)
	}
	if page.Total < len(page.Messages) {
		page.Total = len(page.Messages)
	}
	if listed.NextPageToken != "" {
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(listed.NextPageToken))
	}
	return page, nil
}

type gmailMessageRef struct {
	ID       string `json:"id"`
	ThreadID string `json:"thread_id,omitempty"`
}

type gmailSyncCursor struct {
	Version           int               `json:"v"`
	HistoryID         string            `json:"history_id,omitempty"`
	PageToken         string            `json:"page_token,omitempty"`
	Snapshot          bool              `json:"snapshot,omitempty"`
	SnapshotHistoryID string            `json:"snapshot_history_id,omitempty"`
	Pending           []gmailMessageRef `json:"pending,omitempty"`
	Deleted           []string          `json:"deleted,omitempty"`
	LatestHistoryID   string            `json:"latest_history_id,omitempty"`
}

func encodeGmailCursor(c gmailSyncCursor) string {
	c.Version = 1
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeGmailCursor(raw string) (gmailSyncCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(data) > 1<<20 {
		return gmailSyncCursor{}, fmt.Errorf("invalid Gmail sync cursor")
	}
	var c gmailSyncCursor
	if json.Unmarshal(data, &c) != nil || c.Version != 1 || (c.HistoryID == "" && !c.Snapshot && c.SnapshotHistoryID == "") {
		return gmailSyncCursor{}, fmt.Errorf("invalid Gmail sync cursor")
	}
	return c, nil
}

func (a *gmailAdapter) Sync(ctx context.Context, folder, cursor string) (SyncResult, error) {
	var state gmailSyncCursor
	if cursor != "" {
		var err error
		state, err = decodeGmailCursor(cursor)
		if err != nil {
			return SyncResult{}, err
		}
	}
	if cursor == "" {
		historyID, err := a.currentHistoryID(ctx)
		if err != nil {
			return SyncResult{}, err
		}
		return a.syncSnapshotPage(ctx, folder, gmailSyncCursor{Version: 1, Snapshot: true, SnapshotHistoryID: historyID})
	}
	if state.Snapshot {
		return a.syncSnapshotPage(ctx, folder, state)
	}
	if state.HistoryID == "" {
		return SyncResult{}, fmt.Errorf("Gmail sync cursor has no history ID")
	}
	return a.syncHistoryPage(ctx, folder, state)
}

func (a *gmailAdapter) currentHistoryID(ctx context.Context) (string, error) {
	var profile struct {
		HistoryID string `json:"historyId"`
	}
	if err := a.requestLimit(ctx, "read", http.MethodGet, "/profile", nil, nil, &profile, 64<<10); err != nil {
		return "", err
	}
	if profile.HistoryID == "" {
		return "", fmt.Errorf("Gmail profile omitted history ID")
	}
	return profile.HistoryID, nil
}

func (a *gmailAdapter) syncSnapshotPage(ctx context.Context, folder string, state gmailSyncCursor) (SyncResult, error) {
	q := url.Values{"maxResults": {"100"}}
	addGmailFolderFilter(q, a.accountID(), folder)
	if state.PageToken != "" {
		q.Set("pageToken", state.PageToken)
	}
	var page struct {
		Messages      []gmailMessageRef `json:"messages"`
		NextPageToken string            `json:"nextPageToken"`
	}
	if err := a.requestLimit(ctx, "read", http.MethodGet, "/messages", q, nil, &page, 4<<20); err != nil {
		return SyncResult{}, err
	}
	result := SyncResult{Messages: make([]Message, 0, len(page.Messages))}
	for _, ref := range page.Messages {
		m, err := a.readMetadata(ctx, ref.ID, ref.ThreadID, folder)
		if err != nil {
			return SyncResult{}, err
		}
		result.Messages = append(result.Messages, m)
	}
	if page.NextPageToken != "" {
		state.PageToken = page.NextPageToken
		result.NextCursor = encodeGmailCursor(state)
		result.HasMore = true
	} else {
		result.NextCursor = encodeGmailCursor(gmailSyncCursor{Version: 1, HistoryID: state.SnapshotHistoryID})
		result.RemoteSnapshotComplete = true
	}
	return result, nil
}

type gmailHistoryPage struct {
	HistoryID     string `json:"historyId"`
	NextPageToken string `json:"nextPageToken"`
	History       []struct {
		ID            string            `json:"id"`
		Messages      []gmailMessageRef `json:"messages"`
		MessagesAdded []struct {
			Message gmailMessageRef `json:"message"`
		} `json:"messagesAdded"`
		MessagesDeleted []struct {
			Message gmailMessageRef `json:"message"`
		} `json:"messagesDeleted"`
		LabelsAdded []struct {
			Message gmailMessageRef `json:"message"`
		} `json:"labelsAdded"`
		LabelsRemoved []struct {
			Message gmailMessageRef `json:"message"`
		} `json:"labelsRemoved"`
	} `json:"history"`
}

func (a *gmailAdapter) syncHistoryPage(ctx context.Context, folder string, state gmailSyncCursor) (SyncResult, error) {
	if len(state.Pending) == 0 && len(state.Deleted) == 0 && (state.LatestHistoryID == "" || state.PageToken != "") {
		q := url.Values{"startHistoryId": {state.HistoryID}, "maxResults": {"100"},
			"historyTypes": {"messageAdded", "messageDeleted", "labelAdded", "labelRemoved"}}
		if state.PageToken != "" {
			q.Set("pageToken", state.PageToken)
		}
		var page gmailHistoryPage
		err := a.requestLimit(ctx, "read", http.MethodGet, "/history", q, nil, &page, 8<<20)
		if err != nil {
			var api *gmailAPIError
			if errors.As(err, &api) && api.StatusCode == http.StatusNotFound {
				fresh, profileErr := a.currentHistoryID(ctx)
				if profileErr != nil {
					return SyncResult{}, profileErr
				}
				result, snapshotErr := a.syncSnapshotPage(ctx, folder, gmailSyncCursor{Version: 1, Snapshot: true, SnapshotHistoryID: fresh})
				result.ResetSnapshot = true
				return result, snapshotErr
			}
			return SyncResult{}, err
		}
		refs := make(map[string]string)
		deleted := map[string]struct{}{}
		for _, item := range page.History {
			for _, ref := range item.Messages {
				addGmailRef(refs, ref)
			}
			for _, event := range item.MessagesAdded {
				addGmailRef(refs, event.Message)
			}
			for _, event := range item.LabelsAdded {
				addGmailRef(refs, event.Message)
			}
			for _, event := range item.LabelsRemoved {
				addGmailRef(refs, event.Message)
			}
			for _, event := range item.MessagesDeleted {
				if event.Message.ID != "" {
					deleted[event.Message.ID] = struct{}{}
				}
			}
		}
		if len(refs)+len(deleted) > maxGmailHistoryIDs {
			return SyncResult{}, fmt.Errorf("Gmail history page exceeds change limit")
		}
		state.Pending = make([]gmailMessageRef, 0, len(refs))
		for id, thread := range refs {
			state.Pending = append(state.Pending, gmailMessageRef{ID: id, ThreadID: thread})
		}
		state.Deleted = make([]string, 0, len(deleted))
		for id := range deleted {
			state.Deleted = append(state.Deleted, id)
		}
		state.PageToken = page.NextPageToken
		state.LatestHistoryID = page.HistoryID
		if state.LatestHistoryID == "" {
			state.LatestHistoryID = state.HistoryID
		}
	}
	result := SyncResult{Messages: []Message{}}
	label := gmailProviderLabel(a.accountID(), folder)
	work := maxGmailSyncMessages
	for work > 0 && len(state.Pending) > 0 {
		ref := state.Pending[0]
		state.Pending = state.Pending[1:]
		m, err := a.readMetadata(ctx, ref.ID, ref.ThreadID, folder)
		if err != nil {
			var api *gmailAPIError
			if errors.As(err, &api) && api.StatusCode == http.StatusNotFound {
				state.Deleted = append(state.Deleted, ref.ID)
				continue
			}
			return SyncResult{}, err
		}
		if label != "" && !containsMailString(m.Labels, label) {
			state.Deleted = append(state.Deleted, ref.ID)
			continue
		}
		result.Messages = append(result.Messages, m)
		work--
	}
	for work > 0 && len(state.Deleted) > 0 {
		result.DeletedRemoteIDs = append(result.DeletedRemoteIDs, state.Deleted[0])
		state.Deleted = state.Deleted[1:]
		work--
	}
	more := len(state.Pending) > 0 || len(state.Deleted) > 0 || state.PageToken != ""
	if more {
		result.HasMore = true
		result.NextCursor = encodeGmailCursor(state)
	} else {
		next := state.LatestHistoryID
		if next == "" {
			next = state.HistoryID
		}
		result.NextCursor = encodeGmailCursor(gmailSyncCursor{Version: 1, HistoryID: next})
	}
	return result, nil
}

func addGmailRef(refs map[string]string, ref gmailMessageRef) {
	if ref.ID == "" {
		return
	}
	if refs[ref.ID] == "" {
		refs[ref.ID] = ref.ThreadID
	}
}

func containsMailString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func (a *gmailAdapter) readMessage(ctx context.Context, id, thread, folder string) (Message, error) {
	metadata, e := a.readMetadata(ctx, id, thread, folder)
	if e != nil {
		return Message{}, e
	}
	raw, e := a.Raw(ctx, folder, id)
	if e != nil {
		return Message{}, e
	}
	subj, from, to, cc, body, html, atts, htmlSkipped, e := ParseMIME(raw, maxGmailResponseBytes)
	if e != nil {
		return Message{}, e
	}
	metadata.Subject, metadata.From, metadata.To, metadata.CC = subj, from, to, cc
	metadata.ReplyTo = ParseReplyTo(raw)
	metadata.Text, metadata.HTML, metadata.HTMLSkipped, metadata.Attachments = body, html, htmlSkipped, atts
	metadata.Raw = append([]byte(nil), raw...)
	if mid, irt, refs, e := ParseThreadHeaders(raw); e == nil {
		metadata.MessageID, metadata.InReplyTo, metadata.References = mid, irt, refs
	}
	return metadata, nil
}

func (a *gmailAdapter) readMetadata(ctx context.Context, id, thread, folder string) (Message, error) {
	var meta struct {
		LabelIDs     []string `json:"labelIds"`
		InternalDate string   `json:"internalDate"`
		ThreadID     string   `json:"threadId"`
		Payload      struct {
			Headers []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
		} `json:"payload"`
	}
	q := url.Values{"format": {"metadata"}, "metadataHeaders": {"From", "Reply-To", "To", "Cc", "Subject", "Date", "Message-ID", "In-Reply-To", "References"}}
	if err := a.requestLimit(ctx, "read", http.MethodGet, "/messages/"+url.PathEscape(id), q, nil, &meta, 256<<10); err != nil {
		return Message{}, err
	}
	threadID := thread
	if meta.ThreadID != "" {
		threadID = meta.ThreadID
	}
	folderID := folder
	if folderID == "" {
		folderID = gmailFolderID(a.accountID(), "INBOX")
	}
	m := Message{AccountID: a.accountID(), FolderID: folderID, RemoteID: id, ThreadID: threadID, Unread: false, Labels: append([]string(nil), meta.LabelIDs...)}
	for _, l := range meta.LabelIDs {
		switch l {
		case "UNREAD":
			m.Unread = true
		case "STARRED":
			m.Starred = true
		}
	}
	if n, e := strconv.ParseInt(meta.InternalDate, 10, 64); e == nil && n > 0 {
		m.Date = time.UnixMilli(n)
	}
	for _, h := range meta.Payload.Headers {
		v := strings.TrimSpace(h.Value)
		switch strings.ToLower(h.Name) {
		case "subject":
			m.Subject = v
		case "from":
			if parsed, err := mail.ParseAddress(v); err == nil {
				m.From = parsed.Address
			}
		case "reply-to":
			m.ReplyTo = parseGmailAddressList(v)
		case "to":
			m.To = parseGmailAddressList(v)
		case "cc":
			m.CC = parseGmailAddressList(v)
		case "message-id":
			m.MessageID = v
		case "in-reply-to":
			m.InReplyTo = v
		case "references":
			m.References = strings.Fields(v)
		case "date":
			if m.Date.IsZero() {
				m.Date, _ = mail.ParseDate(v)
			}
		}
	}
	return m, nil
}

func parseGmailAddressList(raw string) []string {
	parsed, err := mail.ParseAddressList(raw)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(parsed))
	for _, address := range parsed {
		if address != nil {
			out = append(out, address.Address)
		}
	}
	return out
}
func (a *gmailAdapter) Read(ctx context.Context, folder, id string) (Message, error) {
	return a.readMessage(ctx, id, "", folder)
}
func (a *gmailAdapter) Raw(ctx context.Context, folder, id string) ([]byte, error) {
	var got struct {
		Raw string `json:"raw"`
	}
	q := url.Values{"format": {"raw"}}
	if e := a.request(ctx, "read", http.MethodGet, "/messages/"+url.PathEscape(id), q, nil, &got); e != nil {
		return nil, e
	}
	data, e := base64.RawURLEncoding.DecodeString(got.Raw)
	if e != nil {
		data, e = base64.URLEncoding.DecodeString(got.Raw)
	}
	if e != nil {
		return nil, fmt.Errorf("invalid Gmail raw message")
	}
	if len(data) > maxGmailResponseBytes {
		return nil, fmt.Errorf("Gmail message exceeds size limit")
	}
	return data, nil
}
func (a *gmailAdapter) Attachment(ctx context.Context, folder, id, part string) (io.ReadCloser, error) {
	raw, e := a.Raw(ctx, folder, id)
	if e != nil {
		return nil, e
	}
	_, _, _, _, _, _, atts, _, e := ParseMIME(raw, maxGmailResponseBytes)
	if e != nil {
		return nil, e
	}
	for _, x := range atts {
		if x.ID == part {
			return io.NopCloser(bytes.NewReader(x.Data)), nil
		}
	}
	return nil, fmt.Errorf("attachment not found")
}
func (a *gmailAdapter) Mutate(ctx context.Context, folder, id string, mu Mutation) error {
	grant := "modify"
	add, remove := []string{}, []string{}
	switch mu.Kind {
	case MutationRead:
		remove = []string{"UNREAD"}
	case MutationUnread:
		add = []string{"UNREAD"}
	case MutationStar:
		add = []string{"STARRED"}
	case MutationUnstar:
		remove = []string{"STARRED"}
	case MutationMove:
		dest := gmailProviderLabel(a.accountID(), mu.DestinationFolderID)
		if dest == "" {
			return fmt.Errorf("destination label required")
		}
		add = []string{dest}
		cur := gmailProviderLabel(a.accountID(), folder)
		if cur != "" {
			remove = []string{cur}
		}
	case MutationArchive:
		remove = []string{"INBOX"}
	case MutationTrash:
		add = []string{"TRASH"}
		remove = []string{"INBOX"}
	case MutationRestore:
		add = []string{"INBOX"}
		remove = []string{"TRASH"}
	case MutationJunk:
		add = []string{"SPAM"}
		remove = []string{"INBOX"}
	case MutationLabelAdd:
		if !validMailLabel(mu.Label) {
			return fmt.Errorf("invalid label")
		}
		add = []string{mu.Label}
	case MutationLabelRemove:
		if !validMailLabel(mu.Label) {
			return fmt.Errorf("invalid label")
		}
		remove = []string{mu.Label}
	case MutationDelete:
		grant = "delete"
		return a.request(ctx, grant, http.MethodDelete, "/messages/"+url.PathEscape(id), nil, nil, nil)
	default:
		return fmt.Errorf("unsupported Gmail mutation")
	}
	return a.request(ctx, grant, http.MethodPost, "/messages/"+url.PathEscape(id)+"/modify", nil, map[string][]string{"addLabelIds": add, "removeLabelIds": remove}, nil)
}
func (a *gmailAdapter) Send(ctx context.Context, out Outgoing) (SendResult, error) {
	if _, e := validatedRecipients(out.Draft.To); e != nil {
		return SendResult{Rejected: true}, e
	}
	for _, list := range [][]string{out.Draft.CC, out.Draft.BCC} {
		if _, e := validatedRecipients(list); e != nil {
			return SendResult{Rejected: true}, e
		}
	}
	from := a.cfg.FromAddress
	if from == "" {
		from = a.cfg.Address
	}
	raw, e := buildGmailMIME(from, out.Draft, time.Now())
	if e != nil {
		return SendResult{Rejected: true}, e
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	var result struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
	}
	e = a.request(ctx, "send", http.MethodPost, "/messages/send", nil, map[string]string{"raw": encoded}, &result)
	if e != nil {
		var api *gmailAPIError
		if errors.As(e, &api) && api.StatusCode >= 400 && api.StatusCode < 500 {
			return SendResult{Rejected: true}, e
		}
		return SendResult{}, e
	}
	if result.ID == "" {
		return SendResult{}, fmt.Errorf("Gmail accepted send without a message id")
	}
	messageID, inReplyTo, refs, _ := ParseThreadHeaders(raw)
	m := Message{AccountID: a.accountID(), RemoteID: result.ID, ThreadID: result.ThreadID, MessageID: messageID, InReplyTo: inReplyTo, References: refs, Subject: out.Draft.Subject, From: from, To: append([]string(nil), out.Draft.To...), CC: append([]string(nil), out.Draft.CC...), BCC: append([]string(nil), out.Draft.BCC...), Date: time.Now(), Raw: append([]byte(nil), raw...)}
	return SendResult{Confirmed: true, RemoteID: result.ID, Message: &m}, nil
}
func (a *gmailAdapter) AppendSent(context.Context, Outgoing, SendResult) error { return nil }
func (a *gmailAdapter) AppendSentTo(ctx context.Context, _ Outgoing, result SendResult, folder Folder) error {
	label := strings.TrimSpace(folder.ProviderID)
	if label == "" || result.RemoteID == "" {
		return fmt.Errorf("sent label or message ID is unavailable")
	}
	return a.request(ctx, "modify", http.MethodPost, "/messages/"+url.PathEscape(result.RemoteID)+"/modify", nil,
		map[string][]string{"addLabelIds": {label}, "removeLabelIds": {}}, nil)
}
func (a *gmailAdapter) accountID() string        { return a.cfg.Source + ":" + a.cfg.ID }
func gmailFolderID(account, label string) string { return account + ":gmail:" + url.PathEscape(label) }
func addGmailFolderFilter(q url.Values, account, folder string) {
	label := gmailProviderLabel(account, folder)
	if label == "" {
		return
	}
	q.Set("labelIds", label)
	if label == "SPAM" || label == "TRASH" {
		q.Set("includeSpamTrash", "true")
	}
}
func gmailProviderLabel(account, id string) string {
	prefix := account + ":gmail:"
	if !strings.HasPrefix(id, prefix) {
		return id
	}
	v, e := url.PathUnescape(strings.TrimPrefix(id, prefix))
	if e != nil {
		return ""
	}
	return v
}
func (a *gmailAdapter) CreateFolder(ctx context.Context, name string) (Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "\r\n\x00") {
		return Folder{}, fmt.Errorf("invalid label name")
	}
	var out struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	err := a.request(ctx, "create", http.MethodPost, "/labels", nil, map[string]string{"name": name, "labelListVisibility": "labelShow", "messageListVisibility": "show"}, &out)
	if err != nil {
		return Folder{}, err
	}
	return Folder{ID: gmailFolderID(a.accountID(), out.ID), AccountID: a.accountID(), ProviderID: out.ID, Name: out.Name, Kind: "label"}, nil
}
func (a *gmailAdapter) DeleteFolder(ctx context.Context, id string) error {
	label := gmailProviderLabel(a.accountID(), id)
	if strings.ToUpper(label) == "INBOX" || strings.ToUpper(label) == "SENT" {
		return fmt.Errorf("cannot delete system label")
	}
	return a.request(ctx, "modify", http.MethodDelete, "/labels/"+url.PathEscape(label), nil, nil, nil)
}
func (a *gmailAdapter) RenameFolder(context.Context, string, string) error {
	return fmt.Errorf("Gmail labels cannot be renamed")
}
