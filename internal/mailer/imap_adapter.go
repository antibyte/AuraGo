package mailer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/mail"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
)

const maxIMAPMessageBytes = 16 << 20

type imapAdapter struct{ cfg TransportConfig }
type imapCursor struct {
	Validity uint32 `json:"validity"`
	LastUID  uint32 `json:"last_uid"`
}

func (a *imapAdapter) withClient(ctx context.Context, fn func(*imapclient.Client) error) error {
	c, conn, stop, err := dialIMAP(ctx, a.cfg)
	if err != nil {
		return fmt.Errorf("IMAP connection failed: %w", err)
	}
	c.Timeout = 30 * time.Second
	defer stop()
	defer conn.Close()
	err = fn(c)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == nil {
		_ = c.Logout()
	}
	return err
}
func (a *imapAdapter) Test(ctx context.Context) error {
	return a.withClient(ctx, func(c *imapclient.Client) error { return c.Noop() })
}
func (a *imapAdapter) Folders(ctx context.Context) ([]Folder, error) {
	out := []Folder{}
	err := a.withClient(ctx, func(c *imapclient.Client) error {
		ch := make(chan *imap.MailboxInfo, 256)
		done := make(chan error, 1)
		go func() { done <- c.List("", "*", ch) }()
		for info := range ch {
			name := info.Name
			kind := imapFolderKind(name)
			out = append(out, Folder{ID: imapFolderID(a.cfg, name), AccountID: a.accountID(), ProviderID: name, Name: name, Kind: kind, ReadOnly: a.cfg.ReadOnly})
		}
		if err := <-done; err != nil {
			return err
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, err
}
func (a *imapAdapter) Sync(ctx context.Context, folderID, cursor string) (SyncResult, error) {
	result := SyncResult{Messages: []Message{}}
	err := a.withClient(ctx, func(c *imapclient.Client) error {
		folder := a.providerFolder(folderID)
		status, e := c.Select(folder, true)
		if e != nil {
			return e
		}
		result.UIDValidity = status.UidValidity
		uids, e := c.UidSearch(imap.NewSearchCriteria())
		if e != nil {
			return e
		}
		sort.Slice(uids, func(i, j int) bool { return uids[i] < uids[j] })
		var cur imapCursor
		if cursor != "" {
			if b, e := base64.RawURLEncoding.DecodeString(cursor); e == nil {
				_ = json.Unmarshal(b, &cur)
			}
		}
		if cur.Validity != 0 && cur.Validity != status.UidValidity {
			cur = imapCursor{}
			result.ResetSnapshot = true
		}
		newUIDs := make([]uint32, 0, len(uids))
		for _, uid := range uids {
			if uid > cur.LastUID {
				newUIDs = append(newUIDs, uid)
			}
		}
		batch := newUIDs
		if len(batch) > 100 {
			batch = batch[:100]
		}
		messages, e := a.fetchMetadataUIDs(c, folder, batch, status.UidValidity)
		if e != nil {
			return e
		}
		result.Messages = messages
		if len(newUIDs) > len(batch) {
			last := batch[len(batch)-1]
			data, _ := json.Marshal(imapCursor{Validity: status.UidValidity, LastUID: last})
			result.NextCursor = base64.RawURLEncoding.EncodeToString(data)
			result.HasMore = true
		} else {
			result.NextCursor = ""
			result.RemoteSnapshotComplete = true
		}
		return nil
	})
	return result, err
}

// Search performs a provider-side IMAP text search, then fetches only bounded
// envelope/flag/thread metadata. Message bodies remain available on demand via
// Read and Raw, which both use BODY.PEEK.
func (a *imapAdapter) Search(ctx context.Context, folderID string, query MessageQuery) (MessagePage, error) {
	text := strings.TrimSpace(query.Query)
	if text == "" || len(text) > 2048 {
		return MessagePage{}, fmt.Errorf("invalid IMAP search query")
	}
	limit := query.Limit
	if limit < 1 || limit > 100 {
		limit = 100
	}
	var page MessagePage
	err := a.withClient(ctx, func(c *imapclient.Client) error {
		folder := a.providerFolder(folderID)
		status, err := c.Select(folder, true)
		if err != nil {
			return err
		}
		criteria := imap.NewSearchCriteria()
		criteria.Text = []string{text}
		if query.Unread {
			criteria.WithoutFlags = append(criteria.WithoutFlags, imap.SeenFlag)
		}
		if query.Starred {
			criteria.WithFlags = append(criteria.WithFlags, imap.FlaggedFlag)
		}
		uids, err := c.UidSearch(criteria)
		if err != nil {
			return err
		}
		sort.Slice(uids, func(i, j int) bool { return uids[i] > uids[j] })
		page.Total = len(uids)
		if len(uids) > limit {
			uids = uids[:limit]
		}
		messages, err := a.fetchMetadataUIDs(c, folder, uids, status.UidValidity)
		if err != nil {
			return err
		}
		page.Messages = messages
		page.Limit = limit
		return nil
	})
	return page, err
}
func (a *imapAdapter) fetchUIDs(c *imapclient.Client, folder string, uids []uint32) ([]Message, error) {
	if len(uids) == 0 {
		return []Message{}, nil
	}
	set := new(imap.SeqSet)
	for _, uid := range uids {
		set.AddNum(uid)
	}
	peek := &imap.BodySectionName{Peek: true}
	ch := make(chan *imap.Message, len(uids))
	err := c.UidFetch(set, []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, peek.FetchItem()}, ch)
	if err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(uids))
	for im := range ch {
		raw, e := readIMAPBody(im.GetBody(peek), maxIMAPMessageBytes)
		if e != nil {
			return nil, e
		}
		msg, e := a.messageFromRaw(raw, im.Uid, folder, im.Flags, im.InternalDate)
		if e != nil {
			return nil, e
		}
		out = append(out, msg)
	}
	return out, nil
}
func (a *imapAdapter) fetchMetadataUIDs(c *imapclient.Client, folder string, uids []uint32, validity uint32) ([]Message, error) {
	if len(uids) == 0 {
		return []Message{}, nil
	}
	set := new(imap.SeqSet)
	for _, uid := range uids {
		set.AddNum(uid)
	}
	headers := &imap.BodySectionName{Peek: true, BodyPartName: imap.BodyPartName{Specifier: imap.HeaderSpecifier, Fields: []string{"Reply-To", "Message-ID", "In-Reply-To", "References"}}}
	ch := make(chan *imap.Message, len(uids))
	err := c.UidFetch(set, []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, headers.FetchItem()}, ch)
	if err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(uids))
	for im := range ch {
		remoteID := imapRemoteID(validity, im.Uid)
		m := Message{AccountID: a.accountID(), FolderID: imapFolderID(a.cfg, folder), RemoteID: remoteID, ThreadID: remoteID, Date: im.InternalDate, Unread: true}
		for _, flag := range im.Flags {
			switch strings.ToLower(flag) {
			case "\\seen":
				m.Unread = false
			case "\\flagged":
				m.Starred = true
			default:
				m.Labels = append(m.Labels, flag)
			}
		}
		if env := im.Envelope; env != nil {
			m.Subject = env.Subject
			m.Date = env.Date
			if m.Date.IsZero() {
				m.Date = im.InternalDate
			}
			m.MessageID = env.MessageId
			m.InReplyTo = env.InReplyTo
			m.From = imapAddressList(env.From)
			m.ReplyTo = imapAddressStrings(env.ReplyTo)
			m.To = imapAddressStrings(env.To)
			m.CC = imapAddressStrings(env.Cc)
			m.BCC = imapAddressStrings(env.Bcc)
		}
		if raw, e := readIMAPBody(im.GetBody(headers), 64<<10); e == nil && len(raw) > 0 {
			m.ReplyTo = ParseReplyTo(raw)
			if messageID, inReplyTo, refs, parseErr := ParseThreadHeaders(raw); parseErr == nil {
				if m.MessageID == "" {
					m.MessageID = messageID
				}
				m.InReplyTo = inReplyTo
				m.References = refs
			}
		}
		out = append(out, m)
	}
	return out, nil
}
func imapAddressList(values []*imap.Address) string {
	if len(values) == 0 {
		return ""
	}
	a := values[0]
	addr := a.Address()
	if a.PersonalName != "" {
		return (&mail.Address{Name: a.PersonalName, Address: addr}).String()
	}
	return addr
}
func imapAddressStrings(values []*imap.Address) []string {
	out := make([]string, 0, len(values))
	for _, a := range values {
		if a == nil {
			continue
		}
		addr := a.Address()
		if a.PersonalName != "" {
			addr = (&mail.Address{Name: a.PersonalName, Address: addr}).String()
		}
		out = append(out, addr)
	}
	return out
}
func (a *imapAdapter) messageFromRaw(raw []byte, uid uint32, folder string, flags []string, received time.Time) (Message, error) {
	subject, from, to, cc, body, html, attachments, err := ParseMIME(raw, maxIMAPMessageBytes)
	if err != nil {
		return Message{}, err
	}
	id := fmt.Sprint(uid)
	m := Message{AccountID: a.accountID(), FolderID: imapFolderID(a.cfg, folder), RemoteID: id, ThreadID: id, Subject: subject, From: from, To: to, CC: cc, Text: body, HTML: html, Date: received, Attachments: attachments, Raw: append([]byte(nil), raw...)}
	for _, f := range flags {
		switch strings.ToLower(f) {
		case "\\seen":
			m.Unread = false
		case "\\flagged":
			m.Starred = true
		default:
			m.Labels = append(m.Labels, f)
		}
	}
	// IMAP FETCH FLAGS absent \\Seen means unread.
	m.Unread = true
	for _, f := range flags {
		if strings.EqualFold(f, imap.SeenFlag) {
			m.Unread = false
		}
	}
	if mid, irt, refs, e := ParseThreadHeaders(raw); e == nil {
		m.MessageID = mid
		m.InReplyTo = irt
		m.References = refs
	}
	m.ReplyTo = ParseReplyTo(raw)
	if m.Date.IsZero() {
		m.Date = time.Now()
	}
	return m, nil
}
func readIMAPBody(r io.Reader, limit int64) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("IMAP body missing")
	}
	data, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("IMAP message exceeds size limit")
	}
	return data, nil
}
func (a *imapAdapter) Read(ctx context.Context, folder, id string) (Message, error) {
	var out Message
	err := a.withClient(ctx, func(c *imapclient.Client) error {
		f := a.providerFolder(folder)
		status, e := c.Select(f, true)
		if e != nil {
			return e
		}
		uid, e := parseIMAPRemoteID(id, status.UidValidity)
		if e != nil {
			return fmt.Errorf("invalid IMAP message id")
		}
		msgs, e := a.fetchUIDs(c, f, []uint32{uid})
		if e != nil {
			return e
		}
		if len(msgs) == 0 {
			return fmt.Errorf("message not found")
		}
		out = msgs[0]
		out.RemoteID = imapRemoteID(status.UidValidity, uid)
		out.ThreadID = out.RemoteID
		return nil
	})
	return out, err
}
func imapRemoteID(validity, uid uint32) string {
	return strconv.FormatUint(uint64(validity), 10) + ":" + strconv.FormatUint(uint64(uid), 10)
}

