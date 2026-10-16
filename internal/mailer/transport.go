package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"aurago/internal/security"
	imapclient "github.com/emersion/go-imap/client"
	"github.com/emersion/go-sasl"
)

type TransportConfig struct {
	ID, Source, Name, Address                  string
	ReceiveProtocol, ReceiveTLS                string
	IMAPHost                                   string
	IMAPPort                                   int
	POP3Host                                   string
	POP3Port                                   int
	Username, Password                         string
	SMTPHost                                   string
	SMTPPort                                   int
	SMTPTransport                              string
	SMTPUsername, SMTPPassword, SMTPAuthMethod string
	FromAddress                                string
	ReadOnly                                   bool
	AgentAccess                                bool
	OAuthSource, OAuthAccountID, OAuthProvider string
	OAuthClientID                              string
	OAuthClientSecret                          string
	AgentMailInboxID                           string
	AgentMailBaseURL, AgentMailAPIKey          string
	Token                                      func(context.Context, string, bool) (string, error)
	HTTPClient                                 *http.Client
	TLSConfig                                  *tls.Config
	DialContext                                func(context.Context, string, string) (net.Conn, error)
}

func AccountFromTransportConfig(c TransportConfig) Account {
	protocol := strings.ToLower(strings.TrimSpace(c.ReceiveProtocol))
	capabilities := []string{"read", "search", "send"}
	switch strings.ToLower(strings.TrimSpace(c.ReceiveProtocol)) {
	case "imap":
		capabilities = append(capabilities, "folders_read", "read_state", "star", "move", "archive", "trash", "restore", "delete", "junk", "labels_write", "folders_create")
	case "pop3":
		capabilities = append(capabilities, "folders_read", "folders_create", "read_state", "star", "move", "archive", "trash", "restore", "delete", "junk", "labels_write")
	case "gmail_api":
		capabilities = append(capabilities, "folders_read", "read_state", "star", "move", "archive", "trash", "restore", "delete", "junk", "labels_write", "folders_create")
	case "agentmail":
		capabilities = []string{"read", "search", "send", "folders_read", "read_state", "star", "move", "archive", "trash", "restore", "delete", "junk", "labels_write"}
	}
	if c.ReadOnly {
		// Folder listing remains a read operation for every protocol. POP3's
		// folders are synthesized locally, but Mailer still gates the endpoint
		// on this capability before returning them.
		capabilities = []string{"read", "search", "folders_read"}
	}
	return Account{ID: c.Source + ":" + c.ID, Source: c.Source, Protocol: protocol, Name: c.Name, Address: c.Address, Capabilities: capabilities, AgentAccess: c.AgentAccess, ReadOnly: c.ReadOnly}
}

// ProfiledAdapter can retrieve the authenticated mailbox address during an
// explicit connection test or account link operation.
type ProfiledAdapter interface {
	Profile(context.Context) (string, error)
}

// SendTester probes outbound authentication without submitting a message.
type SendTester interface {
	TestSend(context.Context) error
}

// AdapterFor constructs the receive adapter and binds SMTP send to the same
// account. OAuth credentials and secret resolution stay in the server owner.
func AdapterFor(cfg TransportConfig) (Adapter, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.ReceiveProtocol)) {
	case "imap":
		return &imapAdapter{cfg: cfg}, nil
	case "pop3":
		return &pop3Adapter{cfg: cfg}, nil
	case "gmail_api":
		return &gmailAdapter{cfg: cfg}, nil
	case "agentmail":
		return newAgentMailAdapter(cfg)
	default:
		return nil, fmt.Errorf("unsupported receive protocol")
	}
}

func dialTransport(ctx context.Context, cfg TransportConfig, network, address string, timeout time.Duration) (net.Conn, error) {
	if cfg.DialContext != nil {
		return cfg.DialContext(ctx, network, address)
	}
	return (&net.Dialer{Timeout: timeout}).DialContext(ctx, network, address)
}

