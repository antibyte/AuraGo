package meshcentral

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestConnectTrustsConfiguredCA(t *testing.T) {
	srv := newTLSMeshServer(t)
	defer srv.Close()

	c, err := NewClient(srv.URL, "admin", "pass", "", false)
	if err != nil {
		t.Fatal(err)
	}
	c.SetLogger(testLogger())
	c.SetCAFile(writeCertPEM(t, srv.Certificate()))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatalf("ConnectContext: %v", err)
	}
	c.Close()
}

func TestConnectRejectsDifferentCA(t *testing.T) {
	srv := newTLSMeshServer(t)
	defer srv.Close()

	c, err := NewClient(srv.URL, "admin", "pass", "", false)
	if err != nil {
		t.Fatal(err)
	}
	c.SetLogger(testLogger())
	c.SetCAFile(writeCertPEM(t, differentSelfSignedCert(t)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.ConnectContext(ctx); err == nil {
		c.Close()
		t.Fatal("expected TLS failure for a different CA")
	}
}

func TestConnectMissingCAFileFails(t *testing.T) {
	srv := newTLSMeshServer(t)
	defer srv.Close()

	c, err := NewClient(srv.URL, "admin", "pass", "", false)
	if err != nil {
		t.Fatal(err)
	}
	c.SetLogger(testLogger())
	c.SetCAFile(filepath.Join(t.TempDir(), "missing.pem"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = c.ConnectContext(ctx)
	if err == nil {
		c.Close()
		t.Fatal("expected missing CA file to fail")
	}
}

func newTLSMeshServer(t *testing.T) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`<html random="nonce123"></html>`))
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "meshcom", Value: "session123"})
	})
	mux.HandleFunc("/control.ashx", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		_ = ws.WriteJSON(map[string]interface{}{
			"action": "serverinfo",
			"serverinfo": map[string]interface{}{
				"serverVersion": "test",
			},
		})
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if json.Unmarshal(msg, &data) != nil {
				continue
			}
			if data["action"] == "serverinfo" {
				rid, _ := data["reqid"].(float64)
				_ = ws.WriteJSON(map[string]interface{}{
					"reqid":         int(rid),
					"action":        "serverinfo",
					"serverVersion": "test",
				})
			}
		}
	})
	return httptest.NewTLSServer(mux)
}

func writeCertPEM(t *testing.T, cert *x509.Certificate) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.pem")
	body := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func differentSelfSignedCert(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"other.example"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}
