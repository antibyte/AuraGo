package desktop

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/remote"
	"aurago/internal/security"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	filesTestUser     = "files-fixture"
	filesTestPassword = "files-fixture-password"
)

type filesFixture struct {
	db       *sql.DB
	vault    *security.Vault
	logger   *slog.Logger
	listener net.Listener
	hostKey  ssh.PublicKey
	auths    *atomic.Int32
	sshDir   string
}

func (f *filesFixture) port() int { return f.listener.Addr().(*net.TCPAddr).Port }

func (f *filesFixture) knownHostsPath() string { return filepath.Join(f.sshDir, "known_hosts") }

func (f *filesFixture) writeKnownHosts(t *testing.T, key ssh.PublicKey) {
	t.Helper()
	line := knownhosts.Line([]string{f.listener.Addr().String()}, key) + "\n"
	if err := os.WriteFile(f.knownHostsPath(), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
}

func resetFileAccessGlobals(t *testing.T) {
	t.Helper()
	previousInsecure := remote.InsecureHostKey
	remote.InsecureHostKey = false
	previousNow, previousSMB := fileProbeNow, fileSMBPort
	previousOpener := remoteFSOpener
	reset := func() {
		probeCache.mu.Lock()
		probeCache.entries = map[string]fileProbeEntry{}
		probeCache.mu.Unlock()
		fileAuthBackoff.mu.Lock()
		fileAuthBackoff.until = map[string]time.Time{}
		fileAuthBackoff.mu.Unlock()
		hostKeyPending.mu.Lock()
		hostKeyPending.entries = map[string]pendingHostKey{}
		hostKeyPending.mu.Unlock()
	}
	reset()
	t.Cleanup(func() {
		remote.InsecureHostKey = previousInsecure
		fileProbeNow, fileSMBPort = previousNow, previousSMB
		remoteFSOpener = previousOpener
		reset()
	})
}

// newFilesFixture starts a local SSH+SFTP server that counts authentication
// attempts and points HOME at a temp dir (no known_hosts yet).
func newFilesFixture(t *testing.T) *filesFixture {
	t.Helper()
	resetFileAccessGlobals(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	auths := &atomic.Int32{}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, candidate []byte) (*ssh.Permissions, error) {
			auths.Add(1)
			if conn.User() != filesTestUser || string(candidate) != filesTestPassword {
				return nil, errors.New("bad credentials")
			}
			return nil, nil
		},
		PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
			auths.Add(1)
			return nil, errors.New("no keys")
		},
	}
	cfg.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	handlers := sftp.InMemHandler()
	var conns sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			conns.Add(1)
			go func() {
				defer conns.Done()
				serveSFTPHandlerTestConnection(c, cfg, handlers)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
		conns.Wait()
	})

	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("files/pw", filesTestPassword); err != nil {
		t.Fatal(err)
	}
	return &filesFixture{
		db: db, vault: vault, logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		listener: listener, hostKey: signer.PublicKey(), auths: auths, sshDir: sshDir,
	}
}

func (f *filesFixture) addSSHDevice(t *testing.T, id string) {
	t.Helper()
	if err := inventory.AddDevice(f.db, inventory.DeviceRecord{
		ID: id, Name: id, Type: "server", Protocol: inventory.ProtocolSSH,
		IPAddress: "127.0.0.1", Port: f.port(), Username: filesTestUser, VaultSecretID: "files/pw",
	}); err != nil {
		t.Fatal(err)
	}
}

func (f *filesFixture) addRDPDevice(t *testing.T, id string, fa *inventory.FileAccess, vaultID string) {
	t.Helper()
	if err := inventory.AddDevice(f.db, inventory.DeviceRecord{
		ID: id, Name: id, Type: "server", Protocol: inventory.ProtocolRDP,
		IPAddress: "127.0.0.1", Port: 3389, Username: filesTestUser, VaultSecretID: vaultID,
	}); err != nil {
		t.Fatal(err)
	}
	if fa != nil {
		if err := inventory.SetDeviceFileAccess(f.db, id, *fa); err != nil {
			t.Fatal(err)
		}
	}
}

func allowAll(*http.Request) bool { return true }

