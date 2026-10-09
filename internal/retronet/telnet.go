package retronet

import "sync"

// Telnet command bytes (RFC 854).
const (
	telnetSE   byte = 240
	telnetSB   byte = 250
	telnetWILL byte = 251
	telnetWONT byte = 252
	telnetDO   byte = 253
	telnetDONT byte = 254
	telnetIAC  byte = 255
)

// Telnet options the negotiator accepts (everything else is refused).
const (
	optBinary byte = 0  // RFC 856
	optEcho   byte = 1  // RFC 857
	optSGA    byte = 3  // RFC 858
	optTTYPE  byte = 24 // RFC 1091
	optNAWS   byte = 31 // RFC 1073
)

// TTYPE subnegotiation verbs (RFC 1091).
const (
	ttypeIS   byte = 0
	ttypeSEND byte = 1
)

// maxSubnegotiation bounds the buffered payload of one IAC SB ... IAC SE.
// Longer subnegotiations are consumed and ignored.
const maxSubnegotiation = 1024

// optState is the RFC 1143 state of one option in one direction.
type optState uint8

const (
	optNo      optState = iota // disabled
	optWantYes                 // we asked to enable it and wait for the answer
	optYes                     // enabled
)

// parseState is the position of the byte parser inside the Telnet stream.
type parseState uint8

const (
	psData   parseState = iota // application data
	psIAC                      // after IAC
	psWill                     // after IAC WILL
	psWont                     // after IAC WONT
	psDo                       // after IAC DO
	psDont                     // after IAC DONT
	psSB                       // after IAC SB, expecting the option byte
	psSBData                   // inside a subnegotiation
	psSBIAC                    // after IAC inside a subnegotiation
)

// Telnet negotiates options for one connection (RFC 854 state machine). It
// keeps its parser state across Feed calls, so sequences split across reads
// are handled. It is safe for one reading and one writing goroutine.
type Telnet struct {
	mu         sync.Mutex
	termType   string
	cols, rows int
	state      parseState
	sbOpt      byte
	sb         []byte
	sbOverflow bool
	us         [256]optState // options we perform (server sends DO/DONT)
	him        [256]optState // options the server performs (server sends WILL/WONT)
	refusedUs  [256]bool     // DO for an unsupported option was answered once
	refusedHim [256]bool     // WILL for an unsupported option was answered once
	started    bool
}

// NewTelnet creates the negotiator for one connection. termType is announced
// through TTYPE (Entry.TermType), cols and rows through NAWS.
func NewTelnet(termType string, cols, rows int) *Telnet {
	return &Telnet{termType: termType, cols: cols, rows: rows}
}

// Initial returns the negotiation the client sends right after connecting:
// IAC WILL TTYPE, IAC WILL NAWS, IAC DO SGA, IAC WILL SGA. Later calls return nil.
func (t *Telnet) Initial() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.started {
		return nil
	}
	t.started = true
	var out []byte
	out = t.requestUs(out, optTTYPE)
	out = t.requestUs(out, optNAWS)
	out = t.requestHim(out, optSGA)
	out = t.requestUs(out, optSGA)
	return out
}

// Feed consumes bytes from the server. It returns application data (IAC sequences removed,
// IAC IAC collapsed) and the negotiation bytes that must be written back to the server.
// Sequences split across calls are handled.
func (t *Telnet) Feed(in []byte) (data, reply []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	data = make([]byte, 0, len(in))
	for _, b := range in {
		data, reply = t.step(b, data, reply)
	}
	return data, reply
}

// RemoteEcho reports kludge character mode: the server WILL ECHO and WILL SGA.
func (t *Telnet) RemoteEcho() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.him[optEcho] == optYes && t.him[optSGA] == optYes
}

