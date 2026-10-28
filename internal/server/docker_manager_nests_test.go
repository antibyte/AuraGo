package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/invasion"
)

// dmTestCAPEM returns a self-signed CA certificate in PEM form.
func dmTestCAPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "docker-manager-test-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestDockerManagerNestListAndImport(t *testing.T) {
	f := newDMFixture(t)
	db, err := invasion.InitDB(filepath.Join(t.TempDir(), "invasion.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f.s.InvasionDB = db

	const nestSecret = "NEST-SSH-PRIVATE-KEY"
	ca := dmTestCAPEM(t)
	if err := f.s.Vault.WriteSecret("nest_ssh_edge", nestSecret); err != nil {
		t.Fatal(err)
	}
	create := func(n invasion.NestRecord) string {
		id, err := invasion.CreateNest(db, n)
		if err != nil {
			t.Fatalf("CreateNest(%s): %v", n.Name, err)
		}
		return id
	}
	sshID := create(invasion.NestRecord{Name: "Edge SSH", Host: "192.0.2.20", Port: 2222, Username: "deploy", VaultSecretID: "nest_ssh_edge", DeployMethod: "docker_ssh", Active: true})
	tlsID := create(invasion.NestRecord{Name: "NAS TLS", Host: "nas.lan", Port: 2376, DeployMethod: "docker_remote", DockerTLS: "tls", Active: true})
	localID := create(invasion.NestRecord{Name: "This box", DeployMethod: "docker_local", Active: true})
	plainID := create(invasion.NestRecord{Name: "Plain SSH", Host: "192.0.2.30", DeployMethod: "ssh", Active: true})
	tlsMaterial, _ := json.Marshal(map[string]string{"ca": ca})
	if err := f.s.Vault.WriteSecret(invasion.DockerTLSVaultKey(tlsID), string(tlsMaterial)); err != nil {
		t.Fatal(err)
	}

	code, body := f.do(http.MethodGet, "/nests", nil)
	nests := dmArray(t, body, "nests")
	if code != http.StatusOK || len(nests) != 3 {
		t.Fatalf("nests = %d %s, want the three Docker nests", code, f.lastRaw)
	}
	for _, id := range []string{sshID, tlsID, localID} {
		dmFind(t, nests, "id", id)
	}
	for _, n := range nests {
		if n["id"] == plainID {
			t.Fatal("a plain ssh nest is listed as a Docker nest")
		}
	}

	// docker_ssh -> ssh host; the secret is copied, not shared.
	code, body = f.do(http.MethodPost, "/hosts/import-nest/"+sshID, nil)
	host := dmObject(t, body, "host")
	if code != http.StatusCreated || host["kind"] != "ssh" || host["ssh_user"] != "deploy" || host["port"] != float64(2222) ||
		host["address"] != "192.0.2.20" || host["name"] != "Edge SSH" || host["secret_set"] != true {
		t.Fatalf("ssh import = %d %s", code, f.lastRaw)
	}
	if strings.Contains(f.lastRaw, nestSecret) {
		t.Fatal("the import answer echoes the nest secret")
	}
	sshHost := dmString(t, host, "id")
	if got, err := f.s.Vault.ReadSecret(dockerhost.SSHSecretKey(sshHost)); err != nil || got != nestSecret {
		t.Fatalf("copied ssh secret = %q, %v", got, err)
	}
	// Host names are unique: importing the same nest twice is a validation error.
	code, body = f.do(http.MethodPost, "/hosts/import-nest/"+sshID, nil)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_host")
	if body["field"] != "name" {
		t.Fatalf("second import = %s, want field name", f.lastRaw)
	}

	// docker_remote with tls -> tls host with the TLS bundle copied.
	code, body = f.do(http.MethodPost, "/hosts/import-nest/"+tlsID, nil)
	host = dmObject(t, body, "host")
	if code != http.StatusCreated || host["kind"] != "tls" || host["port"] != float64(2376) || host["secret_set"] != true {
		t.Fatalf("tls import = %d %s", code, f.lastRaw)
	}
	tlsHost := dmString(t, host, "id")
	if raw, err := f.s.Vault.ReadSecret(dockerhost.TLSSecretKey(tlsHost)); err != nil || !strings.Contains(raw, "BEGIN CERTIFICATE") {
		t.Fatalf("copied tls bundle = %q, %v", raw, err)
	}

	// Deleting the imported host leaves the nest's own secrets alone.
	f.do(http.MethodDelete, "/hosts/"+sshHost, nil)
	f.do(http.MethodDelete, "/hosts/"+tlsHost, nil)
	if got, err := f.s.Vault.ReadSecret("nest_ssh_edge"); err != nil || got != nestSecret {
		t.Fatalf("nest ssh secret after deleting the host = %q, %v", got, err)
	}
	if _, err := f.s.Vault.ReadSecret(invasion.DockerTLSVaultKey(tlsID)); err != nil {
		t.Fatalf("nest tls bundle after deleting the host: %v", err)
	}

	code, body = f.do(http.MethodPost, "/hosts/import-nest/"+localID, nil)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "nest_not_docker")
	if msg, _ := body["message"].(string); !strings.Contains(msg, "already the Local host") {
		t.Fatalf("local nest message = %q", msg)
	}
	code, body = f.do(http.MethodPost, "/hosts/import-nest/"+plainID, nil)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "nest_not_docker")
	code, body = f.do(http.MethodPost, "/hosts/import-nest/no-such-nest", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "nest_not_found")
}

func TestDockerManagerNestsWithoutInvasion(t *testing.T) {
	f := newDMFixture(t)
	code, body := f.do(http.MethodGet, "/nests", nil)
	if nests := dmArray(t, body, "nests"); code != http.StatusOK || len(nests) != 0 {
		t.Fatalf("nests without Invasion Control = %d %s, want an empty list", code, f.lastRaw)
	}
	code, body = f.do(http.MethodPost, "/hosts/import-nest/anything", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "nest_not_found")
}
