package invasion

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/testutil"
)

func pemBlock(kind string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}))
}

// newTestClientPKI returns a client CA (PEM and pool) and a client
// certificate with its PKCS#8 key, all valid for one hour.
func newTestClientPKI(t *testing.T) (caPEM string, pool *x509.CertPool, certPEM, keyPEM string) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "AuraGo test client CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "aurago-master"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	pool = x509.NewCertPool()
	pool.AddCert(caCert)
	return pemBlock("CERTIFICATE", caDER), pool, pemBlock("CERTIFICATE", clientDER), pemBlock("PRIVATE KEY", keyDER)
}

func dockerTLSSecret(t *testing.T, material DockerTLSMaterial) []byte {
	t.Helper()
	raw, err := json.Marshal(material)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// dockerVersionHandler answers every path with Engine version metadata, which
// satisfies both the negotiation probe and Validate's GET /version.
func dockerVersionHandler(hits *atomic.Int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			hits.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"ApiVersion": "1.45", "MinAPIVersion": "1.25", "Version": "27.0.0"})
	})
}

func nestForTLSMock(ts *httptest.Server, mode string) NestRecord {
	addr := ts.Listener.Addr().(*net.TCPAddr)
	return NestRecord{ID: "12345678-abcd-ef12-3456-7890abcdef12", Host: addr.IP.String(), Port: addr.Port, DeployMethod: "docker_remote", DockerTLS: mode}
}

func TestDockerConnector_apiURL_RemoteTLS(t *testing.T) {
	c := &DockerConnector{}
	cases := map[int]string{0: "https://10.0.0.5:2376/", 2380: "https://10.0.0.5:2380/"}
	for port, prefix := range cases {
		got := c.apiURL(NestRecord{Host: "10.0.0.5", Port: port, DeployMethod: "docker_remote", DockerTLS: DockerTLSServer}, "/version")
		if want := fmt.Sprintf("%s%s/version", prefix, dockerAPIVersion); got != want {
			t.Fatalf("apiURL(port %d) = %q, want %q", port, got, want)
		}
	}
}

func TestDockerConnectorTLSVerifiesEngineAgainstStoredCA(t *testing.T) {
	ts := testutil.NewHTTPSServer(t, dockerVersionHandler(nil))
	defer ts.Close()
	secret := dockerTLSSecret(t, DockerTLSMaterial{CA: pemBlock("CERTIFICATE", ts.Certificate().Raw)})
	if err := (&DockerConnector{}).Validate(context.Background(), nestForTLSMock(ts, DockerTLSServer), secret); err != nil {
		t.Fatalf("Validate over TLS with the engine CA: %v", err)
	}
}

func TestDockerConnectorTLSRejectsEngineOutsideTrustedCA(t *testing.T) {
	ts := testutil.NewHTTPSServer(t, dockerVersionHandler(nil))
	defer ts.Close()
	ts.Config.ErrorLog = log.New(io.Discard, "", 0)
	err := (&DockerConnector{}).Validate(context.Background(), nestForTLSMock(ts, DockerTLSServer), dockerTLSSecret(t, DockerTLSMaterial{}))
	if err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("Validate against an untrusted engine = %v, want a certificate error", err)
	}
}

func TestDockerConnectorTLSNeverFallsBackToPlainHTTP(t *testing.T) {
	var hits atomic.Int64
	ts := testutil.NewHTTPServer(t, dockerVersionHandler(&hits))
	defer ts.Close()
	ts.Config.ErrorLog = log.New(io.Discard, "", 0)
	if err := (&DockerConnector{}).Validate(context.Background(), nestForTLSMock(ts, DockerTLSServer), dockerTLSSecret(t, DockerTLSMaterial{})); err == nil {
		t.Fatal("a TLS nest talked to a plain-HTTP engine")
	}
	if hits.Load() != 0 {
		t.Fatalf("plain-HTTP handler served %d requests for a TLS nest, want 0", hits.Load())
	}
}

