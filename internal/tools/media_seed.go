package tools

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// seedEntry describes one sample media file in the assets/media_samples/metadata.json manifest.
type seedEntry struct {
	Filename         string   `json:"filename"`
	MediaType        string   `json:"media_type"`
	TargetDir        string   `json:"target_dir"`
	WebPathPrefix    string   `json:"web_path_prefix"`
	Description      string   `json:"description"`
	Format           string   `json:"format"`
	Tags             []string `json:"tags"`
	Prompt           string   `json:"prompt,omitempty"`
	Provider         string   `json:"provider,omitempty"`
	Model            string   `json:"model,omitempty"`
	Size             string   `json:"size,omitempty"`
	Quality          string   `json:"quality,omitempty"`
	GenerationTimeMs int64    `json:"generation_time_ms,omitempty"`
	CostEstimate     float64  `json:"cost_estimate,omitempty"`
}

// SeedWelcomeMedia copies bundled sample files into dataDir and registers them in the
// media registry on the first start. All errors are non-fatal and only logged as warnings.
func SeedWelcomeMedia(db *sql.DB, dataDir, installDir string, logger *slog.Logger) {
	// Look for assets in installDir first; if binary lives inside a 'bin/' subdirectory
	// the actual install root (where assets/ is extracted) is one level up.
	candidates := []string{
		filepath.Join(installDir, "assets", "media_samples", "metadata.json"),
		filepath.Join(filepath.Dir(installDir), "assets", "media_samples", "metadata.json"),
	}
	var manifestPath string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			manifestPath = p
			break
		}
	}
	if manifestPath == "" {
		logger.Warn("SeedWelcomeMedia: manifest not found, skipping", "searched", candidates[0])
		return
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		logger.Warn("SeedWelcomeMedia: manifest not found, skipping", "path", manifestPath, "error", err)
		return
	}

	var entries []seedEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		logger.Warn("SeedWelcomeMedia: failed to parse manifest", "error", err)
		return
	}

	srcDir := filepath.Dir(manifestPath) // same directory as the manifest
	for _, e := range entries {
		if err := seedOneFile(db, srcDir, dataDir, e, logger); err != nil {
			logger.Warn("SeedWelcomeMedia: failed to seed file", "filename", e.Filename, "error", err)
		}
	}
}

func seedOneFile(db *sql.DB, srcDir, dataDir string, e seedEntry, logger *slog.Logger) error {
	targetDir := filepath.Join(dataDir, e.TargetDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", targetDir, err)
	}

	dst := filepath.Join(targetDir, e.Filename)
	// Skip copy if file already exists (idempotent)
	if _, err := os.Stat(dst); err == nil {
		logger.Debug("SeedWelcomeMedia: file already present, skipping copy", "path", dst)
	} else {
		src := filepath.Join(srcDir, e.Filename)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s → %s: %w", src, dst, err)
		}
	}

	fi, err := os.Stat(dst)
	if err != nil {
		return fmt.Errorf("stat %s: %w", dst, err)
	}

	// Compute file hash for deduplication — prevents duplicate rows on every restart.
	fileHash, err := computeMediaFileHash(dst)
	if err != nil {
		logger.Warn("SeedWelcomeMedia: could not hash file, deduplication may fail", "path", dst, "error", err)
	}

	item := MediaItem{
		MediaType:        e.MediaType,
		SourceTool:       "seed",
		Filename:         e.Filename,
		FilePath:         dst,
		WebPath:          e.WebPathPrefix + e.Filename,
		FileSize:         fi.Size(),
		Format:           e.Format,
		Description:      e.Description,
		Tags:             e.Tags,
		Prompt:           e.Prompt,
		Provider:         e.Provider,
		Model:            e.Model,
		Size:             e.Size,
		Quality:          e.Quality,
		GenerationTimeMs: e.GenerationTimeMs,
		CostEstimate:     e.CostEstimate,
		Hash:             fileHash,
	}
	id, skipped, regErr := RegisterMedia(db, item)
	if regErr != nil {
		return fmt.Errorf("register %s: %w", e.Filename, regErr)
	}
	if skipped {
		logger.Debug("SeedWelcomeMedia: already registered", "filename", e.Filename, "id", id)
	} else {
		logger.Info("SeedWelcomeMedia: seeded media item", "filename", e.Filename, "media_type", e.MediaType, "id", id)
	}
	return nil
}

func copyFile(src, dst string) error {
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

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// computeMediaFileHash returns the SHA-256 hex digest of a file.
func computeMediaFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
