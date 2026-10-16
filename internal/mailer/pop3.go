package mailer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

const maxPOP3MessageBytes = 16 << 20
const maxPOP3SyncBytes = 32 << 20
const maxPOP3IDScanBytes = 64 << 20
const maxPOP3Messages = 20000
const maxPOP3FallbackHashes = 4096

type pop3Adapter struct{ cfg TransportConfig }

type pop3SyncCursor struct {
	Version        int            `json:"v"`
	Next           int            `json:"next"`
	Num            int            `json:"num"`
	FallbackCounts map[string]int `json:"fallback_counts,omitempty"`
}

func (a *pop3Adapter) connect(ctx context.Context) (*textproto.Conn, net.Conn, func(), error) {
	if strings.ContainsAny(a.cfg.Username, "\r\n\x00") || strings.ContainsAny(a.cfg.Password, "\r\n\x00") {
		return nil, nil, func() {}, fmt.Errorf("invalid POP3 credentials")
	}
	port := a.cfg.POP3Port
	if port <= 0 {
		if strings.EqualFold(a.cfg.ReceiveTLS, "starttls") {
			port = 110
		} else {
			port = 995
		}
	}
	host := strings.TrimSpace(a.cfg.POP3Host)
	if host == "" {
		host = a.cfg.IMAPHost
	}
	conn, err := dialTransport(ctx, a.cfg, "tcp", net.JoinHostPort(host, strconv.Itoa(port)), 10*time.Second)
	if err != nil {
		return nil, nil, func() {}, err
	}
	cancelConn := conn
	afterCancel := context.AfterFunc(ctx, func() { _ = cancelConn.Close() })
	stop := func() { afterCancel() }
	fail := func(err error) (*textproto.Conn, net.Conn, func(), error) {
		stop()
		_ = conn.Close()
		return nil, nil, func() {}, err
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if a.cfg.ReceiveTLS == "" || strings.EqualFold(a.cfg.ReceiveTLS, "tls") {
		tc := tls.Client(conn, transportTLSConfig(a.cfg, host))
		if err = tc.HandshakeContext(ctx); err != nil {
			return fail(err)
		}
		conn = tc
	}
	tc := textproto.NewConn(conn)
	greeting, err := tc.ReadLine()
	if err != nil || !strings.HasPrefix(greeting, "+OK") {
		return fail(fmt.Errorf("POP3 greeting rejected"))
	}
	if a.cfg.ReceiveTLS == "starttls" {
		if err = popExpectOK(tc, "STLS"); err != nil {
			return fail(fmt.Errorf("POP3 STLS unavailable: %w", err))
		}
		if tc.R.Buffered() != 0 {
			return fail(fmt.Errorf("unexpected POP3 data before TLS upgrade"))
		}
		tlsConn := tls.Client(conn, transportTLSConfig(a.cfg, host))
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			return fail(err)
		}
		conn = tlsConn
		tc = textproto.NewConn(conn)
	} else if !strings.EqualFold(a.cfg.ReceiveTLS, "tls") {
		return fail(fmt.Errorf("POP3 requires TLS or STLS"))
	}
	if a.cfg.OAuthSource != "" || strings.EqualFold(a.cfg.SMTPAuthMethod, "xoauth2") {
		if a.cfg.Token == nil {
			return fail(fmt.Errorf("POP3 OAuth token source unavailable"))
		}
		token, tokenErr := a.cfg.Token(ctx, "read", false)
		if tokenErr != nil {
			return fail(tokenErr)
		}
		initial := base64.StdEncoding.EncodeToString([]byte("user=" + a.cfg.Username + "\x01auth=Bearer " + token + "\x01\x01"))
		if err = popExpectOK(tc, "AUTH XOAUTH2 "+initial); err != nil {
			return fail(fmt.Errorf("POP3 OAuth authentication failed"))
		}
	} else if err = popExpectOK(tc, "USER "+a.cfg.Username); err == nil {
		err = popExpectOK(tc, "PASS "+a.cfg.Password)
	}
	if err != nil {
		return fail(fmt.Errorf("POP3 authentication failed"))
	}
	return tc, conn, stop, nil
}

func popExpectOK(c *textproto.Conn, cmd string) error {
	id, err := c.Cmd("%s", cmd)
	if err != nil {
		return err
	}
	c.StartResponse(id)
	line, e := c.ReadLine()
	c.EndResponse(id)
	if e != nil {
		return e
	}
	if !strings.HasPrefix(line, "+OK") {
		return fmt.Errorf("server rejected command")
	}
	return nil
}

