package invasion

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MaxArtifactSizeBytes int64 = 100 << 20

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
	if expectedSize <= 0 {
		return StoredArtifact{}, fmt.Errorf("expected artifact size is required")
	}
	if expectedSize > MaxArtifactSizeBytes {
		return StoredArtifact{}, fmt.Errorf("artifact exceeds maximum size %d", MaxArtifactSizeBytes)
	}
	expectedSHA = normalizeSHA256(expectedSHA)
	if len(expectedSHA) != 64 {
		return StoredArtifact{}, fmt.Errorf("expected artifact sha256 is required")
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
	if actualSHA != expectedSHA {
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

func (s ArtifactStorage) CleanupTempFiles(maxAge time.Duration) (int, error) {
	baseDir := strings.TrimSpace(s.BaseDir)
	if baseDir == "" {
		return 0, fmt.Errorf("artifact storage base directory is required")
	}
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	err := filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasPrefix(d.Name(), ".upload-") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(cutoff) {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		removed++
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return removed, err
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
		return 0, fmt.Errorf("expected artifact size is required")
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
