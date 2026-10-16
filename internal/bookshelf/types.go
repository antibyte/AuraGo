package bookshelf

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

const (
	ProviderLocal  = "local"
	ProviderWebDAV = "webdav"
	ProviderSMB    = "smb"
	ProviderKoofr  = "koofr"
)

var (
	ErrNotFound          = errors.New("bookshelf: not found")
	ErrConflict          = errors.New("bookshelf: revision conflict")
	ErrStaleRendition    = errors.New("bookshelf: rendition changed")
	ErrClosed            = errors.New("bookshelf: service closed")
	ErrInvalidPath       = errors.New("bookshelf: invalid relative path")
	ErrUnsupported       = errors.New("bookshelf: unsupported source or format")
	ErrLimitExceeded     = errors.New("bookshelf: configured limit exceeded")
	ErrInsufficientDisk  = errors.New("bookshelf: insufficient free disk space")
	ErrSourceRevoked     = errors.New("bookshelf: source access revoked")
	ErrSourceUnavailable = errors.New("bookshelf: source temporarily unavailable")
)

// MaxRenditionBytes bounds a generated EPUB stored alongside the catalog.
const MaxRenditionBytes int64 = 256 << 20

// MaxSourceBookBytes is the largest original book the service can prepare.
// Servers may lower this at runtime to match their configured upload policy.
const MaxSourceBookBytes int64 = 256 << 20

// Source stores connection coordinates only. Credentials remain in the Vault.
type Source struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Provider       string `json:"provider"`
	MountID        string `json:"mount_id,omitempty"`
	Path           string `json:"path"`
	Recursive      bool   `json:"recursive"`
	Host           string `json:"host,omitempty"`
	Share          string `json:"share,omitempty"`
	Username       string `json:"username,omitempty"`
	Domain         string `json:"domain,omitempty"`
	Generation     int64  `json:"generation"`
	RefreshMinutes int    `json:"refresh_minutes"`
}

type Metadata struct {
	Title       string   `json:"title"`
	Author      string   `json:"author,omitempty"`
	Series      string   `json:"series,omitempty"`
	Description string   `json:"description,omitempty"`
	Format      string   `json:"format"`
	Tags        []string `json:"tags,omitempty"`
}

