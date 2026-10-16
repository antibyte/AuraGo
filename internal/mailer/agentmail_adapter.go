package mailer

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"sort"
	"strings"
	"time"

	"aurago/internal/agentmail"
)

const maxAgentMailAttachmentBytes = 25 << 20

type agentMailAdapter struct {
	cfg    TransportConfig
	client *agentmail.Client
}

func newAgentMailAdapter(cfg TransportConfig) (Adapter, error) {
	if cfg.AgentMailInboxID == "" || cfg.AgentMailAPIKey == "" {
		return nil, fmt.Errorf("AgentMail inbox and Vault key are required")
	}
	c, err := agentmail.NewClient(agentmail.ClientConfig{BaseURL: cfg.AgentMailBaseURL, APIKey: cfg.AgentMailAPIKey, HTTPClient: cfg.HTTPClient, MaxResponseBytes: 8 << 20})
	if err != nil {
		return nil, err
	}
	return &agentMailAdapter{cfg: cfg, client: c}, nil
}
func (a *agentMailAdapter) Test(ctx context.Context) error {
	_, err := a.client.GetInbox(ctx, a.cfg.AgentMailInboxID)
	return err
}
func (a *agentMailAdapter) TestSend(ctx context.Context) error { return a.Test(ctx) }
func (a *agentMailAdapter) Folders(ctx context.Context) ([]Folder, error) {
	account := a.accountID()
	labels, err := a.discoverAgentMailFolderLabels(ctx)
	if err != nil {
		return nil, err
	}
	names := map[string]struct{}{"inbox": {}, "sent": {}, "archive": {}, "junk": {}}
	for _, hiddenLabel := range []string{"trash", "spam"} {
		options := agentmail.ListMessagesOptions{Limit: 1, Labels: []string{hiddenLabel}, IncludeTrash: hiddenLabel == "trash", IncludeSpam: hiddenLabel == "spam"}
		if _, probeErr := a.client.ListMessages(ctx, a.cfg.AgentMailInboxID, options); probeErr == nil {
			names[hiddenLabel] = struct{}{}
		} else {
			var apiErr *agentmail.APIError
			if errors.As(probeErr, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
				continue // The account key cannot read this protected provider folder.
			}
			return nil, fmt.Errorf("probe AgentMail %s folder: %w", hiddenLabel, probeErr)
		}
	}
	for _, label := range labels {
		names[label] = struct{}{}
	}

	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	out := make([]Folder, 0, len(ordered))
	for _, name := range ordered {
		kind := "label"
		if name == "inbox" || name == "sent" || name == "archive" || name == "junk" || name == "trash" || name == "spam" {
			kind = name
		}
		out = append(out, Folder{ID: agentMailFolderID(account, name), AccountID: account, ProviderID: name, Name: name, Kind: kind, ReadOnly: a.cfg.ReadOnly})
	}
	return out, nil
}
func (a *agentMailAdapter) Sync(ctx context.Context, folder, cursor string) (SyncResult, error) {
	label := agentMailLabel(a.accountID(), folder)
	if label == "" {
		label = "inbox"
	}
	providerLabel := agentMailProviderLabel(label)
	res, err := a.client.ListMessages(ctx, a.cfg.AgentMailInboxID, agentmail.ListMessagesOptions{
		Limit: 100, Cursor: cursor, Labels: []string{providerLabel},
		IncludeSpam: label == "spam", IncludeTrash: label == "trash",
	})
	if err != nil {
		return SyncResult{}, err
	}
	out := SyncResult{Messages: make([]Message, 0, len(res.Messages)), NextCursor: res.NextCursor, HasMore: res.HasMore,
		RemoteSnapshotComplete: !res.HasMore && res.NextCursor == ""}
	for _, raw := range res.Messages {
		if !agentMailMessageInFolder(raw, label) {
			continue
		}
		m := agentMailMessage(a.cfg, raw)
		out.Messages = append(out.Messages, m)
	}
	return out, nil
}
func (a *agentMailAdapter) Read(ctx context.Context, folder, id string) (Message, error) {
	m, e := a.client.GetMessage(ctx, a.cfg.AgentMailInboxID, id)
	if e != nil {
		return Message{}, e
	}
	return agentMailMessage(a.cfg, *m), nil
}
func (a *agentMailAdapter) Raw(ctx context.Context, folder, id string) ([]byte, error) {
	return a.client.GetRawMessage(ctx, a.cfg.AgentMailInboxID, id)
}
func (a *agentMailAdapter) Attachment(ctx context.Context, folder, id, part string) (io.ReadCloser, error) {
	data, _, e := a.client.DownloadAttachment(ctx, a.cfg.AgentMailInboxID, id, part, maxAgentMailAttachmentBytes)
	if e != nil {
		return nil, e
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (a *agentMailAdapter) Mutate(ctx context.Context, folder, id string, mu Mutation) error {
	if a.cfg.ReadOnly {
		return fmt.Errorf("AgentMail account is read-only")
	}
	add, remove := []string{}, []string{}
	invalidateFolders := false
	switch mu.Kind {
	case MutationRead:
		add = []string{"read"}
		remove = []string{"unread"}
	case MutationUnread:
		add = []string{"unread"}
		remove = []string{"read"}
	case MutationStar:
		add = []string{"starred"}
		remove = []string{"unstarred"}
	case MutationUnstar:
		add = []string{"unstarred"}
		remove = []string{"starred"}
	case MutationLabelAdd:
		if !validMailLabel(mu.Label) || agentMailSystemLabel(mu.Label) {
			return fmt.Errorf("invalid label")
		}
		add = []string{mu.Label}
		invalidateFolders = true
	case MutationLabelRemove:
		if !validMailLabel(mu.Label) || agentMailSystemLabel(mu.Label) {
			return fmt.Errorf("invalid label")
		}
		remove = []string{mu.Label}
		invalidateFolders = true
	case MutationMove:
		label := agentMailLabel(a.accountID(), mu.DestinationFolderID)
		if label == "" {
			return fmt.Errorf("destination label is required")
		}
		sourceLabel := agentMailLabel(a.accountID(), folder)
		switch label {
		case "inbox":
			remove = []string{"archive", "trash"}
		case "archive":
			remove = []string{"trash", "junk"}
			add = []string{"archive"}
		case "trash":
			remove = []string{"archive", "junk"}
			add = []string{"trash"}
		case "junk":
			remove = []string{"archive", "trash"}
			add = []string{"junk"}
		case "spam":
			return fmt.Errorf("AgentMail classified Spam folder cannot be assigned as a label")
		case "sent", "received":
			return fmt.Errorf("AgentMail system folders cannot be assigned as custom labels")
		default:
			if !validMailLabel(label) {
				return fmt.Errorf("invalid destination label")
			}
			add = []string{label}
			invalidateFolders = true
		}
		if sourceLabel != "" && sourceLabel != label && sourceLabel != "inbox" && sourceLabel != "received" && sourceLabel != "sent" && sourceLabel != "spam" {
			remove = appendMailLabel(remove, sourceLabel)
			invalidateFolders = invalidateFolders || !agentMailSystemLabel(sourceLabel)
		}
	case MutationArchive:
		remove = []string{"trash", "junk"}
		add = []string{"archive"}
	case MutationTrash:
		remove = []string{"archive", "junk"}
		add = []string{"trash"}
	case MutationRestore:
		remove = []string{"trash", "archive", "junk"}
	case MutationJunk:
		remove = []string{"trash", "archive"}
		add = []string{"junk"}
	case MutationDelete:
		return a.client.DeleteMessage(ctx, a.cfg.AgentMailInboxID, id)
	default:
		return fmt.Errorf("unsupported AgentMail mutation")
	}
	_, err := a.client.UpdateMessage(ctx, a.cfg.AgentMailInboxID, id, agentmail.UpdateMessageRequest{AddLabels: add, RemoveLabels: remove})
	if err == nil && invalidateFolders {
		invalidateAgentMailFolderCache(a.cfg)
	}
	return err
}
func (a *agentMailAdapter) Send(ctx context.Context, out Outgoing) (SendResult, error) {
	if a.cfg.ReadOnly {
		return SendResult{Rejected: true}, fmt.Errorf("AgentMail account is read-only")
	}
	if out.Draft.ReplyRemoteID != "" && !validAgentMailReplyID(out.Draft.ReplyRemoteID) {
		return SendResult{Rejected: true}, fmt.Errorf("invalid AgentMail reply target")
	}
	to, e := validatedRecipients(out.Draft.To)
	if e != nil {
		return SendResult{Rejected: true}, e
	}
	cc, e := validatedRecipients(out.Draft.CC)
	if e != nil {
		return SendResult{Rejected: true}, e
	}
	bcc, e := validatedRecipients(out.Draft.BCC)
	if e != nil {
		return SendResult{Rejected: true}, e
	}
	attachments := make([]agentmail.OutgoingAttachment, 0, len(out.Draft.Attachments))
	var total int64
	for _, att := range out.Draft.Attachments {
		total += int64(len(att.Data))
		if total > maxAgentMailAttachmentBytes {
			return SendResult{Rejected: true}, fmt.Errorf("attachments exceed size limit")
		}
		attachments = append(attachments, agentmail.OutgoingAttachment{Filename: safeFilename(att.Filename), ContentType: att.ContentType, ContentBase64: base64.StdEncoding.EncodeToString(att.Data)})
	}
	htmlBody := out.Draft.HTML
	if htmlBody != "" {
		htmlBody, e = SanitizeHTML(htmlBody)
		if e != nil {
			return SendResult{Rejected: true}, fmt.Errorf("sanitize HTML body: %w", e)
		}
	}

	// AgentMail's native reply endpoint preserves its provider thread but does
	// not accept a subject field. Use it only when the user's reviewed subject
	// is exactly the standard reply subject; otherwise a regular send with the
	// reviewed subject and validated RFC thread headers is safer than silently
	// discarding the user's edit.
	var original *agentmail.Message
	useNativeReply := false
	if out.Draft.ReplyRemoteID != "" {
		original, e = a.client.GetMessage(ctx, a.cfg.AgentMailInboxID, out.Draft.ReplyRemoteID)
		if e != nil {
			return SendResult{Rejected: true}, fmt.Errorf("load AgentMail reply target before sending: %w", e)
		}
		if original == nil {
			return SendResult{Rejected: true}, fmt.Errorf("load AgentMail reply target before sending: empty response")
		}
		useNativeReply = out.Draft.Subject == standardAgentMailReplySubject(original.Subject)
	}
	threadHeaders, e := agentMailThreadHeaders(out.Draft, original)
	if e != nil {
		return SendResult{Rejected: true}, e
	}
	if (out.Draft.ReplyRemoteID != "" || out.Draft.InReplyTo != "" || len(out.Draft.References) > 0) &&
		out.Draft.ReplyRemoteID == "" && len(threadHeaders) == 0 {
		return SendResult{Rejected: true}, fmt.Errorf("AgentMail reply is missing RFC thread headers")
	}

	var resp *agentmail.Message
	if useNativeReply {
		resp, e = a.client.ReplyMessage(ctx, a.cfg.AgentMailInboxID, out.Draft.ReplyRemoteID, agentmail.ReplyMessageRequest{
			To: to, CC: cc, BCC: bcc, Text: out.Draft.Text, HTML: htmlBody, Attachments: attachments,
		})
	} else {
		resp, e = a.client.SendMessage(ctx, a.cfg.AgentMailInboxID, agentmail.SendMessageRequest{
			To: to, CC: cc, BCC: bcc, Subject: out.Draft.Subject, Text: out.Draft.Text, HTML: htmlBody, Attachments: attachments, Headers: threadHeaders,
		})
	}
	if e != nil {
		var api *agentmail.APIError
		if errors.As(e, &api) && api.StatusCode >= 400 && api.StatusCode < 500 {
			return SendResult{Rejected: true}, e
		}
		return SendResult{}, e
	}
	if resp == nil || resp.ID == "" {
		return SendResult{}, fmt.Errorf("AgentMail accepted send but did not return a message id")
	}
	m := agentMailMessage(a.cfg, *resp)
	return SendResult{Confirmed: true, RemoteID: resp.ID, Message: &m}, nil
}

func appendMailLabel(labels []string, label string) []string {
	for _, existing := range labels {
		if strings.EqualFold(existing, label) {
			return labels
		}
	}
	return append(labels, label)
}

func validAgentMailReplyID(id string) bool {
	return id != "" && len(id) <= 998 && !strings.ContainsAny(id, "\r\n\x00")
}

func standardAgentMailReplySubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "Re:"
	}
	if strings.HasPrefix(strings.ToLower(subject), "re:") {
		return subject
	}
	return "Re: " + subject
}

