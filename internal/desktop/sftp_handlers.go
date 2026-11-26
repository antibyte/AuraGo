package desktop

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strings"

	"aurago/internal/security"
)

const maxSFTPUploadBytes int64 = 50 << 20

// driveLetterPattern matches a Windows drive prefix such as `C:` in a remote path segment.
var driveLetterPattern = regexp.MustCompile(`^[A-Za-z]:`)

var errSFTPWindowsAbsolutePath = errors.New("absolute Windows or UNC paths are not allowed; use a path relative to the remote home")

func normalizeSFTPRemotePath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", fmt.Errorf("remote path is required")
	}
	p = strings.ReplaceAll(p, "\\", "/")
	// Check UNC on the raw path (path.Clean collapses `//`). A bare `//` (or
	// `\\`) still means the remote home; a host after it does not.
	if strings.HasPrefix(p, "//") && strings.Trim(p, "/") != "" {
		return "", errSFTPWindowsAbsolutePath
	}
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("remote path contains invalid character")
	}
	if strings.HasPrefix(p, "~") {
		return "", fmt.Errorf("remote home shortcuts are not allowed")
	}
	if sftpPathHasParentSegment(p) {
		return "", fmt.Errorf("remote path traversal is not allowed")
	}
	cleaned := path.Clean("/" + strings.TrimLeft(p, "/"))
	// Check the drive on the cleaned path so `./C:/x` cannot slip through.
	if first, _, _ := strings.Cut(strings.TrimPrefix(cleaned, "/"), "/"); driveLetterPattern.MatchString(first) {
		return "", errSFTPWindowsAbsolutePath
	}
	if isSensitiveSFTPAbsolutePath(cleaned) {
		return "", fmt.Errorf("remote path is outside the allowed SFTP workspace")
	}
	if cleaned == "/" {
		return ".", nil
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}

func sftpPathHasParentSegment(raw string) bool {
	for _, part := range strings.Split(raw, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func isSensitiveSFTPAbsolutePath(cleaned string) bool {
	for _, root := range []string{"/etc", "/root", "/proc", "/sys", "/dev", "/run", "/var/run"} {
		if cleaned == root || strings.HasPrefix(cleaned, root+"/") {
			return true
		}
	}
	return false
}

// sftpEntry represents a file/directory entry in the SFTP listing.
type sftpEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

// jsonSFTPError writes a JSON error response.
func jsonSFTPError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func validateSFTPDeviceID(w http.ResponseWriter, r *http.Request, bodyDeviceID string, requireQuery bool) bool {
	guardedDeviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	if guardedDeviceID == "" {
		if !requireQuery {
			return true
		}
		jsonSFTPError(w, "missing device_id", http.StatusBadRequest)
		return false
	}
	if guardedDeviceID != bodyDeviceID {
		jsonSFTPError(w, "device_id mismatch", http.StatusBadRequest)
		return false
	}
	return true
}

// jsonOK writes a JSON success response.
func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// openFS opens the file backend for a request and writes the error response
// itself when that fails.
func openFS(w http.ResponseWriter, r *http.Request, deps remoteFilesDeps, deviceID string) (remoteFS, bool) {
	fsys, err := remoteFSOpener(r, deviceID, deps)
	if err != nil {
		writeRemoteFSError(w, err)
		return nil, false
	}
	return fsys, true
}

// HandleSFTPList returns a handler that lists directory contents.
func HandleSFTPList(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, opts ...RemoteFilesOptions) http.HandlerFunc {
	deps := newRemoteFilesDeps(inventoryDB, vault, logger, opts)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonSFTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
		dirPath := r.URL.Query().Get("path")
		if deviceID == "" {
			jsonSFTPError(w, "missing device_id", http.StatusBadRequest)
			return
		}
		if dirPath == "" {
			dirPath = "/"
		}
		dirPath, err := normalizeSFTPRemotePath(dirPath)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}

		fsys, ok := openFS(w, r, deps, deviceID)
		if !ok {
			return
		}
		defer fsys.Close()

		entries, err := fsys.ReadDir(r.Context(), dirPath)
		if err != nil {
			jsonSFTPError(w, fmt.Sprintf("read directory: %v", err), http.StatusBadGateway)
			return
		}
		if entries == nil {
			entries = []sftpEntry{}
		}
		jsonOK(w, map[string]interface{}{"entries": entries, "backend": fsys.Backend(), "root_label": fsys.RootLabel()})
	}
}

// HandleSFTPStat returns a handler that stats a single file/directory.
func HandleSFTPStat(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, opts ...RemoteFilesOptions) http.HandlerFunc {
	deps := newRemoteFilesDeps(inventoryDB, vault, logger, opts)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonSFTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
		filePath := r.URL.Query().Get("path")
		if deviceID == "" || filePath == "" {
			jsonSFTPError(w, "missing device_id or path", http.StatusBadRequest)
			return
		}
		filePath, err := normalizeSFTPRemotePath(filePath)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}

		fsys, ok := openFS(w, r, deps, deviceID)
		if !ok {
			return
		}
		defer fsys.Close()

		entry, err := fsys.Stat(r.Context(), filePath)
		if err != nil {
			jsonSFTPError(w, fmt.Sprintf("stat: %v", err), http.StatusBadGateway)
			return
		}
		jsonOK(w, entry)
	}
}

