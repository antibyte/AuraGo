package mailer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/textproto"
	"testing"
	"time"
)

func TestReadOnlyPOP3KeepsFolderListingCapability(t *testing.T) {
	account := AccountFromTransportConfig(TransportConfig{ID: "read-only", Source: "email", ReceiveProtocol: "pop3", ReadOnly: true})
	for _, capability := range []string{"read", "search", "folders_read"} {
		if !hasCapability(account.Capabilities, capability) {
			t.Errorf("read-only POP3 account lost %q capability: %v", capability, account.Capabilities)
		}
	}
	for _, capability := range []string{"send", "delete", "move", "archive", "trash", "restore"} {
		if hasCapability(account.Capabilities, capability) {
			t.Errorf("read-only POP3 account retained mutating %q capability: %v", capability, account.Capabilities)
		}
	}
}

func TestSharedEmailTransportsRejectWrongTLSModesBeforeDial(t *testing.T) {
	dials := 0
	dial := func(context.Context, string, string) (net.Conn, error) {
		dials++
		client, server := net.Pipe()
		server.Close()
		return client, nil
	}
	if err := TestIMAPAuth(context.Background(), TransportConfig{IMAPHost: "imap.example.test", ReceiveTLS: "none", DialContext: dial}); err == nil {
		t.Fatal("IMAP accepted an unencrypted transport mode")
	}
	if dials != 0 {
		t.Fatalf("invalid IMAP mode opened %d connections", dials)
	}

	cfg := TransportConfig{SMTPHost: "smtp.example.test", SMTPTransport: "plain", DialContext: dial}
	if err := TestSMTPAuth(context.Background(), cfg); err == nil {
		t.Fatal("SMTP auth test accepted an unencrypted transport mode")
	}
	if _, err := SendSMTP(context.Background(), cfg, Outgoing{Draft: Draft{To: []string{"to@example.test"}}}); err == nil {
		t.Fatal("SMTP delivery accepted an unencrypted transport mode")
	}
	if dials != 0 {
		t.Fatalf("invalid SMTP mode opened %d connections", dials)
	}
}

func TestSMTPTransportModeDefaultsAndValidation(t *testing.T) {
	for _, tc := range []struct {
		configured string
		port       int
		want       string
		wantErr    bool
	}{
		{port: 465, want: "tls"},
		{port: 587, want: "starttls"},
		{configured: " STARTTLS ", port: 465, want: "starttls"},
		{configured: "plain", port: 587, wantErr: true},
	} {
		got, err := smtpTransportMode(tc.configured, tc.port)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("smtpTransportMode(%q, %d) unexpectedly succeeded", tc.configured, tc.port)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("smtpTransportMode(%q, %d) = %q, %v; want %q", tc.configured, tc.port, got, err, tc.want)
		}
	}
}

