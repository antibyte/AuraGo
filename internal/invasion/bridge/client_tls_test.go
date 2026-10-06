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
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const pinRejectText = "master certificate matches neither the pinned fingerprint nor a trusted chain"

func newTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func serverCertTemplate(t *testing.T, dnsNames []string, ips []net.IP) *x509.Certificate {
	t.Helper()
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}
	return &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"AuraGo Self-Signed"}, CommonName: "AuraGo"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     dnsNames,
		IPAddresses:  ips,
	}
}

func newTestCertificate(t *testing.T, tmpl, parent *x509.Certificate, key *ecdsa.PrivateKey, signer *ecdsa.PrivateKey) tls.Certificate {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, signer)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
}

// newSelfSignedTestCert returns a fresh self-signed server certificate for
// localhost/127.0.0.1, like the one a master generates.
func newSelfSignedTestCert(t *testing.T) tls.Certificate {
	t.Helper()
	key := newTestKey(t)
	tmpl := serverCertTemplate(t, []string{"localhost"}, []net.IP{net.IPv4(127, 0, 0, 1)})
	return newTestCertificate(t, tmpl, tmpl, key, key)
}

// testCA stands in for the CA behind a TLS-terminating proxy (public CA, or a
// private CA installed in the egg host's trust store).
type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCA(t *testing.T) testCA {
	t.Helper()
	key := newTestKey(t)
	tmpl := serverCertTemplate(t, nil, nil)
	tmpl.Subject = pkix.Name{CommonName: "AuraGo Test CA"}
	tmpl.IsCA = true
	tmpl.BasicConstraintsValid = true
	tmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	tmpl.ExtKeyUsage = nil
	return testCA{cert: newTestCertificate(t, tmpl, tmpl, key, key).Leaf, key: key}
}

func (ca testCA) pool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	return pool
}

func (ca testCA) issue(t *testing.T, dnsNames []string, ips []net.IP) tls.Certificate {
	t.Helper()
	return newTestCertificate(t, serverCertTemplate(t, dnsNames, ips), ca.cert, newTestKey(t), ca.key)
}

