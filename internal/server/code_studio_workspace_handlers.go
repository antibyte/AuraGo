package server

import (
	"crypto"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"os"
	pathpkg "path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"aurago/internal/credentials"
	"aurago/internal/desktop"
	"aurago/internal/inventory"
	"aurago/internal/security"
	"aurago/internal/uid"

	"golang.org/x/crypto/ssh"
)

func registerCodeStudioWorkspaceRoutes(mux *http.ServeMux, h codeStudioHandlers) {
	mux.HandleFunc("/api/code-studio/workspaces", h.handleWorkspaces)
	mux.HandleFunc("/api/code-studio/workspaces/", h.handleWorkspaceItem)
	mux.HandleFunc("/api/code-studio/remote/devices", h.handleRemoteDevices)
	mux.HandleFunc("/api/code-studio/remote/browse", h.handleRemoteBrowse)
	mux.HandleFunc("/api/code-studio/remote/trust", h.handleRemoteTrust)
}

func (h codeStudioHandlers) auditCodeStudioRemote(r *http.Request, action, deviceID string, details map[string]interface{}) {
	if h.server == nil || h.server.DesktopService == nil {
		return
	}
	behindProxy := h.server.Cfg != nil && h.server.Cfg.Server.HTTPS.BehindProxy
	_ = h.server.DesktopService.AuditWithRequest(r.Context(), action, deviceID, details, desktop.SourceUser,
		desktop.AuditRequestInfo{ClientIP: ClientIP(r, behindProxy), UserAgent: r.UserAgent()})
}

func codeStudioCleanLabel(raw string) (string, error) {
	label := strings.TrimSpace(raw)
	if len([]rune(label)) > 120 || strings.IndexFunc(label, unicode.IsControl) >= 0 {
		return "", errors.New("invalid label")
	}
	return label, nil
}

func (h codeStudioHandlers) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.requirePermission(w, r, desktopScopeRead) {
			h.listWorkspaces(w, r)
		}
	case http.MethodPost:
		if h.requirePermission(w, r, desktopScopeWrite) {
			h.createWorkspace(w, r)
		}
	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h codeStudioHandlers) handleWorkspaceItem(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/code-studio/workspaces/"), "/")
	id, action, _ := strings.Cut(rest, "/")
	if id == codeStudioLocalWorkspaceID {
		jsonError(w, "the local workspace cannot be changed", http.StatusBadRequest)
		return
	}
	if !codeStudioWorkspaceIDPattern.MatchString(id) {
		writeCodeStudioOpError(w, codeStudioWorkspaceNotFound())
		return
	}
	switch {
	case action == "" && r.Method == http.MethodDelete:
		if h.requirePermission(w, r, desktopScopeWrite) {
			h.deleteWorkspace(w, r, id)
		}
	case action == "connect" && r.Method == http.MethodPost:
		if h.requirePermission(w, r, desktopScopeWrite) {
			h.connectWorkspace(w, r, id)
		}
	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h codeStudioHandlers) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	out := []codeStudioWorkspaceInfo{codeStudioLocalWorkspaceInfo()}
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		// SSH workspace records (host, user, root) are remote-access data: hidden from narrow bearer tokens.
		writeJSON(w, map[string]interface{}{"status": "ok", "workspaces": out})
		return
	}
	store, err := h.workspaceStore(r.Context())
	if err == nil {
		var recs []codeStudioWorkspaceRecord
		if recs, err = store.List(r.Context()); err == nil {
			remote := h.remoteState()
			for _, rec := range recs {
				_, connected := remote.pool.connected(rec.DeviceID)
				out = append(out, codeStudioSSHWorkspaceInfo(rec, h.codeStudioSSHTargetFor(remote, rec.DeviceID), connected))
			}
		}
	}
	if err != nil {
		h.codeStudioLogger().Warn("code studio workspace list incomplete", "error", err)
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "workspaces": out})
}

