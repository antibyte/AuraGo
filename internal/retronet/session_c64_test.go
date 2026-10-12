package retronet

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
)

// A C64 board speaks raw TCP: the session sends no Telnet negotiation, a leading 0xFF is the
// PETSCII character pi, keystrokes go out as PETSCII with Enter as a bare CR, resizes are
// ignored and no echo control is sent.
func TestManagerC64RawSession(t *testing.T) {
	greeting := []byte{0xFF, 0x0D, 'H', 'I', 0x0D} // pi first: 0xFF followed by a non-command byte
	f := startTelnetFixture(t, telnetFixtureScript{greeting: greeting})
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := sessStart(ctx, m, f.entry(KindC64, CharsetPETSCII), Size{Cols: 132, Rows: 50}, c)

	connected := c.waitControl(t, controlConnected)
	if connected.Protocol != "telnet" || connected.Kind != "c64" || connected.Charset != "petscii" {
		t.Fatalf("connected = %+v, want telnet/c64/petscii", connected)
	}
	c.waitOutput(t, string(C64Glyph(true, 0x5E))) // 0xFF reached the decoder as screen code 0x5E
	c.typeText("a\r")
	c.resize(80, 50)
	c.typeText("\x7f")
	keys := []byte{0x41, 0x0D, 0x14} // PETSCII a, RETURN, DEL
	f.waitReceived(t, keys)

	cancel()
	res := sessAwait(t, done)
	sessExpectResult(t, c, res, CodeNoCarrier, ReasonRemoteClosed)
	if got := f.receivedBytes(); !bytes.Equal(got, keys) {
		t.Fatalf("board received % x, want only % x (no negotiation, no CR NUL, no NAWS)", got, keys)
	}
	if len(c.controlsOfType(controlEcho)) != 0 {
		t.Fatalf("controls = %s, want no echo control for a C64 board", sessFormatControls(c.controlLog()))
	}
	if res.BytesIn != int64(len(greeting)) || res.BytesOut != int64(len(keys)) {
		t.Fatalf("bytes in/out = %d/%d, want %d/%d", res.BytesIn, res.BytesOut, len(greeting), len(keys))
	}
}

// A board whose first chunk opens with IAC DO/WILL gets Telnet for the rest of the call: the
// initial negotiation, TTYPE "PETSCII" and NAWS 40x25 go out, negotiation never reaches the
// decoder, IAC IAC is one 0xFF (pi), Enter stays a bare CR, resizes send no NAWS and the
// server's WILL ECHO produces no echo control.
func TestManagerC64SwitchesToTelnetWhenTheBoardOpensWithIAC(t *testing.T) {
	greeting := []byte{
		tfIAC, tfDO, tfTTYPE, tfIAC, tfDO, tfNAWS, tfIAC, tfWILL, tfECHO, tfIAC, tfWILL, tfSGA,
		tfIAC, tfSB, tfTTYPE, 1, tfIAC, tfSE,
		'H', 'I', tfIAC, tfIAC, 0x0D,
	}
	f := startTelnetFixture(t, telnetFixtureScript{greeting: greeting})
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := sessStart(ctx, m, f.entry(KindC64, CharsetPETSCII), Size{Cols: 132, Rows: 50}, c)

	f.waitReceived(t, NewTelnet("PETSCII", 40, 25).Initial())
	f.waitReceived(t, tfTTypeIs("PETSCII"))
	f.waitReceived(t, tfNAWSFor(40, 25))
	want := string(NewCodec(CharsetPETSCII).Decode(nil, []byte{'H', 'I', 0xFF, 0x0D}))
	c.waitOutput(t, want)
	if got := c.output(); got != want {
		t.Fatalf("output = %q, want only the decoded board text %q", got, want)
	}
	if got := c.waitControl(t, controlConnected); got.Kind != "c64" || got.Charset != "petscii" {
		t.Fatalf("connected = %+v, want kind c64, charset petscii", got)
	}
	c.typeText("a\r")
	c.resize(100, 40)
	c.typeText("\x7f")
	f.waitReceived(t, []byte{0x41, 0x0D, 0x14})

	cancel()
	sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonRemoteClosed)
	received := f.receivedBytes()
	if bytes.Contains(received, tfNAWSFor(100, 40)) {
		t.Fatal("C64 session sent NAWS for a browser resize")
	}
	if bytes.Contains(received, []byte{0x0D, 0x00}) || bytes.Contains(received, []byte{0x0D, 0x0A}) {
		t.Fatalf("Enter must be a bare CR in Telnet mode too; board received % x", received)
	}
	if len(c.controlsOfType(controlEcho)) != 0 {
		t.Fatalf("controls = %s, want no echo control for a C64 board", sessFormatControls(c.controlLog()))
	}
}