func agentMailThreadHeaders(d Draft, original *agentmail.Message) (map[string]string, error) {
	inReplyTo := d.InReplyTo
	references := append([]string(nil), d.References...)
	if original != nil {
		if inReplyTo == "" {
			inReplyTo = agentMailHeader(original.Headers, "Message-ID")
		}
		if len(references) == 0 {
			references = append(references, original.References...)
		}
	}
	headers := make(map[string]string, 2)
	var normalizedInReplyTo string
	if inReplyTo != "" {
		id, err := safeMessageID(inReplyTo)
		if err != nil {
			return nil, fmt.Errorf("invalid AgentMail In-Reply-To header")
		}
		normalizedInReplyTo = id
		headers["In-Reply-To"] = normalizedInReplyTo
	}
	if normalizedInReplyTo != "" {
		found := false
		for _, ref := range references {
			if id, err := safeMessageID(ref); err == nil && strings.EqualFold(id, normalizedInReplyTo) {
				found = true
				break
			}
		}
		if !found {
			references = append(references, normalizedInReplyTo)
		}
	}
	if len(references) > 100 {
		return nil, fmt.Errorf("too many AgentMail References values")
	}
	cleanReferences := make([]string, 0, len(references))
	seenReferences := make(map[string]struct{}, len(references))
	for _, ref := range references {
		id, err := safeMessageID(ref)
		if err != nil {
			return nil, fmt.Errorf("invalid AgentMail References header")
		}
		key := strings.ToLower(id)
		if _, exists := seenReferences[key]; exists {
			continue
		}
		seenReferences[key] = struct{}{}
		cleanReferences = append(cleanReferences, id)
	}
	if len(cleanReferences) > 0 {
		value := strings.Join(cleanReferences, " ")
		if len(value) > 998 {
			return nil, fmt.Errorf("AgentMail References header exceeds size limit")
		}
		headers["References"] = value
	}
	if len(headers) == 0 {
		return nil, nil
	}
	return headers, nil
}

