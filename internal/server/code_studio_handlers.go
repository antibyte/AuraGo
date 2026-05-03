package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	pathpkg "path"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

const (
	codeStudioWorkspaceRoot = "/workspace"
	codeStudioMaxExecTime   = 300 * time.Second
	codeStudioMaxUploadSize = int64(50 * 1024 * 1024)
)

type codeStudioDockerAPI interface {
	Exec(ctx context.Context, containerID string, cmd []string, timeout time.Duration) (codeStudioExecResult, error)
	CreateTerminalExec(ctx context.Context, containerID string, cols, rows int) (string, error)
	StartExec(ctx context.Context, execID string) ([]byte, error)
	ResizeExec(ctx context.Context, execID string, cols, rows int) error
}

type codeStudioExecResult struct {
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
}

type codeStudioFileEntry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Type     string    `json:"type"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

type codeStudioHandlers struct {
	server *Server
	docker codeStudioDockerAPI
}

func newCodeStudioHandlers(s *Server) codeStudioHandlers {
	return codeStudioHandlers{
		server: s,
		docker: newCodeStudioDockerAdapter(desktop.ConfigFromAuraConfig(s.ConfigSnapshot()), s.Logger),
	}
}

func registerCodeStudioRoutes(mux *http.ServeMux, s *Server) {
	handlers := newCodeStudioHandlers(s)
	mux.HandleFunc("/api/code-studio/status", handlers.handleStatus)
	mux.HandleFunc("/api/code-studio/files", handlers.handleFiles)
	mux.HandleFunc("/api/code-studio/file", handlers.handleFile)
	mux.HandleFunc("/api/code-studio/directory", handlers.handleDirectory)
	mux.HandleFunc("/api/code-studio/upload", handlers.handleUpload)
	mux.HandleFunc("/api/code-studio/download", handlers.handleDownload)
	mux.HandleFunc("/api/code-studio/exec", handlers.handleExec)
	mux.HandleFunc("/api/code-studio/terminal", handlers.handleTerminal)
}

func (h codeStudioHandlers) codeContainer(ctx context.Context, start bool) (*desktop.CodeContainerService, string, error) {
	svc, _, err := h.server.getDesktopService(ctx)
	if err != nil {
		return nil, "", err
	}
	container := svc.CodeContainer()
	if container == nil {
		return nil, "", fmt.Errorf("code studio container service is not available")
	}
	if start {
		if err := container.EnsureStarted(ctx); err != nil {
			return nil, "", err
		}
	}
	containerID, err := container.ContainerID(ctx)
	if err != nil {
		return nil, "", err
	}
	return container, containerID, nil
}

func (h codeStudioHandlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	svc, _, err := h.server.getDesktopService(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	status := svc.CodeContainer().Status(r.Context())
	writeJSON(w, map[string]interface{}{"status": "ok", "code_studio": status})
}

func (h codeStudioHandlers) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path, err := sanitizeCodeStudioPath(r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	cmd := []string{"find", path, "-maxdepth", "1", "-mindepth", "1", "-printf", "%y|%s|%T@|%p\n"}
	result, err := h.docker.Exec(r.Context(), containerID, cmd, 30*time.Second)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if result.ExitCode != 0 {
		jsonError(w, strings.TrimSpace(result.Output), http.StatusBadRequest)
		return
	}
	entries, err := parseCodeStudioFindOutput(result.Output)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "path": path, "files": entries})
}

func (h codeStudioHandlers) handleFile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleReadFile(w, r)
	case http.MethodPut:
		h.handleWriteFile(w, r)
	case http.MethodPatch:
		h.handleMoveFile(w, r)
	case http.MethodDelete:
		h.handleDeleteFile(w, r)
	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h codeStudioHandlers) handleReadFile(w http.ResponseWriter, r *http.Request) {
	path, err := sanitizeCodeStudioPath(r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	maxSize := h.maxFileSizeBytes()
	script := fmt.Sprintf("stat -c '%%s|%%Y' %s && base64 -w0 %s", shellQuote(path), shellQuote(path))
	result, err := h.docker.Exec(r.Context(), containerID, []string{"sh", "-c", script}, 30*time.Second)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if result.ExitCode != 0 {
		jsonError(w, strings.TrimSpace(result.Output), http.StatusBadRequest)
		return
	}
	lines := strings.SplitN(result.Output, "\n", 2)
	if len(lines) != 2 {
		jsonError(w, "invalid file read response", http.StatusBadGateway)
		return
	}
	size, modified, err := parseCodeStudioStatLine(strings.TrimSpace(lines[0]))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if size > maxSize {
		jsonError(w, "file exceeds configured maximum size", http.StatusRequestEntityTooLarge)
		return
	}
	content, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[1]))
	if err != nil {
		jsonError(w, "invalid file content encoding", http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]interface{}{
		"status":  "ok",
		"path":    path,
		"content": string(content),
		"entry": codeStudioFileEntry{
			Name:     pathpkg.Base(path),
			Path:     path,
			Type:     "file",
			Size:     size,
			Modified: modified,
		},
	})
}

func (h codeStudioHandlers) handleWriteFile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	path, err := sanitizeCodeStudioPath(firstNonEmpty(body.Path, r.URL.Query().Get("path")))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if int64(len(body.Content)) > h.maxFileSizeBytes() {
		jsonError(w, "file exceeds configured maximum size", http.StatusRequestEntityTooLarge)
		return
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(body.Content))
	script := fmt.Sprintf("mkdir -p %s && printf %%s %s | base64 -d > %s", shellQuote(pathpkg.Dir(path)), shellQuote(encoded), shellQuote(path))
	result, err := h.docker.Exec(r.Context(), containerID, []string{"sh", "-c", script}, 30*time.Second)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if result.ExitCode != 0 {
		jsonError(w, strings.TrimSpace(result.Output), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "path": path})
}

func (h codeStudioHandlers) handleMoveFile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	oldPath, err := sanitizeCodeStudioPath(body.OldPath)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	newPath, err := sanitizeCodeStudioPath(body.NewPath)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	script := fmt.Sprintf("mkdir -p %s && mv -- %s %s", shellQuote(pathpkg.Dir(newPath)), shellQuote(oldPath), shellQuote(newPath))
	result, err := h.docker.Exec(r.Context(), containerID, []string{"sh", "-c", script}, 30*time.Second)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if result.ExitCode != 0 {
		jsonError(w, strings.TrimSpace(result.Output), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "old_path": oldPath, "new_path": newPath})
}

func (h codeStudioHandlers) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	path, err := sanitizeCodeStudioPath(r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if path == codeStudioWorkspaceRoot {
		jsonError(w, "cannot delete workspace root", http.StatusBadRequest)
		return
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	result, err := h.docker.Exec(r.Context(), containerID, []string{"rm", "-rf", "--", path}, 30*time.Second)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if result.ExitCode != 0 {
		jsonError(w, strings.TrimSpace(result.Output), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "path": path})
}

func (h codeStudioHandlers) handleDirectory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	path, err := sanitizeCodeStudioPath(body.Path)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	result, err := h.docker.Exec(r.Context(), containerID, []string{"mkdir", "-p", "--", path}, 30*time.Second)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if result.ExitCode != 0 {
		jsonError(w, strings.TrimSpace(result.Output), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "path": path})
}

func (h codeStudioHandlers) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, codeStudioMaxUploadSize)
	if err := r.ParseMultipartForm(codeStudioMaxUploadSize); err != nil {
		jsonError(w, "Invalid upload", http.StatusBadRequest)
		return
	}
	destDir, err := sanitizeCodeStudioPath(r.FormValue("path"))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	h.writeUploadedFile(w, r, destDir, file, header)
}

func (h codeStudioHandlers) writeUploadedFile(w http.ResponseWriter, r *http.Request, destDir string, file multipart.File, header *multipart.FileHeader) {
	if header == nil || strings.TrimSpace(header.Filename) == "" {
		jsonError(w, "filename is required", http.StatusBadRequest)
		return
	}
	name := pathpkg.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	path, err := sanitizeCodeStudioPath(pathpkg.Join(destDir, name))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	var buf bytes.Buffer
	if _, err := io.CopyN(&buf, file, h.maxFileSizeBytes()+1); err != nil && err != io.EOF {
		jsonError(w, "Failed to read upload", http.StatusBadRequest)
		return
	}
	if int64(buf.Len()) > h.maxFileSizeBytes() {
		jsonError(w, "file exceeds configured maximum size", http.StatusRequestEntityTooLarge)
		return
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	script := fmt.Sprintf("mkdir -p %s && printf %%s %s | base64 -d > %s", shellQuote(pathpkg.Dir(path)), shellQuote(encoded), shellQuote(path))
	result, err := h.docker.Exec(r.Context(), containerID, []string{"sh", "-c", script}, 30*time.Second)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if result.ExitCode != 0 {
		jsonError(w, strings.TrimSpace(result.Output), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "path": path})
}

func (h codeStudioHandlers) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path, err := sanitizeCodeStudioPath(r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	result, err := h.docker.Exec(r.Context(), containerID, []string{"base64", "-w0", path}, 30*time.Second)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if result.ExitCode != 0 {
		jsonError(w, strings.TrimSpace(result.Output), http.StatusBadRequest)
		return
	}
	content, err := base64.StdEncoding.DecodeString(strings.TrimSpace(result.Output))
	if err != nil {
		jsonError(w, "invalid file content encoding", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(pathpkg.Base(path)))
	_, _ = w.Write(content)
}

func (h codeStudioHandlers) handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Command    string   `json:"command"`
		Args       []string `json:"args"`
		CWD        string   `json:"cwd"`
		TimeoutSec int      `json:"timeout_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	cmd, err := normalizeCodeStudioExecCommand(body.Command, body.Args, body.CWD)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	timeout := normalizeCodeStudioExecTimeout(body.TimeoutSec)
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	result, err := h.docker.Exec(r.Context(), containerID, cmd, timeout)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "exit_code": result.ExitCode, "output": result.Output})
}

