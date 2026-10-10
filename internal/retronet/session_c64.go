package retronet

import (
	"context"
	"net"
)

// runC64 pumps one Commodore 64 session (KindC64) over conn and returns the end reason.
//
// C64 boards sit behind tcpser-style bridges that speak raw TCP, where 0xFF is the PETSCII
// character pi, not Telnet IAC. So the session sends no Telnet negotiation, passes the
// board's bytes to the PETSCII codec unchanged and writes encoded input without IAC
// escaping. Only when the very first chunk from the board begins with IAC WILL/WONT/DO/DONT
// does the call switch to Telnet for its remaining lifetime: Initial() goes out at that
// moment, Feed strips negotiation and answers TTYPE "PETSCII" and NAWS 40x25, and input is
// IAC-escaped. The PETSCII codec maps Enter ("\r" -> 0x0D) in both modes; geometry stays
// 40x25 (c64Cols x c64Rows, the engine's screen), browser resizes are ignored and no "echo"
// control is sent (C64 boards always echo in character mode).
func (m *Manager) runC64(ctx context.Context, client Client, conn net.Conn, stats *sessionStats) string {
	codec := NewCodec(CharsetPETSCII) // stateful: one per session
	out := sessionWriter{w: conn, deadline: conn, stats: stats}
	connected := Control{Type: controlConnected, Protocol: string(ProtocolTelnet), Kind: string(KindC64), Charset: string(CharsetPETSCII)}
	if err := client.SendControl(connected); err != nil {
		return endReason(ctx, ReasonRemoteClosed)
	}
	var tn *Telnet // set when the board opened with Telnet
	first := true
	var text []byte // decode buffer, reused for every read
	return m.pump(ctx, client, conn, stats, pumpHandlers{
		remote: func(p []byte) error {
			if first {
				first = false
				if opensTelnet(p) {
					tn = NewTelnet(termTypePETSCII, c64Cols, c64Rows)
					if err := out.write(tn.Initial()); err != nil {
						return err
					}
				}
			}
			data := p
			if tn != nil {
				var reply []byte
				data, reply = tn.Feed(p)
				if err := out.write(reply); err != nil {
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
			encoded := codec.Encode(nil, p)
			if tn != nil {
				encoded = appendEscaped(nil, encoded)
			}
			return out.write(encoded)
		},
		resize: func(Size) error { return nil }, // C64 geometry stays 40x25
	})
}

// opensTelnet reports whether p, the first chunk a C64 board sent, begins with IAC followed
// by WILL, WONT, DO or DONT. Any other start, a lone 0xFF included, keeps the call raw.
func opensTelnet(p []byte) bool {
	return len(p) >= 2 && p[0] == telnetIAC && p[1] >= telnetWILL && p[1] <= telnetDONT
}
