package mailer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/mail"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
)

const maxIMAPMessageBytes = 16 << 20

type imapAdapter struct {
	cfg     TransportConfig
	mu      sync.Mutex
	session *imapSyncSession
}

type imapSyncSession struct {
	client *imapclient.Client
	conn   net.Conn
	stop   func()
}

type imapCursor struct {
	Validity uint32 `json:"validity"`
	LastUID  uint32 `json:"last_uid"`
}

func encodeIMAPCursor(validity, lastUID uint32) string {
	data, err := json.Marshal(imapCursor{Validity: validity, LastUID: lastUID})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func (a *imapAdapter) currentSyncSession() *imapSyncSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.session
}

// StartSyncSession dials and logs in once. Later withClient calls in this
// adapter reuse that connection until stop logs out and closes it.
func (a *imapAdapter) StartSyncSession(ctx context.Context) (func(), error) {
	c, conn, stop, err := dialIMAP(ctx, a.cfg)
	if err != nil {
		return nil, fmt.Errorf("IMAP connection failed: %w", err)
	}
	c.Timeout = 30 * time.Second
	a.mu.Lock()
	if a.session != nil {
		a.mu.Unlock()
		_ = c.Logout()
		stop()
		_ = conn.Close()
		return nil, fmt.Errorf("IMAP sync session already open")
	}
	a.session = &imapSyncSession{client: c, conn: conn, stop: stop}
	a.mu.Unlock()
	return func() { closeIMAPSession(a.takeSyncSession(), true) }, nil
}

func (a *imapAdapter) takeSyncSession() *imapSyncSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	sess := a.session
	a.session = nil
	return sess
}

func (a *imapAdapter) dropSyncSession() {
	closeIMAPSession(a.takeSyncSession(), false)
}

func closeIMAPSession(sess *imapSyncSession, logout bool) {
	if sess == nil {
		return
	}
	if logout && sess.client != nil {
		_ = sess.client.Logout()
	}
	if sess.stop != nil {
		sess.stop()
	}
	if sess.conn != nil {
		_ = sess.conn.Close()
	}
}

func (a *imapAdapter) withClient(ctx context.Context, fn func(*imapclient.Client) error) error {
	if sess := a.currentSyncSession(); sess != nil {
		if err := sess.conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
			a.dropSyncSession()
			return err
		}
		sess.client.Timeout = 30 * time.Second
		err := fn(sess.client)
		if err != nil || ctx.Err() != nil {
			// A failed command can leave the shared connection unusable.
			// Drop it so the next folder dials a new one.
			a.dropSyncSession()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		return nil
	}
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
		var cur imapCursor
		if cursor != "" {
			if b, decodeErr := base64.RawURLEncoding.DecodeString(cursor); decodeErr == nil {
				_ = json.Unmarshal(b, &cur)
			}
		}
		if cur.Validity != 0 && cur.Validity != status.UidValidity {
			cur = imapCursor{}
			result.ResetSnapshot = true
		}
		// The highest UID has no successor. Skip the search and keep the cursor.
		if cur.LastUID == ^uint32(0) && !result.ResetSnapshot {
			result.NextCursor = encodeIMAPCursor(status.UidValidity, cur.LastUID)
			result.RemoteSnapshotComplete = true
			return nil
		}
		criteria := imap.NewSearchCriteria()
		// The first snapshot and a UIDVALIDITY reset still search every UID.
		// Later pages and steady state ask only for UIDs above the cursor.
		if cur.LastUID > 0 && !result.ResetSnapshot {
			criteria.Uid = new(imap.SeqSet)
			criteria.Uid.AddRange(cur.LastUID+1, 0)
		}
		uids, e := c.UidSearch(criteria)
		if e != nil {
			return e
		}
		sort.Slice(uids, func(i, j int) bool { return uids[i] < uids[j] })
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
		last := cur.LastUID
		if len(batch) > 0 {
			last = batch[len(batch)-1]
		}
		result.NextCursor = encodeIMAPCursor(status.UidValidity, last)
		if len(newUIDs) > len(batch) {
			result.HasMore = true
		} else {
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
	pageLimit, cursorUID, exhausted, err := imapSearchPageLimit(query.Cursor)
	if err != nil {
		return MessagePage{}, err
	}
	if exhausted {
		page.Limit = limit
		return page, nil
	}
	err = a.withClient(ctx, func(c *imapclient.Client) error {
		folder := a.providerFolder(folderID)
		status, err := c.Select(folder, true)
		if err != nil {
			return err
		}
		criteria := imap.NewSearchCriteria()
		criteria.Text = []string{text}
		criteria.Uid = pageLimit
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
		filtered := make([]uint32, 0, len(uids))
		for _, uid := range uids {
			if cursorUID == 0 || uint64(uid) < uint64(cursorUID) {
				filtered = append(filtered, uid)
			}
		}
		if len(filtered) > limit {
			filtered = filtered[:limit]
			page.NextCursor = strconv.FormatUint(uint64(filtered[len(filtered)-1]), 10)
		}
		messages, err := a.fetchMetadataUIDs(c, folder, filtered, status.UidValidity)
		if err != nil {
			return err
		}
		page.Messages = messages
		page.Limit = limit
		return nil
	})
	return page, err
}

// imapSearchPageLimit restricts a later page to UIDs below the cursor. The
// cursor is the smallest UID already returned. An empty cursor searches the
// whole folder. Cursor 1 has no smaller UID.
func imapSearchPageLimit(cursor string) (*imap.SeqSet, uint32, bool, error) {
	if cursor == "" {
		return nil, 0, false, nil
	}
	parsed, err := strconv.ParseUint(cursor, 10, 32)
	if err != nil || parsed == 0 {
		return nil, 0, false, fmt.Errorf("invalid IMAP search cursor")
	}
	cursorUID := uint32(parsed)
	if cursorUID == 1 {
		return nil, cursorUID, true, nil
	}
	set := new(imap.SeqSet)
	set.AddRange(1, cursorUID-1)
	return set, cursorUID, false, nil
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
	snippet := &imap.BodySectionName{Peek: true, BodyPartName: imap.BodyPartName{Specifier: imap.TextSpecifier}, Partial: []int{0, 2048}}
	ch := make(chan *imap.Message, len(uids))
	err := c.UidFetch(set, []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, imap.FetchBodyStructure, headers.FetchItem(), snippet.FetchItem()}, ch)
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
		if preview, e := readIMAPBody(im.GetBody(snippet), 2048); e == nil {
			m.Snippet = imapListSnippet(preview)
		}
		m.AttachmentCount = imapAttachmentCount(im.BodyStructure)
		out = append(out, m)
	}
	return out, nil
}
func imapAttachmentCount(part *imap.BodyStructure) int {
	if part == nil {
		return 0
	}
	if len(part.Parts) > 0 {
		total := 0
		for _, child := range part.Parts {
			total += imapAttachmentCount(child)
		}
		return total
	}
	if part.BodyStructure != nil {
		return imapAttachmentCount(part.BodyStructure)
	}
	if strings.EqualFold(part.MIMEType, "text") {
		return 0
	}
	if strings.EqualFold(part.MIMEType, "image") && strings.EqualFold(part.Disposition, "inline") {
		return 0
	}
	return 1
}

