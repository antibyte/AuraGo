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