func (a *pop3Adapter) withConn(ctx context.Context, fn func(*textproto.Conn) error) error {
	c, _, stop, err := a.connect(ctx)
	if err != nil {
		return err
	}
	defer stop()
	defer c.Close()
	err = fn(c)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
func (a *pop3Adapter) Test(ctx context.Context) error {
	return a.withConn(ctx, func(c *textproto.Conn) error { return nil })
}
func (a *pop3Adapter) TestSend(ctx context.Context) error { return TestSMTPAuth(ctx, a.cfg) }
func (a *pop3Adapter) Folders(context.Context) ([]Folder, error) {
	return []Folder{{ID: a.cfg.Source + ":" + a.cfg.ID + ":inbox", AccountID: a.cfg.Source + ":" + a.cfg.ID, ProviderID: "INBOX", Name: "Inbox", Kind: "inbox", ReadOnly: true}}, nil
}
func (a *pop3Adapter) Sync(ctx context.Context, folder, cursor string) (SyncResult, error) {
	if folder != "" && !strings.HasSuffix(strings.ToLower(folder), ":inbox") {
		return SyncResult{}, fmt.Errorf("POP3 supports Inbox only")
	}
	result := SyncResult{Messages: []Message{}}
	err := a.withConn(ctx, func(c *textproto.Conn) error {
		// STAT is a single-line response. Use UIDL and LIST snapshots; when UIDL
		// is unavailable, message content hashes provide stable duplicate keys.
		num, size, err := popStat(c)
		if err != nil {
			return err
		}
		if num == 0 {
			result.RemoteSnapshotComplete = true
			return nil
		}
		if num > maxPOP3Messages {
			return fmt.Errorf("POP3 mailbox has too many messages to scan safely")
		}
		uids, _, err := popUIDLs(c, num)
		if err != nil {
			return err
		}
		start := 0
		fallbackCounts := make(map[string]int)
		legacyCursor := false
		if cursor != "" {
			state, legacy, decodeErr := decodePOPSyncCursor(cursor)
			if decodeErr != nil {
				return decodeErr
			}
			legacyCursor = legacy
			if state.Num == 0 || state.Num == num {
				start = state.Next
				if state.FallbackCounts != nil {
					fallbackCounts = state.FallbackCounts
				}
			} // A changed mailbox restarts the bounded snapshot.
		}
		if start > num {
			start = 0
			fallbackCounts = make(map[string]int)
		}
		if start > 0 && legacyCursor {
			var scannedBytes int64
			for i := 1; i <= start; i++ {
				if uids[i] != "" {
					continue
				}
				prior, e := popRetrieve(c, i)
				if e != nil {
					return e
				}
				scannedBytes += int64(len(prior))
				if scannedBytes > maxPOP3IDScanBytes {
					return fmt.Errorf("POP3 fallback ID scan exceeds size limit")
				}
				if _, e = popRemoteID(prior, "", fallbackCounts); e != nil {
					return e
				}
			}
		}
		end := num
		if end-start > 50 {
			end = start + 50
		}
		totalRaw := int64(0)
		for i := start + 1; i <= end; i++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			raw, e := popRetrieve(c, i)
			if e != nil {
				return e
			}
			if len(raw) > maxPOP3MessageBytes {
				return fmt.Errorf("POP3 message exceeds size limit")
			}
			if totalRaw+int64(len(raw)) > maxPOP3SyncBytes {
				end = i - 1
				break
			}
			totalRaw += int64(len(raw))
			id, e := popRemoteID(raw, uids[i], fallbackCounts)
			if e != nil {
				return e
			}
			subj, from, to, cc, body, html, attachments, htmlSkipped, readError := parsedPOP3Body(raw)
			msg := Message{AccountID: a.cfg.Source + ":" + a.cfg.ID, FolderID: a.cfg.Source + ":" + a.cfg.ID + ":inbox", RemoteID: id, ThreadID: id, Subject: subj, From: from, To: to, CC: cc, Text: body, HTML: html, HTMLSkipped: htmlSkipped, ReadError: readError, Date: time.Now(), Unread: true, Attachments: attachments}
			msg.ReplyTo = ParseReplyTo(raw)
			msg.Raw = append([]byte(nil), raw...)
			if m, e := mailDate(raw); e == nil {
				msg.Date = m
			}
			if mid, irt, refs, e := ParseThreadHeaders(raw); e == nil {
				msg.MessageID = mid
				msg.InReplyTo = irt
				msg.References = refs
			}
			result.Messages = append(result.Messages, msg)
		}
		if end < num {
			result.HasMore = true
			result.NextCursor, err = encodePOPSyncCursor(pop3SyncCursor{Version: 1, Next: end, Num: num, FallbackCounts: fallbackCounts})
			if err != nil {
				return err
			}
		} else {
			result.RemoteSnapshotComplete = true
			result.NextCursor = ""
		}
		_ = size
		return nil
	})
	return result, err
}
func popStat(c *textproto.Conn) (int, int, error) {
	id, e := c.Cmd("STAT")
	if e != nil {
		return 0, 0, e
	}
	c.StartResponse(id)
	line, e := c.ReadLine()
	c.EndResponse(id)
	if e != nil {
		return 0, 0, e
	}
	if !strings.HasPrefix(line, "+OK") {
		return 0, 0, fmt.Errorf("POP3 STAT rejected")
	}
	f := strings.Fields(line)
	if len(f) < 3 {
		return 0, 0, fmt.Errorf("invalid POP3 STAT")
	}
	n, e := strconv.Atoi(f[1])
	if e != nil {
		return 0, 0, e
	}
	sz, e := strconv.Atoi(f[2])
	return n, sz, e
}
func popUIDLs(c *textproto.Conn, num int) (map[int]string, bool, error) {
	id, e := c.Cmd("UIDL")
	if e != nil {
		return nil, false, nil
	}
	c.StartResponse(id)
	line, e := c.ReadLine()
	if e != nil {
		c.EndResponse(id)
		return nil, false, e
	}
	if !strings.HasPrefix(line, "+OK") {
		c.EndResponse(id)
		return nil, false, nil
	}
	out := map[int]string{}
	r := c.DotReader()
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 1024), 4096)
	scanCount := 0
	for scan.Scan() {
		if scanCount >= maxPOP3Messages {
			c.EndResponse(id)
			return nil, false, fmt.Errorf("POP3 UIDL response exceeds message limit")
		}
		scanCount++
		f := strings.Fields(scan.Text())
		if len(f) == 2 {
			n, e := strconv.Atoi(f[0])
			if e == nil && n > 0 && n <= num && len(f[1]) <= 512 {
				out[n] = f[1]
			}
		}
	}
	e = scan.Err()
	c.EndResponse(id)
	return out, e == nil, e
}
func popRetrieve(c *textproto.Conn, n int) ([]byte, error) {
	id, e := c.Cmd("RETR %d", n)
	if e != nil {
		return nil, e
	}
	c.StartResponse(id)
	line, e := c.ReadLine()
	if e != nil {
		c.EndResponse(id)
		return nil, e
	}
	if !strings.HasPrefix(line, "+OK") {
		c.EndResponse(id)
		return nil, fmt.Errorf("POP3 message unavailable")
	}
	r := c.DotReader()
	data, e := io.ReadAll(io.LimitReader(r, maxPOP3MessageBytes+1))
	c.EndResponse(id)
	return data, e
}
func (a *pop3Adapter) Read(ctx context.Context, folder, id string) (Message, error) {
	raw, e := a.Raw(ctx, folder, id)
	if e != nil {
		return Message{}, e
	}
	subject, from, to, cc, body, html, attachments, htmlSkipped, readError := parsedPOP3Body(raw)
	msg := Message{AccountID: a.cfg.Source + ":" + a.cfg.ID, FolderID: a.cfg.Source + ":" + a.cfg.ID + ":inbox", RemoteID: id, ThreadID: id, Subject: subject, From: from, To: to, CC: cc, Text: body, HTML: html, HTMLSkipped: htmlSkipped, ReadError: readError, Date: time.Now(), Attachments: attachments, Raw: append([]byte(nil), raw...)}
	msg.ReplyTo = ParseReplyTo(raw)
	if d, e := mailDate(raw); e == nil {
		msg.Date = d
	}
	if mid, irt, refs, e := ParseThreadHeaders(raw); e == nil {
		msg.MessageID = mid
		msg.InReplyTo = irt
		msg.References = refs
	}
	return msg, nil
}
func (a *pop3Adapter) Raw(ctx context.Context, folder, id string) ([]byte, error) {
	var out []byte
	err := a.withConn(ctx, func(c *textproto.Conn) error {
		num, _, e := popStat(c)
		if e != nil {
			return e
		}
		if num > maxPOP3Messages {
			return fmt.Errorf("POP3 mailbox has too many messages to scan safely")
		}
		uids, _, e := popUIDLs(c, num)
		if e != nil {
			return e
		}
		fallbackCounts := make(map[string]int)
		var scannedBytes int64
		for i := 1; i <= num; i++ {
			raw, e := popRetrieve(c, i)
			if e != nil {
				return e
			}
			if uids[i] == "" {
				scannedBytes += int64(len(raw))
				if scannedBytes > maxPOP3IDScanBytes {
					return fmt.Errorf("POP3 fallback ID scan exceeds size limit")
				}
			}
			rid, e := popRemoteID(raw, uids[i], fallbackCounts)
			if e != nil {
				return e
			}
			if rid == id {
				out = raw
				return nil
			}
		}
		return fmt.Errorf("message not found")
	})
	return out, err
}