func (a *agentMailAdapter) accountID() string { return a.cfg.Source + ":" + a.cfg.ID }
func agentMailMessage(cfg TransportConfig, raw agentmail.Message) Message {
	from := raw.From.Email
	if from == "" {
		from = raw.FromAlt.Email
	}
	tos := make([]string, 0, len(raw.To))
	for _, x := range raw.To {
		tos = append(tos, x.Email)
	}
	cc := make([]string, 0, len(raw.CC))
	for _, x := range raw.CC {
		cc = append(cc, x.Email)
	}
	replyTo := make([]string, 0, len(raw.ReplyTo))
	for _, x := range raw.ReplyTo {
		if x.Email != "" {
			replyTo = append(replyTo, x.Email)
		}
	}
	bcc := make([]string, 0, len(raw.BCC))
	for _, x := range raw.BCC {
		bcc = append(bcc, x.Email)
	}
	date, _ := time.Parse(time.RFC3339, raw.Timestamp)
	if date.IsZero() {
		date, _ = mail.ParseDate(raw.Timestamp)
	}
	att := make([]Attachment, 0, len(raw.Attachments))
	for _, x := range raw.Attachments {
		att = append(att, Attachment{ID: x.ID, Filename: safeFilename(x.Filename), ContentType: x.ContentType, Size: x.Size})
	}
	hasUnread := false
	hasRead := false
	hasReceived := false
	starred := false
	for _, l := range raw.Labels {
		switch strings.ToLower(l) {
		case "unread":
			hasUnread = true
		case "read":
			hasRead = true
		case "received":
			hasReceived = true
		case "starred", "\\flagged":
			starred = true
		}
	}
	// AgentMail's received-event example includes both labels, but unread state
	// is label-based and records may omit the unread label. Treat received as
	// unread unless an explicit read label takes precedence over a conflict.
	unread := !hasRead && (hasUnread || hasReceived)
	messageID, _ := safeMessageID(agentMailHeader(raw.Headers, "Message-ID"))
	inReplyIDs := parseAgentMailReferences(agentMailHeader(raw.Headers, "In-Reply-To"))
	if len(inReplyIDs) == 0 {
		inReplyIDs = parseAgentMailReferences(raw.InReplyTo)
	}
	inReplyTo := ""
	if len(inReplyIDs) > 0 {
		inReplyTo = inReplyIDs[len(inReplyIDs)-1]
	}
	references := parseAgentMailReferences(agentMailHeader(raw.Headers, "References"))
	if len(references) == 0 {
		references = parseAgentMailReferences(strings.Join(raw.References, " "))
	}
	htmlBody := ""
	if raw.HTML != "" {
		htmlBody, _ = SanitizeHTML(raw.HTML)
	}
	return Message{AccountID: cfg.Source + ":" + cfg.ID, FolderID: agentMailFolderID(cfg.Source+":"+cfg.ID, "inbox"), RemoteID: raw.ID, ThreadID: raw.ThreadID, MessageID: messageID, InReplyTo: inReplyTo, References: references, Subject: raw.Subject, From: from, ReplyTo: replyTo, To: tos, CC: cc, BCC: bcc, Date: date, Text: raw.Text, HTML: htmlBody, Unread: unread, Starred: starred, Labels: append([]string(nil), raw.Labels...), Attachments: att}
}

func agentMailHeader(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(strings.TrimSpace(key), name) {
			return value
		}
	}
	return ""
}

func parseAgentMailReferences(raw string) []string {
	fields := strings.Fields(raw)
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if id, err := safeMessageID(field); err == nil {
			out = append(out, id)
			if len(out) == 100 {
				break
			}
		}
	}
	return out
}
func agentMailFolderID(account, label string) string {
	return account + ":label:" + base64.RawURLEncoding.EncodeToString([]byte(label))
}
func agentMailLabel(account, id string) string {
	prefix := account + ":label:"
	if !strings.HasPrefix(id, prefix) {
		return id
	}
	b, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, prefix))
	if e != nil {
		return ""
	}
	return string(b)
}
func validMailLabel(s string) bool {
	return strings.TrimSpace(s) != "" && len(s) <= 128 && !strings.ContainsAny(s, "\r\n\x00")
}
func validatedRecipients(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		a, e := mail.ParseAddress(strings.TrimSpace(raw))
		if e != nil || strings.ContainsAny(a.Address, "\r\n\x00") {
			return nil, fmt.Errorf("invalid recipient")
		}
		out = append(out, a.Address)
	}
	return out, nil
}
