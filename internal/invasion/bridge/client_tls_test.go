package bridge

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const pinMismatchText = "master certificate does not match the pinned fingerprint"

// newPinTestTLSServer starts a TLS server on IPv4 loopback with its own freshly
// generated self-signed certificate and returns it with the SHA-256 hex of its
// DER leaf. httptest's built-in certificate is shared by every test server, so
// two default servers would carry the same fingerprint. WebSocket requests are
// upgraded and then closed without a challenge; plain requests get 200.
func newPinTestTLSServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"AuraGo Self-Signed"}, CommonName: "AuraGo"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if websocket.IsWebSocketUpgrade(r) {
			if ws, err := upgrader.Upgrade(w, r, nil); err == nil {
				_ = ws.Close()
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	srv.Listener = listener
	srv.Config.ErrorLog = log.New(io.Discard, "", 0) // rejected handshakes are expected
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	sum := sha256.Sum256(srv.Certificate().Raw)
	return srv, hex.EncodeToString(sum[:])
}

func TestEggClientAcceptsPinnedCertAndRejectsOthers(t *testing.T) {
	pinned, pin := newPinTestTLSServer(t)
	other, _ := newPinTestTLSServer(t)

	// The pin wins over a leftover tls_skip_verify.
	for _, skipVerify := range []bool{false, true} {
		c := &EggClient{TLSPinSHA256: pin, TLSSkipVerify: skipVerify}
		client := c.httpClient()

		resp, err := client.Get(pinned.URL)
		if err != nil {
			t.Fatalf("skip_verify=%v: pinned master rejected: %v", skipVerify, err)
		}
		_ = resp.Body.Close()

		resp, err = client.Get(other.URL)
		if err == nil {
			_ = resp.Body.Close()
			t.Fatalf("skip_verify=%v: a master with a different certificate was accepted", skipVerify)
		}
		if !strings.Contains(err.Error(), pinMismatchText) {
			t.Fatalf("skip_verify=%v: error = %v, want the pin mismatch", skipVerify, err)
		}
	}

	// Legacy configs without a pin keep their behaviour: tls_skip_verify accepts
	// any certificate, the default client verifies against the system roots.
	legacy := (&EggClient{TLSSkipVerify: true}).httpClient()
	resp, err := legacy.Get(other.URL)
	if err != nil {
		t.Fatalf("legacy tls_skip_verify client rejected the master: %v", err)
	}
	_ = resp.Body.Close()
	if resp, err := (&EggClient{}).httpClient().Get(other.URL); err == nil {
		_ = resp.Body.Close()
		t.Fatal("a client without pin or skip-verify accepted a self-signed certificate")
	}
}

func TestEggClientConnectDialsOnlyThePinnedMaster(t *testing.T) {
	pinned, pin := newPinTestTLSServer(t)
	other, _ := newPinTestTLSServer(t)
	wsURL := func(s *httptest.Server) string {
		return "wss" + strings.TrimPrefix(s.URL, "https") + "/api/invasion/ws"
	}
	key := strings.Repeat("a", 64)

	c := NewEggClient(wsURL(pinned), "egg", "nest", key, "test", testLogger())
	c.TLSPinSHA256 = pin
	err := c.connect()
	// The fixture closes right after the upgrade, so a successful pinned TLS
	// dial surfaces as the missing protocol challenge.
	if err == nil || strings.Contains(err.Error(), "websocket dial") || !strings.Contains(err.Error(), "missing challenge") {
		t.Fatalf("pinned dial: err = %v, want the missing-challenge error after a successful TLS dial", err)
	}

	c = NewEggClient(wsURL(other), "egg", "nest", key, "test", testLogger())
	c.TLSPinSHA256 = pin
	c.TLSSkipVerify = true
	err = c.connect()
	if err == nil || !strings.Contains(err.Error(), "websocket dial") || !strings.Contains(err.Error(), pinMismatchText) {
		t.Fatalf("mismatching dial: err = %v, want a websocket dial error with the pin mismatch", err)
	}
}

func TestPinnedTLSConfigVerifier(t *testing.T) {
	leaf := []byte("leaf certificate DER")
	sum := sha256.Sum256(leaf)
	pin := hex.EncodeToString(sum[:])

	cfg := pinnedTLSConfig(pin)
	if !cfg.InsecureSkipVerify || cfg.VerifyPeerCertificate == nil {
		t.Fatalf("pinned config must replace chain verification with the fingerprint check: %+v", cfg)
	}
	verify := cfg.VerifyPeerCertificate

	if err := verify(nil, nil); err == nil || !strings.Contains(err.Error(), "no certificate presented") {
		t.Fatalf("empty chain: err = %v", err)
	}
	if err := verify([][]byte{leaf, []byte("intermediate")}, nil); err != nil {
		t.Fatalf("matching leaf: %v", err)
	}
	if err := verify([][]byte{[]byte("another leaf")}, nil); err == nil || !strings.Contains(err.Error(), pinMismatchText) {
		t.Fatalf("mismatching leaf: err = %v", err)
	}
	// Only the leaf counts; a matching certificate further up the chain does not.
	if err := verify([][]byte{[]byte("another leaf"), leaf}, nil); err == nil {
		t.Fatal("a non-leaf match must not satisfy the pin")
	}
	if err := pinnedTLSConfig(strings.ToUpper(pin)).VerifyPeerCertificate([][]byte{leaf}, nil); err != nil {
		t.Fatalf("upper-case pin: %v", err)
	}
}