func (h codeStudioHandlers) handleTerminal(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, containerID, err := h.codeContainer(r.Context(), true)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	cols := parsePositiveInt(r.URL.Query().Get("cols"), 120)
	rows := parsePositiveInt(r.URL.Query().Get("rows"), 30)
	execID, err := h.docker.CreateTerminalExec(r.Context(), containerID, cols, rows)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	conn, err := codeStudioWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	go h.readTerminalControl(conn, execID)
	stream, err := h.docker.StartExec(r.Context(), execID)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		return
	}
	frames, err := demuxDockerAttachStream(bytes.NewReader(stream))
	if err != nil {
		_ = conn.WriteMessage(websocket.BinaryMessage, stream)
		return
	}
	for _, frame := range frames {
		if len(frame.Payload) == 0 {
			continue
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, frame.Payload); err != nil {
			return
		}
	}
}

func (h codeStudioHandlers) readTerminalControl(conn *websocket.Conn, execID string) {
	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		var msg struct {
			Type string `json:"type"`
			Cols int    `json:"cols"`
			Rows int    `json:"rows"`
		}
		if json.Unmarshal(payload, &msg) == nil && msg.Type == "resize" {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = h.docker.ResizeExec(ctx, execID, msg.Cols, msg.Rows)
			cancel()
		}
	}
}

