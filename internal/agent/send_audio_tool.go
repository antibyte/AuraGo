package agent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/media"
	"aurago/internal/tools"
)

// handleSendAudio resolves an audio file path or URL, copies/downloads it into
// data/audio/ and returns a JSON result the agent should include in its reply.
func handleSendAudio(req sendMediaArgs, cfg *config.Config, logger *slog.Logger, mediaRegistryDB *sql.DB) string {
	encode := func(r map[string]interface{}) string {
		b, _ := json.Marshal(r)
		return "Tool Output: " + string(b)
	}

	if req.Path == "" {
		return encode(map[string]interface{}{"status": "error", "message": "path is required"})
	}

	absDataDir, err := filepath.Abs(cfg.Directories.DataDir)
	if err != nil {
		return encode(map[string]interface{}{"status": "error", "message": "failed to resolve data dir: " + err.Error()})
	}
	audioDir := filepath.Join(absDataDir, "audio")
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		return encode(map[string]interface{}{"status": "error", "message": "failed to create audio dir: " + err.Error()})
	}

	title := req.Title
	if title == "" {
		title = req.Caption
	}

	var localPath string

	if strings.HasPrefix(req.Path, "http://") || strings.HasPrefix(req.Path, "https://") {
		saved, err := media.SaveURLToDir(req.Path, audioDir)
		if err != nil {
			return encode(map[string]interface{}{"status": "error", "message": "failed to download audio: " + err.Error()})
		}
		localPath = saved
	} else {
		candidate := resolveAgentFilePath(req.Path, cfg)
		if _, err := os.Stat(candidate); err != nil {
			return encode(map[string]interface{}{"status": "error", "message": "audio file not found: " + req.Path})
		}
		localPath = candidate
		// Copy into audio dir if not already there
		rel, relErr := filepath.Rel(audioDir, localPath)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			ext := filepath.Ext(localPath)
			filename := fmt.Sprintf("audio_%d%s", time.Now().UnixMilli(), ext)
			destPath := filepath.Join(audioDir, filename)
			if err := copyFileLocal(localPath, destPath); err != nil {
				return encode(map[string]interface{}{"status": "error", "message": "failed to copy audio to data dir: " + err.Error()})
			}
			localPath = destPath
		}
	}

	filename := filepath.Base(localPath)
	webPath := "/files/audio/" + filename
	mimeType := audioMIMEType(filename)

	if title == "" {
		title = strings.TrimSuffix(filename, filepath.Ext(filename))
	}

	fileInfo, _ := os.Stat(localPath)
	var fileSize int64
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	if mediaRegistryDB != nil {
		tools.RegisterMedia(mediaRegistryDB, tools.MediaItem{
			MediaType:   "audio",
			SourceTool:  "send_audio",
			Filename:    filename,
			FilePath:    localPath,
			WebPath:     webPath,
			FileSize:    fileSize,
			Format:      strings.TrimPrefix(filepath.Ext(filename), "."),
			Description: title,
			Tags:        []string{"agent-sent"},
		})
	}

	logger.Info("[send_audio] Audio ready", "web_path", webPath, "local_path", localPath)
	return encode(map[string]interface{}{
		"status":     "success",
		"web_path":   webPath,
		"local_path": localPath,
		"title":      title,
		"mime_type":  mimeType,
		"filename":   filename,
	})
}

// audioMIMEType returns a MIME type based on the file extension.
func audioMIMEType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".flac":
		return "audio/flac"
	case ".m4a", ".aac":
		return "audio/mp4"
	case ".opus":
		return "audio/opus"
	case ".webm":
		return "audio/webm"
	default:
		return "audio/mpeg"
	}
}

// resolveAgentFilePath resolves a file path the agent provided, looking in the
// workspace dir first, then falling back to the original path.
func resolveAgentFilePath(agentPath string, cfg *config.Config) string {
	if filepath.IsAbs(agentPath) {
		return agentPath
	}
	absWorkspaceDir, _ := filepath.Abs(cfg.Directories.WorkspaceDir)

	// Strip common workspace prefix patterns the agent might include
	wsClean := filepath.ToSlash(filepath.Clean(cfg.Directories.WorkspaceDir))
	wsClean = strings.TrimPrefix(wsClean, "./")
	candidateSlash := filepath.ToSlash(filepath.Clean(agentPath))
	if strings.HasPrefix(candidateSlash, wsClean+"/") {
		remainder := candidateSlash[len(wsClean)+1:]
		return filepath.Join(absWorkspaceDir, filepath.FromSlash(remainder))
	}

	// Try CWD-relative first, then workspace-relative
	if absCandidate, err := filepath.Abs(agentPath); err == nil {
		if _, statErr := os.Stat(absCandidate); statErr == nil {
			return absCandidate
		}
	}
	return filepath.Join(absWorkspaceDir, agentPath)
}

// detectContentType detects MIME type from file bytes if possible.
func detectContentType(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(buf[:n])
}