func doList(t *testing.T, h http.HandlerFunc, id string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/desktop/sftp/list?"+url.Values{"device_id": {id}, "path": {"/"}}.Encode(), nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return m
}

func TestRemoteFilesSFTPListCarriesBackendAndRootLabel(t *testing.T) {
	f := newFilesFixture(t)
	f.writeKnownHosts(t, f.hostKey)
	f.addSSHDevice(t, "ssh1")
	rec := doList(t, HandleSFTPList(f.db, f.vault, f.logger), "ssh1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["backend"] != "sftp" || body["root_label"] != "~" {
		t.Fatalf("body %v", body)
	}
	if _, ok := body["entries"].([]any); !ok {
		t.Fatalf("entries missing: %v", body)
	}
}

func TestRemoteFilesUnknownHostKeyIs409AndNeverAuthenticates(t *testing.T) {
	f := newFilesFixture(t) // no known_hosts file at all
	f.addSSHDevice(t, "ssh1")
	rec := doList(t, HandleSFTPList(f.db, f.vault, f.logger), "ssh1")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["code"] != "unknown_host_key" || body["fingerprint"] != ssh.FingerprintSHA256(f.hostKey) ||
		body["key_type"] != f.hostKey.Type() || body["host"] == "" || body["error"] == "" {
		t.Fatalf("body %v", body)
	}
	if n := f.auths.Load(); n != 0 {
		t.Fatalf("server saw %d authentication attempts before the key was trusted", n)
	}
	if _, err := os.Stat(f.knownHostsPath()); err == nil {
		t.Fatal("list stored a host key without confirmation")
	}
}

func TestRemoteFilesChangedHostKeyIs409(t *testing.T) {
	f := newFilesFixture(t)
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	otherKey, _ := ssh.NewPublicKey(other)
	f.writeKnownHosts(t, otherKey)
	f.addSSHDevice(t, "ssh1")
	rec := doList(t, HandleSFTPList(f.db, f.vault, f.logger), "ssh1")
	if rec.Code != http.StatusConflict || decodeBody(t, rec)["code"] != "host_key_changed" {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if f.auths.Load() != 0 {
		t.Fatal("authenticated against a changed host key")
	}
}

func trustRequest(f *filesFixture, method, queryID, bodyJSON string) *httptest.ResponseRecorder {
	return trustRequestWith(f, method, queryID, bodyJSON, RemoteFilesOptions{})
}

func trustRequestWith(f *filesFixture, method, queryID, bodyJSON string, base RemoteFilesOptions) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/desktop/sftp/hostkey?device_id="+url.QueryEscape(queryID), strings.NewReader(bodyJSON))
	rec := httptest.NewRecorder()
	var audited []string
	opts := base
	opts.Audit = func(_ *http.Request, action, _ string, _ map[string]any) { audited = append(audited, action) }
	HandleSFTPHostKeyTrust(f.db, f.vault, f.logger, opts)(rec, req)
	if rec.Code == http.StatusOK && len(audited) == 1 {
		rec.Header().Set("X-Test-Audit", audited[0])
	}
	return rec
}