func (h codeStudioHandlers) isAuthenticated(r *http.Request) bool {
	if h.server == nil || h.server.Cfg == nil {
		return false
	}
	h.server.CfgMu.RLock()
	enabled := h.server.Cfg.Auth.Enabled
	secret := h.server.Cfg.Auth.SessionSecret
	h.server.CfgMu.RUnlock()
	if !enabled {
		return true
	}
	return IsAuthenticated(r, secret)
}

func (h codeStudioHandlers) maxFileSizeBytes() int64 {
	if h.server == nil || h.server.Cfg == nil {
		return 1024 * 1024
	}
	h.server.CfgMu.RLock()
	maxMB := h.server.Cfg.VirtualDesktop.MaxFileSizeMB
	h.server.CfgMu.RUnlock()
	if maxMB <= 0 {
		maxMB = 1
	}
	return int64(maxMB) * 1024 * 1024
}

var codeStudioWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		return err == nil && strings.EqualFold(u.Host, r.Host)
	},
}

func sanitizeCodeStudioPath(raw string) (string, error) {
	p := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("code studio path contains a null byte")
	}
	if p == "" {
		p = codeStudioWorkspaceRoot
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == ".." {
			return "", fmt.Errorf("code studio path escapes workspace")
		}
	}
	cleaned := pathpkg.Clean(p)
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	if cleaned != codeStudioWorkspaceRoot && !strings.HasPrefix(cleaned, codeStudioWorkspaceRoot+"/") {
		return "", fmt.Errorf("code studio path must be inside /workspace")
	}
	return cleaned, nil
}