// MetadataOverride fields are pointers so an empty string can deliberately clear a value.
type MetadataOverride struct {
	Title       *string   `json:"title,omitempty"`
	Author      *string   `json:"author,omitempty"`
	Series      *string   `json:"series,omitempty"`
	Description *string   `json:"description,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
}

type Book struct {
	ID                 string           `json:"id"`
	Title              string           `json:"title"`
	Author             string           `json:"author,omitempty"`
	Series             string           `json:"series,omitempty"`
	Description        string           `json:"description,omitempty"`
	Format             string           `json:"format"`
	Tags               []string         `json:"tags,omitempty"`
	SourceID           string           `json:"source_id"`
	Path               string           `json:"path"`
	Size               int64            `json:"size"`
	ContentRevision    string           `json:"content_revision,omitempty"`
	Revision           int64            `json:"revision"`
	CoverURL           string           `json:"cover_url,omitempty"`
	Available          bool             `json:"available"`
	Collections        []string         `json:"collections,omitempty"`
	Renditions         []Rendition      `json:"renditions,omitempty"`
	CoverFallback      CoverFallback    `json:"cover_fallback"`
	Progress           *Progress        `json:"progress,omitempty"`
	ReadingState       string           `json:"reading_state"`
	ReadingStateManual bool             `json:"reading_state_manual"`
	CachePinned        bool             `json:"cache_pinned"`
	Overrides          MetadataOverride `json:"overrides,omitempty"`
	LastScannedAt      time.Time        `json:"last_scanned_at,omitempty"`
}

type Query struct {
	Q            string `json:"q,omitempty"`
	SourceID     string `json:"source_id,omitempty"`
	CollectionID string `json:"collection_id,omitempty"`
	Sort         string `json:"sort,omitempty"`
	Status       string `json:"status,omitempty"`
	Offset       int    `json:"offset"`
	Limit        int    `json:"limit"`
}

type Page struct {
	Books []Book `json:"books"`
	Total int    `json:"total"`
}

type Progress struct {
	Revision          int64           `json:"revision"`
	RenditionRevision string          `json:"rendition_revision,omitempty"`
	Locator           json.RawMessage `json:"locator"`
	Fraction          float64         `json:"fraction"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// Rendition describes a managed, derived EPUB. It never replaces a source file.
type Rendition struct {
	Format           string    `json:"format"`
	BaseRevision     string    `json:"base_revision"`
	Revision         string    `json:"revision"`
	ConverterVersion string    `json:"converter_version"`
	Size             int64     `json:"size"`
	Available        bool      `json:"available"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CoverFallback contains deterministic, accessible text and palette hints for
// a typographic cover when a book has no embedded or user-selected artwork.
type CoverFallback struct {
	Title        string `json:"title"`
	Author       string `json:"author,omitempty"`
	Monogram     string `json:"monogram"`
	ColorIndex   int    `json:"color_index"`
	PatternIndex int    `json:"pattern_index"`
}

// ExtractedMetadata includes bounded, optional embedded raster cover bytes.
type ExtractedMetadata struct {
	Metadata      Metadata
	EmbeddedCover []byte
	CoverMIME     string
}

type Annotation struct {
	ID                string          `json:"id"`
	BookID            string          `json:"book_id"`
	Kind              string          `json:"kind"`
	Locator           json.RawMessage `json:"locator"`
	Text              string          `json:"text,omitempty"`
	Color             string          `json:"color,omitempty"`
	RenditionRevision string          `json:"rendition_revision"`
	Revision          int64           `json:"revision"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// CacheInfo reports bounded private cache usage without disclosing cache paths.
type CacheInfo struct {
	UsedBytes   int64 `json:"used_bytes"`
	PinnedBytes int64 `json:"pinned_bytes"`
	LimitBytes  int64 `json:"limit_bytes"`
	Books       int   `json:"books"`
	PinnedBooks int   `json:"pinned_books"`
}

type Collection struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	Revision  int64     `json:"revision"`
	BookCount int       `json:"book_count"`
	CreatedAt time.Time `json:"created_at"`
}

type Entry struct {
	Path     string
	Name     string
	Format   string
	Size     int64
	Revision string
	IsDir    bool
	Modified time.Time
}

// SourceAdapter is deliberately read-only; implementations must recheck the
// caller's current source grant before serving each operation. Wrap an
// explicitly temporary network/provider failure with ErrSourceUnavailable to
// permit a same-revision pinned cache fallback. Wrap policy, credential,
// visibility, or permission denial with ErrSourceRevoked instead.
type SourceAdapter interface {
	List(ctx context.Context, source Source, dir string) ([]Entry, error)
	Stat(ctx context.Context, source Source, path string) (Entry, error)
	Open(ctx context.Context, source Source, path string) (io.ReadCloser, error)
}

type Options struct {
	Root            string
	Adapters        map[string]SourceAdapter
	MaxFilesPerScan int
	MaxBookBytes    int64
	MaxCacheBytes   int64
	MinFreeBytes    int64
}

type ScanIssue struct {
	Path string `json:"path,omitempty"`
	Code string `json:"code"`
}

type ScanResult struct {
	SourceID string      `json:"source_id"`
	Seen     int         `json:"seen"`
	Added    int         `json:"added"`
	Updated  int         `json:"updated"`
	Removed  int         `json:"removed"`
	Complete bool        `json:"complete"`
	Issues   []ScanIssue `json:"issues,omitempty"`
}

// Lease owns a prepared cache file until Close. Path is for server-side range
// serving and must never be sent to the browser.
type Lease struct {
	File     *os.File
	Path     string
	Size     int64
	Revision string
	service  *Service
	key      string
	once     sync.Once
}

func (l *Lease) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		if l.File != nil {
			_ = l.File.Close()
		}
		if l.service != nil {
			l.service.releaseLease(l.key)
		}
	})
	return nil
}
