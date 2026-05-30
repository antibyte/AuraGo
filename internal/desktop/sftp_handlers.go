package desktop

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"aurago/internal/inventory"
	"aurago/internal/security"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const maxSFTPUploadBytes int64 = 50 << 20

// connectSFTP opens an SSH connection and returns an SFTP client with a cleanup function.
func connectSFTP(deviceID string, inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) (*sftp.Client, func(), error) {
	device, err := inventory.GetDeviceByID(inventoryDB, deviceID)
	if err != nil {
		return nil, nil, fmt.Errorf("device not found: %w", err)
	}

	host, port, username, secret, err := resolveSSHAccess(device, inventoryDB, vault)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve SSH access: %w", err)
	}

	configResult, err := buildSSHConfig(username, secret, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("SSH config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	sshClient, err := ssh.Dial("tcp", addr, configResult.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("SSH dial: %w", err)
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

func normalizeSFTPRemotePath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", fmt.Errorf("remote path is required")
	}
	p = strings.ReplaceAll(p, "\\", "/")
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

// jsonOK writes a JSON success response.
func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// HandleSFTPList returns a handler that lists directory contents via SFTP.
func HandleSFTPList(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) http.HandlerFunc {
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

		client, cleanup, err := connectSFTP(deviceID, inventoryDB, vault, logger)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer cleanup()

		entries, err := client.ReadDir(dirPath)
		if err != nil {
			jsonSFTPError(w, fmt.Sprintf("read directory: %v", err), http.StatusBadGateway)
			return
		}

		result := make([]sftpEntry, 0, len(entries))
		for _, e := range entries {
			result = append(result, sftpEntry{
				Name:    e.Name(),
				Size:    e.Size(),
				Mode:    e.Mode().String(),
				ModTime: e.ModTime().Format(time.RFC3339),
				IsDir:   e.IsDir(),
			})
		}

		jsonOK(w, map[string]interface{}{"entries": result})
	}
}

// HandleSFTPStat returns a handler that stats a single file/directory via SFTP.
func HandleSFTPStat(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) http.HandlerFunc {
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

		client, cleanup, err := connectSFTP(deviceID, inventoryDB, vault, logger)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer cleanup()

		info, err := client.Stat(filePath)
		if err != nil {
			jsonSFTPError(w, fmt.Sprintf("stat: %v", err), http.StatusBadGateway)
			return
		}

		jsonOK(w, sftpEntry{
			Name:    info.Name(),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().Format(time.RFC3339),
			IsDir:   info.IsDir(),
		})
	}
}

// HandleSFTPMkdir returns a handler that creates a directory via SFTP.
func HandleSFTPMkdir(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) http.HandlerFunc {
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
		var err error
		req.Path, err = normalizeSFTPRemotePath(req.Path)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}

		client, cleanup, err := connectSFTP(req.DeviceID, inventoryDB, vault, logger)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer cleanup()

		if err := client.Mkdir(req.Path); err != nil {
			jsonSFTPError(w, fmt.Sprintf("mkdir: %v", err), http.StatusBadGateway)
			return
		}

		jsonOK(w, map[string]bool{"ok": true})
	}
}

// HandleSFTPDelete returns a handler that deletes a file or directory via SFTP.
func HandleSFTPDelete(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) http.HandlerFunc {
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
		var err error
		req.Path, err = normalizeSFTPRemotePath(req.Path)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadRequest)
			return
		}

		client, cleanup, err := connectSFTP(req.DeviceID, inventoryDB, vault, logger)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer cleanup()

		info, err := client.Stat(req.Path)
		if err != nil {
			jsonSFTPError(w, fmt.Sprintf("stat: %v", err), http.StatusBadGateway)
			return
		}

		if info.IsDir() {
			if err := client.RemoveDirectory(req.Path); err != nil {
				jsonSFTPError(w, fmt.Sprintf("remove directory: %v", err), http.StatusBadGateway)
				return
			}
		} else {
			if err := client.Remove(req.Path); err != nil {
				jsonSFTPError(w, fmt.Sprintf("remove: %v", err), http.StatusBadGateway)
				return
			}
		}

		jsonOK(w, map[string]bool{"ok": true})
	}
}

