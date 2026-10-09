package retronet

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const (
	validTelnetJSON = `{"id":"own-abcd1234","name":"My BBS","description":"Late night board","protocol":"telnet","host":"bbs.example.org","port":2323,"kind":"bbs","charset":"cp437"}`
	validSSHJSON    = `{"id":"own-ssh00001","name":"Game","protocol":"ssh","host":"game.example.net","port":22,"user":"guest","host_key":"SHA256:aOOORrmRW4l88GHLFfyFPI/dJFqpE/iwZs3soZHr5LY"}`
)

// entriesDoc wraps entry objects into a version 1 document.
func entriesDoc(entries ...string) string {
	return `{"version":1,"entries":[` + strings.Join(entries, ",") + `]}`
}

// telnetJSON returns a valid telnet entry with one field replaced (raw JSON value).
func telnetJSON(field, raw string) string {
	fields := map[string]string{
		"id": `"own-abcd1234"`, "name": `"My BBS"`, "protocol": `"telnet"`, "host": `"bbs.example.org"`,
		"port": `23`, "kind": `"bbs"`, "charset": `"cp437"`,
	}
	return objectJSON(fields, field, raw)
}

// sshJSON returns a valid ssh entry with one field replaced (raw JSON value).
func sshJSON(field, raw string) string {
	fields := map[string]string{
		"id": `"own-ssh00001"`, "name": `"Game"`, "protocol": `"ssh"`, "host": `"game.example.net"`,
		"port": `22`, "user": `"guest"`,
	}
	return objectJSON(fields, field, raw)
}