// openRemoteRoot connects (409 contract), normalises root (~ allowed) and checks it is a directory.
func (h codeStudioHandlers) openRemoteRoot(r *http.Request, deviceID, rawRoot, trust string) (*codeStudioSSHConn, string, error) {
	conn, err := h.remoteState().pool.get(r.Context(), deviceID, strings.TrimSpace(trust))
	if err != nil {
		return nil, "", err
	}
	home := ""
	if strings.HasPrefix(strings.TrimSpace(rawRoot), "~") {
		home, _ = conn.sftp.Getwd()
	}
	root, err := normalizeCodeStudioRemoteRoot(rawRoot, home)
	if err != nil {
		return nil, "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "invalid_root", Message: err.Error()}
	}
	if info, err := conn.sftp.Stat(root); err != nil || !info.IsDir() {
		if err != nil && codeStudioSSHBroken(err) {
			return nil, "", codeStudioSFTPError(err, root)
		}
		return nil, "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "not_a_directory",
			Message: root + " is not a directory on " + conn.target.Host}
	}
	return conn, root, nil
}

func (h codeStudioHandlers) createWorkspace(w http.ResponseWriter, r *http.Request) {
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		writeCodeStudioOpError(w, codeStudioRemoteScopeError())
		return
	}
	var body struct {
		DeviceID     string `json:"device_id"`
		Root         string `json:"root"`
		Label        string `json:"label"`
		TrustHostKey string `json:"trust_host_key"`
	}
	if err := decodeJSONRequest(w, r, &body, h.server); err != nil {
		jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
		return
	}
	deviceID := strings.TrimSpace(body.DeviceID)
	if deviceID == "" {
		jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}
	label, err := codeStudioCleanLabel(body.Label)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	store, err := h.workspaceStore(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	conn, root, err := h.openRemoteRoot(r, deviceID, body.Root, body.TrustHostKey)
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	probe := newCodeStudioSSHBackend(codeStudioWorkspaceRecord{DeviceID: deviceID, Root: root}, h.remoteState().pool, conn.target)
	caps, capErr := probe.probeCapabilities(r.Context())
	if capErr != nil {
		h.codeStudioLogger().Warn("code studio capability probe failed", "device_id", deviceID, "error", capErr)
	}
	rec, err := store.Upsert(r.Context(), deviceID, root, label, caps)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.auditCodeStudioRemote(r, "code_studio_workspace_open", deviceID, map[string]interface{}{"root": root, "workspace_id": rec.ID})
	writeJSON(w, map[string]interface{}{"status": "ok", "workspace": codeStudioSSHWorkspaceInfo(rec, conn.target, true)})
}

func (h codeStudioHandlers) connectWorkspace(w http.ResponseWriter, r *http.Request, id string) {
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		writeCodeStudioOpError(w, codeStudioRemoteScopeError())
		return
	}
	var body struct {
		TrustHostKey string `json:"trust_host_key"`
	}
	if r.ContentLength > 0 {
		if err := decodeJSONRequest(w, r, &body, h.server); err != nil {
			jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
			return
		}
	}
	store, err := h.workspaceStore(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	rec, err := store.Get(r.Context(), id)
	if err != nil {
		writeCodeStudioOpError(w, codeStudioWorkspaceNotFound())
		return
	}
	conn, _, err := h.openRemoteRoot(r, rec.DeviceID, rec.Root, body.TrustHostKey)
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	if caps, err := newCodeStudioSSHBackend(rec, h.remoteState().pool, conn.target).probeCapabilities(r.Context()); err == nil {
		rec.Capabilities = caps
		_ = store.Touch(r.Context(), rec.ID, caps)
	}
	rec.LastUsedAt = time.Now().UTC()
	writeJSON(w, map[string]interface{}{"status": "ok", "workspace": codeStudioSSHWorkspaceInfo(rec, conn.target, true)})
}

