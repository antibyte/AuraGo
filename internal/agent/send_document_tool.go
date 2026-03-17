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

// handleSendDocument resolves a document path or URL, copies/downloads it into
// data/documents/ and returns a JSON result the agent should include in its reply.
func handleSendDocument(tc ToolCall, cfg *config.Config, logger *slog.Logger, mediaRegistryDB *sql.DB) string {
	encode := func(r map[string]interface{}) string {
		b, _ := json.Marshal(r)
		return "Tool Output: " + string(b)
	}

	if tc.Path == "" {
		return encode(map[string]interface{}{"status": "error", "message": "path is required"})
	}

	absDataDir, err := filepath.Abs(cfg.Directories.DataDir)
	if err != nil {
		return encode(map[string]interface{}{"status": "error", "message": "failed to resolve data dir: " + err.Error()})
	}
	docDir := filepath.Join(absDataDir, "documents")
	if err := os.MkdirAll(docDir, 0755); err != nil {
		return encode(map[string]interface{}{"status": "error", "message": "failed to create documents dir: " + err.Error()})
	}

	title := tc.Title
	if title == "" {
		title = tc.Caption
	}

	var localPath string

	if strings.HasPrefix(tc.Path, "http://") || strings.HasPrefix(tc.Path, "https://") {
		saved, err := media.SaveURLToDir(tc.Path, docDir)
		if err != nil {
			return encode(map[string]interface{}{"status": "error", "message": "failed to download document: " + err.Error()})
		}
		localPath = saved
	} else {
		candidate := resolveAgentFilePath(tc.Path, cfg)
		if _, err := os.Stat(candidate); err != nil {
			return encode(map[string]interface{}{"status": "error", "message": "document file not found: " + tc.Path})
		}
		localPath = candidate
		// Copy into documents dir if not already there
		rel, relErr := filepath.Rel(docDir, localPath)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			ext := filepath.Ext(localPath)
			filename := fmt.Sprintf("doc_%d%s", time.Now().UnixMilli(), ext)
			destPath := filepath.Join(docDir, filename)
			if err := copyFileLocal(localPath, destPath); err != nil {
				return encode(map[string]interface{}{"status": "error", "message": "failed to copy document to data dir: " + err.Error()})
			}
			localPath = destPath
		}
	}

	filename := filepath.Base(localPath)
	webPath := "/files/documents/" + filename
	mimeType := documentMIMEType(filename)
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	inline := isInlineDocument(ext)

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
			MediaType:   "document",
			SourceTool:  "send_document",
			Filename:    filename,
			FilePath:    localPath,
			WebPath:     webPath,
			FileSize:    fileSize,
			Format:      ext,
			Description: title,
			Tags:        []string{"agent-sent"},
		})
	}

	// Provide an inline preview URL for PDFs and text-like docs
	previewURL := ""
	if inline {
		previewURL = webPath + "?inline=1"
	}

	logger.Info("[send_document] Document ready", "web_path", webPath, "local_path", localPath)
	return encode(map[string]interface{}{
		"status":      "success",
		"web_path":    webPath,
		"preview_url": previewURL,
		"local_path":  localPath,
		"title":       title,
		"mime_type":   mimeType,
		"filename":    filename,
		"format":      ext,
	})
}

// documentMIMEType returns a MIME type for common document formats.
func documentMIMEType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".doc":
		return "application/msword"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	default:
		return "application/octet-stream"
	}
}

// isInlineDocument reports whether the format can be viewed inline in the browser.
func isInlineDocument(ext string) bool {
	switch ext {
	case "pdf", "txt", "md", "csv", "json", "xml", "html", "htm":
		return true
	default:
		return false
	}
}
