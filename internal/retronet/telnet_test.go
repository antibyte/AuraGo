package retronet

import (
	"bytes"
	"fmt"
	"testing"
)

// seq builds a byte sequence from bytes, ints (0-255), strings and byte slices.
func seq(parts ...any) []byte {
	var out []byte
	for _, p := range parts {
		switch v := p.(type) {
		case byte:
			out = append(out, v)
		case int:
			out = append(out, byte(v))
		case string:
			out = append(out, v...)
		case []byte:
			out = append(out, v...)
		default:
			panic(fmt.Sprintf("seq: unsupported part %T", p))
		}
	}
	return out
}

// iac builds IAC <cmd> <opt>.
func iac(cmd, opt byte) []byte { return []byte{telnetIAC, cmd, opt} }

func TestTelnetIACIACCollapsesInData(t *testing.T) {
	tn := NewTelnet(termTypeXterm, 80, 24)
	data, reply := tn.Feed(seq("a", telnetIAC, telnetIAC, "b"))
	if !bytes.Equal(data, []byte("a\xffb")) {
		t.Fatalf("data = % X, want 61 FF 62", data)
	}
	if len(reply) != 0 {
		t.Fatalf("reply = % X, want none", reply)
	}
}

func TestTelnetInitial(t *testing.T) {
	tn := NewTelnet(termTypeXterm, 80, 24)
	want := seq(telnetIAC, telnetWILL, optTTYPE, telnetIAC, telnetWILL, optNAWS, telnetIAC, telnetDO, optSGA, telnetIAC, telnetWILL, optSGA)
	if got := tn.Initial(); !bytes.Equal(got, want) {
		t.Fatalf("Initial() = % X, want % X", got, want)
	}
	if got := tn.Initial(); len(got) != 0 {
		t.Fatalf("second Initial() = % X, want nothing", got)
	}
}

func TestTelnetOptionTable(t *testing.T) {
	naws80x24 := seq(telnetIAC, telnetSB, optNAWS, 0, 80, 0, 24, telnetIAC, telnetSE)
	tests := []struct {
		name string
		cmd  byte
		opt  byte
		want []byte
	}{
		{"server WILL BINARY accepted", telnetWILL, optBinary, iac(telnetDO, optBinary)},
		{"server DO BINARY accepted", telnetDO, optBinary, iac(telnetWILL, optBinary)},
		{"server WILL ECHO accepted", telnetWILL, optEcho, iac(telnetDO, optEcho)},
		{"server DO ECHO refused", telnetDO, optEcho, iac(telnetWONT, optEcho)},
		{"server WILL SGA accepted", telnetWILL, optSGA, iac(telnetDO, optSGA)},
		{"server DO SGA accepted", telnetDO, optSGA, iac(telnetWILL, optSGA)},
		{"server DO TTYPE accepted", telnetDO, optTTYPE, iac(telnetWILL, optTTYPE)},
		{"server WILL TTYPE refused", telnetWILL, optTTYPE, iac(telnetDONT, optTTYPE)},
		{"server DO NAWS accepted with size", telnetDO, optNAWS, append(iac(telnetWILL, optNAWS), naws80x24...)},
		{"server WILL NAWS refused", telnetWILL, optNAWS, iac(telnetDONT, optNAWS)},
		{"COMPRESS2 refused", telnetWILL, 86, iac(telnetDONT, 86)},
		{"COMPRESS refused", telnetWILL, 85, iac(telnetDONT, 85)},
		{"GMCP refused", telnetWILL, 201, iac(telnetDONT, 201)},
		{"MSDP refused", telnetWILL, 69, iac(telnetDONT, 69)},
		{"MSSP refused", telnetWILL, 70, iac(telnetDONT, 70)},
		{"server WILL CHARSET refused", telnetWILL, 42, iac(telnetDONT, 42)},
		{"server DO CHARSET refused", telnetDO, 42, iac(telnetWONT, 42)},
		{"LINEMODE refused", telnetDO, 34, iac(telnetWONT, 34)},
		{"NEW-ENVIRON refused", telnetDO, 39, iac(telnetWONT, 39)},
		{"WONT for a disabled option is silent", telnetWONT, optEcho, nil},
		{"DONT for a disabled option is silent", telnetDONT, optNAWS, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tn := NewTelnet(termTypeXterm, 80, 24)
			data, reply := tn.Feed(iac(tc.cmd, tc.opt))
			if len(data) != 0 {
				t.Fatalf("data = % X, want none", data)
			}
			if !bytes.Equal(reply, tc.want) {
				t.Fatalf("first reply = % X, want % X", reply, tc.want)
			}
			if _, again := tn.Feed(iac(tc.cmd, tc.opt)); len(again) != 0 {
				t.Fatalf("repeated request answered again: % X", again)
			}
		})
	}
}

