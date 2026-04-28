package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/media"
)

// handleSendImage resolves the image path or URL, copies/downloads it into the
// workspace images directory, and returns a JSON result with the web path and
// a ready-to-use markdown image string for the agent to include in its response.
func handleSendImage(req sendMediaArgs, cfg *config.Config, logger *slog.Logger) string {
	encode := func(r map[string]interface{}) string {
		b, _ := json.Marshal(r)
		return "Tool Output: " + string(b)
	}

	if req.Path == "" {
		return encode(map[string]interface{}{"status": "error", "message": "path is required"})
	}

	// Always work with absolute paths to avoid mixed-abs/relative issues with filepath.Rel.
	absWorkspaceDir, err := filepath.Abs(cfg.Directories.WorkspaceDir)
	if err != nil {
		return encode(map[string]interface{}{"status": "error", "message": "failed to resolve workspace dir: " + err.Error()})
	}
	imagesDir := filepath.Join(absWorkspaceDir, "images")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return encode(map[string]interface{}{"status": "error", "message": "failed to create images dir: " + err.Error()})
	}

	caption := req.Caption
	var localPath string
	servedWebPath := ""

	if strings.HasPrefix(req.Path, "http://") || strings.HasPrefix(req.Path, "https://") {
		saved, err := media.SaveURLToDir(req.Path, imagesDir)
		if err != nil {
			return encode(map[string]interface{}{"status": "error", "message": "failed to download image: " + err.Error()})
		}
		localPath = saved
	} else {
		if servedPath, webPath, matched, err := resolveServedFilePath(req.Path, cfg); matched {
			if err != nil {
				return encode(map[string]interface{}{"status": "error", "message": "invalid served image path: " + err.Error()})
			}
			if _, err := os.Stat(servedPath); err != nil {
				return encode(map[string]interface{}{"status": "error", "message": "image file not found: " + req.Path})
			}
			localPath = servedPath
			servedWebPath = webPath
		} else {
			candidate := req.Path
			if !filepath.IsAbs(candidate) {
				// The agent may pass full-ish paths like "agent_workspace/workdir/snail.jpg".
				// Strip the workspace dir prefix (clean form) so we get just the filename/sub-path.
				wsClean := filepath.ToSlash(filepath.Clean(cfg.Directories.WorkspaceDir))
				wsClean = strings.TrimPrefix(wsClean, "./")
				candidateSlash := filepath.ToSlash(filepath.Clean(candidate))
				if strings.HasPrefix(candidateSlash, wsClean+"/") {
					// Strip workspace prefix → workspace-relative remainder
					remainder := candidateSlash[len(wsClean)+1:]
					candidate = filepath.Join(absWorkspaceDir, filepath.FromSlash(remainder))
				} else {
					// Try CWD-relative first, fall back to WorkspaceDir-relative
					absCandidate, _ := filepath.Abs(candidate)
					if _, statErr := os.Stat(absCandidate); statErr == nil {
						candidate = absCandidate
					} else {
						candidate = filepath.Join(absWorkspaceDir, candidate)
					}
				}
			}
			if _, err := os.Stat(candidate); err != nil {
				return encode(map[string]interface{}{"status": "error", "message": "image file not found: " + req.Path})
			}
			localPath = candidate
			// If outside workspace dir, copy into images sub-dir so /files/ can serve it
			rel, relErr := filepath.Rel(absWorkspaceDir, localPath)
			if relErr != nil || strings.HasPrefix(rel, "..") {
				ext := filepath.Ext(localPath)
				filename := fmt.Sprintf("img_%d%s", time.Now().UnixMilli(), ext)
				destPath := filepath.Join(imagesDir, filename)
				if err := copyFileLocal(localPath, destPath); err != nil {
					return encode(map[string]interface{}{"status": "error", "message": "failed to copy image to workspace: " + err.Error()})
				}
				localPath = destPath
			}
		}
	}

	webPath := servedWebPath
	if webPath == "" {
		rel, err := filepath.Rel(absWorkspaceDir, localPath)
		if err != nil {
			return encode(map[string]interface{}{"status": "error", "message": "failed to compute web path: " + err.Error()})
		}
		webPath = "/files/" + filepath.ToSlash(rel)
	}
	markdownImage := fmt.Sprintf("![%s](%s)", caption, webPath)

	logger.Info("[send_image] Image ready", "web_path", webPath, "local_path", localPath)
	return encode(map[string]interface{}{
		"status":     "success",
		"web_path":   webPath,
		"local_path": localPath,
		"caption":    caption,
		"markdown":   markdownImage,
	})
}

// copyFileLocal copies src to dst.
func copyFileLocal(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