// WithIMAPClient runs fn with an authenticated IMAP client using the shared
// TLS, timeout, and cancellation handling.
func WithIMAPClient(ctx context.Context, cfg TransportConfig, fn func(*imapclient.Client) error) error {
	if fn == nil {
		return fmt.Errorf("IMAP operation is required")
	}
	c, conn, stop, err := dialIMAP(ctx, cfg)
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

// TestIMAPAuth verifies IMAP credentials without fetching or marking mail.
func TestIMAPAuth(ctx context.Context, cfg TransportConfig) error {
	return WithIMAPClient(ctx, cfg, func(c *imapclient.Client) error { return c.Noop() })
}

func dialIMAP(ctx context.Context, cfg TransportConfig) (*imapclient.Client, net.Conn, func(), error) {
	security.RegisterSensitive(cfg.Username)
	security.RegisterSensitive(cfg.Password)
	receiveTLS := strings.ToLower(strings.TrimSpace(cfg.ReceiveTLS))
	if receiveTLS == "" {
		receiveTLS = "tls"
	}
	if receiveTLS != "tls" && receiveTLS != "starttls" {
		return nil, nil, func() {}, fmt.Errorf("IMAP requires TLS or STARTTLS")
	}
	port := cfg.IMAPPort
	if port <= 0 {
		if receiveTLS == "starttls" {
			port = 143
		} else {
			port = 993
		}
	}
	addr := net.JoinHostPort(strings.TrimSpace(cfg.IMAPHost), fmt.Sprint(port))
	conn, err := dialTransport(ctx, cfg, "tcp", addr, 10*time.Second)
	if err != nil {
		return nil, nil, func() {}, err
	}
	cancelConn := conn
	afterCancel := context.AfterFunc(ctx, func() { _ = cancelConn.Close() })
	stop := func() { afterCancel() }
	fail := func(c *imapclient.Client, err error) (*imapclient.Client, net.Conn, func(), error) {
		stop()
		_ = conn.Close()
		return nil, nil, func() {}, err
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if receiveTLS == "tls" {
		tlsConn := tls.Client(conn, transportTLSConfig(cfg, cfg.IMAPHost))
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			return fail(nil, err)
		}
		conn = tlsConn
	} else if receiveTLS == "starttls" {
		// STARTTLS is negotiated by go-imap after receiving the server greeting.
	} else {
		return fail(nil, fmt.Errorf("IMAP requires TLS or STARTTLS"))
	}
	c, err := imapclient.New(conn)
	if err != nil {
		return fail(nil, err)
	}
	if receiveTLS == "starttls" {
		ok, e := c.SupportStartTLS()
		if e != nil || !ok {
			c.Logout()
			return fail(c, fmt.Errorf("IMAP STARTTLS is unavailable"))
		}
		if e = c.StartTLS(transportTLSConfig(cfg, cfg.IMAPHost)); e != nil {
			c.Logout()
			return fail(c, e)
		}
	}
	if cfg.OAuthSource != "" || strings.EqualFold(cfg.SMTPAuthMethod, "xoauth2") {
		if cfg.Token == nil {
			c.Logout()
			return fail(c, fmt.Errorf("IMAP OAuth token source unavailable"))
		}
		token, e := cfg.Token(ctx, "read", false)
		if e != nil {
			c.Logout()
			return fail(c, e)
		}
		security.RegisterSensitive(token)
		ok, e := c.SupportAuth("XOAUTH2")
		if e != nil || !ok {
			c.Logout()
			return fail(c, fmt.Errorf("IMAP XOAUTH2 is unavailable"))
		}
		e = c.Authenticate(xoauth2Client{username: cfg.Username, token: token})
		if e != nil {
			c.Logout()
			return fail(c, e)
		}
	} else if err = c.Login(cfg.Username, cfg.Password); err != nil {
		c.Logout()
		return fail(c, err)
	}
	return c, conn, stop, nil
}

func transportTLSConfig(cfg TransportConfig, host string) *tls.Config {
	base := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.TLSConfig != nil {
		base = cfg.TLSConfig.Clone()
		if base.MinVersion < tls.VersionTLS12 {
			base.MinVersion = tls.VersionTLS12
		}
	}
	base.ServerName = host
	return base
}

type xoauth2Client struct {
	username, token string
	sent            bool
}

func (x xoauth2Client) Start() (string, []byte, error) {
	return "XOAUTH2", []byte("user=" + x.username + "\x01auth=Bearer " + x.token + "\x01\x01"), nil
}
func (x xoauth2Client) Next(challenge []byte) ([]byte, error) { return []byte{}, nil }

var _ sasl.Client = xoauth2Client{}

func (a *imapAdapter) Send(ctx context.Context, out Outgoing) (SendResult, error) {
	return sendSMTP(ctx, a.cfg, out)
}
func (a *imapAdapter) TestSend(ctx context.Context) error { return TestSMTPAuth(ctx, a.cfg) }

// SendSMTP sends through the configured SMTP transport without requiring a
// receive adapter.
func SendSMTP(ctx context.Context, cfg TransportConfig, out Outgoing) (SendResult, error) {
	return sendSMTP(ctx, cfg, out)
}

func sendSMTP(ctx context.Context, cfg TransportConfig, out Outgoing) (SendResult, error) {
	host := strings.TrimSpace(cfg.SMTPHost)
	if host == "" {
		return SendResult{Rejected: true}, fmt.Errorf("SMTP host is not configured")
	}
	port := cfg.SMTPPort
	if port <= 0 {
		port = 587
	}
	transport, err := smtpTransportMode(cfg.SMTPTransport, port)
	if err != nil {
		return SendResult{Rejected: true}, err
	}
	username := cfg.SMTPUsername
	if username == "" {
		username = cfg.Username
	}
	password := cfg.SMTPPassword
	if strings.EqualFold(cfg.SMTPAuthMethod, "xoauth2") && cfg.Token != nil {
		var e error
		password, e = cfg.Token(ctx, "smtp", true)
		if e != nil {
			return SendResult{}, e
		}
	} else if strings.EqualFold(cfg.SMTPAuthMethod, "xoauth2") {
		return SendResult{}, fmt.Errorf("SMTP OAuth token source unavailable")
	}
	security.RegisterSensitive(username)
	security.RegisterSensitive(password)
	addr := net.JoinHostPort(host, fmt.Sprint(port))
	conn, err := dialTransport(ctx, cfg, "tcp", addr, 10*time.Second)
	if err != nil {
		return SendResult{}, fmt.Errorf("connect SMTP: %w", err)
	}
	cancelConn := conn
	stop := context.AfterFunc(ctx, func() { _ = cancelConn.Close() })
	defer stop()
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(60 * time.Second))
	implicitTLS := transport == "tls"
	if implicitTLS {
		tc := tls.Client(conn, transportTLSConfig(cfg, host))
		if err = tc.HandshakeContext(ctx); err != nil {
			return SendResult{}, fmt.Errorf("SMTP TLS: %w", err)
		}
		conn = tc
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return SendResult{}, fmt.Errorf("SMTP greeting: %w", err)
	}
	defer c.Close()
	if !implicitTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return SendResult{Rejected: true}, fmt.Errorf("SMTP server does not offer STARTTLS")
		}
		if err = c.StartTLS(transportTLSConfig(cfg, host)); err != nil {
			return SendResult{Rejected: true}, fmt.Errorf("SMTP STARTTLS: %w", err)
		}
	}
	if strings.TrimSpace(username) != "" {
		var auth smtp.Auth
		if strings.EqualFold(cfg.SMTPAuthMethod, "xoauth2") {
			auth = xoauth2SMTPAuth{username: username, token: password}
		} else {
			auth = smtp.PlainAuth("", username, password, host)
		}
		if err = c.Auth(auth); err != nil {
			return SendResult{Rejected: true}, fmt.Errorf("SMTP authentication rejected: %w", err)
		}
	}
	from := cfg.FromAddress
	if from == "" {
		from = cfg.Address
	}
	if from == "" {
		from = username
	}
	mimeBytes, err := BuildMIME(from, out.Draft, time.Now())
	if err != nil {
		return SendResult{Rejected: true}, err
	}
	fromAddr, err := mailboxAddress(from)
	if err != nil {
		return SendResult{Rejected: true}, err
	}
	recipients := append(append(append([]string{}, out.Draft.To...), out.Draft.CC...), out.Draft.BCC...)
	if err = c.Mail(fromAddr); err != nil {
		return smtpRejected(err)
	}
	accepted := 0
	for _, raw := range recipients {
		a, e := mailboxAddress(raw)
		if e != nil {
			return SendResult{Rejected: true}, e
		}
		if e = c.Rcpt(a); e != nil {
			if accepted == 0 {
				return smtpRejected(e)
			}
			return SendResult{Rejected: true}, fmt.Errorf("some recipients were rejected; no message body was submitted: %w", e)
		}
		accepted++
	}
	w, err := c.Data()
	if err != nil {
		return smtpRejected(err)
	}
	if _, err = w.Write(mimeBytes); err != nil {
		_ = w.Close()
		return SendResult{}, fmt.Errorf("SMTP delivery outcome unknown after message data began: %w", err)
	}
	if err = w.Close(); err != nil {
		return smtpRejected(err)
	}
	_ = c.Quit()
	messageID, inReplyTo, refs, _ := ParseThreadHeaders(mimeBytes)
	message := Message{AccountID: cfg.Source + ":" + cfg.ID, RemoteID: "", ThreadID: messageID, MessageID: messageID, InReplyTo: inReplyTo, References: refs,
		Subject: out.Draft.Subject, From: from, To: append([]string(nil), out.Draft.To...), CC: append([]string(nil), out.Draft.CC...), Date: time.Now(), Raw: append([]byte(nil), mimeBytes...)}
	return SendResult{Confirmed: true, Message: &message}, nil
}

