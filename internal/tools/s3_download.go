package tools

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// writeS3DownloadRooted stages the transfer inside the workspace; only the final rename holds the publication fence.
func writeS3DownloadRooted(cfg S3Config, dest string, src io.Reader, expected int64) (int64, error) {
	limit := cfg.MaxDownloadBytes
	if limit <= 0 {
		limit = 50 << 20
	}
	if expected < 0 || expected > limit {
		return 0, fmt.Errorf("download exceeds the %d byte limit or has an unknown size", limit)
	}
	if err := requireUnprotectedSystemPath(dest, dest); err != nil {
		return 0, err
	}
	if cfg.Config != nil && IsProtectedSystemPath(dest, cfg.WorkspaceDir, cfg.Config) {
		return 0, fmt.Errorf("download destination is protected")
	}
	rootPath, err := filepath.EvalSymlinks(cfg.WorkspaceDir)
	if err != nil {
		return 0, fmt.Errorf("resolve workspace: %w", err)
	}
	rel, err := filepath.Rel(rootPath, dest)
	if err != nil || !s3PathWithinRoot(rootPath, dest) {
		return 0, fmt.Errorf("download destination must remain inside workspace")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	if err := root.MkdirAll(filepath.Dir(rel), 0o750); err != nil {
		return 0, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return 0, err
	}
	temp := filepath.Join(filepath.Dir(rel), ".aurago_edit_"+hex.EncodeToString(nonce[:]))
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, err
	}
	defer root.Remove(temp)
	reader := &s3DownloadReader{cfg: cfg, src: src, remaining: limit + 1}
	written, copyErr := io.Copy(f, reader)
	if copyErr == nil && written != expected {
		copyErr = fmt.Errorf("incomplete download: expected %d bytes, received %d", expected, written)
	}
	if copyErr == nil {
		copyErr = f.Sync()
	}
	closeErr := f.Close()
	if copyErr != nil {
		return written, copyErr
	}
	if closeErr != nil {
		return written, closeErr
	}
	if err := cfg.publish(func() error { return root.Rename(temp, rel) }); err != nil {
		return written, err
	}
	return written, nil
}

type s3DownloadReader struct {
	cfg       S3Config
	src       io.Reader
	remaining int64
}

func (r *s3DownloadReader) Read(p []byte) (int, error) {
	if err := r.cfg.check(); err != nil {
		return 0, err
	}
	if r.remaining <= 0 {
		return 0, fmt.Errorf("S3 download exceeds its byte limit")
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.src.Read(p)
	r.remaining -= int64(n)
	if r.remaining == 0 {
		return n, fmt.Errorf("S3 download exceeds its byte limit")
	}
	return n, err
}
