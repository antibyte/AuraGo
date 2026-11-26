package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/inventory"
	"aurago/internal/security"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Error codes returned in the JSON error body of the file endpoints.
const (
	codeUnknownHostKey      = "unknown_host_key"
	codeHostKeyChanged      = "host_key_changed"
	codeHostKeyMismatch     = "host_key_mismatch"
	codeHostKeyNotPending   = "host_key_not_pending"
	codeFileAccessAdmin     = "file_access_admin_required"
	codeFileAccessDisabled  = "file_access_disabled"
	codeFileAccessUnsupport = "file_access_unsupported"
	codeFileAccessNoBackend = "file_access_unavailable"
	codeFileAccessBackoff   = "file_access_auth_backoff"
)

// RemoteFilesOptions carries the per-route policy of the file endpoints.
type RemoteFilesOptions struct {
	// AllowStoredRDPSecret reports whether the caller may make the server use
	// a stored RDP credential for file access. nil means never (fail closed).
	AllowStoredRDPSecret func(*http.Request) bool
	// Audit receives audit events; it never gets secrets.
	Audit RDPAuditFunc
}

// remoteFilesDeps bundles the dependencies the handlers hand to the opener.
type remoteFilesDeps struct {
	db     *sql.DB
	vault  *security.Vault
	logger *slog.Logger
	opts   RemoteFilesOptions
}

func newRemoteFilesDeps(db *sql.DB, vault *security.Vault, logger *slog.Logger, opts []RemoteFilesOptions) remoteFilesDeps {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	deps := remoteFilesDeps{db: db, vault: vault, logger: logger}
	if len(opts) > 0 {
		deps.opts = opts[0]
	}
	return deps
}

// remoteFS is the file backend behind the /api/desktop/sftp/* endpoints. Paths
// are already normalised by normalizeSFTPRemotePath ("." is the root).
type remoteFS interface {
	Backend() string   // "sftp" | "smb"
	RootLabel() string // "~" for SFTP, share name for SMB
	ReadDir(ctx context.Context, p string) ([]sftpEntry, error)
	Stat(ctx context.Context, p string) (sftpEntry, error)
	Mkdir(ctx context.Context, p string) error
	Remove(ctx context.Context, p string, isDir bool) error
	Rename(ctx context.Context, from, to string) error
	// Replace renames from over to, replacing an existing file.
	Replace(ctx context.Context, from, to string) error
	Open(ctx context.Context, p string) (io.ReadCloser, int64, error)
	Create(ctx context.Context, p string) (io.WriteCloser, error)
	Close() error
}

// remoteFSOpener is a test seam.
var remoteFSOpener = openRemoteFS

// fileAccessError is an error with an HTTP status and a stable machine code.
type fileAccessError struct {
	Status     int
	Code       string
	Msg        string
	Extra      map[string]string
	RetryAfter int // seconds, 0 = none
}

func (e *fileAccessError) Error() string { return e.Msg }

func newFileAccessError(status int, code, msg string) *fileAccessError {
	return &fileAccessError{Status: status, Code: code, Msg: msg}
}

// writeRemoteFSError writes err as JSON: {error, code?, ...}. Unclassified
// errors keep the historic 502 with {error}.
func writeRemoteFSError(w http.ResponseWriter, err error) {
	var fe *fileAccessError
	if !errors.As(err, &fe) {
		jsonSFTPError(w, err.Error(), http.StatusBadGateway)
		return
	}
	body := map[string]any{"error": fe.Msg, "code": fe.Code}
	for k, v := range fe.Extra {
		body[k] = v
	}
	w.Header().Set("Content-Type", "application/json")
	if fe.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(fe.RetryAfter))
		body["retry_after"] = fe.RetryAfter
	}
	w.WriteHeader(fe.Status)
	_ = json.NewEncoder(w).Encode(body)
}

// hostKeyCapture records host keys seen during a handshake without ever
// trusting them (unless acceptFingerprint matches), so the secret is never
// sent to an unverified host.
type hostKeyCapture struct {
	mu                sync.Mutex
	acceptFingerprint string
	deviceID          string // when set, an unknown key is recorded as pending trust
	seenHost          string
	seenKey           ssh.PublicKey
	accepted          bool
	changed           bool
}

func (c *hostKeyCapture) prompter() sshHostKeyPrompter {
	return func(hostname string, _ net.Addr, key ssh.PublicKey) (bool, error) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.seenHost = knownhosts.Normalize(hostname)
		c.seenKey = key
		if c.acceptFingerprint != "" && ssh.FingerprintSHA256(key) == c.acceptFingerprint {
			c.accepted = true
			return true, nil
		}
		return false, nil
	}
}

