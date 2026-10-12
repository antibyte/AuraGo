package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/cydcreator"
)

// writeTestCert writes a self-signed certificate (optionally preceded by its
// private key, like an autocert cache file) and returns the SHA-256 of its DER.
func writeTestCert(t *testing.T, path string, withKey bool) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "aurago.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"aurago.test"}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	var out []byte
	if withKey {
		keyDER, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		out = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	}
	out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// cydDeviceErrorDetail returns details[key] of a typed CYD Creator error.
func cydDeviceErrorDetail(err error, key string) string {
	var typed *cydcreator.Error
	if !errors.As(err, &typed) || typed == nil {
		return ""
	}
	value, _ := typed.Details[key].(string)
	return value
}

func TestCYDCreatorDeviceBaseURL(t *testing.T) {
	base := func(mutate func(c *config.Config)) (string, error) {
		c := &config.Config{}
		c.Server.Host = "192.168.1.5"
		c.Server.Port = 8088
		mutate(c)
		return cydCreatorDeviceBaseURL(c)
	}
	for _, tc := range []struct {
		name   string
		mutate func(c *config.Config)
		want   string
	}{
		{"http", func(c *config.Config) {}, "http://192.168.1.5:8088"},
		{"http default port", func(c *config.Config) { c.Server.Port = 80 }, "http://192.168.1.5"},
		{"https domain", func(c *config.Config) {
			c.Server.HTTPS.Enabled, c.Server.HTTPS.Domain, c.Server.HTTPS.HTTPSPort = true, "aurago.example", 443
		}, "https://aurago.example"},
		{"https port", func(c *config.Config) {
			c.Server.HTTPS.Enabled, c.Server.HTTPS.Domain, c.Server.HTTPS.HTTPSPort = true, "aurago.example", 8443
		}, "https://aurago.example:8443"},
		{"https host", func(c *config.Config) { c.Server.HTTPS.Enabled, c.Server.HTTPS.HTTPSPort = true, 8443 }, "https://192.168.1.5:8443"},
		{"ipv6", func(c *config.Config) { c.Server.Host = "fd00::5" }, "http://[fd00::5]:8088"},
		{"public url", func(c *config.Config) { c.CYDCreator.OTA.PublicURL = " https://cyd.example/aurago/ " }, "https://cyd.example/aurago"},
	} {
		got, err := base(tc.mutate)
		if err != nil || got != tc.want {
			t.Errorf("%s: %q, %v (want %q)", tc.name, got, err, tc.want)
		}
	}
	if got, err := base(func(c *config.Config) { c.Server.Host = "0.0.0.0" }); err != nil || strings.Contains(got, "0.0.0.0") || !strings.HasPrefix(got, "http://") {
		t.Errorf("wildcard bind: %q, %v", got, err)
	}
	// The device URL rule (cydcreator.ValidateDeviceURL), not the looser
	// admin URL rule: upper-case schemes, IPv6 literals, port 0 and hosts
	// outside [A-Za-z0-9.-] never reach a device.
	for _, bad := range []string{"ftp://cyd.example", "https://user:pw@cyd.example", "https://cyd.example/?x=1", "cyd.example",
		"HTTPS://cyd.example", "http://[fd00::5]:8088", "http://cyd.example:0", "https://cyd_example", "http://b%C3%BCcher.de"} {
		if _, err := base(func(c *config.Config) { c.CYDCreator.OTA.PublicURL = bad }); err == nil {
			t.Errorf("public_url %q should be refused", bad)
		} else if cydcreator.ErrorCode(err) != cydcreator.CodeBadRequest || cydDeviceErrorDetail(err, "reason") != "device_url" {
			t.Errorf("public_url %q: %v, want CYD_BAD_REQUEST reason device_url", bad, err)
		}
	}
}

