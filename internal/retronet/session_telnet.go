package retronet

import (
	"context"
	"net"
)

// telnetEcho is the echo state reported to the browser.
type telnetEcho struct {
	remote bool // kludge character mode: server WILL ECHO and WILL SGA
	hidden bool // line mode without local echo: server WILL ECHO without SGA
}

func currentTelnetEcho(tn *Telnet) telnetEcho {
	remote := tn.RemoteEcho()
	return telnetEcho{remote: remote, hidden: tn.ServerEcho() && !remote}
}

// echoControl builds the "echo" control; both fields are always present.
func echoControl(e telnetEcho) Control {
	remote, hidden := e.remote, e.hidden
	return Control{Type: controlEcho, Remote: &remote, Hidden: &hidden}
}

// runTelnet negotiates and pumps one Telnet session over conn and returns the end reason.
// The initial negotiation goes out right after connect; "echo" is reported initially and
// whenever the character mode or the hidden-input state changes. Every write, negotiation
// replies included, has a write deadline. BBS entries keep their fixed 80x25 geometry.
func (m *Manager) runTelnet(ctx context.Context, e Entry, size Size, client Client, conn net.Conn, stats *sessionStats) string {
	tn := NewTelnet(e.TermType(), size.Cols, size.Rows)
	codec := NewCodec(e.EffectiveCharset())
	out := sessionWriter{w: conn, deadline: conn, stats: stats}
	if err := out.write(tn.Initial()); err != nil {
		return endReason(ctx, ReasonRemoteClosed)
	}
	connected := Control{Type: controlConnected, Protocol: string(ProtocolTelnet), Kind: string(e.Kind), Charset: string(e.EffectiveCharset())}
	if err := client.SendControl(connected); err != nil {
		return endReason(ctx, ReasonRemoteClosed)
	}
	echo := currentTelnetEcho(tn)
	if err := client.SendControl(echoControl(echo)); err != nil {
		return endReason(ctx, ReasonRemoteClosed)
	}
	var text []byte // decode buffer, reused for every read
	return m.pump(ctx, client, conn, stats, pumpHandlers{
		remote: func(p []byte) error {
			data, reply := tn.Feed(p)
			if err := out.write(reply); err != nil {
				return err
			}
			if now := currentTelnetEcho(tn); now != echo {
				echo = now
				if err := client.SendControl(echoControl(now)); err != nil {
					return err
				}
			}
			text = codec.Decode(text[:0], data)
			if len(text) == 0 {
				return nil
			}
			return client.SendData(text)
		},
		input: func(p []byte) error {
			return out.write(tn.EncodeInput(codec.Encode(nil, p), e.Kind))
		},
		resize: func(s Size) error {
			if e.Kind == KindBBS {
				return nil // BBS geometry stays 80x25
			}
			return out.write(tn.Resize(s.Cols, s.Rows))
		},
	})
}
