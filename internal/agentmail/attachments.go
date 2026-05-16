package agentmail

import (
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// PrepareAttachments converts either workspace-local paths or explicit base64
// blobs into AgentMail send payload attachments.
func PrepareAttachments(workspaceDir string, maxAttachmentMB int, inputs []AttachmentInput) ([]OutgoingAttachment, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	workspaceAbs, err := filepath.Abs(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}
	if eval, err := filepath.EvalSymlinks(workspaceAbs); err == nil {
		workspaceAbs = eval
	}
	maxBytes := int64(maxAttachmentMB) * 1024 * 1024

	out := make([]OutgoingAttachment, 0, len(inputs))
	for _, input := range inputs {
		if strings.TrimSpace(input.Base64) != "" {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.Base64))
			if err != nil {
				return nil, fmt.Errorf("decode attachment %q: %w", input.Filename, err)
			}
			if err := checkAttachmentSize(int64(len(decoded)), maxBytes, input.Filename); err != nil {
				return nil, err
			}
			filename := strings.TrimSpace(input.Filename)
			if filename == "" {
				return nil, fmt.Errorf("base64 attachment requires filename")
			}
			out = append(out, OutgoingAttachment{
				Filename:      filepath.Base(filename),
				ContentType:   strings.TrimSpace(input.ContentType),
				ContentBase64: strings.TrimSpace(input.Base64),
			})
			continue
		}

		path := strings.TrimSpace(input.Path)
		if path == "" {
			continue
		}
		resolved, err := resolveWorkspaceFile(workspaceAbs, path)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("stat attachment %q: %w", path, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("attachment %q is a directory", path)
		}
		if err := checkAttachmentSize(info.Size(), maxBytes, path); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return nil, fmt.Errorf("read attachment %q: %w", path, err)
		}
		filename := strings.TrimSpace(input.Filename)
		if filename == "" {
			filename = filepath.Base(resolved)
		}
		contentType := strings.TrimSpace(input.ContentType)
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(filename))
		}
		out = append(out, OutgoingAttachment{
			Filename:      filepath.Base(filename),
			ContentType:   contentType,
			ContentBase64: base64.StdEncoding.EncodeToString(data),
		})
	}
	return out, nil
}

func resolveWorkspaceFile(workspaceAbs, rawPath string) (string, error) {
	candidate := rawPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workspaceAbs, candidate)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve attachment path: %w", err)
	}
	if eval, err := filepath.EvalSymlinks(abs); err == nil {
		abs = eval
	}
	rel, err := filepath.Rel(workspaceAbs, abs)
	if err != nil {
		return "", fmt.Errorf("compare attachment path: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("attachment path %q is outside the workspace", rawPath)
	}
	return abs, nil
}

func checkAttachmentSize(size, maxBytes int64, name string) error {
	if maxBytes >= 0 && size > maxBytes {
		return fmt.Errorf("attachment %q exceeds max size", name)
	}
	return nil
}
