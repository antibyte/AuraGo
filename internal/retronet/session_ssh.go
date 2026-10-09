package retronet

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"
)

// errBrowserGone ends a host-key prompt when the browser disconnects.
var errBrowserGone = &DialError{Reason: ReasonRemoteClosed, Err: errors.New("browser disconnected during the host key prompt")}

// runSSH opens the anonymous SSH shell over conn, asking the browser on first contact with
// an own entry, and pumps it until the session ends.
func (m *Manager) runSSH(ctx context.Context, e Entry, size Size, client Client, conn net.Conn, stats *sessionStats) string {
	var pending atomic.Pointer[Size]
	sess, fingerprint, err := OpenSSH(ctx, conn, e, size.Cols, size.Rows, m.hostKeyDecider(client, &pending))
	if err != nil {
		return endReason(ctx, ReasonOf(err))
	}
	defer sess.Close()
	if e.Own && e.HostKey == "" && m.OnHostKeyAccepted != nil {
		// A failed save is not fatal: the session runs and the next dial asks again.
		_ = m.OnHostKeyAccepted(ctx, e.ID, fingerprint)
	}
	connected := Control{Type: controlConnected, Protocol: string(ProtocolSSH), Charset: string(e.EffectiveCharset())}
	if err := client.SendControl(connected); err != nil {
		return endReason(ctx, ReasonRemoteClosed)
	}
	// An SSH channel has no write deadline: a server that stops reading blocks writes inside
	// x/crypto. Closing the connection after sessionWriteTimeout fails the blocked call and
	// ends the session as remote_closed, like the Telnet write deadline does.
	out := sessionWriter{w: sess, abort: func() { _ = conn.Close() }, stats: stats}
	resize := func(s Size) error {
		return out.bounded(func() error { return sess.Resize(s.Cols, s.Rows) })
	}
	if latest := pending.Load(); latest != nil && *latest != size {
		if err := resize(*latest); err != nil {
			return endReason(ctx, ReasonRemoteClosed)
		}
	}
	codec := NewCodec(e.EffectiveCharset())
	var text []byte // decode buffer, reused for every read
	return m.pump(ctx, client, sess, stats, pumpHandlers{
		remote: func(p []byte) error {
			text = codec.Decode(text[:0], p)
			if len(text) == 0 {
				return nil
			}
			return client.SendData(text)
		},
		input: func(p []byte) error {
			return out.write(codec.Encode(nil, p))
		},
		resize: resize,
	})
}

// hostKeyDecider sends "hostkey_prompt" and waits for the browser's decision. No answer
// within the decision timeout counts as a rejection. Resize events that arrive meanwhile are
// kept in pending; keystrokes are dropped.
func (m *Manager) hostKeyDecider(client Client, pending *atomic.Pointer[Size]) HostKeyDecider {
	return func(ctx context.Context, keyType, fingerprint string) (bool, error) {
		prompt := Control{Type: controlHostKeyPrompt, KeyType: keyType, Fingerprint: fingerprint}
		if err := client.SendControl(prompt); err != nil {
			return false, errBrowserGone
		}
		timer := time.NewTimer(m.hostKeyTimeout())
		defer timer.Stop()
		events := client.Events()
		for {
			select {
			case <-ctx.Done():
				return false, context.Cause(ctx)
			case <-timer.C:
				return false, nil
			case ev, ok := <-events:
				if !ok {
					return false, errBrowserGone
				}
				if ev.Resize != nil {
					size := clampSessionSize(*ev.Resize)
					pending.Store(&size)
				}
				if ev.HostKeyAccept != nil {
					return *ev.HostKeyAccept, nil
				}
			}
		}
	}
}

// hostKeyTimeout is how long the browser may take to answer a host-key prompt.
func (m *Manager) hostKeyTimeout() time.Duration {
	if m.decisionTimeout > 0 {
		return m.decisionTimeout
	}
	return hostKeyDecisionTimeout
}
