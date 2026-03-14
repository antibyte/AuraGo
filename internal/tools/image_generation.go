package tools

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ImageGenConfig holds the resolved provider configuration for image generation.
type ImageGenConfig struct {
	ProviderType string // openai, openrouter, stability, ideogram, google
	BaseURL      string
	APIKey       string
	Model        string
	DataDir      string // path to data/ directory for saving images
}

// ImageGenOptions holds per-request options for image generation.
type ImageGenOptions struct {
	Size        string // e.g. "1024x1024"
	Quality     string // "standard", "hd"
	Style       string // "natural", "vivid"
	SourceImage string // path to source image for image-to-image
}

// ImageGenResult holds the result of an image generation request.
type ImageGenResult struct {
	Filename        string  `json:"filename"`
	WebPath         string  `json:"web_path"`
	Markdown        string  `json:"markdown"`
	Prompt          string  `json:"prompt"`
	EnhancedPrompt  string  `json:"enhanced_prompt,omitempty"`
	Model           string  `json:"model"`
	Provider        string  `json:"provider"`
	Size            string  `json:"size,omitempty"`
	Quality         string  `json:"quality,omitempty"`
	Style           string  `json:"style,omitempty"`
	DurationMs      int64   `json:"duration_ms"`
	CostEstimate    float64 `json:"cost_estimate,omitempty"`
	SourceImage     string  `json:"source_image,omitempty"`
	GeneratedImages int     `json:"generated_images,omitempty"` // for batch requests
}

// GeneratedImageRecord represents a row in the generated_images SQLite table.
type GeneratedImageRecord struct {
	ID               int64   `json:"id"`
	CreatedAt        string  `json:"created_at"`
	Prompt           string  `json:"prompt"`
	EnhancedPrompt   string  `json:"enhanced_prompt,omitempty"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	Size             string  `json:"size,omitempty"`
	Quality          string  `json:"quality,omitempty"`
	Style            string  `json:"style,omitempty"`
	Filename         string  `json:"filename"`
	FileSize         int64   `json:"file_size"`
	SourceImage      string  `json:"source_image,omitempty"`
	GenerationTimeMs int64   `json:"generation_time_ms"`
	CostEstimate     float64 `json:"cost_estimate"`
}

// InitImageGalleryDB initializes the image gallery SQLite database.
func InitImageGalleryDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image gallery database: %w", err)
	}

	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS generated_images (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
		prompt            TEXT NOT NULL,
		enhanced_prompt   TEXT DEFAULT '',
		provider          TEXT NOT NULL,
		model             TEXT NOT NULL,
		size              TEXT DEFAULT '',
		quality           TEXT DEFAULT '',
		style             TEXT DEFAULT '',
		filename          TEXT NOT NULL,
		file_size         INTEGER DEFAULT 0,
		source_image      TEXT DEFAULT '',
		generation_time_ms INTEGER DEFAULT 0,
		cost_estimate     REAL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_generated_images_created_at ON generated_images(created_at);
	CREATE INDEX IF NOT EXISTS idx_generated_images_provider ON generated_images(provider);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create image gallery schema: %w", err)
	}

	db.Exec("PRAGMA user_version = 1")
	return db, nil
}

// SaveGeneratedImage inserts a record into the image gallery database.
func SaveGeneratedImage(db *sql.DB, r *ImageGenResult) (int64, error) {
	if db == nil {
		return 0, nil
	}
	res, err := db.Exec(`INSERT INTO generated_images
		(prompt, enhanced_prompt, provider, model, size, quality, style, filename, file_size, source_image, generation_time_ms, cost_estimate)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Prompt, r.EnhancedPrompt, r.Provider, r.Model, r.Size, r.Quality, r.Style,
		r.Filename, fileSize(r.Filename), r.SourceImage, r.DurationMs, r.CostEstimate,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to save generated image record: %w", err)
	}
	return res.LastInsertId()
}