func TestDockerConnectorTLSFailsClosedOnUnusableMaterial(t *testing.T) {
	var hits atomic.Int64
	ts := testutil.NewHTTPServer(t, dockerVersionHandler(&hits))
	defer ts.Close()
	cases := map[string]struct {
		mode   string
		secret []byte
	}{
		"mtls without certificate": {mode: DockerTLSMutual, secret: dockerTLSSecret(t, DockerTLSMaterial{})},
		"unreadable material":      {mode: DockerTLSServer, secret: []byte("not json")},
		"garbage CA":               {mode: DockerTLSServer, secret: dockerTLSSecret(t, DockerTLSMaterial{CA: "not a certificate"})},
	}
	for name, tc := range cases {
		if err := (&DockerConnector{}).Validate(context.Background(), nestForTLSMock(ts, tc.mode), tc.secret); err == nil {
			t.Fatalf("%s: Validate succeeded", name)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("engine served %d requests with unusable TLS material, want 0", hits.Load())
	}
}

func TestDockerConnectorMutualTLSPresentsClientCertificate(t *testing.T) {
	_, clientCAs, certPEM, keyPEM := newTestClientPKI(t)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	ts := httptest.NewUnstartedServer(dockerVersionHandler(nil))
	ts.Listener = listener
	ts.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}
	ts.Config.ErrorLog = log.New(io.Discard, "", 0)
	ts.StartTLS()
	defer ts.Close()
	engineCA := pemBlock("CERTIFICATE", ts.Certificate().Raw)
	c := &DockerConnector{}

	withClient := dockerTLSSecret(t, DockerTLSMaterial{CA: engineCA, Cert: certPEM, Key: keyPEM})
	if err := c.Validate(context.Background(), nestForTLSMock(ts, DockerTLSMutual), withClient); err != nil {
		t.Fatalf("Validate with the client certificate: %v", err)
	}
	withoutClient := dockerTLSSecret(t, DockerTLSMaterial{CA: engineCA})
	if err := c.Validate(context.Background(), nestForTLSMock(ts, DockerTLSServer), withoutClient); err == nil {
		t.Fatal("an engine that requires client certificates accepted a connection without one")
	}
}

func TestDockerRemoteTLSTransportKeepsProxyFromEnvironment(t *testing.T) {
	nest := NestRecord{ID: "12345678-abcd-ef12-3456-7890abcdef12", Host: "10.0.0.5", DeployMethod: "docker_remote", DockerTLS: DockerTLSServer}
	rt := dockerRemoteTLSTransport(nest, dockerTLSSecret(t, DockerTLSMaterial{}))
	transport, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", rt)
	}
	if transport.Proxy == nil || reflect.ValueOf(transport.Proxy).Pointer() != reflect.ValueOf(http.ProxyFromEnvironment).Pointer() {
		t.Fatal("TLS transport must keep http.ProxyFromEnvironment like the plain docker_remote transport")
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion != tls.VersionTLS12 || transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("TLS config = %+v, want TLS 1.2+ with verification", transport.TLSClientConfig)
	}
}

func TestValidateDockerTLS(t *testing.T) {
	caPEM, _, certPEM, keyPEM := newTestClientPKI(t)
	cases := []struct {
		name     string
		mode     string
		material DockerTLSMaterial
		wantErr  bool
	}{
		{name: "off ignores material", mode: "", material: DockerTLSMaterial{CA: "ignored"}},
		{name: "tls with system roots", mode: DockerTLSServer},
		{name: "tls with CA", mode: DockerTLSServer, material: DockerTLSMaterial{CA: caPEM}},
		{name: "tls rejects garbage CA", mode: DockerTLSServer, material: DockerTLSMaterial{CA: "not a certificate"}, wantErr: true},
		{name: "tls rejects client certificate", mode: DockerTLSServer, material: DockerTLSMaterial{Cert: certPEM, Key: keyPEM}, wantErr: true},
		{name: "mtls needs key", mode: DockerTLSMutual, material: DockerTLSMaterial{Cert: certPEM}, wantErr: true},
		{name: "mtls with pair", mode: DockerTLSMutual, material: DockerTLSMaterial{CA: caPEM, Cert: certPEM, Key: keyPEM}},
		{name: "unknown mode", mode: "starttls", wantErr: true},
	}
	for _, tc := range cases {
		err := ValidateDockerTLS(tc.mode, tc.material)
		if (err != nil) != tc.wantErr {
			t.Fatalf("%s: ValidateDockerTLS error = %v, wantErr %v", tc.name, err, tc.wantErr)
		}
	}
}

func TestDockerRemotePlaintextSkipsTLSNests(t *testing.T) {
	if DockerRemotePlaintext(NestRecord{DeployMethod: "docker_remote", DockerTLS: DockerTLSServer}) {
		t.Fatal("a TLS docker_remote nest was reported as plaintext")
	}
	if !DockerRemotePlaintext(NestRecord{DeployMethod: "docker_remote"}) {
		t.Fatal("a plain docker_remote nest was not reported")
	}
}
