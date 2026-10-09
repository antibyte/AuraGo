package retronet

import (
	"encoding/json"
	"testing"
)

func TestEntryAddress(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{"hostname", "telehack.com", 23, "telehack.com:23"},
		{"ipv4 literal", "87.106.7.15", 23, "87.106.7.15:23"},
		{"ipv6 literal", "2606:4700:4700::1111", 2323, "[2606:4700:4700::1111]:2323"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := (Entry{Host: tc.host, Port: tc.port}).Address(); got != tc.want {
				t.Fatalf("Address() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEntryTermType(t *testing.T) {
	tests := []struct {
		name  string
		entry Entry
		want  string
	}{
		{"bbs", Entry{Protocol: ProtocolTelnet, Kind: KindBBS}, "ANSI"},
		{"world", Entry{Protocol: ProtocolTelnet, Kind: KindWorld}, "XTERM-256COLOR"},
		{"ssh", Entry{Protocol: ProtocolSSH}, "XTERM-256COLOR"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.entry.TermType(); got != tc.want {
				t.Fatalf("TermType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEntryEffectiveCharset(t *testing.T) {
	tests := []struct {
		name  string
		entry Entry
		want  Charset
	}{
		{"telnet cp437", Entry{Protocol: ProtocolTelnet, Charset: CharsetCP437}, CharsetCP437},
		{"telnet latin1", Entry{Protocol: ProtocolTelnet, Charset: CharsetLatin1}, CharsetLatin1},
		{"telnet utf8", Entry{Protocol: ProtocolTelnet, Charset: CharsetUTF8}, CharsetUTF8},
		{"telnet empty", Entry{Protocol: ProtocolTelnet}, CharsetUTF8},
		{"ssh ignores charset", Entry{Protocol: ProtocolSSH, Charset: CharsetCP437}, CharsetUTF8},
		{"ssh empty", Entry{Protocol: ProtocolSSH}, CharsetUTF8},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.entry.EffectiveCharset(); got != tc.want {
				t.Fatalf("EffectiveCharset() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestKindAndCharsetValidity(t *testing.T) {
	for _, k := range []Kind{KindBBS, KindWorld} {
		if !k.valid() {
			t.Errorf("Kind %q should be valid", k)
		}
	}
	for _, k := range []Kind{"", "BBS", "c64", "world "} {
		if k.valid() {
			t.Errorf("Kind %q should be invalid", k)
		}
	}
	for _, c := range []Charset{CharsetUTF8, CharsetCP437, CharsetLatin1} {
		if !c.valid() {
			t.Errorf("Charset %q should be valid", c)
		}
	}
	for _, c := range []Charset{"", "UTF8", "utf-8", "ebcdic", "petscii"} {
		if c.valid() {
			t.Errorf("Charset %q should be invalid", c)
		}
	}
}

func TestEntryJSONWireShape(t *testing.T) {
	ssh := Entry{
		ID:             "sshtron",
		Name:           "SSHTron",
		DescriptionKey: "desktop.terminal_retronet_entry_sshtron",
		Category:       CategoryGames,
		Protocol:       ProtocolSSH,
		Host:           "sshtron.zachlatta.com",
		Port:           22,
		User:           "guest",
		HostKey:        "SHA256:cxRrWvXUmeIL3j/fJQclNpLj9sHYFX8cC5P+QhTILRA",
	}
	own := Entry{
		ID:          "own-abcd1234",
		Name:        "My BBS",
		Description: "Late night board",
		Category:    CategoryOwn,
		Protocol:    ProtocolTelnet,
		Host:        "bbs.example.org",
		Port:        2323,
		Kind:        KindBBS,
		Charset:     CharsetCP437,
		Own:         true,
	}
	tests := []struct {
		name  string
		entry Entry
		want  string
	}{
		{"catalog ssh", ssh, `{"id":"sshtron","name":"SSHTron","description_key":"desktop.terminal_retronet_entry_sshtron","category":"games","protocol":"ssh","host":"sshtron.zachlatta.com","port":22,"user":"guest","host_key":"SHA256:cxRrWvXUmeIL3j/fJQclNpLj9sHYFX8cC5P+QhTILRA","own":false}`},
		{"own telnet", own, `{"id":"own-abcd1234","name":"My BBS","description":"Late night board","category":"own","protocol":"telnet","host":"bbs.example.org","port":2323,"kind":"bbs","charset":"cp437","own":true}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.entry)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(raw) != tc.want {
				t.Fatalf("Marshal() = %s\nwant      %s", raw, tc.want)
			}
		})
	}
}
