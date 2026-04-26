package invasion

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ArtifactStorage stores egg-produced files under a local host directory.
type ArtifactStorage struct {
	BaseDir string
}

type StoredArtifact struct {
	Path      string
	SizeBytes int64
	SHA256    string
}

func NewArtifactStorage(baseDir string) ArtifactStorage {
	return ArtifactStorage{BaseDir: baseDir}
}

func (s ArtifactStorage) Save(artifact ArtifactRecord, r io.Reader, expectedSize int64, expectedSHA string) (StoredArtifact, error) {
	if strings.TrimSpace(s.BaseDir) == "" {
		return StoredArtifact{}, fmt.Errorf("artifact storage base directory is required")
	}
	if strings.TrimSpace(artifact.NestID) == "" {
		return StoredArtifact{}, fmt.Errorf("nest_id is required")
	}
	if strings.TrimSpace(artifact.ID) == "" {
		return StoredArtifact{}, fmt.Errorf("artifact id is required")
	}
	filename := SanitizeArtifactFilename(artifact.Filename)
	if filename == "" {
		filename = "artifact.bin"
	}
	targetDir := filepath.Join(s.BaseDir, safeArtifactPathSegment(artifact.NestID), safeArtifactPathSegment(artifact.ID))
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		return StoredArtifact{}, fmt.Errorf("create artifact directory: %w", err)
	}
	tmp, err := os.CreateTemp(targetDir, ".upload-*")
	if err != nil {
		return StoredArtifact{}, fmt.Errorf("create artifact temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	written, err := copyWithOptionalLimit(io.MultiWriter(tmp, hasher), r, expectedSize)
	if err != nil {
		return StoredArtifact{}, err
	}
	if expectedSize > 0 && written != expectedSize {
		return StoredArtifact{}, fmt.Errorf("artifact size mismatch: got %d bytes, expected %d", written, expectedSize)
	}
	actualSHA := hex.EncodeToString(hasher.Sum(nil))
	if expectedSHA = normalizeSHA256(expectedSHA); expectedSHA != "" && actualSHA != expectedSHA {
		return StoredArtifact{}, fmt.Errorf("artifact sha256 mismatch: got %s, expected %s", actualSHA, expectedSHA)
	}
	if err := tmp.Close(); err != nil {
		return StoredArtifact{}, fmt.Errorf("close artifact temp file: %w", err)
	}
	finalPath := filepath.Join(targetDir, filename)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return StoredArtifact{}, fmt.Errorf("move artifact into place: %w", err)
	}
	cleanup = false
	return StoredArtifact{Path: finalPath, SizeBytes: written, SHA256: actualSHA}, nil
}

func safeArtifactPathSegment(value string) string {
	segment := SanitizeArtifactFilename(value)
	if segment == "" || segment == "artifact.bin" {
		return "unknown"
	}
	return segment
}

func copyWithOptionalLimit(dst io.Writer, src io.Reader, expectedSize int64) (int64, error) {
	if expectedSize <= 0 {
		n, err := io.Copy(dst, src)
		if err != nil {
			return n, fmt.Errorf("write artifact: %w", err)
		}
		return n, nil
	}
	limited := io.LimitReader(src, expectedSize+1)
	n, err := io.Copy(dst, limited)
	if err != nil {
		return n, fmt.Errorf("write artifact: %w", err)
	}
	if n > expectedSize {
		return n, fmt.Errorf("artifact exceeds expected size %d", expectedSize)
	}
	return n, nil
}
