package agent

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"aurago/internal/config"
)

type servedFileRoot struct {
	prefix string
	root   string
}

func resolveServedFilePath(rawPath string, cfg *config.Config) (localPath, webPath string, matched bool, err error) {
	u, parseErr := url.Parse(rawPath)
	if parseErr != nil || u.Scheme != "" || u.Host != "" {
		return "", "", false, nil
	}

	docDir := cfg.Tools.DocumentCreator.OutputDir
	if docDir == "" {
		docDir = filepath.Join(cfg.Directories.DataDir, "documents")
	}
	roots := []servedFileRoot{
		{prefix: "/files/generated_images/", root: filepath.Join(cfg.Directories.DataDir, "generated_images")},
		{prefix: "/files/generated_videos/", root: filepath.Join(cfg.Directories.DataDir, "generated_videos")},
		{prefix: "/files/audio/", root: filepath.Join(cfg.Directories.DataDir, "audio")},
		{prefix: "/files/documents/", root: docDir},
		{prefix: "/files/downloads/", root: filepath.Join(cfg.Directories.DataDir, "downloads")},
	}

	for _, root := range roots {
		if !strings.HasPrefix(u.Path, root.prefix) {
			continue
		}
		rel, cleanErr := cleanServedRelPath(strings.TrimPrefix(u.Path, root.prefix))
		if cleanErr != nil {
			return "", "", true, cleanErr
		}
		absRoot, absErr := filepath.Abs(root.root)
		if absErr != nil {
			return "", "", true, absErr
		}
		local := filepath.Join(absRoot, filepath.FromSlash(rel))
		if relToRoot, relErr := filepath.Rel(absRoot, local); relErr != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
			return "", "", true, fmt.Errorf("served file path escapes its root")
		}
		return local, root.prefix + path.Clean("/" + rel)[1:], true, nil
	}
	return "", "", false, nil
}

func cleanServedRelPath(rawRel string) (string, error) {
	rel, err := url.PathUnescape(rawRel)
	if err != nil {
		return "", err
	}
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return "", fmt.Errorf("served file path is missing a filename")
	}
	if strings.Contains(rel, "\\") {
		return "", fmt.Errorf("served file path contains a path separator")
	}
	parts := strings.Split(rel, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", fmt.Errorf("served file path contains traversal")
		}
		cleaned = append(cleaned, part)
	}
	if len(cleaned) == 0 {
		return "", fmt.Errorf("served file path is missing a filename")
	}
	return strings.Join(cleaned, "/"), nil
}