// classify turns a failed dial into a coded error when the capture explains it.
func (c *hostKeyCapture) classify(err error) error {
	if c == nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.changed {
		return newFileAccessError(http.StatusConflict, codeHostKeyChanged,
			"the SSH host key of this host has changed; file access is blocked until known_hosts is fixed")
	}
	if c.seenKey != nil && !c.accepted {
		if c.deviceID != "" {
			hostKeyPending.add(c.deviceID, ssh.FingerprintSHA256(c.seenKey))
		}
		fe := newFileAccessError(http.StatusConflict, codeUnknownHostKey, "unknown SSH host key; confirm the fingerprint to trust it")
		fe.Extra = map[string]string{
			"host":        c.seenHost,
			"key_type":    c.seenKey.Type(),
			"fingerprint": ssh.FingerprintSHA256(c.seenKey),
		}
		return fe
	}
	return err
}

func isSSHAuthFailure(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unable to authenticate")
}

// dialSFTPClient dials host:port, authenticates and opens an SFTP client. With
// a non-nil hk the host key is captured instead of prompted for, and host key
// problems are returned as coded errors.
func dialSFTPClient(ctx context.Context, host string, port int, user string, secret []byte, logger *slog.Logger, hk *hostKeyCapture) (*sftp.Client, func(), error) {
	var prompter sshHostKeyPrompter
	if hk != nil {
		prompter = hk.prompter()
	}
	res, err := buildSSHConfig(user, secret, logger, prompter)
	if err != nil {
		return nil, nil, fmt.Errorf("SSH config: %w", err)
	}
	if hk != nil && !res.InsecureHostKey {
		inner := res.Config.HostKeyCallback
		res.Config.HostKeyCallback = func(hostname string, remoteAddr net.Addr, key ssh.PublicKey) error {
			err := inner(hostname, remoteAddr, key)
			var keyErr *knownhosts.KeyError
			if errors.As(err, &keyErr) && len(keyErr.Want) > 0 {
				hk.mu.Lock()
				hk.changed = true
				hk.mu.Unlock()
			}
			return err
		}
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	sshClient, err := dialDesktopSSH(ctx, addr, res.Config)
	if err != nil {
		return nil, nil, hk.classify(fmt.Errorf("SSH dial: %w", err))
	}
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, nil, fmt.Errorf("SFTP client: %w", err)
	}
	cleanup := func() {
		sftpClient.Close()
		sshClient.Close()
	}
	return sftpClient, cleanup, nil
}

// connectSFTP opens an SFTP client for an SSH device. Kept for callers that
// work on a raw client; the HTTP handlers go through remoteFSOpener.
func connectSFTP(deviceID string, inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) (*sftp.Client, func(), error) {
	device, err := inventory.GetDeviceByID(inventoryDB, deviceID)
	if err != nil {
		return nil, nil, fmt.Errorf("device not found: %w", err)
	}
	host, port, username, secret, err := resolveSSHAccess(device, inventoryDB, vault)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve SSH access: %w", err)
	}
	return dialSFTPClient(context.Background(), host, port, username, secret, logger, nil)
}

// openRemoteFS resolves the device's file target from the inventory (never from
// the request), applies the RDP gate and backoff, and connects.
func openRemoteFS(r *http.Request, deviceID string, deps remoteFilesDeps) (remoteFS, error) {
	ctx := r.Context()
	device, err := inventory.GetDeviceByID(deps.db, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}
	protocol := strings.ToLower(strings.TrimSpace(device.Protocol))
	isRDP := protocol == inventory.ProtocolRDP
	if isRDP && (deps.opts.AllowStoredRDPSecret == nil || !deps.opts.AllowStoredRDPSecret(r)) {
		return nil, newFileAccessError(http.StatusForbidden, codeFileAccessAdmin,
			"file access to RDP devices needs a signed-in administrator session")
	}

	var settings inventory.FileAccess
	if isRDP {
		settings, err = inventory.GetDeviceFileAccess(deps.db, deviceID)
		if err != nil {
			return nil, err
		}
		if settings.Mode == inventory.FileAccessOff {
			return nil, newFileAccessError(http.StatusForbidden, codeFileAccessDisabled, "file access is off for this device")
		}
		if remaining := fileAuthBackoff.remaining(deviceID); remaining > 0 {
			return nil, backoffError(remaining)
		}
		if settings.Mode == inventory.FileAccessAuto {
			host, _, _, _, rerr := resolveRDPTarget(device, deps.db)
			if rerr != nil {
				return nil, rerr
			}
			backend, perr := probeFileBackend(ctx, deviceID, host, settings)
			if perr != nil {
				return nil, perr
			}
			settings.Mode = backend
		}
	}

	target, err := resolveFileTarget(device, deps.db, deps.vault, settings)
	if err != nil {
		return nil, err
	}
	defer func() {
		for i := range target.Secret {
			target.Secret[i] = 0
		}
	}()

	switch target.Backend {
	case inventory.FileAccessSMB:
		fsys, err := openSMBFS(ctx, target)
		if err != nil {
			return nil, noteFileDialFailure(deviceID, isRDP, settings, err, smbIsAuthFailure(err))
		}
		fileAuthBackoff.clear(deviceID)
		return fsys, nil
	default:
		hk := &hostKeyCapture{deviceID: deviceID}
		client, cleanup, err := dialSFTPClient(ctx, target.Host, target.Port, target.Username, target.Secret, deps.logger, hk)
		if err != nil {
			return nil, noteFileDialFailure(deviceID, isRDP, settings, err, isSSHAuthFailure(err))
		}
		fileAuthBackoff.clear(deviceID)
		return newSFTPFS(ctx, client, cleanup), nil
	}
}

