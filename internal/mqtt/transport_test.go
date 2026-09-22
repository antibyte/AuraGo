package mqtt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"github.com/gorilla/websocket"
)

func TestTestConnectionContextUsesTLSCertificateAndSNI(t *testing.T) {
	certPEM, keyPEM := testCertificate(t, "localhost")
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go serveTestCONNACK(t, listener)

	caPath := t.TempDir() + string(os.PathSeparator) + "ca.pem"
	if err := os.WriteFile(caPath, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Broker = "mqtts://localhost:" + portString(listener.Addr().String())
	cfg.MQTT.ConnectTimeout = 3
	cfg.MQTT.TLS.CAFile = caPath
	if err := TestConnection(cfg, nil); err != nil {
		t.Fatalf("TLS MQTT test connection: %v", err)
	}
}

func TestTestConnectionContextSupportsWebsocketBroker(t *testing.T) {
	upgrader := websocket.Upgrader{Subprotocols: []string{"mqtt"}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, _, _ = conn.ReadMessage()
		_ = conn.WriteMessage(websocket.BinaryMessage, []byte{0x20, 0x02, 0x00, 0x00})
		time.Sleep(20 * time.Millisecond)
	}))
	defer server.Close()
	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Broker = "ws" + strings.TrimPrefix(server.URL, "http")
	cfg.MQTT.ConnectTimeout = 3
	if err := TestConnection(cfg, nil); err != nil {
		t.Fatalf("websocket MQTT test connection: %v", err)
	}
}

func testCertificate(t *testing.T, dnsName string) ([]byte, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: dnsName},
		DNSNames:              []string{dnsName},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(pemBlockForKey(key))
}

func pemBlockForKey(key *rsa.PrivateKey) *pem.Block {
	return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
}

func serveTestCONNACK(t *testing.T, listener net.Listener) {
	t.Helper()
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	first := make([]byte, 2)
	if _, err := io.ReadFull(conn, first); err != nil {
		return
	}
	_, _ = conn.Write([]byte{0x20, 0x02, 0x00, 0x00})
}

func portString(address string) string {
	parts := strings.Split(address, ":")
	return parts[len(parts)-1]
}