func parseCodeStudioFindOutput(output string) ([]codeStudioFileEntry, error) {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		return []codeStudioFileEntry{}, nil
	}
	entries := make([]codeStudioFileEntry, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			return nil, fmt.Errorf("invalid find output line")
		}
		entryType := "file"
		if parts[0] == "d" {
			entryType = "directory"
		}
		size, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid find size: %w", err)
		}
		modifiedFloat, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid find timestamp: %w", err)
		}
		path, err := sanitizeCodeStudioPath(parts[3])
		if err != nil {
			return nil, err
		}
		sec := int64(modifiedFloat)
		nsec := int64((modifiedFloat - float64(sec)) * 1e9)
		entries = append(entries, codeStudioFileEntry{
			Name:     pathpkg.Base(path),
			Path:     path,
			Type:     entryType,
			Size:     size,
			Modified: time.Unix(sec, nsec).UTC(),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "directory"
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

func parseCodeStudioStatLine(line string) (int64, time.Time, error) {
	parts := strings.Split(line, "|")
	if len(parts) != 2 {
		return 0, time.Time{}, fmt.Errorf("invalid stat response")
	}
	size, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("invalid stat size: %w", err)
	}
	modified, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("invalid stat timestamp: %w", err)
	}
	return size, time.Unix(modified, 0).UTC(), nil
}

func normalizeCodeStudioExecCommand(command string, args []string, cwd string) ([]string, error) {
	if len(args) > 0 {
		if strings.TrimSpace(args[0]) == "" {
			return nil, fmt.Errorf("command is required")
		}
		return args, nil
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if strings.TrimSpace(cwd) == "" {
		return []string{"sh", "-lc", command}, nil
	}
	path, err := sanitizeCodeStudioPath(cwd)
	if err != nil {
		return nil, err
	}
	return []string{"sh", "-lc", "cd " + shellQuote(path) + " && " + command}, nil
}

func normalizeCodeStudioExecTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return 30 * time.Second
	}
	timeout := time.Duration(seconds) * time.Second
	if timeout > codeStudioMaxExecTime {
		return codeStudioMaxExecTime
	}
	return timeout
}

type dockerAttachFrame struct {
	Stream  byte
	Payload []byte
}

