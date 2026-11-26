package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/remote"
	"aurago/internal/security"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sftpEndpoint resolves host and port of the device's SFTP service from the
// inventory without touching the vault.
func sftpEndpoint(r *http.Request, device inventory.DeviceRecord, deps remoteFilesDeps) (host string, port int, user string, err error) {
	protocol := strings.ToLower(strings.TrimSpace(device.Protocol))
	switch protocol {
	case "", inventory.ProtocolSSH:
		host = strings.TrimSpace(device.IPAddress)
		if host == "" {
			host = strings.TrimSpace(device.Name)
		}
		user = strings.TrimSpace(device.Username)
		port = device.Port
		if port <= 0 {
			port = 22
		}
		if id := strings.TrimSpace(device.CredentialID); id != "" {
			cred, cerr := credentials.GetByID(deps.db, id)
			if cerr != nil {
				return "", 0, "", fmt.Errorf("linked credential %q could not be loaded: %w", id, cerr)
			}
			if h := strings.TrimSpace(cred.Host); h != "" {
				host = h
			}
			if u := strings.TrimSpace(cred.Username); u != "" {
				user = u
			}
		}
		if host == "" {
			return "", 0, "", errors.New("device host is missing")
		}
		return host, port, user, nil
	case inventory.ProtocolRDP:
		settings, serr := inventory.GetDeviceFileAccess(deps.db, device.ID)
		if serr != nil {
			return "", 0, "", serr
		}
		if settings.Mode == inventory.FileAccessOff {
			return "", 0, "", newFileAccessError(http.StatusForbidden, codeFileAccessDisabled, "file access is off for this device")
		}
		rdpHost, _, rdpUser, _, rerr := resolveRDPTarget(device, deps.db)
		if rerr != nil {
			return "", 0, "", rerr
		}
		switch settings.Mode {
		case inventory.FileAccessSFTP:
		case inventory.FileAccessAuto:
			backend, perr := probeFileBackend(r.Context(), device.ID, rdpHost, settings)
			if perr != nil {
				return "", 0, "", perr
			}
			if backend != inventory.FileAccessSFTP {
				return "", 0, "", newFileAccessError(http.StatusConflict, codeFileAccessNoBackend, "this host offers no SFTP service")
			}
		default:
			return "", 0, "", newFileAccessError(http.StatusConflict, codeFileAccessNoBackend, "this device uses SMB file access; there is no SSH host key to trust")
		}
		port = settings.SFTPPort
		if port <= 0 {
			port = inventory.DefaultFileAccessSFTPPort
		}
		return rdpHost, port, rdpUser, nil
	default:
		return "", 0, "", newFileAccessError(http.StatusBadRequest, codeFileAccessUnsupport,
			fmt.Sprintf("file access is not available for %q devices", protocol))
	}
}

// HandleSFTPHostKeyTrust returns the endpoint that stores a host key the user
// confirmed. The body carries the fingerprint shown in the prompt; the server
// dials the inventory host with no credentials and stores the key only if it
// still presents exactly that fingerprint.
func HandleSFTPHostKeyTrust(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, opts ...RemoteFilesOptions) http.HandlerFunc {
	deps := newRemoteFilesDeps(inventoryDB, vault, logger, opts)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			jsonSFTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			DeviceID    string `json:"device_id"`
			Fingerprint string `json:"fingerprint"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			jsonSFTPError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.DeviceID = strings.TrimSpace(req.DeviceID)
		req.Fingerprint = strings.TrimSpace(req.Fingerprint)
		if req.DeviceID == "" || req.Fingerprint == "" {
			jsonSFTPError(w, "missing device_id or fingerprint", http.StatusBadRequest)
			return
		}
		if !validateSFTPDeviceID(w, r, req.DeviceID, true) {
			return
		}
		device, err := inventory.GetDeviceByID(deps.db, req.DeviceID)
		if err != nil {
			jsonSFTPError(w, "device not found", http.StatusNotFound)
			return
		}
		// RDP devices: the same strong-caller rule as file access itself.
		if strings.EqualFold(strings.TrimSpace(device.Protocol), inventory.ProtocolRDP) &&
			(deps.opts.AllowStoredRDPSecret == nil || !deps.opts.AllowStoredRDPSecret(r)) {
			writeRemoteFSError(w, newFileAccessError(http.StatusForbidden, codeFileAccessAdmin,
				"file access to RDP devices needs a signed-in administrator session"))
			return
		}
		host, port, user, err := sftpEndpoint(r, device, deps)
		if err != nil {
			writeRemoteFSError(w, err)
			return
		}
		if remote.InsecureHostKey {
			jsonOK(w, map[string]bool{"ok": true})
			return
		}

		// Only a key a file operation just showed to the user can be trusted.
		pending := hostKeyPending.get(device.ID)
		if pending == "" {
			writeRemoteFSError(w, newFileAccessError(http.StatusConflict, codeHostKeyNotPending,
				"no host key is waiting for confirmation; open the Files tab again"))
			return
		}
		if pending != req.Fingerprint {
			writeRemoteFSError(w, newFileAccessError(http.StatusConflict, codeHostKeyMismatch,
				"the fingerprint does not match the host key that was shown; try again"))
			return
		}

		hk := &hostKeyCapture{acceptFingerprint: req.Fingerprint}
		cfg := &ssh.ClientConfig{
			User:    firstNonEmpty(user, "aurago"),
			Timeout: 15 * time.Second,
		}
		knownHostsFile, err := defaultKnownHostsFile()
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadGateway)
			return
		}
		inner, err := knownHostsCallback(knownHostsFile, hk.prompter(), deps.logger)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadGateway)
			return
		}
		cfg.HostKeyCallback = func(hostname string, remoteAddr net.Addr, key ssh.PublicKey) error {
			err := inner(hostname, remoteAddr, key)
			var keyErr *knownhosts.KeyError
			if errors.As(err, &keyErr) && len(keyErr.Want) > 0 {
				hk.mu.Lock()
				hk.changed = true
				hk.mu.Unlock()
			}
			return err
		}

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		client, dialErr := dialDesktopSSH(ctx, net.JoinHostPort(host, strconv.Itoa(port)), cfg)
		if client != nil {
			client.Close()
		}

		hk.mu.Lock()
		changed, accepted, seen := hk.changed, hk.accepted, hk.seenKey
		hk.mu.Unlock()
		hostKeyPending.consume(device.ID)
		switch {
		case changed:
			writeRemoteFSError(w, newFileAccessError(http.StatusConflict, codeHostKeyChanged,
				"the SSH host key of this host has changed; file access is blocked until known_hosts is fixed"))
			return
		case seen != nil && !accepted:
			writeRemoteFSError(w, newFileAccessError(http.StatusConflict, codeHostKeyMismatch,
				"the host key changed while it was being confirmed; try again"))
			return
		case accepted:
			// The key was stored; the authentication failure that follows is expected.
		case dialErr != nil && !isSSHAuthFailure(dialErr):
			jsonSFTPError(w, fmt.Sprintf("SSH dial: %v", dialErr), http.StatusBadGateway)
			return
		}
		if accepted {
			deps.opts.Audit.emit(r, "desktop_sftp_hostkey_trusted", device.ID, map[string]any{
				"host":        knownhosts.Normalize(net.JoinHostPort(host, strconv.Itoa(port))),
				"fingerprint": req.Fingerprint,
			})
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