func TestTelnetAcknowledgementsOfInitialRequestsAreSilent(t *testing.T) {
	tn := NewTelnet(termTypeANSI, 80, 25)
	tn.Initial()
	_, reply := tn.Feed(seq(telnetIAC, telnetDO, optTTYPE, telnetIAC, telnetWILL, optSGA, telnetIAC, telnetDO, optSGA))
	if len(reply) != 0 {
		t.Fatalf("acknowledgements answered: % X", reply)
	}
	_, reply = tn.Feed(iac(telnetDO, optNAWS))
	want := seq(telnetIAC, telnetSB, optNAWS, 0, 80, 0, 25, telnetIAC, telnetSE)
	if !bytes.Equal(reply, want) {
		t.Fatalf("DO NAWS after our WILL = % X, want only the size % X", reply, want)
	}
}

func TestTelnetRefusalOfInitialRequestsIsSilent(t *testing.T) {
	tn := NewTelnet(termTypeANSI, 80, 25)
	tn.Initial()
	_, reply := tn.Feed(seq(
		telnetIAC, telnetDONT, optNAWS,
		telnetIAC, telnetDONT, optTTYPE,
		telnetIAC, telnetWONT, optSGA,
		telnetIAC, telnetDONT, optSGA,
	))
	if len(reply) != 0 {
		t.Fatalf("refusals answered: % X", reply)
	}
	if _, reply = tn.Feed(seq(telnetIAC, telnetSB, optTTYPE, ttypeSEND, telnetIAC, telnetSE)); len(reply) != 0 {
		t.Fatalf("TTYPE SEND answered after DONT TTYPE: % X", reply)
	}
}

func TestTelnetStateChangesAreAcknowledged(t *testing.T) {
	tn := NewTelnet(termTypeXterm, 80, 24)
	steps := []struct {
		in   []byte
		want []byte
	}{
		{iac(telnetWILL, optEcho), iac(telnetDO, optEcho)},
		{iac(telnetWONT, optEcho), iac(telnetDONT, optEcho)},
		{iac(telnetWONT, optEcho), nil},
		{iac(telnetWILL, optEcho), iac(telnetDO, optEcho)},
		{iac(telnetDO, optBinary), iac(telnetWILL, optBinary)},
		{iac(telnetDONT, optBinary), iac(telnetWONT, optBinary)},
		{iac(telnetDONT, optBinary), nil},
	}
	for i, step := range steps {
		if _, reply := tn.Feed(step.in); !bytes.Equal(reply, step.want) {
			t.Fatalf("step %d: Feed(% X) reply = % X, want % X", i, step.in, reply, step.want)
		}
	}
}

func TestTelnetTTYPEAnswerPerKind(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindBBS, "ANSI"},
		{KindWorld, "XTERM-256COLOR"},
	}
	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			entry := Entry{Protocol: ProtocolTelnet, Kind: tc.kind}
			tn := NewTelnet(entry.TermType(), 80, 25)
			tn.Initial()
			send := seq(telnetIAC, telnetSB, optTTYPE, ttypeSEND, telnetIAC, telnetSE)
			want := seq(telnetIAC, telnetSB, optTTYPE, ttypeIS, tc.want, telnetIAC, telnetSE)
			_, reply := tn.Feed(append(iac(telnetDO, optTTYPE), send...))
			if !bytes.Equal(reply, want) {
				t.Fatalf("TTYPE SEND reply = % X, want % X", reply, want)
			}
			if _, reply = tn.Feed(send); !bytes.Equal(reply, want) {
				t.Fatalf("second TTYPE SEND reply = % X, want % X", reply, want)
			}
		})
	}
}

func TestTelnetIgnoresOtherSubnegotiations(t *testing.T) {
	tn := NewTelnet(termTypeXterm, 80, 24)
	tn.Initial()
	tn.Feed(iac(telnetDO, optTTYPE))
	data, reply := tn.Feed(seq(
		telnetIAC, telnetSB, optTTYPE, ttypeIS, "VT100", telnetIAC, telnetSE,
		telnetIAC, telnetSB, 201, "Core.Hello {}", telnetIAC, telnetSE,
		"ok",
	))
	if len(reply) != 0 {
		t.Fatalf("reply = % X, want none", reply)
	}
	if string(data) != "ok" {
		t.Fatalf("data = %q, want %q", data, "ok")
	}
}