func demuxDockerAttachStream(r io.Reader) ([]dockerAttachFrame, error) {
	var frames []dockerAttachFrame
	for {
		header := make([]byte, 8)
		if _, err := io.ReadFull(r, header); err != nil {
			if err == io.EOF {
				return frames, nil
			}
			if err == io.ErrUnexpectedEOF {
				return nil, err
			}
			return nil, err
		}
		size := binary.BigEndian.Uint32(header[4:8])
		if size == 0 {
			frames = append(frames, dockerAttachFrame{Stream: header[0]})
			continue
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
		frames = append(frames, dockerAttachFrame{Stream: header[0], Payload: payload})
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func (a codeStudioDockerAdapter) Exec(ctx context.Context, containerID string, cmd []string, timeout time.Duration) (codeStudioExecResult, error) {
	if timeout <= 0 || timeout > codeStudioMaxExecTime {
		timeout = codeStudioMaxExecTime
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	payload := map[string]interface{}{
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          cmd,
		"Tty":          false,
	}
	body, _ := json.Marshal(payload)
	data, code, err := tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/exec", string(body))
	if err != nil {
		return codeStudioExecResult{}, err
	}
	if code != http.StatusCreated {
		return codeStudioExecResult{}, fmt.Errorf("docker exec create failed: %s", strings.TrimSpace(string(data)))
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ID == "" {
		return codeStudioExecResult{}, fmt.Errorf("parse docker exec id: %w", err)
	}
	startData, startCode, err := tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, "/exec/"+url.PathEscape(created.ID)+"/start", `{"Detach":false,"Tty":false}`)
	if err != nil {
		return codeStudioExecResult{}, err
	}
	if startCode != http.StatusOK {
		return codeStudioExecResult{}, fmt.Errorf("docker exec start failed: %s", strings.TrimSpace(string(startData)))
	}
	frames, err := demuxDockerAttachStream(bytes.NewReader(startData))
	output := string(startData)
	if err == nil {
		var buf bytes.Buffer
		for _, frame := range frames {
			buf.Write(frame.Payload)
		}
		output = buf.String()
	}
	exitCode := -1
	inspectData, inspectCode, inspectErr := tools.DockerRequestContext(context.Background(), a.cfg, http.MethodGet, "/exec/"+url.PathEscape(created.ID)+"/json", "")
	if inspectErr == nil && inspectCode == http.StatusOK {
		var inspect struct {
			ExitCode int `json:"ExitCode"`
		}
		if json.Unmarshal(inspectData, &inspect) == nil {
			exitCode = inspect.ExitCode
		}
	}
	return codeStudioExecResult{ExitCode: exitCode, Output: output}, nil
}

func (a codeStudioDockerAdapter) CreateTerminalExec(ctx context.Context, containerID string, cols, rows int) (string, error) {
	payload := map[string]interface{}{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          []string{"/bin/bash", "-l"},
		"Tty":          false,
	}
	body, _ := json.Marshal(payload)
	data, code, err := tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/exec", string(body))
	if err != nil {
		return "", err
	}
	if code != http.StatusCreated {
		return "", fmt.Errorf("docker terminal create failed: %s", strings.TrimSpace(string(data)))
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ID == "" {
		return "", fmt.Errorf("parse docker terminal exec id: %w", err)
	}
	_ = a.ResizeExec(ctx, created.ID, cols, rows)
	return created.ID, nil
}

func (a codeStudioDockerAdapter) StartExec(ctx context.Context, execID string) ([]byte, error) {
	data, code, err := tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, "/exec/"+url.PathEscape(execID)+"/start", `{"Detach":false,"Tty":false}`)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("docker terminal start failed: %s", strings.TrimSpace(string(data)))
	}
	return data, nil
}

func (a codeStudioDockerAdapter) ResizeExec(ctx context.Context, execID string, cols, rows int) error {
	cols = parsePositiveInt(strconv.Itoa(cols), 120)
	rows = parsePositiveInt(strconv.Itoa(rows), 30)
	endpoint := fmt.Sprintf("/exec/%s/resize?h=%d&w=%d", url.PathEscape(execID), rows, cols)
	data, code, err := tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, endpoint, "")
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusCreated && code != http.StatusNoContent {
		return fmt.Errorf("docker terminal resize failed: %s", strings.TrimSpace(string(data)))
	}
	return nil
}