func TestSMTPDeliveryBytesAndDefiniteVsAmbiguousOutcome(t *testing.T) {
	for _, mode := range []string{"accepted", "rejected", "ambiguous"} {
		t.Run(mode, func(t *testing.T) {
			serverTLS, clientTLS := smtpFixtureTLS(t)
			clientConn, serverRaw := net.Pipe()
			defer clientConn.Close()
			defer serverRaw.Close()
			serverConn := tls.Server(serverRaw, serverTLS)
			defer serverConn.Close()
			type fixtureResult struct {
				data []byte
				err  error
			}
			fixtureDone := make(chan fixtureResult, 1)
			go func() {
				_ = serverConn.SetDeadline(time.Now().Add(5 * time.Second))
				_, _ = io.WriteString(serverConn, "220 fixture ESMTP\r\n")
				reader := textproto.NewReader(bufio.NewReader(serverConn))
				var captured []byte
				for {
					line, readErr := reader.ReadLine()
					if readErr != nil {
						fixtureDone <- fixtureResult{data: captured, err: readErr}
						return
					}
					switch {
					case bytes.HasPrefix(bytes.ToUpper([]byte(line)), []byte("EHLO ")) || bytes.HasPrefix(bytes.ToUpper([]byte(line)), []byte("HELO ")):
						_, _ = io.WriteString(serverConn, "250 fixture\r\n")
					case bytes.HasPrefix(bytes.ToUpper([]byte(line)), []byte("MAIL FROM:")):
						_, _ = io.WriteString(serverConn, "250 sender ok\r\n")
					case bytes.HasPrefix(bytes.ToUpper([]byte(line)), []byte("RCPT TO:")):
						if mode == "rejected" {
							_, _ = io.WriteString(serverConn, "550 recipient rejected\r\n")
						} else {
							_, _ = io.WriteString(serverConn, "250 recipient ok\r\n")
						}
					case bytes.Equal(bytes.ToUpper([]byte(line)), []byte("DATA")):
						_, _ = io.WriteString(serverConn, "354 send data\r\n")
						captured, readErr = reader.ReadDotBytes()
						if readErr != nil {
							fixtureDone <- fixtureResult{data: captured, err: readErr}
							return
						}
						if mode == "ambiguous" {
							fixtureDone <- fixtureResult{data: captured}
							_ = serverRaw.Close() // Drop the connection without an SMTP result.
							return
						}
						_, _ = io.WriteString(serverConn, "250 queued\r\n")
					case bytes.Equal(bytes.ToUpper([]byte(line)), []byte("QUIT")):
						_, _ = io.WriteString(serverConn, "221 bye\r\n")
						fixtureDone <- fixtureResult{data: captured}
						return
					default:
						_, _ = io.WriteString(serverConn, "500 unexpected command\r\n")
					}
				}
			}()

			cfg := TransportConfig{ID: "smtp-fixture", Source: "email", Address: "sender@example.test", FromAddress: "sender@example.test", SMTPHost: "127.0.0.1", SMTPPort: 465, SMTPTransport: "tls", TLSConfig: clientTLS,
				DialContext: func(context.Context, string, string) (net.Conn, error) { return clientConn, nil }}
			out := Outgoing{Draft: Draft{ID: "stable-draft-id", To: []string{"to@example.test"}, Subject: "fixture", Text: "body"}}
			result, sendErr := sendSMTP(context.Background(), cfg, out)
			fixture := <-fixtureDone
			if mode == "accepted" {
				expectedData := []byte(nil)
				if result.Message != nil {
					expectedData = bytes.ReplaceAll(result.Message.Raw, []byte("\r\n"), []byte("\n"))
				}
				wireData := bytes.TrimSuffix(fixture.data, []byte("\n")) // DotReader strips CR but keeps the DATA framing newline.
				if sendErr != nil || !result.Confirmed || result.Rejected || result.Message == nil || !bytes.Equal(expectedData, wireData) {
					raw := []byte(nil)
					if result.Message != nil {
						raw = result.Message.Raw
					}
					t.Fatalf("accepted delivery did not retain transmitted bytes: raw=%d/%q wire=%d/%q err=%v fixtureErr=%v", len(raw), raw, len(fixture.data), fixture.data, sendErr, fixture.err)
				}
				if _, _, _, err := ParseThreadHeaders(fixture.data); err != nil {
					t.Fatalf("transmitted MIME missing stable thread identity: %v", err)
				}
			} else if mode == "rejected" {
				if sendErr == nil || !result.Rejected || result.Confirmed || len(fixture.data) != 0 {
					t.Fatalf("definite recipient rejection misclassified: result=%#v err=%v data=%q", result, sendErr, fixture.data)
				}
			} else if sendErr == nil || result.Rejected || result.Confirmed || len(fixture.data) == 0 {
				t.Fatalf("ambiguous delivery was marked retry-safe: result=%#v err=%v", result, sendErr)
			}
		})
	}
}

func smtpFixtureTLS(t *testing.T) (*tls.Config, *tls.Config) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(42), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
}