func TestCYDCreatorDeviceEndpointPinsTheServedCertificate(t *testing.T) {
	ctx := context.Background()
	https := func(mode, domain string) *config.Config {
		c := &config.Config{}
		c.Directories.DataDir = t.TempDir()
		c.Server.HTTPS.Enabled, c.Server.HTTPS.CertMode, c.Server.HTTPS.Domain, c.Server.HTTPS.HTTPSPort = true, mode, domain, 8443
		return c
	}

	self := https("selfsigned", "")
	self.Server.Host = "192.168.1.5"
	want := writeTestCert(t, config.SelfSignedCertFile(self.Directories.DataDir), false)
	if got, err := cydCreatorDeviceEndpoint(ctx, self); err != nil || got.URL != "https://192.168.1.5:8443" || got.CertSHA256 != want {
		t.Fatalf("self-signed: %+v, %v (want pin %s)", got, err, want)
	}

	custom := https("custom", "aurago.example")
	custom.Server.HTTPS.CertFile = filepath.Join(custom.Directories.DataDir, "my", "cert.pem")
	want = writeTestCert(t, custom.Server.HTTPS.CertFile, false)
	if got, err := cydCreatorDeviceEndpoint(ctx, custom); err != nil || got.CertSHA256 != want {
		t.Fatalf("custom: %+v, %v", got, err)
	}

	acme := https("auto", "aurago.example")
	want = writeTestCert(t, filepath.Join(acme.Directories.DataDir, "certs", "aurago.example"), true)
	if got, err := cydCreatorDeviceEndpoint(ctx, acme); err != nil || got.URL != "https://aurago.example:8443" || got.CertSHA256 != want {
		t.Fatalf("let's encrypt cache: %+v, %v", got, err)
	}

	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	public := &config.Config{}
	public.CYDCreator.OTA.PublicURL = srv.URL
	sum := sha256.Sum256(srv.Certificate().Raw)
	if got, err := cydCreatorDeviceEndpoint(ctx, public); err != nil || got.URL != srv.URL || got.CertSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("public url probe: %+v, %v", got, err)
	}

	plain := &config.Config{}
	plain.Server.Host, plain.Server.Port = "192.168.1.5", 8088
	if got, err := cydCreatorDeviceEndpoint(ctx, plain); err != nil || got.URL != "http://192.168.1.5:8088" || got.CertSHA256 != "" {
		t.Fatalf("http: %+v, %v", got, err)
	}

	// D4: a pin that cannot be read is its own reason with the public_url
	// hint and passes cydError unmasked (never a bare CYD_INTERNAL).
	unreachable := &config.Config{}
	unreachable.CYDCreator.OTA.PublicURL = "https://127.0.0.1:1"
	_, err := cydCreatorDeviceEndpoint(ctx, unreachable)
	if cydcreator.ErrorCode(err) != cydcreator.CodeBadRequest || cydDeviceErrorDetail(err, "reason") != "tls_pin" ||
		cydDeviceErrorDetail(err, "hint") != "cyd_creator.ota.public_url" || !strings.Contains(err.Error(), "TLS certificate") {
		t.Fatalf("unreachable public url: %v", err)
	}
	rec := httptest.NewRecorder()
	(&Server{}).cydError(rec, httptest.NewRequest(http.MethodPost, "/api/desktop/cyd-creator/projects/p/devices", nil), err)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"reason":"tls_pin"`) {
		t.Fatalf("cydError masks the pin failure: %d %s", rec.Code, rec.Body.String())
	}

	broken := &config.Config{}
	broken.CYDCreator.OTA.PublicURL = "ftp://x"
	if _, err := cydCreatorDeviceEndpoint(ctx, broken); cydcreator.ErrorCode(err) != cydcreator.CodeBadRequest ||
		cydDeviceErrorDetail(err, "reason") != "device_url" {
		t.Fatalf("invalid public url: %v", err)
	}

	// A derived URL the firmware cannot use (IPv6 bind) is refused before any
	// probe, with the hint to set public_url.
	ipv6 := &config.Config{}
	ipv6.Server.Host, ipv6.Server.Port = "fd00::5", 8088
	if _, err := cydCreatorDeviceEndpoint(ctx, ipv6); cydcreator.ErrorCode(err) != cydcreator.CodeBadRequest ||
		cydDeviceErrorDetail(err, "reason") != "device_url" || cydDeviceErrorDetail(err, "hint") != "cyd_creator.ota.public_url" {
		t.Fatalf("ipv6 bind: %v", err)
	}
}
