package agent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/media"
	"aurago/internal/tools"
)

// handleSendVideo resolves a video file path or URL, copies/downloads it into
// data/generated_videos/, and returns a JSON result for inline chat playback.
func handleSendVideo(req sendMediaArgs, cfg *config.Config, logger *slog.Logger, mediaRegistryDB *sql.DB) string {
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
	videoDir := filepath.Join(absDataDir, "generated_videos")
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		return encode(map[string]interface{}{"status": "error", "message": "failed to create video dir: " + err.Error()})
	}

	title := firstNonEmptyToolString(req.Title, req.Caption)
	var localPath string
	copiedOrDownloaded := false

	if strings.HasPrefix(req.Path, "http://") || strings.HasPrefix(req.Path, "https://") {
		saved, err := media.SaveURLToDir(req.Path, videoDir)
		if err != nil {
			return encode(map[string]interface{}{"status": "error", "message": "failed to download video: " + err.Error()})
		}
		localPath = saved
		copiedOrDownloaded = true
	} else {
		candidate := resolveAgentFilePath(req.Path, cfg)
		if _, err := os.Stat(candidate); err != nil {
			return encode(map[string]interface{}{"status": "error", "message": "video file not found: " + req.Path})
		}
		localPath = candidate
		rel, relErr := filepath.Rel(videoDir, localPath)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			ext := filepath.Ext(localPath)
			filename := fmt.Sprintf("video_%d%s", time.Now().UnixMilli(), ext)
			destPath := filepath.Join(videoDir, filename)
			if err := copyFileLocal(localPath, destPath); err != nil {
				return encode(map[string]interface{}{"status": "error", "message": "failed to copy video to data dir: " + err.Error()})
			}
			localPath = destPath
			copiedOrDownloaded = true
		}
	}

	filename := filepath.Base(localPath)
	format := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if !isSupportedVideoFormat(format) {
		if copiedOrDownloaded {
			_ = os.Remove(localPath)
		}
		return encode(map[string]interface{}{"status": "error", "message": "unsupported video format: " + format})
	}

	webPath := "/files/generated_videos/" + filename
	mimeType := videoMIMEType(filename)
	if title == "" {
		title = strings.TrimSuffix(filename, filepath.Ext(filename))
	}

	fileInfo, _ := os.Stat(localPath)
	var fileSize int64
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	fileHash := ""
	if hash, hashErr := tools.ComputeMediaFileHash(localPath); hashErr == nil {
		fileHash = hash
	}

	if mediaRegistryDB != nil {
		if regID, dup, regErr := tools.RegisterMedia(mediaRegistryDB, tools.MediaItem{
			MediaType:   "video",
			SourceTool:  "send_video",
			Filename:    filename,
			FilePath:    localPath,
			WebPath:     webPath,
			FileSize:    fileSize,
			Format:      format,
			Description: title,
			Tags:        []string{"agent-sent"},
			Hash:        fileHash,
		}); regErr != nil {
			logger.Warn("Auto-register video in media registry failed", "filename", filename, "error", regErr)
		} else if !dup {
			logger.Debug("Auto-registered video in media registry", "id", regID, "filename", filename)
		}
	}

	logger.Info("[send_video] Video ready", "web_path", webPath, "local_path", localPath)
	return encode(map[string]interface{}{
		"status":     "success",
		"web_path":   webPath,
		"local_path": localPath,
		"title":      title,
		"mime_type":  mimeType,
		"filename":   filename,
		"format":     format,
		"file_size":  fileSize,
	})
}

func isSupportedVideoFormat(format string) bool {
	switch strings.ToLower(strings.TrimPrefix(format, ".")) {
	case "mp4", "m4v", "mov", "webm", "ogv", "ogg":
		return true
	default:
		return false
	}
}

func videoMIMEType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".ogv", ".ogg":
		return "video/ogg"
	default:
		return "video/mp4"
	}
}