// HandleSFTPMkdir returns a handler that creates a directory.
func HandleSFTPMkdir(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, opts ...RemoteFilesOptions) http.HandlerFunc {
	deps := newRemoteFilesDeps(inventoryDB, vault, logger, opts)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonSFTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			DeviceID string `json:"device_id"`
			Path     string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonSFTPError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.DeviceID = strings.TrimSpace(req.DeviceID)
		if req.DeviceID == "" || req.Path == "" {
			jsonSFTPError(w, "missing device_id or path", http.StatusBadRequest)
			return
		}
		if !validateSFTPDeviceID(w, r, req.DeviceID, true) {
			return
		}
		var err error
		req.Path, err = normalizeSFTPRemotePath(req.Path)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}

		fsys, ok := openFS(w, r, deps, req.DeviceID)
		if !ok {
			return
		}
		defer fsys.Close()

		if err := fsys.Mkdir(r.Context(), req.Path); err != nil {
			jsonSFTPError(w, fmt.Sprintf("mkdir: %v", err), http.StatusBadGateway)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

// HandleSFTPDelete returns a handler that deletes a file or directory.
func HandleSFTPDelete(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, opts ...RemoteFilesOptions) http.HandlerFunc {
	deps := newRemoteFilesDeps(inventoryDB, vault, logger, opts)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonSFTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			DeviceID string `json:"device_id"`
			Path     string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonSFTPError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.DeviceID = strings.TrimSpace(req.DeviceID)
		if req.DeviceID == "" || req.Path == "" {
			jsonSFTPError(w, "missing device_id or path", http.StatusBadRequest)
			return
		}
		if !validateSFTPDeviceID(w, r, req.DeviceID, true) {
			return
		}
		var err error
		req.Path, err = normalizeSFTPRemotePath(req.Path)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}

		fsys, ok := openFS(w, r, deps, req.DeviceID)
		if !ok {
			return
		}
		defer fsys.Close()

		info, err := fsys.Stat(r.Context(), req.Path)
		if err != nil {
			jsonSFTPError(w, fmt.Sprintf("stat: %v", err), http.StatusBadGateway)
			return
		}
		if err := fsys.Remove(r.Context(), req.Path, info.IsDir); err != nil {
			label := "remove"
			if info.IsDir {
				label = "remove directory"
			}
			jsonSFTPError(w, fmt.Sprintf("%s: %v", label, err), http.StatusBadGateway)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

// HandleSFTPRename returns a handler that renames a file or directory.
func HandleSFTPRename(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, opts ...RemoteFilesOptions) http.HandlerFunc {
	deps := newRemoteFilesDeps(inventoryDB, vault, logger, opts)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonSFTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			DeviceID string `json:"device_id"`
			OldPath  string `json:"old_path"`
			NewPath  string `json:"new_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonSFTPError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.DeviceID = strings.TrimSpace(req.DeviceID)
		if req.DeviceID == "" || req.OldPath == "" || req.NewPath == "" {
			jsonSFTPError(w, "missing device_id, old_path, or new_path", http.StatusBadRequest)
			return
		}
		if !validateSFTPDeviceID(w, r, req.DeviceID, true) {
			return
		}
		var err error
		req.OldPath, err = normalizeSFTPRemotePath(req.OldPath)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}
		req.NewPath, err = normalizeSFTPRemotePath(req.NewPath)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}

		fsys, ok := openFS(w, r, deps, req.DeviceID)
		if !ok {
			return
		}
		defer fsys.Close()

		if err := fsys.Rename(r.Context(), req.OldPath, req.NewPath); err != nil {
			jsonSFTPError(w, fmt.Sprintf("rename: %v", err), http.StatusBadGateway)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

// copyRemoteFile streams src to dst on one backend and reports the first error
// including the destination's close error (SMB reports write failures there).
func copyRemoteFile(r *http.Request, fsys remoteFS, src, dst string) error {
	in, _, err := fsys.Open(r.Context(), src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()
	if err := writeRemoteFileSafely(r.Context(), fsys, dst, in); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

// HandleSFTPCopy returns a handler that copies a file (read + write through the server).
func HandleSFTPCopy(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, opts ...RemoteFilesOptions) http.HandlerFunc {
	deps := newRemoteFilesDeps(inventoryDB, vault, logger, opts)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonSFTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			DeviceID string `json:"device_id"`
			SrcPath  string `json:"src_path"`
			DstPath  string `json:"dst_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonSFTPError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.DeviceID = strings.TrimSpace(req.DeviceID)
		if req.DeviceID == "" || req.SrcPath == "" || req.DstPath == "" {
			jsonSFTPError(w, "missing device_id, src_path, or dst_path", http.StatusBadRequest)
			return
		}
		if !validateSFTPDeviceID(w, r, req.DeviceID, true) {
			return
		}
		var err error
		req.SrcPath, err = normalizeSFTPRemotePath(req.SrcPath)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}
		req.DstPath, err = normalizeSFTPRemotePath(req.DstPath)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}

		fsys, ok := openFS(w, r, deps, req.DeviceID)
		if !ok {
			return
		}
		defer fsys.Close()

		if err := copyRemoteFile(r, fsys, req.SrcPath, req.DstPath); err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadGateway)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