func TestTelnetMalformedAndOversizedSubnegotiations(t *testing.T) {
	tn := NewTelnet(termTypeXterm, 80, 24)
	tn.Initial()
	tn.Feed(iac(telnetDO, optTTYPE))
	// IAC WILL inside a subnegotiation ends it; the WILL is still handled.
	data, reply := tn.Feed(seq(telnetIAC, telnetSB, optTTYPE, ttypeSEND, telnetIAC, telnetWILL, optEcho, "x"))
	if want := iac(telnetDO, optEcho); !bytes.Equal(reply, want) {
		t.Fatalf("malformed SB reply = % X, want % X", reply, want)
	}
	if string(data) != "x" {
		t.Fatalf("malformed SB data = %q, want %q", data, "x")
	}
	// An oversized TTYPE SEND is consumed and ignored.
	huge := seq(telnetIAC, telnetSB, optTTYPE, ttypeSEND, bytes.Repeat([]byte("z"), 2*maxSubnegotiation), telnetIAC, telnetSE, "y")
	data, reply = tn.Feed(huge)
	if len(reply) != 0 {
		t.Fatalf("oversized SB answered: % X", reply)
	}
	if string(data) != "y" {
		t.Fatalf("oversized SB data = %q, want %q", data, "y")
	}
}

