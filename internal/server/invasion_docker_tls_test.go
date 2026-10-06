package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/invasion"
	"aurago/internal/security"
)

func newInvasionTLSTestServer(t *testing.T) *Server {
	t.Helper()
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	return &Server{InvasionDB: setupInvasionTestDB(t), Vault: vault, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// testDockerTLSPEMs returns a CA plus a client certificate and PKCS#8 key it signed.
func testDockerTLSPEMs(t *testing.T) (caPEM, certPEM, keyPEM string) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "AuraGo test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, BasicConstraintsValid: true,
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
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "aurago-master"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	encode := func(kind string, der []byte) string {
		return string(pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}))
	}
	return encode("CERTIFICATE", caDER), encode("CERTIFICATE", clientDER), encode("PRIVATE KEY", keyDER)
}

func invasionTLSRequest(t *testing.T, handler http.HandlerFunc, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	return rec
}

func createMutualTLSNest(t *testing.T, s *Server) (string, string, string, string) {
	t.Helper()
	ca, cert, key := testDockerTLSPEMs(t)
	rec := invasionTLSRequest(t, handleInvasionNests(s), http.MethodPost, "/api/invasion/nests", map[string]any{
		"name": "tls-nest", "access_type": "docker", "host": "10.0.0.5", "deploy_method": "docker_remote", "active": true,
		"docker_tls": "mtls", "docker_tls_ca": ca, "docker_tls_cert": cert, "docker_tls_key": key,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, body %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "PRIVATE KEY") {
		t.Fatal("create response leaked the client key")
	}
	var created struct {
		ID   string `json:"id"`
		Port int    `json:"port"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Port != 2376 {
		t.Fatalf("port = %d, want the Docker TLS default 2376", created.Port)
	}
	return created.ID, ca, cert, key
}

func TestCreateDockerRemoteNestWithMutualTLSStoresMaterialInVault(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	id, ca, cert, key := createMutualTLSNest(t, s)
	nest, err := invasion.GetNest(s.InvasionDB, id)
	if err != nil || nest.DockerTLS != "mtls" {
		t.Fatalf("nest DockerTLS = %q, %v; want mtls", nest.DockerTLS, err)
	}
	material, err := s.loadNestDockerTLS(id)
	if err != nil {
		t.Fatal(err)
	}
	if material.CA != strings.TrimSpace(ca) || material.Cert != strings.TrimSpace(cert) || material.Key != strings.TrimSpace(key) {
		t.Fatal("vault material does not match the submitted PEMs")
	}
}

func TestCreateNestRejectsInvalidDockerTLS(t *testing.T) {
	ca, cert, key := testDockerTLSPEMs(t)
	cases := map[string]map[string]any{
		"mtls without key":        {"deploy_method": "docker_remote", "docker_tls": "mtls", "docker_tls_cert": cert},
		"garbage CA":              {"deploy_method": "docker_remote", "docker_tls": "tls", "docker_tls_ca": "not a pem"},
		"client cert in tls mode": {"deploy_method": "docker_remote", "docker_tls": "tls", "docker_tls_cert": cert, "docker_tls_key": key},
		"TLS on the SSH method":   {"deploy_method": "ssh", "docker_tls": "tls"},
		"unknown mode":            {"deploy_method": "docker_remote", "docker_tls": "starttls"},
		"PEMs without a TLS mode": {"deploy_method": "docker_remote", "docker_tls": "", "docker_tls_ca": ca},
	}
	for name, fields := range cases {
		s := newInvasionTLSTestServer(t)
		body := map[string]any{"name": "nest", "access_type": "docker", "host": "10.0.0.5", "active": true}
		for k, v := range fields {
			body[k] = v
		}
		rec := invasionTLSRequest(t, handleInvasionNests(s), http.MethodPost, "/api/invasion/nests", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400 (body %s)", name, rec.Code, rec.Body.String())
		}
		if nests, _ := invasion.ListNests(s.InvasionDB); len(nests) != 0 {
			t.Fatalf("%s: a rejected request created %d nests", name, len(nests))
		}
	}
}

func TestUpdateNestWithoutDockerTLSFieldKeepsEncryption(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	id, _, cert, _ := createMutualTLSNest(t, s)
	rec := invasionTLSRequest(t, handleInvasionNest(s), http.MethodPut, "/api/invasion/nests/"+id, map[string]any{
		"name": "tls-nest", "notes": "renamed by an older client", "access_type": "docker", "host": "10.0.0.5", "port": 2376,
		"active": true, "deploy_method": "docker_remote", "target_arch": "linux/amd64", "route": "direct",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body %s", rec.Code, rec.Body.String())
	}
	nest, _ := invasion.GetNest(s.InvasionDB, id)
	if nest.DockerTLS != "mtls" {
		t.Fatalf("DockerTLS = %q after an update without docker_tls, want mtls kept", nest.DockerTLS)
	}
	if material, _ := s.loadNestDockerTLS(id); material.Cert != strings.TrimSpace(cert) {
		t.Fatal("update without PEM fields dropped the stored client certificate")
	}
}

func TestUpdateNestSwitchingDockerTLSOffRemovesMaterial(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	id, _, _, _ := createMutualTLSNest(t, s)
	rec := invasionTLSRequest(t, handleInvasionNest(s), http.MethodPut, "/api/invasion/nests/"+id, map[string]any{
		"name": "tls-nest", "access_type": "docker", "host": "10.0.0.5", "port": 2375, "active": true,
		"deploy_method": "docker_remote", "target_arch": "linux/amd64", "route": "direct", "docker_tls": "",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body %s", rec.Code, rec.Body.String())
	}
	if nest, _ := invasion.GetNest(s.InvasionDB, id); nest.DockerTLS != "" {
		t.Fatalf("DockerTLS = %q, want plain", nest.DockerTLS)
	}
	if material, err := s.loadNestDockerTLS(id); err != nil || material != (invasion.DockerTLSMaterial{}) {
		t.Fatalf("material = %+v, %v; want removed", material, err)
	}
}

func TestDeleteNestRemovesDockerTLSMaterial(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	id, _, _, _ := createMutualTLSNest(t, s)
	rec := invasionTLSRequest(t, handleInvasionNest(s), http.MethodDelete, "/api/invasion/nests/"+id, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body %s", rec.Code, rec.Body.String())
	}
	if _, err := s.Vault.ReadSecret(invasion.DockerTLSVaultKey(id)); err != security.ErrSecretNotFound {
		t.Fatalf("vault read after delete = %v, want ErrSecretNotFound", err)
	}
}

func TestInvasionTransportSecretChoosesCredentialByTransport(t *testing.T) {
	s := newInvasionTLSTestServer(t)
	id, _, cert, _ := createMutualTLSNest(t, s)
	tlsNest, _ := invasion.GetNest(s.InvasionDB, id)
	secret, err := s.invasionTransportSecret(tlsNest)
	if err != nil || !strings.Contains(string(secret), "BEGIN CERTIFICATE") {
		t.Fatalf("TLS nest credential = %q, %v; want the TLS material", secret, err)
	}
	var material invasion.DockerTLSMaterial
	if err := json.Unmarshal(secret, &material); err != nil || material.Cert != strings.TrimSpace(cert) {
		t.Fatalf("TLS credential does not decode to the stored material: %v", err)
	}

	if err := s.Vault.WriteSecret("nest_plain", "ssh-password"); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"ssh", "docker_remote"} {
		plain := invasion.NestRecord{ID: "12345678-0000-0000-0000-000000000000", DeployMethod: method, VaultSecretID: "nest_plain"}
		got, err := s.invasionTransportSecret(plain)
		if err != nil || string(got) != "ssh-password" {
			t.Fatalf("%s credential = %q, %v; want the nest secret as before", method, got, err)
		}
	}
	if got, err := s.invasionTransportSecret(invasion.NestRecord{DeployMethod: "docker_remote"}); err != nil || got != nil {
		t.Fatalf("nest without secret = %q, %v; want nil", got, err)
	}
}

func TestResolveNestDockerTLSServerModeDropsStoredClientCertificate(t *testing.T) {
	ca, cert, key := testDockerTLSPEMs(t)
	tlsMode := "tls"
	mode, material, err := resolveNestDockerTLS("docker_remote", "mtls",
		invasion.DockerTLSMaterial{CA: strings.TrimSpace(ca), Cert: strings.TrimSpace(cert), Key: strings.TrimSpace(key)},
		nestDockerTLSRequest{DockerTLS: &tlsMode})
	if err != nil || mode != "tls" || material == nil || material.Cert != "" || material.Key != "" || material.CA == "" {
		t.Fatalf("resolve = %q, %+v, %v; want tls keeping only the CA", mode, material, err)
	}
}

func TestInvasionSecurityHintsSkipTLSDockerRemoteNests(t *testing.T) {
	db := setupInvasionTestDB(t)
	if _, err := invasion.CreateNest(db, invasion.NestRecord{Name: "secure", Active: true, DeployMethod: "docker_remote", Host: "10.0.0.5", DockerTLS: "tls"}); err != nil {
		t.Fatal(err)
	}
	if hints := invasionSecurityHints(db, nil); len(hints) != 0 {
		t.Fatalf("hints = %#v, want none for a TLS nest", hints)
	}
}