// HandleSFTPMove returns a handler that moves a file (a rename on the remote host).
func HandleSFTPMove(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, opts ...RemoteFilesOptions) http.HandlerFunc {
	deps := newRemoteFilesDeps(inventoryDB, vault, logger, opts)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonSFTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			DeviceID string `json:"device_id"`
			SrcPath  string `json:"src_path"`
			DstPath  string `json:"dst_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonSFTPError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.DeviceID = strings.TrimSpace(req.DeviceID)
		if req.DeviceID == "" || req.SrcPath == "" || req.DstPath == "" {
			jsonSFTPError(w, "missing device_id, src_path, or dst_path", http.StatusBadRequest)
			return
		}
		if !validateSFTPDeviceID(w, r, req.DeviceID, true) {
			return
		}
		var err error
		req.SrcPath, err = normalizeSFTPRemotePath(req.SrcPath)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}
		req.DstPath, err = normalizeSFTPRemotePath(req.DstPath)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}

		fsys, ok := openFS(w, r, deps, req.DeviceID)
		if !ok {
			return
		}
		defer fsys.Close()

		if err := fsys.Rename(r.Context(), req.SrcPath, req.DstPath); err != nil {
			jsonSFTPError(w, fmt.Sprintf("move: %v", err), http.StatusBadGateway)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

// HandleSFTPUpload returns a handler that uploads a file to the remote host.
func HandleSFTPUpload(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, opts ...RemoteFilesOptions) http.HandlerFunc {
	deps := newRemoteFilesDeps(inventoryDB, vault, logger, opts)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonSFTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxSFTPUploadBytes)
		if err := r.ParseMultipartForm(maxSFTPUploadBytes); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				jsonSFTPError(w, "upload exceeds the 50 MiB limit", http.StatusRequestEntityTooLarge)
				return
			}
			jsonSFTPError(w, "invalid multipart form", http.StatusBadRequest)
			return
		}
		deviceID := ""
		if r.MultipartForm != nil {
			values := r.MultipartForm.Value["device_id"]
			if len(values) == 1 {
				deviceID = strings.TrimSpace(values[0])
			}
		}
		if !validateSFTPDeviceID(w, r, deviceID, false) {
			return
		}
		remotePath := r.FormValue("remote_path")
		if deviceID == "" || remotePath == "" {
			jsonSFTPError(w, "missing device_id or remote_path", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			jsonSFTPError(w, "missing file field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		if strings.HasSuffix(remotePath, "/") || strings.HasSuffix(remotePath, "\\") {
			remotePath = remotePath + path.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
		}
		remotePath, err = normalizeSFTPRemotePath(remotePath)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}

		fsys, ok := openFS(w, r, deps, deviceID)
		if !ok {
			return
		}
		defer fsys.Close()

		if err := writeRemoteFileSafely(r.Context(), fsys, remotePath, file); err != nil {
			jsonSFTPError(w, fmt.Sprintf("upload: %v", err), http.StatusBadGateway)
			return
		}

		jsonOK(w, map[string]interface{}{"ok": true, "path": remotePath})
	}
}

// HandleSFTPDownload returns a handler that downloads a file from the remote host.
func HandleSFTPDownload(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, opts ...RemoteFilesOptions) http.HandlerFunc {
	deps := newRemoteFilesDeps(inventoryDB, vault, logger, opts)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonSFTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
		filePath := r.URL.Query().Get("path")
		if deviceID == "" || filePath == "" {
			jsonSFTPError(w, "missing device_id or path", http.StatusBadRequest)
			return
		}
		filePath, err := normalizeSFTPRemotePath(filePath)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}

		fsys, ok := openFS(w, r, deps, deviceID)
		if !ok {
			return
		}
		defer fsys.Close()

		src, size, err := fsys.Open(r.Context(), filePath)
		if err != nil {
			jsonSFTPError(w, fmt.Sprintf("open remote file: %v", err), http.StatusBadGateway)
			return
		}
		defer src.Close()

		disposition := mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(filePath)})
		if disposition == "" {
			disposition = "attachment"
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", disposition)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))

		if _, err := io.Copy(w, src); err != nil {
			deps.logger.Warn("SFTP download stream error", "error", err)
		}
	}
}