func (h codeStudioHandlers) deleteWorkspace(w http.ResponseWriter, r *http.Request, id string) {
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		writeCodeStudioOpError(w, codeStudioRemoteScopeError())
		return
	}
	store, err := h.workspaceStore(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	rec, err := store.Get(r.Context(), id)
	if err == nil {
		err = store.Delete(r.Context(), id)
	}
	if err != nil {
		writeCodeStudioOpError(w, codeStudioWorkspaceNotFound())
		return
	}
	if n, err := store.CountDevice(r.Context(), rec.DeviceID); err == nil && n == 0 {
		h.remoteState().pool.release(rec.DeviceID)
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "id": id})
}

func (h codeStudioHandlers) handleRemoteTrust(w http.ResponseWriter, r *http.Request) {
	if !h.requirePermission(w, r, desktopScopeWrite) {
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		writeCodeStudioOpError(w, codeStudioRemoteScopeError())
		return
	}
	var body struct {
		DeviceID    string `json:"device_id"`
		Fingerprint string `json:"fingerprint"`
	}
	if err := decodeJSONRequest(w, r, &body, h.server); err != nil {
		jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
		return
	}
	if strings.TrimSpace(body.DeviceID) == "" || !strings.HasPrefix(strings.TrimSpace(body.Fingerprint), "SHA256:") {
		jsonError(w, "device_id and a SHA256 fingerprint are required", http.StatusBadRequest)
		return
	}
	if _, err := h.remoteState().pool.get(r.Context(), strings.TrimSpace(body.DeviceID), strings.TrimSpace(body.Fingerprint)); err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok"})
}

