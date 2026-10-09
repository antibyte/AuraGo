package retronet

import (
	"bytes"
	"fmt"
	"testing"
)

// feedChunks feeds the chunks in order and returns the concatenated results.
func feedChunks(tn *Telnet, chunks ...[]byte) (data, reply []byte) {
	for _, chunk := range chunks {
		d, r := tn.Feed(chunk)
		data = append(data, d...)
		reply = append(reply, r...)
	}
	return data, reply
}

func TestTelnetEmptySubnegotiationDoesNotSwallowData(t *testing.T) {
	answeredTTYPE := seq(iac(telnetDO, optTTYPE), telnetIAC, telnetSB, optTTYPE, ttypeSEND, telnetIAC, telnetSE)
	tests := []struct {
		name      string
		pre       []byte // fed first, its output is ignored
		in        []byte
		wantData  string
		wantReply []byte
	}{
		{"empty sb then data", nil, seq(telnetIAC, telnetSB, telnetIAC, telnetSE, "h", "i"), "hi", nil},
		{"empty sb between data", nil, seq("a", telnetIAC, telnetSB, telnetIAC, telnetSE, "b"), "ab", nil},
		{"empty sb twice", nil, seq(telnetIAC, telnetSB, telnetIAC, telnetSE, telnetIAC, telnetSB, telnetIAC, telnetSE, "ok"), "ok", nil},
		{"empty sb then command", nil, seq(telnetIAC, telnetSB, telnetIAC, telnetWILL, optEcho, "x"), "x", iac(telnetDO, optEcho)},
		{"stale ttype state is not answered", answeredTTYPE, seq(telnetIAC, telnetSB, telnetIAC, telnetSE, "z"), "z", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			check := func(label string, chunks ...[]byte) {
				t.Helper()
				tn := NewTelnet(termTypeXterm, 80, 24)
				tn.Feed(tc.pre)
				data, reply := feedChunks(tn, chunks...)
				if string(data) != tc.wantData {
					t.Fatalf("%s: data = %q, want %q", label, data, tc.wantData)
				}
				if !bytes.Equal(reply, tc.wantReply) {
					t.Fatalf("%s: reply = % X, want % X", label, reply, tc.wantReply)
				}
			}
			check("whole", tc.in)
			for i := 0; i <= len(tc.in); i++ {
				check(fmt.Sprintf("split at %d", i), tc.in[:i], tc.in[i:])
			}
			single := make([][]byte, len(tc.in))
			for i := range tc.in {
				single[i] = tc.in[i : i+1]
			}
			check("byte by byte", single...)
		})
	}
}

// Two clients wired back to back must not keep answering each other.
func TestTelnetTwoClientsSettle(t *testing.T) {
	a := NewTelnet(termTypeANSI, 80, 25)
	b := NewTelnet(termTypeXterm, 80, 24)
	toB, toA := a.Initial(), b.Initial()
	rounds := 0
	for len(toA) > 0 || len(toB) > 0 {
		rounds++
		if rounds > 3 {
			t.Fatalf("negotiation did not settle after 3 rounds: toA = % X, toB = % X", toA, toB)
		}
		dataB, replyB := b.Feed(toB)
		dataA, replyA := a.Feed(toA)
		if len(dataA) != 0 || len(dataB) != 0 {
			t.Fatalf("round %d leaked application data: a = % X, b = % X", rounds, dataA, dataB)
		}
		toA, toB = replyB, replyA
	}
}

// A server that speaks first must not be asked for the same options again.
func TestTelnetInitialSkipsOptionsRequestedByServer(t *testing.T) {
	tests := []struct {
		name   string
		server []byte
		want   []byte
	}{
		{
			"server asked for everything",
			seq(telnetIAC, telnetDO, optTTYPE, telnetIAC, telnetDO, optNAWS, telnetIAC, telnetWILL, optSGA, telnetIAC, telnetDO, optSGA),
			nil,
		},
		{
			"server asked for TTYPE only",
			iac(telnetDO, optTTYPE),
			seq(telnetIAC, telnetWILL, optNAWS, telnetIAC, telnetDO, optSGA, telnetIAC, telnetWILL, optSGA),
		},
		{
			"server offered SGA both ways",
			seq(telnetIAC, telnetWILL, optSGA, telnetIAC, telnetDO, optSGA),
			seq(telnetIAC, telnetWILL, optTTYPE, telnetIAC, telnetWILL, optNAWS),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tn := NewTelnet(termTypeXterm, 80, 24)
			tn.Feed(tc.server)
			if got := tn.Initial(); !bytes.Equal(got, tc.want) {
				t.Fatalf("Initial() = % X, want % X", got, tc.want)
			}
			if _, again := tn.Feed(tc.server); len(again) != 0 {
				t.Fatalf("repeated server requests answered again: % X", again)
			}
		})
	}
}