func TestTelnetRemoteEcho(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want bool
	}{
		{"nothing negotiated", nil, false},
		{"echo only", iac(telnetWILL, optEcho), false},
		{"sga only", iac(telnetWILL, optSGA), false},
		{"echo and sga", seq(telnetIAC, telnetWILL, optEcho, telnetIAC, telnetWILL, optSGA), true},
		{"sga then echo", seq(telnetIAC, telnetWILL, optSGA, telnetIAC, telnetWILL, optEcho), true},
		{"echo withdrawn", seq(telnetIAC, telnetWILL, optEcho, telnetIAC, telnetWILL, optSGA, telnetIAC, telnetWONT, optEcho), false},
		{"sga withdrawn", seq(telnetIAC, telnetWILL, optEcho, telnetIAC, telnetWILL, optSGA, telnetIAC, telnetWONT, optSGA), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tn := NewTelnet(termTypeXterm, 80, 24)
			tn.Feed(tc.in)
			if got := tn.RemoteEcho(); got != tc.want {
				t.Fatalf("RemoteEcho() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestTelnetServerEchoIsIndependentOfSGA covers MUDs that hide a password
// prompt with WILL ECHO but never enable SGA: ServerEcho follows the ECHO
// option alone, RemoteEcho still needs ECHO and SGA.
func TestTelnetServerEchoIsIndependentOfSGA(t *testing.T) {
	steps := []struct {
		name       string
		in         []byte
		serverEcho bool
		remoteEcho bool
	}{
		{"after initial", nil, false, false},
		{"will echo only", iac(telnetWILL, optEcho), true, false},
		{"will sga", iac(telnetWILL, optSGA), true, true},
		{"wont echo", iac(telnetWONT, optEcho), false, false},
		{"will echo again", iac(telnetWILL, optEcho), true, true},
		{"wont sga", iac(telnetWONT, optSGA), true, false},
	}
	run := func(label string, chunk func(in []byte) [][]byte) {
		t.Helper()
		tn := NewTelnet(termTypeXterm, 80, 24)
		tn.Initial()
		for _, step := range steps {
			for _, part := range chunk(step.in) {
				tn.Feed(part)
			}
			if got := tn.ServerEcho(); got != step.serverEcho {
				t.Fatalf("%s, %s: ServerEcho() = %v, want %v", label, step.name, got, step.serverEcho)
			}
			if got := tn.RemoteEcho(); got != step.remoteEcho {
				t.Fatalf("%s, %s: RemoteEcho() = %v, want %v", label, step.name, got, step.remoteEcho)
			}
		}
	}
	run("whole", func(in []byte) [][]byte { return [][]byte{in} })
	for split := 0; split <= 3; split++ {
		run(fmt.Sprintf("split at %d", split), func(in []byte) [][]byte {
			at := min(split, len(in))
			return [][]byte{in[:at], in[at:]}
		})
	}
	run("byte by byte", func(in []byte) [][]byte {
		parts := make([][]byte, len(in))
		for i := range in {
			parts[i] = in[i : i+1]
		}
		return parts
	})
}

func TestTelnetServerEchoWithoutNegotiation(t *testing.T) {
	tn := NewTelnet(termTypeXterm, 80, 24)
	if tn.ServerEcho() {
		t.Fatal("ServerEcho() = true before any negotiation")
	}
	tn.Feed(iac(telnetDO, optEcho)) // we refuse to echo for the server; that is not ServerEcho
	if tn.ServerEcho() {
		t.Fatal("ServerEcho() = true after DO ECHO (our side refused)")
	}
}

func TestTelnetSequencesSplitAtEveryByteBoundary(t *testing.T) {
	stream := seq(
		"Hi",
		telnetIAC, telnetWILL, optEcho,
		telnetIAC, telnetWILL, optSGA,
		"a", telnetIAC, telnetIAC, "b",
		telnetIAC, telnetDO, optTTYPE,
		telnetIAC, telnetSB, optTTYPE, ttypeSEND, telnetIAC, telnetSE,
		telnetIAC, telnetDO, optNAWS,
		telnetIAC, telnetSB, 200, 1, telnetIAC, telnetIAC, 2, telnetIAC, telnetSE,
		telnetIAC, telnetWILL, 86,
		telnetIAC, 249, // GA
		"\x1b[0m",
		telnetIAC, 241, // NOP
		"end",
	)
	wantData := seq("Hi", "a", 255, "b", "\x1b[0m", "end")
	wantReply := seq(
		telnetIAC, telnetDO, optEcho,
		telnetIAC, telnetSB, optTTYPE, ttypeIS, "XTERM-256COLOR", telnetIAC, telnetSE,
		telnetIAC, telnetSB, optNAWS, 0, 80, 0, 24, telnetIAC, telnetSE,
		telnetIAC, telnetDONT, 86,
	)
	run := func(label string, chunks ...[]byte) {
		t.Helper()
		tn := NewTelnet(termTypeXterm, 80, 24)
		tn.Initial()
		var data, reply []byte
		for _, chunk := range chunks {
			d, r := tn.Feed(chunk)
			data = append(data, d...)
			reply = append(reply, r...)
		}
		if !bytes.Equal(data, wantData) {
			t.Fatalf("%s: data = % X, want % X", label, data, wantData)
		}
		if !bytes.Equal(reply, wantReply) {
			t.Fatalf("%s: reply = % X, want % X", label, reply, wantReply)
		}
		if !tn.RemoteEcho() {
			t.Fatalf("%s: RemoteEcho() = false, want true", label)
		}
	}
	run("whole stream", stream)
	for i := 0; i <= len(stream); i++ {
		run(fmt.Sprintf("split at %d", i), stream[:i], stream[i:])
	}
	single := make([][]byte, len(stream))
	for i := range stream {
		single[i] = stream[i : i+1]
	}
	run("byte by byte", single...)
}

func TestTelnetNAWSEncoding(t *testing.T) {
	tests := []struct {
		name       string
		cols, rows int
		want       []byte
	}{
		{"80x24", 80, 24, seq(telnetIAC, telnetSB, optNAWS, 0, 80, 0, 24, telnetIAC, telnetSE)},
		{"width 255 escaped", 255, 25, seq(telnetIAC, telnetSB, optNAWS, 0, 255, 255, 0, 25, telnetIAC, telnetSE)},
		{"height 255 escaped", 132, 255, seq(telnetIAC, telnetSB, optNAWS, 0, 132, 0, 255, 255, telnetIAC, telnetSE)},
		{"width 511 escapes low byte", 511, 50, seq(telnetIAC, telnetSB, optNAWS, 1, 255, 255, 0, 50, telnetIAC, telnetSE)},
		{"clamped to 16 bits", 70000, -3, seq(telnetIAC, telnetSB, optNAWS, 255, 255, 255, 255, 0, 0, telnetIAC, telnetSE)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tn := NewTelnet(termTypeXterm, 80, 24)
			if got := tn.Resize(tc.cols, tc.rows); got != nil {
				t.Fatalf("Resize() before NAWS agreement = % X, want nil", got)
			}
			_, reply := tn.Feed(iac(telnetDO, optNAWS))
			if want := append(iac(telnetWILL, optNAWS), tc.want...); !bytes.Equal(reply, want) {
				t.Fatalf("DO NAWS reply = % X, want % X", reply, want)
			}
			if got := tn.Resize(tc.cols, tc.rows); !bytes.Equal(got, tc.want) {
				t.Fatalf("Resize() = % X, want % X", got, tc.want)
			}
		})
	}
}

func TestTelnetResizeAfterNAWSWithdrawn(t *testing.T) {
	tn := NewTelnet(termTypeXterm, 80, 24)
	tn.Initial()
	tn.Feed(iac(telnetDO, optNAWS))
	if got := tn.Resize(100, 40); len(got) == 0 {
		t.Fatal("Resize() while NAWS is active returned nothing")
	}
	if _, reply := tn.Feed(iac(telnetDONT, optNAWS)); !bytes.Equal(reply, iac(telnetWONT, optNAWS)) {
		t.Fatalf("DONT NAWS reply = % X, want IAC WONT NAWS", reply)
	}
	if got := tn.Resize(120, 40); got != nil {
		t.Fatalf("Resize() after DONT NAWS = % X, want nil", got)
	}
}

func TestTelnetEncodeInput(t *testing.T) {
	tests := []struct {
		name   string
		kind   Kind
		binary bool
		in     string
		want   string
	}{
		{"world enter", KindWorld, false, "\r", "\r\n"},
		{"world enter with lf not doubled", KindWorld, false, "\r\n", "\r\n"},
		{"world line", KindWorld, false, "look\r", "look\r\n"},
		{"world two enters", KindWorld, false, "\r\r", "\r\n\r\n"},
		{"world ignores binary", KindWorld, true, "\r", "\r\n"},
		{"world bare lf unchanged", KindWorld, false, "\n", "\n"},
		{"bbs enter", KindBBS, false, "\r", "\r\x00"},
		{"bbs enter with lf", KindBBS, false, "\r\n", "\r\x00"},
		{"bbs enter in binary", KindBBS, true, "\r", "\r"},
		{"bbs keys", KindBBS, false, "y\x1b[A\x7f", "y\x1b[A\x7f"},
		{"iac escaped bbs", KindBBS, false, "\xff", "\xff\xff"},
		{"iac escaped world", KindWorld, false, "x\xffy", "x\xff\xffy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tn := NewTelnet(termTypeANSI, 80, 25)
			if tc.binary {
				if _, reply := tn.Feed(iac(telnetDO, optBinary)); !bytes.Equal(reply, iac(telnetWILL, optBinary)) {
					t.Fatalf("DO BINARY reply = % X, want IAC WILL BINARY", reply)
				}
			}
			if got := tn.EncodeInput([]byte(tc.in), tc.kind); string(got) != tc.want {
				t.Fatalf("EncodeInput(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTelnetEnterFollowsBinaryState(t *testing.T) {
	tn := NewTelnet(termTypeANSI, 80, 25)
	tn.Feed(iac(telnetDO, optBinary))
	if got := tn.EncodeInput([]byte("\r"), KindBBS); string(got) != "\r" {
		t.Fatalf("Enter with BINARY = %q, want CR", got)
	}
	if _, reply := tn.Feed(iac(telnetDONT, optBinary)); !bytes.Equal(reply, iac(telnetWONT, optBinary)) {
		t.Fatalf("DONT BINARY reply = % X, want IAC WONT BINARY", reply)
	}
	if got := tn.EncodeInput([]byte("\r"), KindBBS); string(got) != "\r\x00" {
		t.Fatalf("Enter after BINARY ended = %q, want CR NUL", got)
	}
}

func TestTelnetIACIACRoundTrip(t *testing.T) {
	tn := NewTelnet(termTypeXterm, 80, 24)
	out := tn.EncodeInput([]byte{0xFF, 'a', 0xFF}, KindWorld)
	if want := []byte{0xFF, 0xFF, 'a', 0xFF, 0xFF}; !bytes.Equal(out, want) {
		t.Fatalf("EncodeInput = % X, want % X", out, want)
	}
	peer := NewTelnet(termTypeXterm, 80, 24)
	if data, _ := peer.Feed(out); !bytes.Equal(data, []byte{0xFF, 'a', 0xFF}) {
		t.Fatalf("Feed(EncodeInput) = % X, want FF 61 FF", data)
	}
}