// Only the first chunk decides. A board that starts raw and later sends IAC WILL/DO bytes stays
// raw for the whole call: those bytes are PETSCII data (0xFF is pi), and the board gets neither
// Initial() nor a negotiation reply.
func TestManagerC64LaterIACStaysData(t *testing.T) {
	greeting := []byte{'H', 'I', 0x0D}
	later := []byte{tfIAC, tfWILL, tfECHO, tfIAC, tfDO, tfTTYPE} // written once the first key arrived
	f := startTelnetFixture(t, telnetFixtureScript{greeting: greeting, afterInput: later})
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := sessStart(ctx, m, f.entry(KindC64, CharsetPETSCII), Size{Cols: 80, Rows: 25}, c)

	pi := string(C64Glyph(true, 0x5E)) // the lower/upper set is active from the start
	c.waitOutput(t, string(NewCodec(CharsetPETSCII).Decode(nil, greeting)))
	if strings.Contains(c.output(), pi) {
		t.Fatalf("output %q shows pi before the board sent 0xFF", c.output())
	}
	c.typeText("a")
	sessWaitFor(t, func() bool { return strings.Count(c.output(), pi) == 2 }, func() string {
		return "both later 0xFF bytes decoded as pi; output so far: " + strconv.Quote(c.output())
	})
	c.typeText("\r")
	keys := []byte{0x41, 0x0D} // PETSCII a, RETURN
	f.waitReceived(t, keys)

	cancel()
	sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonRemoteClosed)
	if got := f.receivedBytes(); !bytes.Equal(got, keys) {
		t.Fatalf("board received % x, want only the keys % x (no Initial(), no negotiation reply)", got, keys)
	}
	if len(c.controlsOfType(controlEcho)) != 0 {
		t.Fatalf("controls = %s, want no echo control for a C64 board", sessFormatControls(c.controlLog()))
	}
}

func TestOpensTelnet(t *testing.T) {
	cases := []struct {
		name  string
		chunk []byte
		want  bool
	}{
		{"IAC WILL", []byte{tfIAC, tfWILL, tfECHO}, true},
		{"IAC WONT", []byte{tfIAC, tfWONT, tfECHO}, true},
		{"IAC DO", []byte{tfIAC, tfDO, tfTTYPE, 'x'}, true},
		{"IAC DONT", []byte{tfIAC, tfDONT, tfNAWS}, true},
		{"pi then text", []byte{0xFF, 'A'}, false},
		{"pi pi", []byte{0xFF, 0xFF}, false},
		{"IAC SB", []byte{tfIAC, tfSB, tfTTYPE}, false},
		{"IAC SE", []byte{tfIAC, tfSE}, false},
		{"lone pi", []byte{0xFF}, false},
		{"empty", nil, false},
		{"command later", []byte{'A', tfIAC, tfDO, tfTTYPE}, false},
	}
	for _, tc := range cases {
		if got := opensTelnet(tc.chunk); got != tc.want {
			t.Errorf("%s: opensTelnet(% x) = %v, want %v", tc.name, tc.chunk, got, tc.want)
		}
	}
}

func TestSessionSizePinsFixedGeometries(t *testing.T) {
	browser := Size{Cols: 132, Rows: 50}
	cases := []struct {
		name  string
		entry Entry
		want  Size
	}{
		{"c64", Entry{Protocol: ProtocolTelnet, Kind: KindC64}, Size{Cols: 40, Rows: 25}},
		{"bbs", Entry{Protocol: ProtocolTelnet, Kind: KindBBS}, Size{Cols: 80, Rows: 25}},
		{"world", Entry{Protocol: ProtocolTelnet, Kind: KindWorld}, browser},
		{"ssh", Entry{Protocol: ProtocolSSH}, browser},
	}
	for _, tc := range cases {
		if got := sessionSize(tc.entry, browser); got != tc.want {
			t.Errorf("%s: sessionSize = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}