// TestSMTPAuth performs the TLS and authentication handshake only; it never
// submits MAIL FROM, RCPT TO, or DATA commands.
func TestSMTPAuth(ctx context.Context, cfg TransportConfig) error {
	host := strings.TrimSpace(cfg.SMTPHost)
	if host == "" {
		return fmt.Errorf("SMTP host is not configured")
	}
	port := cfg.SMTPPort
	if port <= 0 {
		port = 587
	}
	transport, err := smtpTransportMode(cfg.SMTPTransport, port)
	if err != nil {
		return err
	}
	username := cfg.SMTPUsername
	if username == "" {
		username = cfg.Username
	}
	password := cfg.SMTPPassword
	if strings.EqualFold(cfg.SMTPAuthMethod, "xoauth2") {
		if cfg.Token == nil {
			return fmt.Errorf("OAuth token source unavailable")
		}
		var e error
		password, e = cfg.Token(ctx, "send", true)
		if e != nil {
			return e
		}
	}
	security.RegisterSensitive(username)
	security.RegisterSensitive(password)
	conn, err := dialTransport(ctx, cfg, "tcp", net.JoinHostPort(host, fmt.Sprint(port)), 10*time.Second)
	if err != nil {
		return fmt.Errorf("connect SMTP: %w", err)
	}
	cancelConn := conn
	stop := context.AfterFunc(ctx, func() { _ = cancelConn.Close() })
	defer stop()
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	implicit := transport == "tls"
	if implicit {
		tc := tls.Client(conn, transportTLSConfig(cfg, host))
		if err = tc.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("SMTP TLS: %w", err)
		}
		conn = tc
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("SMTP greeting: %w", err)
	}
	defer c.Close()
	if !implicit {
		ok, _ := c.Extension("STARTTLS")
		if !ok {
			return fmt.Errorf("SMTP server does not offer STARTTLS")
		}
		if err = c.StartTLS(transportTLSConfig(cfg, host)); err != nil {
			return fmt.Errorf("SMTP STARTTLS: %w", err)
		}
	}
	if username != "" {
		var auth smtp.Auth
		if strings.EqualFold(cfg.SMTPAuthMethod, "xoauth2") {
			auth = xoauth2SMTPAuth{username: username, token: password}
		} else {
			auth = smtp.PlainAuth("", username, password, host)
		}
		if err = c.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication rejected")
		}
	}
	return nil
}

