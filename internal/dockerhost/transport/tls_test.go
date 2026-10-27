package transport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

// selfSignedPEM returns one self-signed certificate (usable as CA and as client
// certificate) with its PKCS#8 key, both PEM encoded.
func selfSignedPEM(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "AuraGo transport test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
}

// The error texts are the ones Invasion's nest validation showed before the
// code moved here; they are part of the API answers of the nest handlers.
func TestTLSClientConfigErrorTextsAreUnchanged(t *testing.T) {
	certPEM, keyPEM := selfSignedPEM(t)
	cases := []struct {
		name     string
		mode     string
		material TLSMaterial
		want     string // exact error text, or prefix when ending in "..."
	}{
		{"off", TLSOff, TLSMaterial{}, "docker_tls is off; no TLS configuration applies"},
		{"unknown mode", "starttls", TLSMaterial{}, `invalid docker_tls "starttls" (must be empty, tls or mtls)`},
		{"garbage CA", TLSServer, TLSMaterial{CA: "not a certificate"}, "docker_tls_ca contains no PEM certificate"},
		{"tls rejects client certificate", TLSServer, TLSMaterial{Cert: certPEM, Key: keyPEM}, "a client certificate requires docker_tls mtls"},
		{"mtls needs key", TLSMutual, TLSMaterial{Cert: certPEM}, "docker_tls mtls requires docker_tls_cert and docker_tls_key"},
		{"mtls needs cert", TLSMutual, TLSMaterial{Key: keyPEM}, "docker_tls mtls requires docker_tls_cert and docker_tls_key"},
		{"mtls bad pair", TLSMutual, TLSMaterial{Cert: "x", Key: "y"}, "docker_tls_cert/docker_tls_key: ..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := TLSClientConfig(tc.mode, tc.material)
			if err == nil || cfg != nil {
				t.Fatalf("TLSClientConfig = (%v, %v), want an error", cfg, err)
			}
			if prefix, ok := strings.CutSuffix(tc.want, "..."); ok {
				if !strings.HasPrefix(err.Error(), prefix) {
					t.Fatalf("error = %q, want prefix %q", err.Error(), prefix)
				}
				return
			}
			if err.Error() != tc.want {
				t.Fatalf("error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestTLSClientConfigBuildsVerifyingConfigs(t *testing.T) {
	certPEM, keyPEM := selfSignedPEM(t)
	cases := []struct {
		name      string
		mode      string
		material  TLSMaterial
		wantRoots bool
		wantPairs int
	}{
		{"tls with system roots", TLSServer, TLSMaterial{}, false, 0},
		{"tls with CA", " tls ", TLSMaterial{CA: certPEM}, true, 0},
		{"mtls with pair", TLSMutual, TLSMaterial{CA: certPEM, Cert: certPEM, Key: keyPEM}, true, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := TLSClientConfig(tc.mode, tc.material)
			if err != nil {
				t.Fatalf("TLSClientConfig: %v", err)
			}
			if cfg.MinVersion != tls.VersionTLS12 || cfg.InsecureSkipVerify {
				t.Fatalf("config = TLS min %x, insecure %v; want TLS 1.2+ with verification", cfg.MinVersion, cfg.InsecureSkipVerify)
			}
			if (cfg.RootCAs != nil) != tc.wantRoots || len(cfg.Certificates) != tc.wantPairs {
				t.Fatalf("roots set = %v, client certificates = %d; want %v and %d", cfg.RootCAs != nil, len(cfg.Certificates), tc.wantRoots, tc.wantPairs)
			}
		})
	}
}