func imapListSnippet(raw []byte) string {
	text := string(raw)
	if index := strings.Index(text, "\r\n\r\n"); index >= 0 && imapSnippetHeader(text[:index]) {
		text = text[index+4:]
	} else if index := strings.Index(text, "\n\n"); index >= 0 && imapSnippetHeader(text[:index]) {
		text = text[index+2:]
	}
	text = stripIMAPSnippetTags(text)
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > 180 {
		return string(runes[:180])
	}
	return text
}

func stripIMAPSnippetTags(text string) string {
	runes := []rune(text)
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(runes); {
		if runes[i] == '<' {
			if end := imapSnippetTagEnd(runes, i); end > i {
				b.WriteByte(' ')
				i = end + 1
				continue
			}
		}
		if runes[i] < 32 {
			b.WriteByte(' ')
		} else {
			b.WriteRune(runes[i])
		}
		i++
	}
	return b.String()
}

func imapSnippetTagEnd(runes []rune, start int) int {
	if start+4 <= len(runes) && string(runes[start:start+4]) == "<!--" {
		for j := start + 4; j+2 < len(runes); j++ {
			if runes[j] == '-' && runes[j+1] == '-' && runes[j+2] == '>' {
				return j + 2
			}
		}
		return -1
	}
	i := start + 1
	if i < len(runes) && (runes[i] == '/' || runes[i] == '!') {
		i++
	}
	if i >= len(runes) || !asciiLetter(runes[i]) {
		return -1
	}
	i++
	for i < len(runes) && (asciiLetter(runes[i]) || asciiDigit(runes[i])) {
		i++
	}
	if i >= len(runes) {
		return -1
	}
	switch runes[i] {
	case '>', ' ', '\t', '\r', '\n', '/':
	default:
		return -1
	}
	for j := i; j < len(runes); j++ {
		if runes[j] == '>' {
			return j
		}
	}
	return -1
}

func asciiLetter(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }
func asciiDigit(r rune) bool  { return r >= '0' && r <= '9' }

func imapSnippetHeader(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "content-type:") || strings.Contains(lower, "content-transfer-encoding:") || strings.Contains(lower, "mime-version:")
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
	subject, from, to, cc, body, html, attachments, htmlSkipped, err := ParseMIME(raw, maxIMAPMessageBytes)
	if err != nil {
		return Message{}, err
	}
	id := fmt.Sprint(uid)
	m := Message{AccountID: a.accountID(), FolderID: imapFolderID(a.cfg, folder), RemoteID: id, ThreadID: id, Subject: subject, From: from, To: to, CC: cc, Text: body, HTML: html, HTMLSkipped: htmlSkipped, Date: received, Attachments: attachments, Raw: append([]byte(nil), raw...)}
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
	_, _, _, _, _, _, as, _, e := ParseMIME(raw, maxIMAPMessageBytes)
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