func smtpTransportMode(configured string, port int) (string, error) {
	transport := strings.ToLower(strings.TrimSpace(configured))
	if transport == "" {
		if port == 465 {
			transport = "tls"
		} else {
			transport = "starttls"
		}
	}
	if transport != "tls" && transport != "starttls" {
		return "", fmt.Errorf("SMTP transport must use TLS or STARTTLS")
	}
	return transport, nil
}

func smtpRejected(err error) (SendResult, error) {
	var pe *textproto.Error
	if errors.As(err, &pe) && pe.Code >= 400 && pe.Code < 600 {
		return SendResult{Rejected: true}, err
	}
	return SendResult{}, err
}
func mailboxAddress(raw string) (string, error) {
	a, e := mail.ParseAddress(strings.TrimSpace(raw))
	if e != nil || a.Address == "" || strings.ContainsAny(a.Address, "\r\n\x00") {
		return "", fmt.Errorf("invalid mail address")
	}
	return a.Address, nil
}

type xoauth2SMTPAuth struct{ username, token string }

func (x xoauth2SMTPAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "XOAUTH2", []byte("user=" + x.username + "\x01auth=Bearer " + x.token + "\x01\x01"), nil
}
func (x xoauth2SMTPAuth) Next(challenge []byte, more bool) ([]byte, error) {
	if more {
		return []byte{}, nil
	}
	return nil, nil
}