// HandleSFTPRename returns a handler that renames/moves a file or directory via SFTP.
func HandleSFTPRename(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) http.HandlerFunc {
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

		client, cleanup, err := connectSFTP(req.DeviceID, inventoryDB, vault, logger)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer cleanup()

		if err := client.Rename(req.OldPath, req.NewPath); err != nil {
			jsonSFTPError(w, fmt.Sprintf("rename: %v", err), http.StatusBadGateway)
			return
		}

		jsonOK(w, map[string]bool{"ok": true})
	}
}

// HandleSFTPCopy returns a handler that copies a file via SFTP (read + write through the tunnel).
func HandleSFTPCopy(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) http.HandlerFunc {
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

		client, cleanup, err := connectSFTP(req.DeviceID, inventoryDB, vault, logger)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer cleanup()

		srcFile, err := client.Open(req.SrcPath)
		if err != nil {
			jsonSFTPError(w, fmt.Sprintf("open source: %v", err), http.StatusBadGateway)
			return
		}
		defer srcFile.Close()

		dstFile, err := client.Create(req.DstPath)
		if err != nil {
			jsonSFTPError(w, fmt.Sprintf("create destination: %v", err), http.StatusBadGateway)
			return
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			jsonSFTPError(w, fmt.Sprintf("copy: %v", err), http.StatusBadGateway)
			return
		}

		jsonOK(w, map[string]bool{"ok": true})
	}
}

// HandleSFTPMove returns a handler that moves a file via SFTP (copy + delete).
func HandleSFTPMove(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) http.HandlerFunc {
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

		client, cleanup, err := connectSFTP(req.DeviceID, inventoryDB, vault, logger)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer cleanup()

		if err := client.Rename(req.SrcPath, req.DstPath); err != nil {
			jsonSFTPError(w, fmt.Sprintf("move: %v", err), http.StatusBadGateway)
			return
		}

		jsonOK(w, map[string]bool{"ok": true})
	}
}

// HandleSFTPUpload returns a handler that uploads a file to the remote host via SFTP.
func HandleSFTPUpload(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonSFTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxSFTPUploadBytes)
		if err := r.ParseMultipartForm(maxSFTPUploadBytes); err != nil {
			jsonSFTPError(w, "invalid multipart form", http.StatusBadRequest)
			return
		}
		deviceID := strings.TrimSpace(r.FormValue("device_id"))
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

		client, cleanup, err := connectSFTP(deviceID, inventoryDB, vault, logger)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer cleanup()

		dstFile, err := client.Create(remotePath)
		if err != nil {
			jsonSFTPError(w, fmt.Sprintf("create remote file: %v", err), http.StatusBadGateway)
			return
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, file); err != nil {
			jsonSFTPError(w, fmt.Sprintf("upload: %v", err), http.StatusBadGateway)
			return
		}

		jsonOK(w, map[string]interface{}{"ok": true, "path": remotePath})
	}
}

// HandleSFTPDownload returns a handler that downloads a file from the remote host via SFTP.
func HandleSFTPDownload(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) http.HandlerFunc {
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

		client, cleanup, err := connectSFTP(deviceID, inventoryDB, vault, logger)
		if err != nil {
			jsonSFTPError(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer cleanup()

		srcFile, err := client.Open(filePath)
		if err != nil {
			jsonSFTPError(w, fmt.Sprintf("open remote file: %v", err), http.StatusBadGateway)
			return
		}
		defer srcFile.Close()

		stat, err := srcFile.Stat()
		if err != nil {
			jsonSFTPError(w, fmt.Sprintf("stat remote file: %v", err), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, path.Base(filePath)))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))

		if _, err := io.Copy(w, srcFile); err != nil {
			logger.Warn("SFTP download stream error", "error", err)
		}
	}
}