func parseIMAPRemoteID(raw string, currentValidity uint32) (uint32, error) {
	uidText := raw
	if validityText, value, ok := strings.Cut(raw, ":"); ok {
		validity, err := strconv.ParseUint(validityText, 10, 32)
		if err != nil || uint32(validity) != currentValidity {
			return 0, fmt.Errorf("stale IMAP UID validity")
		}
		uidText = value
	}
	uid, err := strconv.ParseUint(uidText, 10, 32)
	if err != nil || uid == 0 {
		return 0, fmt.Errorf("invalid IMAP UID")
	}
	return uint32(uid), nil
}

func (a *imapAdapter) Raw(ctx context.Context, folder, id string) ([]byte, error) {
	var out []byte
	err := a.withClient(ctx, func(c *imapclient.Client) error {
		f := a.providerFolder(folder)
		status, e := c.Select(f, true)
		if e != nil {
			return e
		}
		uid, e := parseIMAPRemoteID(id, status.UidValidity)
		if e != nil {
			return fmt.Errorf("invalid IMAP message id")
		}
		set := new(imap.SeqSet)
		set.AddNum(uid)
		peek := &imap.BodySectionName{Peek: true}
		ch := make(chan *imap.Message, 1)
		if e = c.UidFetch(set, []imap.FetchItem{peek.FetchItem()}, ch); e != nil {
			return e
		}
		for im := range ch {
			out, e = readIMAPBody(im.GetBody(peek), maxIMAPMessageBytes)
			if e != nil {
				return e
			}
		}
		if out == nil {
			return fmt.Errorf("message not found")
		}
		return nil
	})
	return out, err
}
func (a *imapAdapter) Attachment(ctx context.Context, folder, id, part string) (io.ReadCloser, error) {
	raw, e := a.Raw(ctx, folder, id)
	if e != nil {
		return nil, e
	}
	_, _, _, _, _, _, as, e := ParseMIME(raw, maxIMAPMessageBytes)
	if e != nil {
		return nil, e
	}
	for _, x := range as {
		if x.ID == part {
			return io.NopCloser(bytes.NewReader(x.Data)), nil
		}
	}
	return nil, fmt.Errorf("attachment not found")
}
func (a *imapAdapter) Mutate(ctx context.Context, folder, id string, mu Mutation) error {
	return a.withClient(ctx, func(c *imapclient.Client) error {
		f := a.providerFolder(folder)
		status, e := c.Select(f, false)
		if e != nil {
			return e
		}
		uid, e := parseIMAPRemoteID(id, status.UidValidity)
		if e != nil {
			return fmt.Errorf("invalid IMAP message id")
		}
		set := new(imap.SeqSet)
		set.AddNum(uid)
		switch mu.Kind {
		case MutationRead, MutationUnread, MutationStar, MutationUnstar:
			flag := imap.SeenFlag
			if mu.Kind == MutationStar || mu.Kind == MutationUnstar {
				flag = imap.FlaggedFlag
			}
			op := imap.FlagsOp(imap.AddFlags)
			if mu.Kind == MutationUnread || mu.Kind == MutationUnstar {
				op = imap.FlagsOp(imap.RemoveFlags)
			}
			return c.UidStore(set, imap.FormatFlagsOp(op, true), []interface{}{flag}, nil)
		case MutationMove, MutationArchive, MutationTrash, MutationRestore, MutationJunk:
			dest := mu.DestinationFolderID
			if dest == "" {
				switch mu.Kind {
				case MutationArchive:
					dest = "Archive"
				case MutationTrash:
					dest = "Trash"
				case MutationRestore:
					dest = "INBOX"
				case MutationJunk:
					dest = "Junk"
				}
			}
			if strings.HasPrefix(dest, a.accountID()+":folder:") {
				dest = a.providerFolder(dest)
			}
			if dest == "" {
				return fmt.Errorf("destination folder is required")
			}
			return c.UidMove(set, dest)
		case MutationDelete:
			if e := c.UidStore(set, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil); e != nil {
				return e
			}
			return nil
		case MutationLabelAdd, MutationLabelRemove:
			if !validMailLabel(mu.Label) || strings.HasPrefix(mu.Label, "\\") || strings.Contains(mu.Label, ":") {
				return fmt.Errorf("invalid IMAP keyword")
			}
			op := imap.FlagsOp(imap.AddFlags)
			if mu.Kind == MutationLabelRemove {
				op = imap.FlagsOp(imap.RemoveFlags)
			}
			return c.UidStore(set, imap.FormatFlagsOp(op, true), []interface{}{mu.Label}, nil)
		default:
			return fmt.Errorf("unsupported IMAP mutation")
		}
	})
}
func (a *imapAdapter) AppendSent(ctx context.Context, out Outgoing, result SendResult) error {
	return a.AppendSentTo(ctx, out, result, Folder{ProviderID: "Sent"})
}