// noteFileDialFailure records an auth failure (RDP devices only) and drops the
// cached auto-probe result, then returns err (or the backoff error).
func noteFileDialFailure(deviceID string, isRDP bool, settings inventory.FileAccess, err error, authFailure bool) error {
	probeCache.drop(deviceID)
	var fe *fileAccessError
	if errors.As(err, &fe) {
		return err
	}
	if isRDP && authFailure {
		fileAuthBackoff.record(deviceID)
		return fmt.Errorf("the host rejected the stored credential: %w", err)
	}
	return err
}

// ---- SFTP backend ----

type sftpFS struct {
	client  *sftp.Client
	cleanup func()
	once    sync.Once
}

func newSFTPFS(ctx context.Context, client *sftp.Client, cleanup func()) *sftpFS {
	f := &sftpFS{client: client, cleanup: cleanup}
	// An aborted browser request must not leave the connection open.
	context.AfterFunc(ctx, func() { _ = f.Close() })
	return f
}

func (f *sftpFS) Backend() string   { return "sftp" }
func (f *sftpFS) RootLabel() string { return "~" }

func sftpEntryFromInfo(info os.FileInfo) sftpEntry {
	return sftpEntry{
		Name:    info.Name(),
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		ModTime: info.ModTime().Format(time.RFC3339),
		IsDir:   info.IsDir(),
	}
}

func (f *sftpFS) ReadDir(_ context.Context, p string) ([]sftpEntry, error) {
	infos, err := f.client.ReadDir(p)
	if err != nil {
		return nil, err
	}
	out := make([]sftpEntry, 0, len(infos))
	for _, e := range infos {
		out = append(out, sftpEntryFromInfo(e))
	}
	return out, nil
}

func (f *sftpFS) Stat(_ context.Context, p string) (sftpEntry, error) {
	info, err := f.client.Stat(p)
	if err != nil {
		return sftpEntry{}, err
	}
	return sftpEntryFromInfo(info), nil
}

func (f *sftpFS) Mkdir(_ context.Context, p string) error { return f.client.Mkdir(p) }

func (f *sftpFS) Remove(_ context.Context, p string, isDir bool) error {
	if isDir {
		return f.client.RemoveDirectory(p)
	}
	return f.client.Remove(p)
}

func (f *sftpFS) Rename(_ context.Context, from, to string) error { return f.client.Rename(from, to) }

func (f *sftpFS) Replace(_ context.Context, from, to string) error {
	if _, ok := f.client.HasExtension("posix-rename@openssh.com"); ok {
		return f.client.PosixRename(from, to)
	}
	// Plain SFTP rename refuses to overwrite: remove the target first.
	if info, err := f.client.Lstat(to); err == nil && !info.IsDir() {
		if err := f.client.Remove(to); err != nil {
			return err
		}
	}
	return f.client.Rename(from, to)
}

func (f *sftpFS) Open(_ context.Context, p string) (io.ReadCloser, int64, error) {
	file, err := f.client.Open(p)
	if err != nil {
		return nil, 0, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	return file, info.Size(), nil
}

func (f *sftpFS) Create(_ context.Context, p string) (io.WriteCloser, error) {
	return f.client.Create(p)
}

func (f *sftpFS) Close() error {
	f.once.Do(func() {
		if f.cleanup != nil {
			f.cleanup()
		}
	})
	return nil
}