func popRemoteID(raw []byte, uidl string, fallbackCounts map[string]int) (string, error) {
	if uidl != "" {
		return "uidl:" + uidl, nil
	}
	sum := sha256.Sum256(raw)
	key := hex.EncodeToString(sum[:])
	if fallbackCounts == nil {
		return "", fmt.Errorf("POP3 fallback ID state is unavailable")
	}
	fallbackCounts[key]++
	if len(fallbackCounts) > maxPOP3FallbackHashes {
		return "", fmt.Errorf("POP3 fallback ID state exceeds limit")
	}
	return "sha256:" + key + ":" + strconv.Itoa(fallbackCounts[key]), nil
}

func decodePOPSyncCursor(raw string) (pop3SyncCursor, bool, error) {
	if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
		return pop3SyncCursor{Next: n}, true, nil
	}
	if len(raw) > 1<<20 {
		return pop3SyncCursor{}, false, fmt.Errorf("invalid POP3 sync cursor")
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(data) > 1<<20 {
		return pop3SyncCursor{}, false, fmt.Errorf("invalid POP3 sync cursor")
	}
	var state pop3SyncCursor
	if json.Unmarshal(data, &state) != nil || state.Version != 1 || state.Next < 0 || state.Num < 0 || len(state.FallbackCounts) > maxPOP3FallbackHashes {
		return pop3SyncCursor{}, false, fmt.Errorf("invalid POP3 sync cursor")
	}
	return state, false, nil
}