func (a *imapAdapter) AppendSentTo(ctx context.Context, out Outgoing, result SendResult, folder Folder) error {
	var raw []byte
	if result.Message != nil && len(result.Message.Raw) > 0 {
		raw = result.Message.Raw
	} else {
		var e error
		raw, e = BuildMIME(a.cfg.FromAddress, out.Draft, time.Now())
		if e != nil {
			return e
		}
	}
	name := folder.ProviderID
	if name == "" {
		name = a.providerFolder(folder.ID)
	}
	if name == "" || strings.ContainsAny(name, "\r\n\x00") {
		return fmt.Errorf("invalid sent folder")
	}
	return a.withClient(ctx, func(c *imapclient.Client) error {
		return c.Append(name, []string{}, time.Now(), strings.NewReader(string(raw)))
	})
}
func (a *imapAdapter) accountID() string { return a.cfg.Source + ":" + a.cfg.ID }
func imapFolderID(cfg TransportConfig, name string) string {
	return cfg.Source + ":" + cfg.ID + ":folder:" + base64.RawURLEncoding.EncodeToString([]byte(name))
}
func (a *imapAdapter) providerFolder(id string) string {
	prefix := a.accountID() + ":folder:"
	if strings.HasPrefix(id, prefix) {
		b, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, prefix))
		if e == nil {
			return string(b)
		}
	}
	if id == "" {
		return "INBOX"
	}
	return id
}
func imapFolderKind(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "inbox":
		return "inbox"
	case "sent", "sent items", "sent mail":
		return "sent"
	case "archive", "all mail":
		return "archive"
	case "trash", "deleted items", "deleted messages":
		return "trash"
	case "junk", "spam":
		return "junk"
	case "drafts":
		return "drafts"
	default:
		return "user"
	}
}