// objectJSON renders fields in a fixed order; raw "" deletes the field.
func objectJSON(fields map[string]string, field, raw string) string {
	if field != "" {
		if raw == "" {
			delete(fields, field)
		} else {
			fields[field] = raw
		}
	}
	order := []string{"id", "name", "description", "protocol", "host", "port", "kind", "charset", "user", "host_key", "own", "category", "description_key", "extra"}
	parts := make([]string, 0, len(fields))
	for _, key := range order {
		if value, ok := fields[key]; ok {
			parts = append(parts, fmt.Sprintf("%q:%s", key, value))
		}
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func TestValidateEntriesDocumentAcceptsValidDocuments(t *testing.T) {
	valid := map[string]string{
		"default":                  DefaultEntriesDocument,
		"telnet and ssh":           entriesDoc(validTelnetJSON, validSSHJSON),
		"trailing whitespace":      DefaultEntriesDocument + "\n  ",
		"id 8 chars":               entriesDoc(telnetJSON("id", `"own-a1b2c3d4"`)),
		"id 32 chars":              entriesDoc(telnetJSON("id", `"own-`+strings.Repeat("a", 32)+`"`)),
		"name 40 runes":            entriesDoc(telnetJSON("name", `"`+strings.Repeat("Ä", 40)+`"`)),
		"description 80 runes":     entriesDoc(telnetJSON("description", `"`+strings.Repeat("é", 80)+`"`)),
		"empty description":        entriesDoc(telnetJSON("description", `""`)),
		"mixed case host":          entriesDoc(telnetJSON("host", `"BBS.Example.ORG"`)),
		"single label host":        entriesDoc(telnetJSON("host", `"bbs"`)),
		"punycode host":            entriesDoc(telnetJSON("host", `"xn--bcher-kva.example"`)),
		"host 253 chars":           entriesDoc(telnetJSON("host", `"`+longHostname(253)+`"`)),
		"public ipv4":              entriesDoc(telnetJSON("host", `"87.106.7.15"`)),
		"public ipv6":              entriesDoc(telnetJSON("host", `"2606:4700:4700::1111"`)),
		"port 1":                   entriesDoc(telnetJSON("port", `1`)),
		"port 65535":               entriesDoc(telnetJSON("port", `65535`)),
		"world utf8":               entriesDoc(telnetJSON("kind", `"world"`)),
		"latin1":                   entriesDoc(telnetJSON("charset", `"latin1"`)),
		"utf8":                     entriesDoc(telnetJSON("charset", `"utf8"`)),
		"ssh without host key":     entriesDoc(sshJSON("", "")),
		"ssh user punctuation":     entriesDoc(sshJSON("user", `"bbs.user_1-x"`)),
		"64 entries":               manyEntries(MaxOwnEntries),
		"document at size limit":   padTo(DefaultEntriesDocument, MaxEntriesDocumentSize),
		"telnet explicit no user":  entriesDoc(telnetJSON("user", `""`)),
		"ssh explicit empty kind":  entriesDoc(sshJSON("kind", `""`)),
		"ssh empty host key field": entriesDoc(sshJSON("host_key", `""`)),
	}
	for name, value := range valid {
		t.Run(name, func(t *testing.T) {
			if err := ValidateEntriesDocument(value); err != nil {
				t.Fatalf("ValidateEntriesDocument() error = %v", err)
			}
		})
	}
}

func TestValidateEntriesDocumentRejectsInvalidDocuments(t *testing.T) {
	invalid := map[string]string{
		"empty string":                "",
		"not json":                    "retronet",
		"invalid utf8 in name":        entriesDoc(telnetJSON("name", "\"Caf\xe9\"")),
		"unknown top-level field":     `{"version":1,"entries":[],"extra":true}`,
		"version 0":                   `{"version":0,"entries":[]}`,
		"version 2":                   `{"version":2,"entries":[]}`,
		"missing version":             `{"entries":[]}`,
		"null entries":                `{"version":1,"entries":null}`,
		"missing entries":             `{"version":1}`,
		"trailing object":             DefaultEntriesDocument + `{}`,
		"trailing garbage":            DefaultEntriesDocument + `x`,
		"65 entries":                  manyEntries(MaxOwnEntries + 1),
		"document over size limit":    padTo(DefaultEntriesDocument, MaxEntriesDocumentSize+1),
		"unknown entry field":         entriesDoc(telnetJSON("extra", `1`)),
		"stored own flag":             entriesDoc(telnetJSON("own", `true`)),
		"stored category":             entriesDoc(telnetJSON("category", `"own"`)),
		"stored description key":      entriesDoc(telnetJSON("description_key", `"desktop.x"`)),
		"duplicate ids":               entriesDoc(validTelnetJSON, validTelnetJSON),
		"id without prefix":           entriesDoc(telnetJSON("id", `"telehack"`)),
		"catalog id":                  entriesDoc(telnetJSON("id", `"telehack-ssh"`)),
		"id too short":                entriesDoc(telnetJSON("id", `"own-abc1234"`)),
		"id too long":                 entriesDoc(telnetJSON("id", `"own-`+strings.Repeat("a", 33)+`"`)),
		"id uppercase":                entriesDoc(telnetJSON("id", `"own-ABCD1234"`)),
		"id inner dash":               entriesDoc(telnetJSON("id", `"own-abcd-1234"`)),
		"id missing":                  entriesDoc(telnetJSON("id", "")),
		"name missing":                entriesDoc(telnetJSON("name", "")),
		"name blank":                  entriesDoc(telnetJSON("name", `"   "`)),
		"name 41 runes":               entriesDoc(telnetJSON("name", `"`+strings.Repeat("Ä", 41)+`"`)),
		"name newline":                entriesDoc(telnetJSON("name", `"My\nBBS"`)),
		"name escape":                 entriesDoc(telnetJSON("name", `"\u001b[31mRed"`)),
		"description 81 runes":        entriesDoc(telnetJSON("description", `"`+strings.Repeat("é", 81)+`"`)),
		"description control":         entriesDoc(telnetJSON("description", `"a\tb"`)),
		"protocol missing":            entriesDoc(telnetJSON("protocol", "")),
		"protocol http":               entriesDoc(telnetJSON("protocol", `"http"`)),
		"protocol uppercase":          entriesDoc(telnetJSON("protocol", `"TELNET"`)),
		"host missing":                entriesDoc(telnetJSON("host", "")),
		"host leading dash":           entriesDoc(telnetJSON("host", `"-bbs.example.org"`)),
		"host trailing dash":          entriesDoc(telnetJSON("host", `"bbs-.example.org"`)),
		"host empty label":            entriesDoc(telnetJSON("host", `"bbs..example.org"`)),
		"host trailing dot":           entriesDoc(telnetJSON("host", `"bbs.example.org."`)),
		"host underscore":             entriesDoc(telnetJSON("host", `"bbs_1.example.org"`)),
		"host space":                  entriesDoc(telnetJSON("host", `"bbs example.org"`)),
		"host with port":              entriesDoc(telnetJSON("host", `"bbs.example.org:23"`)),
		"host url":                    entriesDoc(telnetJSON("host", `"telnet://bbs.example.org"`)),
		"host bracketed ipv6":         entriesDoc(telnetJSON("host", `"[2606:4700:4700::1111]"`)),
		"host 254 chars":              entriesDoc(telnetJSON("host", `"`+longHostname(254)+`"`)),
		"host label 64 chars":         entriesDoc(telnetJSON("host", `"`+strings.Repeat("a", 64)+`.example.org"`)),
		"host numeric shorthand":      entriesDoc(telnetJSON("host", `"127.1"`)),
		"host numeric final label":    entriesDoc(telnetJSON("host", `"bbs.example.123"`)),
		"literal loopback":            entriesDoc(telnetJSON("host", `"127.0.0.1"`)),
		"literal private 10":          entriesDoc(telnetJSON("host", `"10.0.0.5"`)),
		"literal private 192.168":     entriesDoc(telnetJSON("host", `"192.168.1.10"`)),
		"literal private 172.16":      entriesDoc(telnetJSON("host", `"172.16.0.1"`)),
		"literal cgnat":               entriesDoc(telnetJSON("host", `"100.64.1.1"`)),
		"literal link-local":          entriesDoc(telnetJSON("host", `"169.254.169.254"`)),
		"literal unspecified":         entriesDoc(telnetJSON("host", `"0.0.0.0"`)),
		"literal ipv6 loopback":       entriesDoc(telnetJSON("host", `"::1"`)),
		"literal ipv6 unspecified":    entriesDoc(telnetJSON("host", `"::"`)),
		"literal ula":                 entriesDoc(telnetJSON("host", `"fd00::1"`)),
		"literal ipv6 link-local":     entriesDoc(telnetJSON("host", `"fe80::1"`)),
		"literal v4-mapped loopback":  entriesDoc(telnetJSON("host", `"::ffff:127.0.0.1"`)),
		"port missing":                entriesDoc(telnetJSON("port", "")),
		"port 0":                      entriesDoc(telnetJSON("port", `0`)),
		"port negative":               entriesDoc(telnetJSON("port", `-1`)),
		"port 65536":                  entriesDoc(telnetJSON("port", `65536`)),
		"port 25":                     entriesDoc(telnetJSON("port", `25`)),
		"port 465":                    entriesDoc(telnetJSON("port", `465`)),
		"port 587":                    entriesDoc(telnetJSON("port", `587`)),
		"port string":                 entriesDoc(telnetJSON("port", `"23"`)),
		"port fraction":               entriesDoc(telnetJSON("port", `23.5`)),
		"telnet kind missing":         entriesDoc(telnetJSON("kind", "")),
		"telnet kind c64":             entriesDoc(telnetJSON("kind", `"c64"`)),
		"telnet charset missing":      entriesDoc(telnetJSON("charset", "")),
		"telnet charset ebcdic":       entriesDoc(telnetJSON("charset", `"ebcdic"`)),
		"telnet with user":            entriesDoc(telnetJSON("user", `"guest"`)),
		"telnet with host key":        entriesDoc(telnetJSON("host_key", `"SHA256:aOOORrmRW4l88GHLFfyFPI/dJFqpE/iwZs3soZHr5LY"`)),
		"ssh with kind":               entriesDoc(sshJSON("kind", `"world"`)),
		"ssh with charset":            entriesDoc(sshJSON("charset", `"utf8"`)),
		"ssh user missing":            entriesDoc(sshJSON("user", "")),
		"ssh user uppercase":          entriesDoc(sshJSON("user", `"Guest"`)),
		"ssh user space":              entriesDoc(sshJSON("user", `"a b"`)),
		"ssh user 33 chars":           entriesDoc(sshJSON("user", `"`+strings.Repeat("u", 33)+`"`)),
		"ssh host key md5":            entriesDoc(sshJSON("host_key", `"MD5:16:27:ac:a5:76:28:2d:36:63:1b:56:4d:eb:df:a6:48"`)),
		"ssh host key short":          entriesDoc(sshJSON("host_key", `"SHA256:abc"`)),
		"ssh host key padded":         entriesDoc(sshJSON("host_key", `"SHA256:aOOORrmRW4l88GHLFfyFPI/dJFqpE/iwZs3soZHr5LY="`)),
		"ssh host key without prefix": entriesDoc(sshJSON("host_key", `"aOOORrmRW4l88GHLFfyFPI/dJFqpE/iwZs3soZHr5LY"`)),
	}
	for name, value := range invalid {
		t.Run(name, func(t *testing.T) {
			err := ValidateEntriesDocument(value)
			if err == nil {
				t.Fatal("ValidateEntriesDocument() accepted an invalid document")
			}
			if got := err.Error(); got != "invalid desktop setting value for retronet.entries" {
				t.Fatalf("error = %q, want the fixed message", got)
			}
		})
	}
}

func TestValidateEntriesDocumentNeverEchoesInput(t *testing.T) {
	marker := "own-SECRETMARKER"
	err := ValidateEntriesDocument(entriesDoc(telnetJSON("id", `"`+marker+`"`)))
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "SECRETMARKER") {
		t.Fatalf("error echoes user input: %q", err.Error())
	}
}

func TestParseEntriesDocument(t *testing.T) {
	entries, err := ParseEntriesDocument(entriesDoc(validTelnetJSON, validSSHJSON))
	if err != nil {
		t.Fatalf("ParseEntriesDocument() error = %v", err)
	}
	want := []Entry{
		{ID: "own-abcd1234", Name: "My BBS", Description: "Late night board", Category: CategoryOwn, Protocol: ProtocolTelnet, Host: "bbs.example.org", Port: 2323, Kind: KindBBS, Charset: CharsetCP437, Own: true},
		{ID: "own-ssh00001", Name: "Game", Category: CategoryOwn, Protocol: ProtocolSSH, Host: "game.example.net", Port: 22, User: "guest", HostKey: "SHA256:aOOORrmRW4l88GHLFfyFPI/dJFqpE/iwZs3soZHr5LY", Own: true},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("ParseEntriesDocument() = %+v\nwant %+v", entries, want)
	}
}

func TestParseEntriesDocumentEmptyAndInvalid(t *testing.T) {
	entries, err := ParseEntriesDocument("")
	if err != nil {
		t.Fatalf("ParseEntriesDocument(\"\") error = %v", err)
	}
	if entries == nil || len(entries) != 0 {
		t.Fatalf("ParseEntriesDocument(\"\") = %#v, want an empty non-nil slice", entries)
	}
	if _, err := ParseEntriesDocument(`{"version":2,"entries":[]}`); err == nil {
		t.Fatal("ParseEntriesDocument accepted version 2")
	}
}

func TestEncodeEntriesDocumentRoundTrip(t *testing.T) {
	entries, err := ParseEntriesDocument(entriesDoc(validTelnetJSON, validSSHJSON))
	if err != nil {
		t.Fatalf("ParseEntriesDocument() error = %v", err)
	}
	entries[0].DescriptionKey = "desktop.must_not_be_stored"
	value, err := EncodeEntriesDocument(entries)
	if err != nil {
		t.Fatalf("EncodeEntriesDocument() error = %v", err)
	}
	for _, forbidden := range []string{`"own":`, `"category"`, `"description_key"`} {
		if strings.Contains(value, forbidden) {
			t.Fatalf("encoded document stores %s: %s", forbidden, value)
		}
	}
	want := entriesDoc(validTelnetJSON, validSSHJSON)
	if value != want {
		t.Fatalf("EncodeEntriesDocument() = %s\nwant %s", value, want)
	}
	again, err := ParseEntriesDocument(value)
	if err != nil {
		t.Fatalf("ParseEntriesDocument(encoded) error = %v", err)
	}
	entries[0].DescriptionKey = ""
	if !reflect.DeepEqual(again, entries) {
		t.Fatalf("round trip = %+v\nwant %+v", again, entries)
	}
}

func TestEncodeEntriesDocumentValidates(t *testing.T) {
	empty, err := EncodeEntriesDocument(nil)
	if err != nil || empty != DefaultEntriesDocument {
		t.Fatalf("EncodeEntriesDocument(nil) = %q, %v; want %q", empty, err, DefaultEntriesDocument)
	}
	bad := []Entry{
		{ID: "telehack", Name: "Telehack", Protocol: ProtocolTelnet, Host: "telehack.com", Port: 23, Kind: KindWorld, Charset: CharsetUTF8},
	}
	if _, err := EncodeEntriesDocument(bad); err == nil {
		t.Fatal("EncodeEntriesDocument accepted a catalog ID")
	}
	private := []Entry{
		{ID: "own-abcd1234", Name: "LAN", Protocol: ProtocolTelnet, Host: "192.168.0.2", Port: 23, Kind: KindBBS, Charset: CharsetCP437},
	}
	if _, err := EncodeEntriesDocument(private); err == nil {
		t.Fatal("EncodeEntriesDocument accepted a private literal IP")
	}
}

// manyEntries returns a document with n distinct valid telnet entries.
func manyEntries(n int) string {
	entries := make([]string, n)
	for i := range entries {
		entries[i] = telnetJSON("id", fmt.Sprintf(`"own-%08d"`, i))
	}
	return entriesDoc(entries...)
}

// padTo appends spaces until value is exactly size bytes long.
func padTo(value string, size int) string {
	return value + strings.Repeat(" ", size-len(value))
}

// longHostname returns a host name of exactly n bytes built from labels of
// at most 63 bytes (used with n = 253 and 254).
func longHostname(n int) string {
	var b strings.Builder
	for b.Len() < n {
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(strings.Repeat("a", min(maxLabelLength, n-b.Len())))
	}
	return b.String()
}
