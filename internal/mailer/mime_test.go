package mailer

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildMIMEAlternativesAttachmentsAndSafeHeaders(t *testing.T) {
	draft := Draft{ID: "draft-fixture", To: []string{"to@example.test"}, CC: []string{"cc@example.test"}, BCC: []string{"hidden@example.test"},
		Subject: "hello\r\nBcc: attacker@example.test", Text: "plain body", HTML: `<p>safe</p><script>alert(1)</script>`,
		InReplyTo: "<parent@example.test>", References: []string{"<root@example.test>", "<parent@example.test>"},
		Attachments: []Attachment{{Filename: "note.txt", ContentType: "text/plain", Data: []byte("attachment bytes")}}}
	raw, err := BuildMIME("owner@example.test", draft, time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("\r\nBcc:")) || bytes.Contains(raw, []byte("hidden@example.test")) || bytes.Contains(raw, []byte("\r\nBcc: attacker@example.test")) {
		t.Fatal("SMTP MIME exposed Bcc or accepted a subject header injection")
	}
	id, inReplyTo, refs, err := ParseThreadHeaders(raw)
	if err != nil || id == "" || inReplyTo != "parent@example.test" || len(refs) != 2 || refs[0] != "root@example.test" || refs[1] != "parent@example.test" {
		t.Fatalf("thread headers were not preserved safely: %q %q %#v %v", id, inReplyTo, refs, err)
	}
	subject, _, to, cc, text, html, attachments, err := ParseMIME(raw, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(subject, "hello") || !strings.Contains(subject, "Bcc: attacker@example.test") || len(to) != 1 || len(cc) != 1 || text != "plain body" || !strings.Contains(html, "safe") || strings.Contains(html, "script") {
		t.Fatalf("MIME alternatives or sanitized headers incorrect: subject=%q to=%v cc=%v text=%q html=%q", subject, to, cc, text, html)
	}
	if len(attachments) != 1 || !bytes.Equal(attachments[0].Data, []byte("attachment bytes")) || attachments[0].Filename != "note.txt" {
		t.Fatalf("attachment round trip failed: %#v", attachments)
	}
	gmailRaw, err := buildGmailMIME("owner@example.test", draft, time.Now())
	if err != nil || !bytes.Contains(gmailRaw, []byte("hidden@example.test")) {
		t.Fatalf("Gmail API raw message omitted Bcc recipient: err=%v", err)
	}
}

func TestParseMIMECharsetAndBounds(t *testing.T) {
	raw := []byte("From: sender@example.test\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=iso-8859-1\r\nContent-Transfer-Encoding: 8bit\r\n\r\ncaf\xe9")
	_, _, _, _, text, _, _, err := ParseMIME(raw, 1024)
	if err != nil || text != "café" {
		t.Fatalf("legacy charset was not decoded: text=%q err=%v", text, err)
	}
	if _, _, _, _, _, _, _, err = ParseMIME(bytes.Repeat([]byte("x"), 128), 64); err == nil {
		t.Fatal("oversized raw MIME message was accepted")
	}
}

func TestParseMIMELimitsPartCount(t *testing.T) {
	var raw bytes.Buffer
	fmt.Fprint(&raw, "From: sender@example.test\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=fixture\r\n\r\n")
	for i := 0; i < 513; i++ {
		fmt.Fprintf(&raw, "--fixture\r\nContent-Type: text/plain\r\n\r\npart-%d\r\n", i)
	}
	fmt.Fprint(&raw, "--fixture--\r\n")
	if _, _, _, _, _, _, _, err := ParseMIME(raw.Bytes(), 1<<20); err == nil || !strings.Contains(err.Error(), "too many parts") {
		t.Fatalf("MIME part count was not bounded: %v", err)
	}
}

func TestParseReplyToExtractsOnlyMailboxAddresses(t *testing.T) {
	raw := []byte("From: sender@example.test\r\nReply-To: Support Team <support@example.test>, Escalations <ops@example.test>\r\nSubject: help\r\n\r\nbody\r\n")
	got := ParseReplyTo(raw)
	want := []string{"support@example.test", "ops@example.test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseReplyTo() = %#v, want %#v", got, want)
	}
}