func parsedPOP3Body(raw []byte) (subject, from string, to, cc []string, text, html string, attachments []Attachment, htmlSkipped bool, readError string) {
	subject, from, to, cc, text, html, attachments, htmlSkipped, err := ParseMIME(raw, maxPOP3MessageBytes)
	if err != nil {
		subject, from, to, cc = pop3Headers(raw)
		return subject, from, to, cc, "", "", nil, false, "message_unparsed"
	}
	return subject, from, to, cc, text, html, attachments, htmlSkipped, ""
}

func pop3Headers(raw []byte) (subject, from string, to, cc []string) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil || msg == nil {
		return "", "", nil, nil
	}
	subject = msg.Header.Get("Subject")
	if addrs, err := msg.Header.AddressList("From"); err == nil && len(addrs) > 0 {
		from = addrs[0].Address
	}
	if addrs, err := msg.Header.AddressList("To"); err == nil {
		for _, addr := range addrs {
			if addr.Address != "" {
				to = append(to, addr.Address)
			}
		}
	}
	if addrs, err := msg.Header.AddressList("Cc"); err == nil {
		for _, addr := range addrs {
			if addr.Address != "" {
				cc = append(cc, addr.Address)
			}
		}
	}
	return subject, from, to, cc
}

func encodePOPSyncCursor(state pop3SyncCursor) (string, error) {
	data, err := json.Marshal(state)
	if err != nil || len(data) > 1<<20 {
		return "", fmt.Errorf("POP3 sync cursor exceeds size limit")
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func (a *pop3Adapter) Attachment(ctx context.Context, folder, id, part string) (io.ReadCloser, error) {
	raw, e := a.Raw(ctx, folder, id)
	if e != nil {
		return nil, e
	}
	_, _, _, _, _, _, att, _, e := ParseMIME(raw, maxPOP3MessageBytes)
	if e != nil {
		return nil, e
	}
	for _, x := range att {
		if x.ID == part {
			return io.NopCloser(bytes.NewReader(x.Data)), nil
		}
	}
	return nil, fmt.Errorf("attachment not found")
}
func (a *pop3Adapter) Mutate(context.Context, string, string, Mutation) error {
	return fmt.Errorf("POP3 does not support message mutations")
}
func (a *pop3Adapter) Send(ctx context.Context, out Outgoing) (SendResult, error) {
	return sendSMTP(ctx, a.cfg, out)
}

func mailDate(raw []byte) (time.Time, error) {
	m, e := mail.ReadMessage(bytes.NewReader(raw))
	if e != nil {
		return time.Time{}, e
	}
	return mail.ParseDate(m.Header.Get("Date"))
}