func (a *imapAdapter) CreateFolder(ctx context.Context, name string) (Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "\r\n\x00") {
		return Folder{}, fmt.Errorf("invalid folder name")
	}
	err := a.withClient(ctx, func(c *imapclient.Client) error { return c.Create(name) })
	if err != nil {
		return Folder{}, err
	}
	return Folder{ID: imapFolderID(a.cfg, name), AccountID: a.accountID(), ProviderID: name, Name: name, Kind: "user", ReadOnly: a.cfg.ReadOnly}, nil
}
func (a *imapAdapter) RenameFolder(ctx context.Context, oldID, newName string) error {
	old := a.providerFolder(oldID)
	newName = strings.TrimSpace(newName)
	if strings.ContainsAny(newName, "\r\n\x00") || newName == "" {
		return fmt.Errorf("invalid folder name")
	}
	return a.withClient(ctx, func(c *imapclient.Client) error { return c.Rename(old, newName) })
}
func (a *imapAdapter) DeleteFolder(ctx context.Context, id string) error {
	folder := a.providerFolder(id)
	if strings.EqualFold(folder, "INBOX") {
		return fmt.Errorf("cannot delete Inbox")
	}
	return a.withClient(ctx, func(c *imapclient.Client) error { return c.Delete(folder) })
}