type codeStudioRemoteDevice struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Host        string     `json:"host"`
	Port        int        `json:"port"`
	Username    string     `json:"username"`
	Description string     `json:"description"`
	Tags        []string   `json:"tags"`
	HasSecret   bool       `json:"has_secret"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

func (h codeStudioHandlers) handleRemoteDevices(w http.ResponseWriter, r *http.Request) {
	scope := desktopScopeRead
	if r.Method == http.MethodPost {
		scope = desktopScopeWrite
	}
	if !h.requirePermission(w, r, scope) {
		return
	}
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		writeCodeStudioOpError(w, codeStudioRemoteScopeError())
		return
	}
	if h.server == nil || h.server.InventoryDB == nil {
		jsonError(w, "inventory database not configured", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.listRemoteDevices(w, r)
	case http.MethodPost:
		h.createRemoteDevice(w, r)
	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h codeStudioHandlers) remoteDeviceSummary(d inventory.DeviceRecord, lastUsed map[string]time.Time) codeStudioRemoteDevice {
	a := desktop.DescribeSSHDevice(h.server.InventoryDB, d)
	out := codeStudioRemoteDevice{ID: d.ID, Name: d.Name, Host: a.Host, Port: a.Port, Username: a.Username, Description: d.Description,
		Tags: append([]string{}, d.Tags...), HasSecret: strings.TrimSpace(d.VaultSecretID) != "" || strings.TrimSpace(d.CredentialID) != ""}
	if t, ok := lastUsed[d.ID]; ok {
		out.LastUsedAt = &t
	}
	return out
}

func (h codeStudioHandlers) listRemoteDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := inventory.ListAllDevices(h.server.InventoryDB)
	if err != nil {
		jsonError(w, "Failed to list devices", http.StatusInternalServerError)
		return
	}
	lastUsed := map[string]time.Time{}
	if store, err := h.workspaceStore(r.Context()); err == nil {
		if recs, err := store.List(r.Context()); err == nil {
			for _, rec := range recs {
				if rec.LastUsedAt.After(lastUsed[rec.DeviceID]) {
					lastUsed[rec.DeviceID] = rec.LastUsedAt
				}
			}
		}
	}
	out := []codeStudioRemoteDevice{}
	for _, d := range devices {
		if p := strings.ToLower(strings.TrimSpace(d.Protocol)); p == "" || p == inventory.ProtocolSSH {
			out = append(out, h.remoteDeviceSummary(d, lastUsed))
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	writeJSON(w, map[string]interface{}{"status": "ok", "devices": out})
}

func codeStudioValidHost(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	for _, c := range host {
		if !(c == '.' || c == '-' || c == ':' || c == '_' || c == '%' || c == '[' || c == ']' ||
			(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

// codeStudioPrepareSSHKey validates a private key; a passphrase-protected key is decrypted
// and stored unencrypted in the (encrypted) vault, because the SSH dial path has no
// passphrase support.
func codeStudioPrepareSSHKey(secret, passphrase string) (string, error) {
	if passphrase == "" {
		if _, err := ssh.ParsePrivateKey([]byte(secret)); err != nil {
			var missing *ssh.PassphraseMissingError
			if errors.As(err, &missing) {
				return "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "passphrase_required", Message: "the private key needs its passphrase"}
			}
			return "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "invalid_private_key", Message: "the private key could not be read"}
		}
		return secret, nil
	}
	raw, err := ssh.ParseRawPrivateKeyWithPassphrase([]byte(secret), []byte(passphrase))
	if err != nil {
		return "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "invalid_private_key", Message: "wrong passphrase or unreadable private key"}
	}
	block, err := ssh.MarshalPrivateKey(crypto.PrivateKey(raw), "")
	if err != nil {
		return "", &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "invalid_private_key", Message: "the private key type is not supported"}
	}
	return string(pem.EncodeToMemory(block)), nil
}

func (h codeStudioHandlers) createRemoteDevice(w http.ResponseWriter, r *http.Request) {
	if h.server.Vault == nil {
		jsonError(w, "vault not configured", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Name       string `json:"name"`
		Host       string `json:"host"`
		Port       int    `json:"port"`
		Username   string `json:"username"`
		Auth       string `json:"auth"`
		Secret     string `json:"secret"`
		Passphrase string `json:"passphrase"`
	}
	if err := decodeJSONRequest(w, r, &body, h.server); err != nil {
		jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
		return
	}
	name, host, user := strings.TrimSpace(body.Name), strings.TrimSpace(body.Host), strings.TrimSpace(body.Username)
	if body.Port == 0 {
		body.Port = 22
	}
	switch {
	case name == "" || len([]rune(name)) > 100 || strings.IndexFunc(name, unicode.IsControl) >= 0:
		jsonError(w, "name is required (max 100 characters)", http.StatusBadRequest)
		return
	case !codeStudioValidHost(host):
		jsonError(w, "host is invalid", http.StatusBadRequest)
		return
	case body.Port < 1 || body.Port > 65535:
		jsonError(w, "port must be 1-65535", http.StatusBadRequest)
		return
	case user == "" || strings.IndexFunc(user, unicode.IsSpace) >= 0:
		jsonError(w, "username is required", http.StatusBadRequest)
		return
	case body.Secret == "":
		jsonError(w, "secret is required", http.StatusBadRequest)
		return
	}
	rec := credentials.Record{Name: name + " (SSH)", Type: "ssh", Host: host, Username: user, Description: "Created by Code Studio"}
	secret := body.Secret
	switch body.Auth {
	case "password":
		rec.PasswordVaultID = "credential_password_" + uid.NewString()
	case "key":
		prepared, err := codeStudioPrepareSSHKey(strings.TrimSpace(body.Secret), body.Passphrase)
		if err != nil {
			writeCodeStudioOpError(w, err)
			return
		}
		secret = prepared
		rec.CertificateVaultID = "credential_certificate_" + uid.NewString()
	default:
		jsonError(w, `auth must be "password" or "key"`, http.StatusBadRequest)
		return
	}
	vaultID := rec.PasswordVaultID + rec.CertificateVaultID
	security.RegisterSensitive(secret)
	if err := h.server.Vault.WriteSecret(vaultID, secret); err != nil {
		jsonError(w, "failed to store the secret in the vault", http.StatusInternalServerError)
		return
	}
	credID, err := credentials.Create(h.server.InventoryDB, rec)
	if err != nil {
		_ = h.server.Vault.DeleteSecret(vaultID)
		jsonError(w, "Failed to create credential", http.StatusInternalServerError)
		return
	}
	id, err := inventory.CreateDevice(h.server.InventoryDB, name, "server", inventory.ProtocolSSH, host, body.Port, "", "", credID,
		"Created by Code Studio", []string{"code-studio"}, "")
	if err != nil {
		_ = credentials.Delete(h.server.InventoryDB, credID)
		_ = h.server.Vault.DeleteSecret(vaultID)
		status := http.StatusInternalServerError
		if strings.Contains(strings.ToUpper(err.Error()), "UNIQUE") {
			status = http.StatusConflict
		}
		jsonError(w, "Failed to create device", status)
		return
	}
	device, _ := inventory.GetDeviceByID(h.server.InventoryDB, id)
	h.auditCodeStudioRemote(r, "code_studio_device_create", id, map[string]interface{}{"host": net.JoinHostPort(host, strconv.Itoa(body.Port))})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "device": h.remoteDeviceSummary(device, nil)})
}

type codeStudioBrowseEntry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Type     string    `json:"type"` // directory | file
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	IsGit    bool      `json:"is_git"`
	IsLink   bool      `json:"is_link"`
	Hidden   bool      `json:"hidden"`
}

const codeStudioBrowseLimit = 2000

func (h codeStudioHandlers) handleRemoteBrowse(w http.ResponseWriter, r *http.Request) {
	if !h.requirePermission(w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !codeStudioRemoteCallerAllowed(h.server, r) {
		writeCodeStudioOpError(w, codeStudioRemoteScopeError())
		return
	}
	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	if deviceID == "" {
		jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}
	conn, err := h.remoteState().pool.get(r.Context(), deviceID, "")
	if err != nil {
		writeCodeStudioOpError(w, err) // same 409 host-key contract
		return
	}
	home, err := conn.sftp.Getwd()
	if err != nil || home == "" {
		home = "/"
	}
	raw := r.URL.Query().Get("path")
	if strings.TrimSpace(raw) == "" {
		raw = home
	}
	dir, err := normalizeCodeStudioRemoteRoot(raw, home)
	if err != nil {
		writeCodeStudioOpError(w, &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "invalid_root", Message: err.Error()})
		return
	}
	infos, err := conn.sftp.ReadDir(dir)
	if err != nil {
		writeCodeStudioOpError(w, codeStudioSFTPError(err, dir))
		return
	}
	entries := make([]codeStudioBrowseEntry, 0, len(infos))
	for _, info := range infos {
		e := codeStudioBrowseEntry{Name: info.Name(), Path: pathpkg.Join(dir, info.Name()), Type: "file", Size: info.Size(),
			Modified: info.ModTime().UTC(), Hidden: strings.HasPrefix(info.Name(), "."), IsLink: info.Mode()&os.ModeSymlink != 0}
		if info.IsDir() {
			e.Type = "directory"
		} else if e.IsLink {
			if target, err := conn.sftp.Stat(e.Path); err == nil && target.IsDir() {
				e.Type = "directory"
			}
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "directory"
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	truncated := len(entries) > codeStudioBrowseLimit
	if truncated {
		entries = entries[:codeStudioBrowseLimit]
	}
	// is_git: bounded parallel Lstat of <dir>/.git for the first 500 directories.
	sem, wg, checked := make(chan struct{}, 8), sync.WaitGroup{}, 0
	for i := range entries {
		if entries[i].Type != "directory" || checked >= 500 {
			continue
		}
		checked++
		wg.Add(1)
		sem <- struct{}{}
		go func(e *codeStudioBrowseEntry) {
			defer func() { <-sem; wg.Done() }()
			if _, err := conn.sftp.Lstat(pathpkg.Join(e.Path, ".git")); err == nil {
				e.IsGit = true
			}
		}(&entries[i])
	}
	wg.Wait()
	parent := ""
	if dir != "/" {
		parent = pathpkg.Dir(dir)
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "device_id": deviceID, "path": dir, "home": home,
		"parent": parent, "entries": entries, "truncated": truncated})
}
