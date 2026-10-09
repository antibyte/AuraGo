package retronet

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"
)

// Session limits (spec: 4 sessions, 30 min idle, 4 h maximum, 60 s host-key decision).
const (
	defaultMaxSessions     = 4
	defaultIdleTimeout     = 30 * time.Minute
	defaultMaxDuration     = 4 * time.Hour
	hostKeyDecisionTimeout = 60 * time.Second
)

// Control message types sent to the browser (Contract D).
const (
	controlConnected     = "connected"
	controlEcho          = "echo"
	controlHostKeyPrompt = "hostkey_prompt"
	controlResult        = "result"
)

var (
	// ErrDisabled is the context cause that ends a session because Retro-Net was switched off.
	ErrDisabled = errors.New("retronet disabled")
	// ErrShutdown is the context cause that ends a session because the server shuts down.
	ErrShutdown = errors.New("server shutting down")

	// errSessionTooLong is the context cause installed for Manager.MaxDuration.
	errSessionTooLong = &DialError{Reason: ReasonMaxDuration, Err: errors.New("maximum session length reached")}
)

// Size is a terminal size in character cells.
type Size struct{ Cols, Rows int }

// ClientEvent is one message from the browser.
type ClientEvent struct {
	Data          []byte // keystrokes (UTF-8)
	Resize        *Size
	HostKeyAccept *bool
}