func fingerprintHex(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// pinTestServer is a TLS server on IPv4 loopback with the given certificate
// (httptest's built-in certificate is shared by every test server, so two
// default servers would carry the same fingerprint). WebSocket requests are
// upgraded and then closed without a challenge; plain requests get 200.
type pinTestServer struct {
	*httptest.Server
	pin         string       // SHA-256 hex of the served leaf
	tlsUpgrades atomic.Int32 // WebSocket upgrades that arrived over TLS
}

func startPinTestServer(t *testing.T, cert tls.Certificate) *pinTestServer {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	s := &pinTestServer{pin: fingerprintHex(cert.Certificate[0])}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	s.Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if websocket.IsWebSocketUpgrade(r) {
			if r.TLS != nil {
				s.tlsUpgrades.Add(1)
			}
			if ws, err := upgrader.Upgrade(w, r, nil); err == nil {
				_ = ws.Close()
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	s.Listener = listener
	s.Config.ErrorLog = log.New(io.Discard, "", 0) // rejected handshakes are expected
	s.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	s.StartTLS()
	t.Cleanup(s.Close)
	return s
}

func (s *pinTestServer) wsURL() string {
	return "wss" + strings.TrimPrefix(s.URL, "https") + "/api/invasion/ws"
}

func TestEggClientAcceptsPinnedCertAndRejectsOthers(t *testing.T) {
	pinned := startPinTestServer(t, newSelfSignedTestCert(t))
	other := startPinTestServer(t, newSelfSignedTestCert(t))

	// The pin wins over a leftover tls_skip_verify.
	for _, skipVerify := range []bool{false, true} {
		c := &EggClient{MasterURL: pinned.wsURL(), TLSPinSHA256: pinned.pin, TLSSkipVerify: skipVerify}
		client := c.httpClient()

		resp, err := client.Get(pinned.URL)
		if err != nil {
			t.Fatalf("skip_verify=%v: pinned master rejected: %v", skipVerify, err)
		}
		_ = resp.Body.Close()

		resp, err = client.Get(other.URL)
		if err == nil {
			_ = resp.Body.Close()
			t.Fatalf("skip_verify=%v: a master with a different self-signed certificate was accepted", skipVerify)
		}
		if !strings.Contains(err.Error(), pinRejectText) {
			t.Fatalf("skip_verify=%v: error = %v, want %q", skipVerify, err, pinRejectText)
		}
	}

	// Legacy configs without a pin keep their behaviour: tls_skip_verify accepts
	// any certificate, the default client verifies against the system roots.
	legacy := (&EggClient{MasterURL: other.wsURL(), TLSSkipVerify: true}).httpClient()
	resp, err := legacy.Get(other.URL)
	if err != nil {
		t.Fatalf("legacy tls_skip_verify client rejected the master: %v", err)
	}
	_ = resp.Body.Close()
	resp, err = (&EggClient{MasterURL: other.wsURL()}).httpClient().Get(other.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("a client without pin or skip-verify accepted a self-signed certificate")
	}
	var verifyErr *tls.CertificateVerificationError
	if !errors.As(err, &verifyErr) {
		t.Fatalf("error = %v, want a *tls.CertificateVerificationError from the default verification", err)
	}
}

// A custom route through a TLS-terminating proxy (e.g. Cloudflare Tunnel)
// presents the proxy's certificate, not the pinned master leaf. It is accepted
// when its chain verifies for the dialed host against the egg's trust store.
func TestEggClientAcceptsTrustedChainWhenPinDiffers(t *testing.T) {
	ca := newTestCA(t)
	proxy := startPinTestServer(t, ca.issue(t, []string{"localhost"}, []net.IP{net.IPv4(127, 0, 0, 1)}))
	masterPin := fingerprintHex(newSelfSignedTestCert(t).Certificate[0])

	trusted := &EggClient{MasterURL: proxy.wsURL(), TLSPinSHA256: masterPin, rootCAs: ca.pool()}
	resp, err := trusted.httpClient().Get(proxy.URL)
	if err != nil {
		t.Fatalf("proxy with a trusted chain rejected: %v", err)
	}
	_ = resp.Body.Close()

	c := NewEggClient(proxy.wsURL(), "egg", "nest", strings.Repeat("a", 64), "test", testLogger())
	c.TLSPinSHA256 = masterPin
	c.rootCAs = ca.pool()
	if err := c.connect(); err == nil || strings.Contains(err.Error(), "websocket dial") || !strings.Contains(err.Error(), "missing challenge") {
		t.Fatalf("WebSocket dial through the trusted proxy: err = %v, want the missing-challenge error after a successful TLS dial", err)
	}
	if got := proxy.tlsUpgrades.Load(); got != 1 {
		t.Fatalf("TLS WebSocket upgrades on the proxy = %d, want 1", got)
	}

	// The same proxy whose CA the egg host does not trust is rejected.
	untrusted := &EggClient{MasterURL: proxy.wsURL(), TLSPinSHA256: masterPin}
	resp, err = untrusted.httpClient().Get(proxy.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("proxy with an untrusted chain was accepted")
	}
	if !strings.Contains(err.Error(), pinRejectText) {
		t.Fatalf("error = %v, want %q", err, pinRejectText)
	}
}

// The chain must be valid for the host of the master URL. For an IP address
// the TLS server name (SNI) is empty, so the check must not rely on it.
func TestEggClientRejectsTrustedChainForAnotherHost(t *testing.T) {
	ca := newTestCA(t)
	srv := startPinTestServer(t, ca.issue(t, []string{"other.example"}, nil))
	c := &EggClient{MasterURL: srv.wsURL(), TLSPinSHA256: strings.Repeat("0", 64), rootCAs: ca.pool()}
	resp, err := c.httpClient().Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("a trusted chain for another host was accepted")
	}
	if !strings.Contains(err.Error(), pinRejectText) || !strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("error = %v, want %q naming 127.0.0.1", err, pinRejectText)
	}
}

func TestEggClientConnectDialsOnlyThePinnedMaster(t *testing.T) {
	pinned := startPinTestServer(t, newSelfSignedTestCert(t))
	other := startPinTestServer(t, newSelfSignedTestCert(t))
	key := strings.Repeat("a", 64)

	c := NewEggClient(pinned.wsURL(), "egg", "nest", key, "test", testLogger())
	c.TLSPinSHA256 = pinned.pin
	err := c.connect()
	// The fixture closes right after the upgrade, so a successful pinned TLS
	// dial surfaces as the missing protocol challenge.
	if err == nil || strings.Contains(err.Error(), "websocket dial") || !strings.Contains(err.Error(), "missing challenge") {
		t.Fatalf("pinned dial: err = %v, want the missing-challenge error after a successful TLS dial", err)
	}
	if got := pinned.tlsUpgrades.Load(); got != 1 {
		t.Fatalf("TLS WebSocket upgrades on the pinned master = %d, want 1", got)
	}

	c = NewEggClient(other.wsURL(), "egg", "nest", key, "test", testLogger())
	c.TLSPinSHA256 = pinned.pin
	c.TLSSkipVerify = true
	err = c.connect()
	if err == nil || !strings.Contains(err.Error(), "websocket dial") || !strings.Contains(err.Error(), pinRejectText) {
		t.Fatalf("mismatching dial: err = %v, want a websocket dial error with %q", err, pinRejectText)
	}
	if got := other.tlsUpgrades.Load(); got != 0 {
		t.Fatalf("WebSocket upgrades on the rejected master = %d, want 0", got)
	}
}

func TestPinnedTLSConfigVerifier(t *testing.T) {
	pinnedLeaf := newSelfSignedTestCert(t).Leaf
	otherLeaf := newSelfSignedTestCert(t).Leaf
	ca := newTestCA(t)
	proxyLeaf := ca.issue(t, []string{"master.example"}, nil).Leaf
	pin := fingerprintHex(pinnedLeaf.Raw)

	cfg := pinnedTLSConfig(pin, "master.example", ca.pool())
	if !cfg.InsecureSkipVerify || cfg.VerifyConnection == nil || cfg.VerifyPeerCertificate != nil {
		t.Fatalf("pinned config must replace the built-in verification with VerifyConnection: %+v", cfg)
	}
	verifyWith := func(cfg *tls.Config, certs ...*x509.Certificate) error {
		return cfg.VerifyConnection(tls.ConnectionState{PeerCertificates: certs})
	}

	if err := verifyWith(cfg); err == nil || !strings.Contains(err.Error(), "no certificate presented") {
		t.Fatalf("empty chain: err = %v", err)
	}
	if err := verifyWith(cfg, pinnedLeaf, otherLeaf); err != nil {
		t.Fatalf("matching leaf: %v", err)
	}
	if err := verifyWith(cfg, otherLeaf); err == nil || !strings.Contains(err.Error(), pinRejectText) || !strings.Contains(err.Error(), "master.example") {
		t.Fatalf("mismatching untrusted leaf: err = %v, want %q naming the host", err, pinRejectText)
	}
	// Only the leaf counts; a matching certificate further up the chain does not.
	if err := verifyWith(cfg, otherLeaf, pinnedLeaf); err == nil {
		t.Fatal("a non-leaf match must not satisfy the pin")
	}
	if err := verifyWith(pinnedTLSConfig(strings.ToUpper(pin), "master.example", nil), pinnedLeaf); err != nil {
		t.Fatalf("upper-case pin: %v", err)
	}

	// Trusted chain: accepted for the right host and trust store only.
	if err := verifyWith(cfg, proxyLeaf); err != nil {
		t.Fatalf("trusted chain for the master host: %v", err)
	}
	if err := verifyWith(pinnedTLSConfig(pin, "other.example", ca.pool()), proxyLeaf); err == nil {
		t.Fatal("a trusted chain for another host must be rejected")
	}
	if err := verifyWith(pinnedTLSConfig(pin, "", ca.pool()), proxyLeaf); err == nil || !strings.Contains(err.Error(), pinRejectText) {
		t.Fatalf("unknown master host must fail closed: err = %v", err)
	}
	if err := verifyWith(pinnedTLSConfig(pin, "master.example", nil), proxyLeaf); err == nil {
		t.Fatal("a chain the system roots do not trust must be rejected")
	}
}

func TestValidTLSPin(t *testing.T) {
	pin := fingerprintHex([]byte("certificate"))
	for _, tc := range []struct {
		pin  string
		want bool
	}{
		{pin, true},
		{strings.ToUpper(pin), true},
		{" " + pin + " ", true},
		{"", false},
		{pin[:63], false},
		{pin + "0", false},
		{"g" + pin[1:], false},
		{"sha256:" + pin, false},
	} {
		if got := ValidTLSPin(tc.pin); got != tc.want {
			t.Errorf("ValidTLSPin(%q) = %v, want %v", tc.pin, got, tc.want)
		}
	}
}
