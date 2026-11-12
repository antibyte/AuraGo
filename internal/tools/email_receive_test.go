package tools

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
)

func TestFetchLegacyMessagesDecodesMIMEBeforeGuardian(t *testing.T) {
	latin1 := "Content-Type: text/plain; charset=iso-8859-1\r\nContent-Transfer-Encoding: quoted-printable\r\n"
	windowsText := base64.StdEncoding.EncodeToString([]byte("Kosten: 10 \x80"))
	for _, tc := range []struct {
		name, headers, body, want string
		invalidUTF8               bool
	}{
		{name: "declared charset", headers: latin1, body: "Gr=FC=DFe", want: "Grüße"},
		{name: "multipart alternative", headers: "Content-Type: multipart/alternative; boundary=fixture\r\n",
			body: "--fixture\r\nContent-Type: text/plain; charset=windows-1252\r\nContent-Transfer-Encoding: base64\r\n\r\n" + windowsText + "\r\n--fixture\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<p>HTML alternative</p>\r\n--fixture--\r\n", want: "Kosten: 10 €"},
		{name: "html only", headers: "Content-Type: text/html; charset=utf-8\r\n", body: "<p>Grüße</p><script>hidden script</script>", want: "Grüße"},
		{name: "malformed MIME retains fallback", headers: "Content-Type: multipart/mixed\r\n", body: "fallback body", want: "fallback body"},
		{name: "invalid bytes stay quarantinable", headers: "Content-Type: text/plain; charset=utf-8\r\n", body: "bad \xff", want: "bad \xff", invalidUTF8: true},
	} {
		for _, byUID := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/uid=%t", tc.name, byUID), func(t *testing.T) {
				headers := "From: Sender <sender@example.test>\r\nTo: recipient@example.test\r\nSubject: =?iso-8859-1?Q?Gr=FC=DFe?=\r\nMIME-Version: 1.0\r\n" + tc.headers + "\r\n"
				messages := fetchLegacyIMAPFixture(t, headers, tc.body, byUID)
				if len(messages) != 1 || messages[0].UID != 7 || messages[0].Body != tc.want {
					t.Fatalf("unexpected decoded messages: %#v", messages)
				}
				if messages[0].Subject != "Grüße" || messages[0].From != "Sender <sender@example.test>" {
					t.Fatalf("legacy header presentation changed: %#v", messages[0])
				}
				if utf8.ValidString(messages[0].Body) == tc.invalidUTF8 || utf8.ValidString(messages[0].Snippet) == tc.invalidUTF8 {
					t.Fatalf("unexpected UTF-8 validity: %#v", messages[0])
				}
			})
		}
	}
}

func fetchLegacyIMAPFixture(t *testing.T, headers, body string, byUID bool) []EmailMessage {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() { clientConn.Close(); serverConn.Close() })
	_ = clientConn.SetDeadline(time.Now().Add(5 * time.Second))
	_ = serverConn.SetDeadline(time.Now().Add(5 * time.Second))
	done := make(chan error, 1)
	go func() {
		defer serverConn.Close()
		if _, err := io.WriteString(serverConn, "* OK [CAPABILITY IMAP4rev1] fixture ready\r\n"); err != nil {
			done <- err
			return
		}
		r := bufio.NewReader(serverConn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				done <- err
				return
			}
			parts := strings.SplitN(strings.TrimSpace(line), " ", 3)
			if len(parts) < 2 {
				done <- fmt.Errorf("invalid fixture command")
				return
			}
			tag, command := parts[0], strings.ToUpper(parts[1])
			switch command {
			case "CAPABILITY":
				_, err = fmt.Fprintf(serverConn, "* CAPABILITY IMAP4rev1\r\n%s OK capabilities\r\n", tag)
			case "LOGIN":
				_, err = fmt.Fprintf(serverConn, "%s OK logged in\r\n", tag)
			case "EXAMINE":
				_, err = fmt.Fprintf(serverConn, "* 1 EXISTS\r\n* OK [UIDVALIDITY 1] valid\r\n%s OK [READ-ONLY] selected\r\n", tag)
			case "FETCH", "UID":
				upper := strings.ToUpper(line)
				first := strings.Index(upper, "BODY.PEEK[HEADER.FIELDS ")
				if first < 0 || !strings.Contains(upper, "CONTENT-TYPE") || !strings.Contains(upper, "CONTENT-TRANSFER-ENCODING") || !strings.Contains(upper, "BODY.PEEK[TEXT]") || (command == "UID") != byUID {
					done <- fmt.Errorf("fetch did not request MIME headers and BODY.PEEK sections: %s", line)
					return
				}
				end := strings.IndexByte(line[first:], ']')
				section := strings.Replace(line[first:first+end+1], "BODY.PEEK", "BODY", 1)
				_, err = fmt.Fprintf(serverConn, "* 1 FETCH (UID 7 %s {%d}\r\n%s BODY[TEXT] {%d}\r\n%s)\r\n%s OK fetched\r\n", section, len(headers), headers, len(body), body, tag)
			case "LOGOUT":
				_, err = fmt.Fprintf(serverConn, "* BYE closing\r\n%s OK logout\r\n", tag)
				done <- err
				return
			default:
				err = fmt.Errorf("unexpected fixture command %s", command)
			}
			if err != nil {
				done <- err
				return
			}
		}
	}()
	c, err := imapclient.New(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Login("fixture", "fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Select("INBOX", true); err != nil {
		t.Fatal(err)
	}
	set := new(imap.SeqSet)
	if byUID {
		set.AddNum(7)
	} else {
		set.AddNum(1)
	}
	messages, err := fetchLegacyMessages(c, set, byUID)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Logout(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	return messages
}