func TestTelnetSubnegotiationBufferBoundary(t *testing.T) {
	answer := seq(telnetIAC, telnetSB, optTTYPE, ttypeIS, "XTERM-256COLOR", telnetIAC, telnetSE)
	tests := []struct {
		name     string
		payload  int // bytes between the option byte and IAC SE, including the SEND verb
		answered bool
	}{
		{"one below the limit", maxSubnegotiation - 1, true},
		{"exactly the limit", maxSubnegotiation, true},
		{"one above the limit", maxSubnegotiation + 1, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tn := NewTelnet(termTypeXterm, 80, 24)
			tn.Feed(iac(telnetDO, optTTYPE))
			payload := append([]byte{ttypeSEND}, bytes.Repeat([]byte("z"), tc.payload-1)...)
			data, reply := tn.Feed(seq(telnetIAC, telnetSB, optTTYPE, payload, telnetIAC, telnetSE, "y"))
			if string(data) != "y" {
				t.Fatalf("data = %q, want %q", data, "y")
			}
			if tc.answered && !bytes.Equal(reply, answer) {
				t.Fatalf("reply = % X, want % X", reply, answer)
			}
			if !tc.answered && len(reply) != 0 {
				t.Fatalf("oversized subnegotiation answered: % X", reply)
			}
			if len(tn.sb) > maxSubnegotiation {
				t.Fatalf("buffered %d subnegotiation bytes, limit %d", len(tn.sb), maxSubnegotiation)
			}
			// The parser recovers: the next SEND is answered again.
			_, reply = tn.Feed(seq(telnetIAC, telnetSB, optTTYPE, ttypeSEND, telnetIAC, telnetSE))
			if !bytes.Equal(reply, answer) {
				t.Fatalf("follow-up TTYPE SEND reply = % X, want % X", reply, answer)
			}
		})
	}
}

// FuzzTelnetFeed checks that Feed never panics, never invents application
// data, and gives the same data and replies however the stream is split.
func FuzzTelnetFeed(f *testing.F) {
	seeds := []struct {
		in      []byte
		split   uint16
		initial bool
	}{
		{seq("plain text\r\n"), 4, false},
		{seq("a", telnetIAC, telnetIAC, "b"), 2, true},
		{seq(telnetIAC, telnetWILL, optEcho, telnetIAC, telnetWILL, optSGA, telnetIAC, telnetDO, optNAWS), 5, true},
		{seq(telnetIAC, telnetDO, optTTYPE, telnetIAC, telnetSB, optTTYPE, ttypeSEND, telnetIAC, telnetSE), 7, false},
		{seq(telnetIAC, telnetSB, telnetIAC, telnetSE, "hi"), 2, true},
		{seq(telnetIAC, telnetSB, optTTYPE, ttypeSEND, telnetIAC, telnetWILL, optEcho, "x"), 6, true},
		{seq(telnetIAC, telnetSB, 201, "Core.Hello {}", telnetIAC, telnetSE, telnetIAC, 249, "end"), 9, false},
		{seq(telnetIAC), 0, true},
		{nil, 0, false},
	}
	for _, s := range seeds {
		f.Add(s.in, s.split, s.initial)
	}
	f.Fuzz(func(t *testing.T, in []byte, split uint16, initial bool) {
		whole := NewTelnet(termTypeANSI, 80, 25)
		parts := NewTelnet(termTypeANSI, 80, 25)
		if initial {
			whole.Initial()
			parts.Initial()
		}
		wantData, wantReply := whole.Feed(in)
		if len(wantData) > len(in) {
			t.Fatalf("Feed produced %d data bytes from %d input bytes", len(wantData), len(in))
		}
		at := int(split) % (len(in) + 1)
		gotData, gotReply := feedChunks(parts, in[:at], in[at:])
		if !bytes.Equal(gotData, wantData) {
			t.Fatalf("split at %d: data = % X, whole feed gave % X", at, gotData, wantData)
		}
		if !bytes.Equal(gotReply, wantReply) {
			t.Fatalf("split at %d: reply = % X, whole feed gave % X", at, gotReply, wantReply)
		}
	})
}