// step advances the parser by one byte.
func (t *Telnet) step(b byte, data, reply []byte) ([]byte, []byte) {
	switch t.state {
	case psData:
		if b == telnetIAC {
			t.state = psIAC
		} else {
			data = append(data, b)
		}
	case psIAC:
		t.state = psData
		switch b {
		case telnetIAC:
			data = append(data, telnetIAC)
		case telnetWILL:
			t.state = psWill
		case telnetWONT:
			t.state = psWont
		case telnetDO:
			t.state = psDo
		case telnetDONT:
			t.state = psDont
		case telnetSB:
			t.state = psSB
		}
		// Every other command (NOP, GA, DM, AYT, ...) carries no option and is dropped.
	case psWill:
		t.state = psData
		reply = t.onWill(reply, b)
	case psWont:
		t.state = psData
		reply = t.onWont(reply, b)
	case psDo:
		t.state = psData
		reply = t.onDo(reply, b)
	case psDont:
		t.state = psData
		reply = t.onDont(reply, b)
	case psSB:
		t.sbOpt = b
		t.sb = t.sb[:0]
		t.sbOverflow = false
		t.state = psSBData
	case psSBData:
		if b == telnetIAC {
			t.state = psSBIAC
		} else {
			t.appendSB(b)
		}
	case psSBIAC:
		switch b {
		case telnetSE:
			t.state = psData
			reply = t.onSubnegotiation(reply)
		case telnetIAC:
			t.state = psSBData
			t.appendSB(telnetIAC)
		default:
			// IAC <command> inside a subnegotiation is malformed: drop the
			// subnegotiation and handle the command as if it stood alone.
			t.state = psIAC
			return t.step(b, data, reply)
		}
	}
	return data, reply
}

func (t *Telnet) appendSB(b byte) {
	if len(t.sb) >= maxSubnegotiation {
		t.sbOverflow = true
		return
	}
	t.sb = append(t.sb, b)
}

// acceptHim lists the options the server may enable on its side.
func acceptHim(opt byte) bool {
	return opt == optBinary || opt == optEcho || opt == optSGA
}

// acceptUs lists the options we agree to enable on our side.
func acceptUs(opt byte) bool {
	return opt == optBinary || opt == optSGA || opt == optTTYPE || opt == optNAWS
}

// onWill answers IAC WILL opt. Replies are sent only on state changes; an
// unsupported option is refused once.
func (t *Telnet) onWill(reply []byte, opt byte) []byte {
	if !acceptHim(opt) {
		if t.refusedHim[opt] {
			return reply
		}
		t.refusedHim[opt] = true
		return append(reply, telnetIAC, telnetDONT, opt)
	}
	switch t.him[opt] {
	case optYes:
		return reply
	case optWantYes:
		t.him[opt] = optYes // acknowledges our DO
		return reply
	default:
		t.him[opt] = optYes
		return append(reply, telnetIAC, telnetDO, opt)
	}
}

// onWont answers IAC WONT opt.
func (t *Telnet) onWont(reply []byte, opt byte) []byte {
	switch t.him[opt] {
	case optYes:
		t.him[opt] = optNo
		return append(reply, telnetIAC, telnetDONT, opt)
	case optWantYes:
		t.him[opt] = optNo // refuses our DO
	}
	return reply
}

// onDo answers IAC DO opt. Agreeing to NAWS also sends the current size.
func (t *Telnet) onDo(reply []byte, opt byte) []byte {
	if !acceptUs(opt) {
		if t.refusedUs[opt] {
			return reply
		}
		t.refusedUs[opt] = true
		return append(reply, telnetIAC, telnetWONT, opt)
	}
	switch t.us[opt] {
	case optYes:
		return reply
	case optWantYes:
		t.us[opt] = optYes // acknowledges our WILL
	default:
		t.us[opt] = optYes
		reply = append(reply, telnetIAC, telnetWILL, opt)
	}
	if opt == optNAWS {
		reply = t.appendNAWS(reply)
	}
	return reply
}

// onDont answers IAC DONT opt.
func (t *Telnet) onDont(reply []byte, opt byte) []byte {
	switch t.us[opt] {
	case optYes:
		t.us[opt] = optNo
		return append(reply, telnetIAC, telnetWONT, opt)
	case optWantYes:
		t.us[opt] = optNo // refuses our WILL
	}
	return reply
}