// Control is a JSON control message to the browser (exact wire shape, see Contract D).
type Control struct {
	Type        string `json:"type"`
	Protocol    string `json:"protocol,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Charset     string `json:"charset,omitempty"`
	Remote      *bool  `json:"remote,omitempty"` // echo: kludge character mode (RemoteEcho)
	Hidden      *bool  `json:"hidden,omitempty"` // echo: ServerEcho && !RemoteEcho (line mode, no local echo)
	KeyType     string `json:"key_type,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Code        string `json:"code,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// Client is the browser side of one session (the server package adapts a WebSocket to it).
// SendData must not retain p after it returns: the pump reuses the buffer for the next read.
// SendData and SendControl must return within a bounded time (the WebSocket adapter uses a
// 10 s write deadline) and must fail once the browser is gone: while they block, Run cannot
// observe cancellation or timers.
type Client interface {
	Events() <-chan ClientEvent // closed when the browser disconnects
	SendData(p []byte) error    // UTF-8 terminal output
	SendControl(c Control) error
}

// Result describes how a session ended.
type Result struct {
	Code     string
	Reason   string
	Target   string // pinned ip:port, empty when dialing failed before connect
	BytesIn  int64  // from the service
	BytesOut int64  // to the service
	Duration time.Duration
}

// Manager runs Retro-Net sessions under global limits. The zero value is ready to use; a
// Manager must not be copied after first use.
type Manager struct {
	Dialer      Dialer
	MaxSessions int           // 0 -> 4
	IdleTimeout time.Duration // 0 -> 30 * time.Minute (no user input)
	MaxDuration time.Duration // 0 -> 4 * time.Hour
	// OnHostKeyAccepted persists a first-contact SSH key for an own entry.
	OnHostKeyAccepted func(ctx context.Context, entryID, fingerprint string) error

	active          atomic.Int64  // running sessions
	decisionTimeout time.Duration // tests only: 0 -> hostKeyDecisionTimeout
}

// sessionStats counts the bytes exchanged with the service, negotiation included.
type sessionStats struct {
	bytesIn  int64
	bytesOut int64
}

// Active returns the number of running sessions.
func (m *Manager) Active() int { return int(m.active.Load()) }

// Run dials and pumps one session until it ends. It sends "connected", "echo" (telnet),
// "hostkey_prompt" (when needed) and always exactly one final "result" control, then returns.
// Context cancellation maps to ReasonDisabled when ctx carries ErrDisabled as cause
// (context.Cause), to ReasonServerShutdown when the cause is ErrShutdown, else remote_closed.
//
// A session over the MaxSessions limit ends with BUSY/limit without dialing. Every goroutine
// Run starts has exited and the connection is closed when Run returns. Payload bytes are
// never logged.
func (m *Manager) Run(ctx context.Context, e Entry, size Size, client Client) Result {
	started := time.Now()
	var res Result
	if m.acquire() {
		res = m.sessionInSlot(ctx, e, sessionSize(e, size), client)
	} else {
		res.Reason = ReasonLimit
	}
	res.Code = CodeFor(res.Reason)
	res.Duration = time.Since(started)
	_ = client.SendControl(Control{Type: controlResult, Code: res.Code, Reason: res.Reason})
	return res
}

// sessionInSlot runs one session in a slot that acquire already took. The slot is released
// even when the session panics (a misbehaving Client), so a panic cannot leak it.
func (m *Manager) sessionInSlot(ctx context.Context, e Entry, size Size, client Client) Result {
	defer m.active.Add(-1)
	return m.session(ctx, e, size, client)
}

// session dials e and serves it until it ends. The result carries no Code and Duration yet.
func (m *Manager) session(ctx context.Context, e Entry, size Size, client Client) Result {
	conn, target, err := m.Dialer.DialEntry(ctx, e)
	if err != nil {
		return Result{Reason: endReason(ctx, ReasonOf(err)), Target: target}
	}
	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer func() {
		stopClose()
		_ = conn.Close()
	}()
	var stats sessionStats
	reason := m.serve(ctx, e, size, client, conn, &stats)
	return Result{Reason: reason, Target: target, BytesIn: stats.bytesIn, BytesOut: stats.bytesOut}
}

// serve runs the protocol-specific part of a connected session and returns the end reason.
func (m *Manager) serve(ctx context.Context, e Entry, size Size, client Client, conn net.Conn, stats *sessionStats) string {
	switch e.Protocol {
	case ProtocolTelnet:
		return m.runTelnet(ctx, e, size, client, conn, stats)
	default:
		return ReasonRemoteClosed
	}
}

// acquire takes a session slot unless MaxSessions are already running.
func (m *Manager) acquire() bool {
	limit := int64(m.maxSessions())
	for {
		n := m.active.Load()
		if n >= limit {
			return false
		}
		if m.active.CompareAndSwap(n, n+1) {
			return true
		}
	}
}

func (m *Manager) maxSessions() int {
	if m.MaxSessions > 0 {
		return m.MaxSessions
	}
	return defaultMaxSessions
}

// sessionSize clamps the browser size like the WebSocket handler (cols 20-400, rows 5-200,
// default 80x25) and pins Telnet BBS entries to their fixed 80x25 geometry.
func sessionSize(e Entry, s Size) Size {
	if e.Protocol == ProtocolTelnet && e.Kind == KindBBS {
		return Size{Cols: 80, Rows: 25}
	}
	return clampSessionSize(s)
}

// clampSessionSize applies the wire limits of Contract D to a browser size.
func clampSessionSize(s Size) Size {
	if s.Cols <= 0 {
		s.Cols = 80
	}
	if s.Rows <= 0 {
		s.Rows = 25
	}
	return Size{Cols: min(max(s.Cols, 20), 400), Rows: min(max(s.Rows, 5), 200)}
}

// endReason returns the cancellation reason once ctx is done, else fallback.
func endReason(ctx context.Context, fallback string) string {
	if ctx.Err() != nil {
		return cancelReason(context.Cause(ctx))
	}
	return fallback
}

// cancelReason maps a context cause: ErrDisabled -> disabled, ErrShutdown -> server_shutdown,
// a *DialError (for example the max-duration cause) -> its reason, anything else -> remote_closed.
func cancelReason(cause error) string {
	switch {
	case errors.Is(cause, ErrDisabled):
		return ReasonDisabled
	case errors.Is(cause, ErrShutdown):
		return ReasonServerShutdown
	}
	var dialErr *DialError
	if errors.As(cause, &dialErr) {
		return dialErr.Reason
	}
	return ReasonRemoteClosed
}