func TestRemoteFilesHostKeyTrustStoresKeyThenListWorks(t *testing.T) {
	f := newFilesFixture(t)
	f.addSSHDevice(t, "ssh1")
	list := HandleSFTPList(f.db, f.vault, f.logger)
	if rec := doList(t, list, "ssh1"); rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 first, got %d", rec.Code)
	}
	body, _ := json.Marshal(map[string]string{"device_id": "ssh1", "fingerprint": ssh.FingerprintSHA256(f.hostKey)})
	rec := trustRequest(f, http.MethodPost, "ssh1", string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("trust status %d %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Test-Audit"); got != "desktop_sftp_hostkey_trusted" {
		t.Fatalf("audit action %q", got)
	}
	data, err := os.ReadFile(f.knownHostsPath())
	if err != nil || !strings.Contains(string(data), "ssh-ed25519") {
		t.Fatalf("known_hosts not written: %v %q", err, data)
	}
	if rec := doList(t, list, "ssh1"); rec.Code != http.StatusOK {
		t.Fatalf("list after trust: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRemoteFilesHostKeyTrustRejectsWrongFingerprintAndBadRequests(t *testing.T) {
	f := newFilesFixture(t)
	f.addSSHDevice(t, "ssh1")
	if rec := doList(t, HandleSFTPList(f.db, f.vault, f.logger), "ssh1"); rec.Code != http.StatusConflict {
		t.Fatalf("priming list: %d", rec.Code)
	}
	rec := trustRequest(f, http.MethodPost, "ssh1", `{"device_id":"ssh1","fingerprint":"SHA256:nope"}`)
	if rec.Code != http.StatusConflict || decodeBody(t, rec)["code"] != "host_key_mismatch" {
		t.Fatalf("wrong fingerprint: %d %s", rec.Code, rec.Body.String())
	}
	if _, has := decodeBody(t, rec)["fingerprint"]; has || strings.Contains(rec.Body.String(), ssh.FingerprintSHA256(f.hostKey)) {
		t.Fatalf("mismatch response leaks the presented fingerprint: %s", rec.Body.String())
	}
	if _, err := os.Stat(f.knownHostsPath()); err == nil {
		t.Fatal("a mismatching fingerprint stored a key")
	}
	if rec := trustRequest(f, http.MethodGet, "ssh1", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d", rec.Code)
	}
	if rec := trustRequest(f, http.MethodPost, "other", `{"device_id":"ssh1","fingerprint":"SHA256:x"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("device mismatch: %d", rec.Code)
	}
	if rec := trustRequest(f, http.MethodPost, "ssh1", `{"device_id":"ssh1"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing fingerprint: %d", rec.Code)
	}
	if f.auths.Load() != 0 {
		t.Fatal("the trust endpoint authenticated")
	}
}

func TestRemoteFilesRDPGateAndDisabledMode(t *testing.T) {
	f := newFilesFixture(t)
	f.writeKnownHosts(t, f.hostKey)
	f.addRDPDevice(t, "rdp-off", nil, "files/pw") // no row = off
	f.addSSHDevice(t, "ssh1")

	// No options: fail closed before anything else.
	rec := doList(t, HandleSFTPList(f.db, f.vault, f.logger), "rdp-off")
	if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "file_access_admin_required" {
		t.Fatalf("no options: %d %s", rec.Code, rec.Body.String())
	}
	// Gate passes -> device is off.
	opts := RemoteFilesOptions{AllowStoredRDPSecret: allowAll}
	rec = doList(t, HandleSFTPList(f.db, f.vault, f.logger, opts), "rdp-off")
	if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "file_access_disabled" {
		t.Fatalf("off: %d %s", rec.Code, rec.Body.String())
	}
	// A gate returning false is refused too.
	deny := RemoteFilesOptions{AllowStoredRDPSecret: func(*http.Request) bool { return false }}
	rec = doList(t, HandleSFTPList(f.db, f.vault, f.logger, deny), "rdp-off")
	if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "file_access_admin_required" {
		t.Fatalf("deny: %d %s", rec.Code, rec.Body.String())
	}
	// SSH devices are unaffected by the gate.
	rec = doList(t, HandleSFTPList(f.db, f.vault, f.logger), "ssh1")
	if rec.Code != http.StatusOK {
		t.Fatalf("ssh without options: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRemoteFilesSFTPOverRDPDevice(t *testing.T) {
	f := newFilesFixture(t)
	f.writeKnownHosts(t, f.hostKey)
	f.addRDPDevice(t, "rdp1", &inventory.FileAccess{Mode: "sftp", SFTPPort: f.port()}, "files/pw")
	opts := RemoteFilesOptions{AllowStoredRDPSecret: allowAll}
	rec := doList(t, HandleSFTPList(f.db, f.vault, f.logger, opts), "rdp1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body["backend"] != "sftp" {
		t.Fatalf("body %v", body)
	}
}

func TestRemoteFilesAuthBackoffAfterRejectedCredential(t *testing.T) {
	f := newFilesFixture(t)
	f.writeKnownHosts(t, f.hostKey)
	if err := f.vault.WriteSecret("files/bad", "wrong"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	fileProbeNow = func() time.Time { return now }
	f.addRDPDevice(t, "rdp1", &inventory.FileAccess{Mode: "sftp", SFTPPort: f.port()}, "files/bad")
	h := HandleSFTPList(f.db, f.vault, f.logger, RemoteFilesOptions{AllowStoredRDPSecret: allowAll})

	if rec := doList(t, h, "rdp1"); rec.Code != http.StatusBadGateway {
		t.Fatalf("first attempt: %d %s", rec.Code, rec.Body.String())
	}
	attempts := f.auths.Load()
	rec := doList(t, h, "rdp1")
	if rec.Code != http.StatusTooManyRequests || decodeBody(t, rec)["code"] != "file_access_auth_backoff" || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("second attempt: %d %s %v", rec.Code, rec.Body.String(), rec.Header())
	}
	if f.auths.Load() != attempts {
		t.Fatal("the backoff still contacted the host")
	}

	// After the window, a fixed credential works and leaves no backoff behind.
	now = now.Add(fileBackoffWindow + time.Second)
	if err := f.vault.WriteSecret("files/bad", filesTestPassword); err != nil {
		t.Fatal(err)
	}
	if rec := doList(t, h, "rdp1"); rec.Code != http.StatusOK {
		t.Fatalf("after window: %d %s", rec.Code, rec.Body.String())
	}
	if fileAuthBackoff.remaining("rdp1") != 0 {
		t.Fatal("success left a backoff")
	}
}

func TestRemoteFilesSSHAuthFailureDoesNotBackOff(t *testing.T) {
	f := newFilesFixture(t)
	f.writeKnownHosts(t, f.hostKey)
	if err := inventory.AddDevice(f.db, inventory.DeviceRecord{
		ID: "ssh-bad", Name: "x", Type: "server", Protocol: inventory.ProtocolSSH,
		IPAddress: "127.0.0.1", Port: f.port(), Username: "nobody", VaultSecretID: "files/pw",
	}); err != nil {
		t.Fatal(err)
	}
	h := HandleSFTPList(f.db, f.vault, f.logger)
	for i := 0; i < 2; i++ {
		if rec := doList(t, h, "ssh-bad"); rec.Code != http.StatusBadGateway {
			t.Fatalf("attempt %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
}

func TestRemoteFilesAutoProbe(t *testing.T) {
	resetFileAccessGlobals(t)
	fileProbeTimeout = 300 * time.Millisecond
	t.Cleanup(func() { fileProbeTimeout = 1500 * time.Millisecond })
	listen := func(banner string) net.Listener {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { l.Close() })
		go func() {
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				if banner != "" {
					_, _ = c.Write([]byte(banner))
				}
				time.AfterFunc(2*time.Second, func() { c.Close() })
			}
		}()
		return l
	}
	port := func(l net.Listener) int { return l.Addr().(*net.TCPAddr).Port }
	ctx := t.Context()

	sshL := listen("SSH-2.0-x\r\n")
	settings := inventory.FileAccess{Mode: "auto", SFTPPort: port(sshL), SMBShare: "C$"}
	if got, err := probeFileBackend(ctx, "d1", "127.0.0.1", settings); err != nil || got != "sftp" {
		t.Fatalf("sftp probe: %q %v", got, err)
	}

	// A silent listener on the SFTP port is not SSH; SMB port decides.
	silent := listen("")
	smbL := listen("")
	fileSMBPort = port(smbL)
	settings = inventory.FileAccess{Mode: "auto", SFTPPort: port(silent), SMBShare: "C$"}
	if got, err := probeFileBackend(ctx, "d2", "127.0.0.1", settings); err != nil || got != "smb" {
		t.Fatalf("smb probe: %q %v", got, err)
	}

	// Nothing answers.
	dead, _ := net.Listen("tcp", "127.0.0.1:0")
	deadPort := port(dead)
	dead.Close()
	fileSMBPort = deadPort
	settings = inventory.FileAccess{Mode: "auto", SFTPPort: deadPort, SMBShare: "C$"}
	_, err := probeFileBackend(ctx, "d3", "127.0.0.1", settings)
	if err == nil || err.Code != "file_access_unavailable" {
		t.Fatalf("dead probe: %v", err)
	}

	// Cache: d1 stays sftp even when the listener is gone, until the TTL passes.
	sshL.Close()
	if got, err := probeFileBackend(ctx, "d1", "127.0.0.1", inventory.FileAccess{Mode: "auto", SFTPPort: port(sshL), SMBShare: "C$"}); err != nil || got != "sftp" {
		t.Fatalf("cached: %q %v", got, err)
	}
	base := time.Now()
	fileProbeNow = func() time.Time { return base.Add(fileProbeTTL + time.Second) }
	if _, err := probeFileBackend(ctx, "d1", "127.0.0.1", inventory.FileAccess{Mode: "auto", SFTPPort: port(sshL), SMBShare: "C$"}); err == nil {
		t.Fatal("expected the cache to expire")
	}
}

func TestResolveFileTarget(t *testing.T) {
	f := newFilesFixture(t)
	for id, secret := range map[string]string{"v/dev": "devpw", "v/credpw": "credpw", "v/credkey": "KEYDATA", "v/override": "ovpw"} {
		if err := f.vault.WriteSecret(id, secret); err != nil {
			t.Fatal(err)
		}
	}
	if err := credentials.EnsureSchema(f.db); err != nil {
		t.Fatal(err)
	}
	mkCred := func(rec credentials.Record) string {
		id, err := credentials.Create(f.db, rec)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	credKeyID := mkCred(credentials.Record{Name: "k", Type: "login", Host: "cred.example", Username: "kuser", PasswordVaultID: "v/credpw", CertificateVaultID: "v/credkey"})
	credPwID := mkCred(credentials.Record{Name: "p", Type: "login", Host: "", Username: `LAB\bob`, PasswordVaultID: "v/credpw"})
	credNoPwID := mkCred(credentials.Record{Name: "n", Type: "login", Username: "nopw", CertificateVaultID: "v/credkey"})
	overrideID := mkCred(credentials.Record{Name: "o", Type: "login", Host: "override.example", Username: "ovuser", PasswordVaultID: "v/override"})

	rdp := func(cred, user, vaultID string) inventory.DeviceRecord {
		return inventory.DeviceRecord{ID: "r", Name: "r", Protocol: "rdp", IPAddress: "10.1.1.1", Port: 3389, Username: user, VaultSecretID: vaultID, CredentialID: cred}
	}
	type tc struct {
		name     string
		dev      inventory.DeviceRecord
		fa       inventory.FileAccess
		wantErr  string // substring or code
		backend  string
		host     string
		port     int
		user     string
		domain   string
		secret   string
		share    string
		codeWant string
	}
	cases := []tc{
		{name: "off", dev: rdp("", "u", "v/dev"), fa: inventory.FileAccess{Mode: "off"}, codeWant: "file_access_disabled"},
		{name: "vnc", dev: inventory.DeviceRecord{ID: "v", Protocol: "vnc", IPAddress: "1.1.1.1"}, fa: inventory.FileAccess{Mode: "sftp"}, codeWant: "file_access_unsupported"},
		{name: "none", dev: inventory.DeviceRecord{ID: "v", Protocol: "none", IPAddress: "1.1.1.1"}, fa: inventory.FileAccess{Mode: "sftp"}, codeWant: "file_access_unsupported"},
		{name: "sftp device secret, port from settings", dev: rdp("", "u", "v/dev"), fa: inventory.FileAccess{Mode: "sftp", SFTPPort: 2222}, backend: "sftp", host: "10.1.1.1", port: 2222, user: "u", secret: "devpw"},
		{name: "sftp prefers key over password", dev: rdp(credKeyID, "", ""), fa: inventory.FileAccess{Mode: "sftp", SFTPPort: 22}, backend: "sftp", host: "cred.example", port: 22, user: "kuser", secret: "KEYDATA"},
		{name: "smb splits domain", dev: rdp(credPwID, "", ""), fa: inventory.FileAccess{Mode: "smb", SMBShare: "Data"}, backend: "smb", host: "10.1.1.1", port: fileSMBPort, user: "bob", domain: "LAB", secret: "credpw", share: "Data"},
		{name: "smb keeps user@domain", dev: rdp("", "bob@lab.local", "v/dev"), fa: inventory.FileAccess{Mode: "smb", SMBShare: "C$"}, backend: "smb", host: "10.1.1.1", port: fileSMBPort, user: "bob@lab.local", secret: "devpw", share: "C$"},
		{name: "smb without password", dev: rdp(credNoPwID, "", ""), fa: inventory.FileAccess{Mode: "smb", SMBShare: "C$"}, wantErr: "secret is missing"},
		{name: "override ignores its host", dev: rdp(credKeyID, "", ""), fa: inventory.FileAccess{Mode: "smb", SMBShare: "C$", CredentialID: overrideID}, backend: "smb", host: "cred.example", port: fileSMBPort, user: "ovuser", secret: "ovpw", share: "C$"},
		{name: "override on plain device uses device host", dev: rdp("", "u", "v/dev"), fa: inventory.FileAccess{Mode: "sftp", SFTPPort: 22, CredentialID: overrideID}, backend: "sftp", host: "10.1.1.1", port: 22, user: "ovuser", secret: "ovpw"},
		{name: "unresolved auto", dev: rdp("", "u", "v/dev"), fa: inventory.FileAccess{Mode: "auto"}, wantErr: "not resolved"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveFileTarget(c.dev, f.db, f.vault, c.fa)
			if c.codeWant != "" {
				var fe *fileAccessError
				if !errors.As(err, &fe) || fe.Code != c.codeWant {
					t.Fatalf("want code %s, got %v", c.codeWant, err)
				}
				return
			}
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("want error %q, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Backend != c.backend || got.Host != c.host || got.Port != c.port || got.Username != c.user ||
				got.Domain != c.domain || string(got.Secret) != c.secret || got.Share != c.share {
				t.Fatalf("got %+v (secret %q)", got, got.Secret)
			}
		})
	}
	// SSH device stays on the device port.
	ssh1 := inventory.DeviceRecord{ID: "s", Name: "s", Protocol: "ssh", IPAddress: "10.2.2.2", Port: 2200, Username: "su", VaultSecretID: "v/dev"}
	got, err := resolveFileTarget(ssh1, f.db, f.vault, inventory.FileAccess{})
	if err != nil || got.Backend != "sftp" || got.Port != 2200 || got.Host != "10.2.2.2" {
		t.Fatalf("ssh: %+v %v", got, err)
	}
}

// ---- fake backend handler tests ----

type fakeRemoteFS struct {
	removed      []string
	removedDir   []bool
	created      map[string]*bytes.Buffer
	createClosed map[string]bool
	files        map[string]string
	aborted      int
}

func newFakeRemoteFS() *fakeRemoteFS {
	return &fakeRemoteFS{created: map[string]*bytes.Buffer{}, createClosed: map[string]bool{}, files: map[string]string{}}
}

type fakeWriter struct {
	buf     *bytes.Buffer
	onDone  func()
	onAbort func()
}

func (w *fakeWriter) Abort(error) { w.onAbort() }

func (w *fakeWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *fakeWriter) Close() error                { w.onDone(); return nil }

func (f *fakeRemoteFS) Backend() string   { return "smb" }
func (f *fakeRemoteFS) RootLabel() string { return "C$" }
func (f *fakeRemoteFS) ReadDir(context.Context, string) ([]sftpEntry, error) {
	return []sftpEntry{{Name: "docs", IsDir: true}}, nil
}
func (f *fakeRemoteFS) Stat(_ context.Context, p string) (sftpEntry, error) {
	return sftpEntry{Name: path.Base(p), IsDir: p == "docs"}, nil
}
func (f *fakeRemoteFS) Mkdir(context.Context, string) error { return nil }
func (f *fakeRemoteFS) Remove(_ context.Context, p string, isDir bool) error {
	f.removed = append(f.removed, p)
	delete(f.created, p)
	f.removedDir = append(f.removedDir, isDir)
	return nil
}
func (f *fakeRemoteFS) Replace(_ context.Context, from, to string) error {
	buf, ok := f.created[from]
	if !ok {
		return errors.New("no such temp file")
	}
	f.files[to] = buf.String()
	delete(f.created, from)
	return nil
}
func (f *fakeRemoteFS) Rename(context.Context, string, string) error { return nil }
func (f *fakeRemoteFS) Open(context.Context, string) (io.ReadCloser, int64, error) {
	return io.NopCloser(strings.NewReader("hello")), 5, nil
}
func (f *fakeRemoteFS) Create(_ context.Context, p string) (io.WriteCloser, error) {
	buf := &bytes.Buffer{}
	f.created[p] = buf
	return &fakeWriter{buf: buf, onDone: func() { f.createClosed[p] = true }, onAbort: func() { f.aborted++ }}, nil
}
func (f *fakeRemoteFS) Close() error { return nil }

func newMultipart(t *testing.T, dst *bytes.Buffer, fields map[string]string, fileField, fileName, contents string) string {
	t.Helper()
	w := multipart.NewWriter(dst)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	fw, err := w.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(fw, contents)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w.FormDataContentType()
}

func TestRemoteFilesHandlersOverFakeBackend(t *testing.T) {
	resetFileAccessGlobals(t)
	fake := newFakeRemoteFS()
	remoteFSOpener = func(r *http.Request, id string, deps remoteFilesDeps) (remoteFS, error) { return fake, nil }
	h := func(f func(*sql.DB, *security.Vault, *slog.Logger, ...RemoteFilesOptions) http.HandlerFunc) http.HandlerFunc {
		return f(nil, nil, nil)
	}

	rec := doList(t, h(HandleSFTPList), "dev")
	body := decodeBody(t, rec)
	if rec.Code != 200 || body["backend"] != "smb" || body["root_label"] != "C$" {
		t.Fatalf("list: %d %v", rec.Code, body)
	}

	// Download headers
	req := httptest.NewRequest(http.MethodGet, "/d?device_id=dev&path=/docs/a.txt", nil)
	rec = httptest.NewRecorder()
	h(HandleSFTPDownload)(rec, req)
	if rec.Code != 200 || rec.Header().Get("Content-Length") != "5" || rec.Body.String() != "hello" ||
		!strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment") || !strings.Contains(rec.Header().Get("Content-Disposition"), "a.txt") {
		t.Fatalf("download: %d %v %q", rec.Code, rec.Header(), rec.Body.String())
	}

	// Upload goes through Create and Close
	var mp bytes.Buffer
	mw := newMultipart(t, &mp, map[string]string{"device_id": "dev", "remote_path": "docs/new.txt"}, "file", "new.txt", "payload")
	req = httptest.NewRequest(http.MethodPost, "/u?device_id=dev", &mp)
	req.Header.Set("Content-Type", mw)
	rec = httptest.NewRecorder()
	h(HandleSFTPUpload)(rec, req)
	if rec.Code != 200 || fake.files["docs/new.txt"] != "payload" || len(fake.created) != 0 {
		t.Fatalf("upload: %d %s created=%v", rec.Code, rec.Body.String(), fake.created)
	}

	// Delete: directory vs file
	for _, tc := range []struct {
		path  string
		isDir bool
	}{{"docs", true}, {"docs/a.txt", false}} {
		req = httptest.NewRequest(http.MethodPost, "/x?device_id=dev", strings.NewReader(fmt.Sprintf(`{"device_id":"dev","path":%q}`, tc.path)))
		rec = httptest.NewRecorder()
		h(HandleSFTPDelete)(rec, req)
		if rec.Code != 200 {
			t.Fatalf("delete %s: %d %s", tc.path, rec.Code, rec.Body.String())
		}
	}
	if len(fake.removed) != 2 || !fake.removedDir[0] || fake.removedDir[1] {
		t.Fatalf("removed %v %v", fake.removed, fake.removedDir)
	}

	// Opener errors surface with their code.
	remoteFSOpener = func(*http.Request, string, remoteFilesDeps) (remoteFS, error) {
		return nil, newFileAccessError(http.StatusForbidden, codeFileAccessAdmin, "needs admin")
	}
	rec = doList(t, h(HandleSFTPList), "dev")
	if rec.Code != 403 || decodeBody(t, rec)["code"] != "file_access_admin_required" {
		t.Fatalf("opener error: %d %s", rec.Code, rec.Body.String())
	}
}

type failingReader struct {
	data string
	pos  int
}

func (r *failingReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, errors.New("boom: source failed mid-stream")
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestWriteRemoteFileSafelyFakeFailureLeavesDestinationUntouched(t *testing.T) {
	fake := newFakeRemoteFS()
	fake.files["docs/keep.txt"] = "original"
	err := writeRemoteFileSafely(context.Background(), fake, "docs/keep.txt", &failingReader{data: "partial"})
	if err == nil {
		t.Fatal("expected the copy to fail")
	}
	if fake.files["docs/keep.txt"] != "original" || len(fake.created) != 0 || fake.aborted != 1 {
		t.Fatalf("files=%v created=%v aborted=%d", fake.files, fake.created, fake.aborted)
	}
	if err := writeRemoteFileSafely(context.Background(), fake, "docs/keep.txt", strings.NewReader("new")); err != nil {
		t.Fatal(err)
	}
	if fake.files["docs/keep.txt"] != "new" || len(fake.created) != 0 {
		t.Fatalf("success path: files=%v created=%v", fake.files, fake.created)
	}
}

func TestWriteRemoteFileSafelyOverRealSFTP(t *testing.T) {
	f := newFilesFixture(t)
	f.writeKnownHosts(t, f.hostKey)
	f.addSSHDevice(t, "ssh1")
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	fsys, err := openRemoteFS(req, "ssh1", newRemoteFilesDeps(f.db, f.vault, f.logger, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer fsys.Close()
	ctx := context.Background()
	if err := writeRemoteFileSafely(ctx, fsys, "keep.txt", strings.NewReader("original")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := writeRemoteFileSafely(ctx, fsys, "keep.txt", &failingReader{data: "partial"}); err == nil {
		t.Fatal("expected failure")
	}
	entries, err := fsys.ReadDir(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "keep.txt" || entries[0].Size != int64(len("original")) {
		t.Fatalf("leftovers after failed write: %+v", entries)
	}
	// Overwrite success replaces the contents and leaves no temp file.
	if err := writeRemoteFileSafely(ctx, fsys, "keep.txt", strings.NewReader("replaced!!")); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	entries, _ = fsys.ReadDir(ctx, ".")
	if len(entries) != 1 || entries[0].Size != int64(len("replaced!!")) {
		t.Fatalf("after overwrite: %+v", entries)
	}
}

func TestHostKeyTrustNeedsAPendingKey(t *testing.T) {
	f := newFilesFixture(t)
	f.addSSHDevice(t, "ssh1")
	body, _ := json.Marshal(map[string]string{"device_id": "ssh1", "fingerprint": ssh.FingerprintSHA256(f.hostKey)})
	rec := trustRequest(f, http.MethodPost, "ssh1", string(body))
	if rec.Code != http.StatusConflict || decodeBody(t, rec)["code"] != "host_key_not_pending" {
		t.Fatalf("no pending: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(f.knownHostsPath()); err == nil {
		t.Fatal("stored a key that was never shown")
	}
	// Single use: a successful trust consumes the entry.
	doList(t, HandleSFTPList(f.db, f.vault, f.logger), "ssh1")
	if rec := trustRequest(f, http.MethodPost, "ssh1", string(body)); rec.Code != http.StatusOK {
		t.Fatalf("trust: %d %s", rec.Code, rec.Body.String())
	}
	if hostKeyPending.get("ssh1") != "" {
		t.Fatal("pending entry not consumed")
	}
}

func TestHostKeyTrustForRDPNeedsStrongCaller(t *testing.T) {
	f := newFilesFixture(t)
	f.addRDPDevice(t, "rdp1", &inventory.FileAccess{Mode: "sftp", SFTPPort: f.port()}, "files/pw")
	hostKeyPending.add("rdp1", ssh.FingerprintSHA256(f.hostKey))
	body, _ := json.Marshal(map[string]string{"device_id": "rdp1", "fingerprint": ssh.FingerprintSHA256(f.hostKey)})
	for name, opts := range map[string]RemoteFilesOptions{
		"none": {},
		"deny": {AllowStoredRDPSecret: func(*http.Request) bool { return false }},
	} {
		rec := trustRequestWith(f, http.MethodPost, "rdp1", string(body), opts)
		if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "file_access_admin_required" {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	if _, err := os.Stat(f.knownHostsPath()); err == nil {
		t.Fatal("key stored for a weak caller")
	}
	rec := trustRequestWith(f, http.MethodPost, "rdp1", string(body), RemoteFilesOptions{AllowStoredRDPSecret: allowAll})
	if rec.Code != http.StatusOK {
		t.Fatalf("strong caller: %d %s", rec.Code, rec.Body.String())
	}
}
