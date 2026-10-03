package media

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// StageWorkspaceImage gives external bot libraries a private immutable image snapshot.
func StageWorkspaceImage(workspace, rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/files/") {
		return "", fmt.Errorf("invalid workspace image URL")
	}
	rel := strings.TrimPrefix(parsed.Path, "/files/")
	if !fs.ValidPath(rel) || strings.ContainsAny(rel, "\\:\x00") {
		return "", fmt.Errorf("invalid workspace image path")
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return "", err
	}
	defer root.Close()
	parts := strings.Split(rel, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("workspace image contains a symlink")
		}
	}
	file, err := root.Open(rel)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 50<<20 {
		return "", fmt.Errorf("invalid workspace image file")
	}
	data, err := io.ReadAll(io.LimitReader(file, (50<<20)+1))
	if err != nil {
		return "", err
	}
	if len(data) > 50<<20 {
		return "", fmt.Errorf("workspace image exceeds limit")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("workspace file is not a supported image: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 40_000_000 {
		return "", fmt.Errorf("workspace image dimensions exceed limit")
	}
	output, err := os.CreateTemp("", "aurago-bot-image-*"+filepath.Ext(rel))
	if err != nil {
		return "", err
	}
	_, writeErr := output.Write(data)
	closeErr := output.Close()
	if writeErr != nil || closeErr != nil {
		os.Remove(output.Name())
		return "", fmt.Errorf("stage workspace image: %v %v", writeErr, closeErr)
	}
	return output.Name(), nil
}