// onSubnegotiation handles a complete IAC SB ... IAC SE. Only TTYPE SEND is
// answered; every other subnegotiation is ignored.
func (t *Telnet) onSubnegotiation(reply []byte) []byte {
	if t.sbOverflow || t.sbOpt != optTTYPE || len(t.sb) == 0 || t.sb[0] != ttypeSEND || t.us[optTTYPE] == optNo {
		return reply
	}
	reply = append(reply, telnetIAC, telnetSB, optTTYPE, ttypeIS)
	reply = appendEscaped(reply, []byte(t.termType))
	return append(reply, telnetIAC, telnetSE)
}

// requestUs sends IAC WILL opt unless the option is already on or requested.
func (t *Telnet) requestUs(dst []byte, opt byte) []byte {
	if t.us[opt] != optNo {
		return dst
	}
	t.us[opt] = optWantYes
	return append(dst, telnetIAC, telnetWILL, opt)
}

// requestHim sends IAC DO opt unless the option is already on or requested.
func (t *Telnet) requestHim(dst []byte, opt byte) []byte {
	if t.him[opt] != optNo {
		return dst
	}
	t.him[opt] = optWantYes
	return append(dst, telnetIAC, telnetDO, opt)
}

// appendNAWS appends IAC SB NAWS <width16> <height16> IAC SE with 0xFF bytes doubled.
func (t *Telnet) appendNAWS(dst []byte) []byte {
	cols, rows := clampWindowSize(t.cols), clampWindowSize(t.rows)
	dst = append(dst, telnetIAC, telnetSB, optNAWS)
	dst = appendEscaped(dst, []byte{byte(cols >> 8), byte(cols), byte(rows >> 8), byte(rows)})
	return append(dst, telnetIAC, telnetSE)
}

// clampWindowSize limits a NAWS dimension to the 16-bit wire range.
func clampWindowSize(n int) int {
	if n < 0 {
		return 0
	}
	if n > 0xFFFF {
		return 0xFFFF
	}
	return n
}

// appendEscaped appends p with every 0xFF doubled (IAC IAC).
func appendEscaped(dst, p []byte) []byte {
	for _, b := range p {
		if b == telnetIAC {
			dst = append(dst, telnetIAC, telnetIAC)
		} else {
			dst = append(dst, b)
		}
	}
	return dst
}

// Resize stores the size and returns the NAWS subnegotiation to send (nil if NAWS is not active).
func (t *Telnet) Resize(cols, rows int) []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cols, t.rows = cols, rows
	if t.us[optNAWS] != optYes {
		return nil
	}
	return t.appendNAWS(nil)
}

// EncodeInput escapes 0xFF as IAC IAC and maps Enter. The browser always sends Enter as "\r".
// KindWorld: "\r" -> "\r\n" (an already following "\n" is not doubled).
// KindBBS: "\r" -> "\r\x00" unless we transmit BINARY (then "\r").
// For both kinds an "\n" directly after "\r" in p belongs to that Enter.
func (t *Telnet) EncodeInput(p []byte, kind Kind) []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]byte, 0, len(p)+4)
	for i := 0; i < len(p); i++ {
		switch b := p[i]; b {
		case telnetIAC:
			out = append(out, telnetIAC, telnetIAC)
		case '\r':
			out = t.appendEnter(out, kind)
			if i+1 < len(p) && p[i+1] == '\n' {
				i++
			}
		default:
			out = append(out, b)
		}
	}
	return out
}

// appendEnter appends the Enter sequence for kind.
func (t *Telnet) appendEnter(dst []byte, kind Kind) []byte {
	switch {
	case kind == KindWorld:
		return append(dst, '\r', '\n')
	case t.us[optBinary] == optYes:
		return append(dst, '\r')
	default:
		return append(dst, '\r', 0)
	}
}
