package server

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"

	"aurago/internal/desktop"
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

func (h codeStudioHandlers) handleRemoteDevices(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", http.StatusNotImplemented)
}

func (h codeStudioHandlers) handleRemoteBrowse(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", http.StatusNotImplemented)
}
