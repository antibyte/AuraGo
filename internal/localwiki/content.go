package localwiki

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"mime"
	"strings"
	"unicode/utf8"

	"aurago/internal/zim"
)

// ErrInvalidPath reports a content path that can never name a ZIM entry.
var ErrInvalidPath = errors.New("localwiki: invalid content path")

const (
	// maxContentPathBytes bounds content lookups; real article paths stay far below it.
	maxContentPathBytes = 1024
	// randomAttempts bounds how often Random skips title-list entries that are not articles.
	randomAttempts = 8
)

// randomIndex picks a title-list position; tests replace it.
var randomIndex = rand.IntN

// ContentItem is one blob of the open edition, ready for http.ServeContent.
type ContentItem struct {
	Reader   io.ReadSeeker
	MimeType string
	Size     int64
	ETag     string
	// Path is the content path after redirects. It differs from the requested
	// path when that entry was a redirect; HTTP callers then redirect the browser
	// so relative links inside the article resolve against the target.
	Path string
}

// contentArchive is the part of *zim.Archive that content serving needs.
type contentArchive interface {
	ContentNamespace() byte
	EntryByPath(ns byte, path string) (zim.Entry, error)
	Resolve(e zim.Entry) (zim.Entry, error)
	Open(e zim.Entry) (*io.SectionReader, error)
	UUID() [16]byte
	ArticleCount() int
	ArticleAt(i int) (zim.Entry, error)
	MainEntry() (zim.Entry, error)
}

var _ contentArchive = (*zim.Archive)(nil)

// Content returns the blob stored under path in the archive's content namespace
// (C, or A for legacy archives). Redirects are resolved; lookups never leave the
// content namespace and never touch the filesystem.
func (l *Library) Content(path string) (ContentItem, error) {
	if l == nil || l.archive == nil {
		return ContentItem{}, fmt.Errorf("content: %w", zim.ErrNotFound)
	}
	return archiveContent(l.archive, path)
}

// ContentETag returns the ETag Content sends for path and the path after
// redirects, without opening the blob: no cluster is decompressed, so a
// browser's revalidation can be answered with 304 cheaply.
func (l *Library) ContentETag(path string) (etag, resolved string, err error) {
	if l == nil || l.archive == nil {
		return "", "", fmt.Errorf("content: %w", zim.ErrNotFound)
	}
	return archiveContentETag(l.archive, path)
}

// Random returns a random article of the open edition.
func (l *Library) Random() (Ref, error) {
	if l == nil || l.archive == nil {
		return Ref{}, fmt.Errorf("random article: %w", zim.ErrNotFound)
	}
	return archiveRandom(l.archive, randomIndex)
}

// Main returns the main page of the open edition.
func (l *Library) Main() (Ref, error) {
	if l == nil || l.archive == nil {
		return Ref{}, fmt.Errorf("main page: %w", zim.ErrNotFound)
	}
	return archiveMain(l.archive)
}

// resolveContent finds the entry Content serves for path: redirects
// followed, never leaving the content namespace.
func resolveContent(a contentArchive, path string) (zim.Entry, error) {
	if !validContentPath(path) {
		return zim.Entry{}, ErrInvalidPath
	}
	ns := a.ContentNamespace()
	entry, err := a.EntryByPath(ns, path)
	if err != nil {
		return zim.Entry{}, fmt.Errorf("content %q: %w", path, err)
	}
	resolved, err := a.Resolve(entry)
	if err != nil {
		return zim.Entry{}, fmt.Errorf("content %q: %w", path, err)
	}
	if resolved.Namespace != ns || resolved.IsRedirect {
		return zim.Entry{}, fmt.Errorf("content %q: %w", path, zim.ErrNotFound)
	}
	return resolved, nil
}

func archiveContentETag(a contentArchive, path string) (string, string, error) {
	resolved, err := resolveContent(a, path)
	if err != nil {
		return "", "", err
	}
	return contentETag(a.UUID(), resolved.Path), resolved.Path, nil
}

func archiveContent(a contentArchive, path string) (ContentItem, error) {
	resolved, err := resolveContent(a, path)
	if err != nil {
		return ContentItem{}, err
	}
	section, err := a.Open(resolved)
	if err != nil {
		return ContentItem{}, fmt.Errorf("content %q: %w", path, err)
	}
	return ContentItem{
		Reader:   section,
		MimeType: contentMimeType(resolved.MimeType),
		Size:     section.Size(),
		ETag:     contentETag(a.UUID(), resolved.Path),
		Path:     resolved.Path,
	}, nil
}

func archiveRandom(a contentArchive, pick func(int) int) (Ref, error) {
	n := a.ArticleCount()
	if n <= 0 {
		return Ref{}, fmt.Errorf("random article: %w", zim.ErrNotFound)
	}
	ns := a.ContentNamespace()
	for attempt := 0; attempt < randomAttempts; attempt++ {
		entry, err := a.ArticleAt(pick(n))
		if err != nil {
			return Ref{}, fmt.Errorf("random article: %w", err)
		}
		resolved, err := a.Resolve(entry)
		// A v0 title list may hold deprecated entries (no redirect, no MIME type);
		// they fail the HTML check and are skipped like images.
		if err != nil || resolved.Namespace != ns || resolved.IsRedirect || !isHTMLMime(resolved.MimeType) {
			continue
		}
		return Ref{Title: resolved.Title, Path: resolved.Path}, nil
	}
	return Ref{}, fmt.Errorf("random article: %w", zim.ErrNotFound)
}

func archiveMain(a contentArchive) (Ref, error) {
	entry, err := a.MainEntry()
	if err != nil {
		return Ref{}, fmt.Errorf("main page: %w", err)
	}
	if entry.Namespace != a.ContentNamespace() || entry.IsRedirect {
		return Ref{}, fmt.Errorf("main page: %w", zim.ErrNotFound)
	}
	return Ref{Title: entry.Title, Path: entry.Path}, nil
}

func validContentPath(path string) bool {
	return path != "" && len(path) <= maxContentPathBytes && utf8.ValidString(path) && !strings.ContainsRune(path, 0)
}

// contentETag is a strong validator from the archive UUID and the resolved path;
// the path is hashed so the header stays ASCII and quote-free.
func contentETag(uuid [16]byte, path string) string {
	sum := sha256.Sum256([]byte(path))
	return `"` + hex.EncodeToString(uuid[:]) + "-" + hex.EncodeToString(sum[:16]) + `"`
}

// contentMimeType normalises the ZIM MIME type; text is UTF-8 in every ZIM.
func contentMimeType(raw string) string {
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(raw))
	if err != nil || !strings.Contains(mediaType, "/") || strings.Contains(mediaType, "*") {
		return "application/octet-stream"
	}
	if strings.HasPrefix(mediaType, "text/") && params["charset"] == "" {
		params["charset"] = "utf-8"
	}
	if formatted := mime.FormatMediaType(mediaType, params); formatted != "" {
		return formatted
	}
	return "application/octet-stream"
}

func isHTMLMime(raw string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(raw))
	return err == nil && mediaType == "text/html"
}
