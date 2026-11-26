package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/security"
)

// fileTarget is where and how the file backend connects. It is built from the
// inventory only; nothing in it comes from the HTTP request.
type fileTarget struct {
	Backend  string // inventory.FileAccessSFTP | inventory.FileAccessSMB
	Host     string
	Port     int
	Username string
	Domain   string
	Secret   []byte
	Share    string
}

// resolveFileTarget builds the target for a device. SSH devices always use SFTP
// on the device port. RDP devices follow settings (mode "auto" must already be
// resolved to sftp or smb by the caller).
func resolveFileTarget(device inventory.DeviceRecord, db *sql.DB, vault *security.Vault, settings inventory.FileAccess) (fileTarget, error) {
	protocol := strings.ToLower(strings.TrimSpace(device.Protocol))
	switch protocol {
	case "", inventory.ProtocolSSH:
		host, port, user, secret, err := resolveSSHAccess(device, db, vault)
		if err != nil {
			return fileTarget{}, fmt.Errorf("resolve SSH access: %w", err)
		}
		return fileTarget{Backend: inventory.FileAccessSFTP, Host: host, Port: port, Username: user, Secret: secret}, nil
	case inventory.ProtocolRDP:
		return resolveRDPFileTarget(device, db, vault, settings)
	default:
		return fileTarget{}, newFileAccessError(http.StatusBadRequest, codeFileAccessUnsupport,
			fmt.Sprintf("file access is not available for %q devices", protocol))
	}
}

func resolveRDPFileTarget(device inventory.DeviceRecord, db *sql.DB, vault *security.Vault, settings inventory.FileAccess) (fileTarget, error) {
	switch settings.Mode {
	case inventory.FileAccessSFTP, inventory.FileAccessSMB:
	case "", inventory.FileAccessOff:
		return fileTarget{}, newFileAccessError(http.StatusForbidden, codeFileAccessDisabled, "file access is off for this device")
	default:
		return fileTarget{}, fmt.Errorf("file access mode %q is not resolved", settings.Mode)
	}
	host, _, username, cred, err := resolveRDPTarget(device, db)
	if err != nil {
		return fileTarget{}, err
	}
	// An override credential supplies user and secret only; its Host is ignored
	// so file access always targets the RDP device's own host.
	if id := strings.TrimSpace(settings.CredentialID); id != "" {
		oc, err := credentials.GetByID(db, id)
		if err != nil {
			return fileTarget{}, fmt.Errorf("file access credential could not be loaded")
		}
		cred = &oc
		if u := strings.TrimSpace(oc.Username); u != "" {
			username = u
		}
	}

	secretID := ""
	switch {
	case settings.Mode == inventory.FileAccessSMB && cred != nil:
		secretID = strings.TrimSpace(cred.PasswordVaultID)
	case settings.Mode == inventory.FileAccessSMB:
		secretID = strings.TrimSpace(device.VaultSecretID)
	case cred != nil && strings.TrimSpace(cred.CertificateVaultID) != "":
		secretID = strings.TrimSpace(cred.CertificateVaultID)
	case cred != nil:
		secretID = strings.TrimSpace(cred.PasswordVaultID)
	default:
		secretID = strings.TrimSpace(device.VaultSecretID)
	}
	if username == "" {
		return fileTarget{}, fmt.Errorf("file access username is missing")
	}
	if secretID == "" {
		return fileTarget{}, fmt.Errorf("file access secret is missing: store a password for this device or choose a file access credential")
	}
	if vault == nil {
		return fileTarget{}, fmt.Errorf("vault is not available")
	}
	secret, err := vault.ReadSecret(secretID)
	if err != nil {
		return fileTarget{}, fmt.Errorf("read vault secret %q: %w", secretID, err)
	}

	t := fileTarget{Backend: settings.Mode, Host: host, Username: username, Secret: []byte(secret)}
	if settings.Mode == inventory.FileAccessSMB {
		t.Domain, t.Username = splitRDPUsername(username)
		t.Share = strings.TrimSpace(settings.SMBShare)
		if t.Share == "" {
			t.Share = inventory.DefaultFileAccessSMBShare
		}
		t.Port = fileSMBPort
		return t, nil
	}
	t.Port = settings.SFTPPort
	if t.Port <= 0 {
		t.Port = inventory.DefaultFileAccessSFTPPort
	}
	return t, nil
}