// ListGeneratedImages returns generated images with optional filtering and pagination.
func ListGeneratedImages(db *sql.DB, provider, searchQuery string, limit, offset int) ([]GeneratedImageRecord, int, error) {
	if db == nil {
		return nil, 0, nil
	}
	if limit <= 0 {
		limit = 50
	}

	var conditions []string
	var args []interface{}
	if provider != "" {
		conditions = append(conditions, "provider = ?")
		args = append(args, provider)
	}
	if searchQuery != "" {
		conditions = append(conditions, "(prompt LIKE ? OR enhanced_prompt LIKE ?)")
		pattern := "%" + searchQuery + "%"
		args = append(args, pattern, pattern)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := db.QueryRow("SELECT COUNT(*) FROM generated_images"+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch page
	query := "SELECT id, created_at, prompt, enhanced_prompt, provider, model, size, quality, style, filename, file_size, source_image, generation_time_ms, cost_estimate FROM generated_images" + where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []GeneratedImageRecord
	for rows.Next() {
		var r GeneratedImageRecord
		if err := rows.Scan(&r.ID, &r.CreatedAt, &r.Prompt, &r.EnhancedPrompt, &r.Provider, &r.Model, &r.Size, &r.Quality, &r.Style, &r.Filename, &r.FileSize, &r.SourceImage, &r.GenerationTimeMs, &r.CostEstimate); err != nil {
			continue
		}
		records = append(records, r)
	}
	return records, total, nil
}

// GetGeneratedImage returns a single image record by ID.
func GetGeneratedImage(db *sql.DB, id int64) (*GeneratedImageRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("image gallery database not initialized")
	}
	var r GeneratedImageRecord
	err := db.QueryRow("SELECT id, created_at, prompt, enhanced_prompt, provider, model, size, quality, style, filename, file_size, source_image, generation_time_ms, cost_estimate FROM generated_images WHERE id = ?", id).Scan(
		&r.ID, &r.CreatedAt, &r.Prompt, &r.EnhancedPrompt, &r.Provider, &r.Model, &r.Size, &r.Quality, &r.Style, &r.Filename, &r.FileSize, &r.SourceImage, &r.GenerationTimeMs, &r.CostEstimate,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// DeleteGeneratedImage removes a record and its file from disk.
func DeleteGeneratedImage(db *sql.DB, id int64, dataDir string) error {
	if db == nil {
		return fmt.Errorf("image gallery database not initialized")
	}
	var filename string
	if err := db.QueryRow("SELECT filename FROM generated_images WHERE id = ?", id).Scan(&filename); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM generated_images WHERE id = ?", id); err != nil {
		return err
	}
	// Best-effort file removal
	if filename != "" {
		imgPath := filepath.Join(dataDir, "generated_images", filename)
		os.Remove(imgPath)
	}
	return nil
}

// ImageGalleryMonthlyCount returns the number of images generated in the current month.
func ImageGalleryMonthlyCount(db *sql.DB) (int, error) {
	if db == nil {
		return 0, nil
	}
	start := time.Now().Format("2006-01") + "-01"
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM generated_images WHERE created_at >= ?", start).Scan(&count)
	return count, err
}

// GenerateImage dispatches image generation to the appropriate backend based on provider type.
func GenerateImage(cfg ImageGenConfig, prompt string, opts ImageGenOptions) (*ImageGenResult, error) {
	start := time.Now()

	var imgData []byte
	var format string
	var err error

	switch strings.ToLower(cfg.ProviderType) {
	case "openrouter":
		imgData, format, err = generateOpenRouter(cfg, prompt, opts)
	case "openai":
		imgData, format, err = generateOpenAI(cfg, prompt, opts)
	case "stability":
		imgData, format, err = generateStability(cfg, prompt, opts)
	case "ideogram":
		imgData, format, err = generateIdeogram(cfg, prompt, opts)
	case "google", "google-imagen":
		imgData, format, err = generateGoogleImagen(cfg, prompt, opts)
	default:
		return nil, fmt.Errorf("unsupported image generation provider type: %q", cfg.ProviderType)
	}
	if err != nil {
		return nil, err
	}

	filename, err := saveImageData(imgData, format, cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to save generated image: %w", err)
	}

	duration := time.Since(start)
	webPath := "/files/generated_images/" + filename

	return &ImageGenResult{
		Filename:     filename,
		WebPath:      webPath,
		Markdown:     fmt.Sprintf("![Generated Image](%s)", webPath),
		Prompt:       prompt,
		Model:        cfg.Model,
		Provider:     cfg.ProviderType,
		Size:         opts.Size,
		Quality:      opts.Quality,
		Style:        opts.Style,
		DurationMs:   duration.Milliseconds(),
		SourceImage:  opts.SourceImage,
		CostEstimate: estimateCost(cfg.ProviderType, cfg.Model),
	}, nil
}

// saveImageData writes raw image bytes to data/generated_images/ with a unique filename.
func saveImageData(data []byte, format string, dataDir string) (string, error) {
	dir := filepath.Join(dataDir, "generated_images")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create generated_images directory: %w", err)
	}

	ext := ".png"
	if format != "" {
		ext = "." + strings.TrimPrefix(format, ".")
	}

	hash := sha256.Sum256(data)
	ts := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("img_%s_%s%s", ts, hex.EncodeToString(hash[:6]), ext)

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write image file: %w", err)
	}
	return filename, nil
}

// estimateCost returns a rough cost estimate for a single image generation.
func estimateCost(providerType, model string) float64 {
	switch strings.ToLower(providerType) {
	case "openai":
		switch {
		case strings.Contains(model, "dall-e-3"):
			return 0.04 // standard 1024x1024
		case strings.Contains(model, "dall-e-2"):
			return 0.02
		default:
			return 0.04
		}
	case "openrouter":
		switch {
		case strings.Contains(model, "flux"):
			return 0.03
		case strings.Contains(model, "gpt-5"):
			return 0.05
		default:
			return 0.04
		}
	case "stability":
		return 0.03
	case "ideogram":
		return 0.05
	case "google", "google-imagen":
		return 0.04
	default:
		return 0.04
	}
}

// fileSize returns the file size in bytes, or 0 on error.
func fileSize(filename string) int64 {
	info, err := os.Stat(filename)
	if err != nil {
		return 0
	}
	return info.Size()
}