// ---- auto probe ----

var (
	// fileSMBPort is the SMB TCP port (a variable so tests can use a local listener).
	fileSMBPort       = 445
	fileProbeTimeout  = 1500 * time.Millisecond
	fileProbeTTL      = 5 * time.Minute
	fileProbeNow      = time.Now
	probeCache        = &fileProbeCache{entries: map[string]fileProbeEntry{}}
	fileAuthBackoff   = &authBackoffStore{until: map[string]time.Time{}}
	fileBackoffWindow = 30 * time.Second
)

type fileProbeEntry struct {
	key     string
	backend string
	expires time.Time
}

type fileProbeCache struct {
	mu      sync.Mutex
	entries map[string]fileProbeEntry // by device id
}

func (c *fileProbeCache) get(deviceID, key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[deviceID]
	if !ok || e.key != key || !fileProbeNow().Before(e.expires) {
		delete(c.entries, deviceID)
		return "", false
	}
	return e.backend, true
}

func (c *fileProbeCache) put(deviceID, key, backend string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[deviceID] = fileProbeEntry{key: key, backend: backend, expires: fileProbeNow().Add(fileProbeTTL)}
}

func (c *fileProbeCache) drop(deviceID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, deviceID)
}

// probeFileBackend decides between SFTP and SMB for mode "auto": SFTP when the
// SFTP port answers with an SSH banner, otherwise SMB when its port accepts a
// connection. The result is cached for fileProbeTTL.
func probeFileBackend(ctx context.Context, deviceID, host string, settings inventory.FileAccess) (string, *fileAccessError) {
	key := fmt.Sprintf("%s|%d|%s|%s", host, settings.SFTPPort, settings.SMBShare, settings.CredentialID)
	if backend, ok := probeCache.get(deviceID, key); ok {
		return backend, nil
	}
	sftpPort := settings.SFTPPort
	if sftpPort <= 0 {
		sftpPort = inventory.DefaultFileAccessSFTPPort
	}
	backend := ""
	if probeSSHBanner(ctx, net.JoinHostPort(host, strconv.Itoa(sftpPort))) {
		backend = inventory.FileAccessSFTP
	} else if probeTCP(ctx, net.JoinHostPort(host, strconv.Itoa(fileSMBPort))) {
		backend = inventory.FileAccessSMB
	}
	if backend == "" {
		return "", newFileAccessError(http.StatusBadGateway, codeFileAccessNoBackend, "no SFTP or SMB service answered on this host")
	}
	probeCache.put(deviceID, key, backend)
	return backend, nil
}

func probeTCP(ctx context.Context, addr string) bool {
	d := net.Dialer{Timeout: fileProbeTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func probeSSHBanner(ctx context.Context, addr string) bool {
	d := net.Dialer{Timeout: fileProbeTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(fileProbeTimeout))
	buf := make([]byte, 4)
	n := 0
	for n < len(buf) {
		m, err := conn.Read(buf[n:])
		n += m
		if err != nil {
			break
		}
	}
	return n == 4 && string(buf) == "SSH-"
}

// ---- auth backoff ----

type authBackoffStore struct {
	mu    sync.Mutex
	until map[string]time.Time
}

func (s *authBackoffStore) record(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.until[deviceID] = fileProbeNow().Add(fileBackoffWindow)
}

func (s *authBackoffStore) clear(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.until, deviceID)
}

// remaining returns the time left of an active backoff (0 when none).
func (s *authBackoffStore) remaining(deviceID string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	until, ok := s.until[deviceID]
	if !ok {
		return 0
	}
	left := until.Sub(fileProbeNow())
	if left <= 0 {
		delete(s.until, deviceID)
		return 0
	}
	return left
}

func backoffError(remaining time.Duration) *fileAccessError {
	secs := int((remaining + time.Second - 1) / time.Second)
	if secs < 1 {
		secs = 1
	}
	fe := newFileAccessError(http.StatusTooManyRequests, codeFileAccessBackoff,
		fmt.Sprintf("the host rejected the stored credential; try again in %d s", secs))
	fe.RetryAfter = secs
	return fe
}
