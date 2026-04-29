// Package tools – filetype_detect: magic-byte file-type detection via net/http.
package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/config"
)

// fileTypeEntry represents the detected type of a single file.
type fileTypeEntry struct {
	Path      string `json:"path"`
	MIME      string `json:"mime"`
	Extension string `json:"extension"`
	Group     string `json:"group,omitempty"`
	Error     string `json:"error,omitempty"`
}

// fileTypeResult is the top-level JSON payload returned by DetectFileType.
type fileTypeResult struct {
	Status string          `json:"status"`
	Files  []fileTypeEntry `json:"files"`
	Total  int             `json:"total"`
	Errors int             `json:"errors"`
}

// detectOne reads up to 512 bytes of a file and returns the detected type.
// Uses net/http.DetectContentType for magic-byte sniffing (stdlib, no external dep).
func detectOne(path string) fileTypeEntry {
	entry := fileTypeEntry{Path: path}
	f, err := os.Open(path) // #nosec G304 – path comes from an OS walk, not user HTTP input
	if err != nil {
		entry.Error = fmt.Sprintf("open: %v", err)
		return entry
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		entry.Error = fmt.Sprintf("read: %v", err)
		return entry
	}

	mime := http.DetectContentType(buf[:n])
	// DetectContentType may return params like "text/plain; charset=utf-8" — strip them.
	if idx := strings.Index(mime, ";"); idx >= 0 {
		mime = strings.TrimSpace(mime[:idx])
	}

	entry.MIME = mime
	// Derive extension from file path as fallback (DetectContentType doesn't provide one).
	entry.Extension = strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if idx := strings.Index(mime, "/"); idx >= 0 {
		entry.Group = mime[:idx]
	} else {
		entry.Group = mime
	}
	return entry
}

// DetectFileType identifies the real file type(s) using magic bytes.
//
// path      – path to a file OR a directory
// recursive – if true and path is a directory, walk subdirectories as well
//
// Returns a JSON object containing a "files" array with MIME type, extension,
// and group (image, video, audio, application …) for each entry.
func DetectFileType(path string, recursive bool) string {
	encode := func(r fileTypeResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if path == "" {
		return encode(fileTypeResult{Status: "error"})
	}

	info, err := os.Stat(path)
	if err != nil {
		return encode(fileTypeResult{
			Status: "error",
			Files:  []fileTypeEntry{{Path: path, Error: fmt.Sprintf("stat: %v", err)}},
			Errors: 1,
		})
	}

	var entries []fileTypeEntry

	if !info.IsDir() {
		// Single file
		entries = append(entries, detectOne(path))
	} else {
		// Directory
		walkFn := func(p string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				entries = append(entries, fileTypeEntry{Path: p, Error: fmt.Sprintf("walk: %v", walkErr)})
				return nil
			}
			if d.IsDir() {
				if p != path && !recursive {
					return filepath.SkipDir
				}
				return nil
			}
			entries = append(entries, detectOne(p))
			return nil
		}
		_ = filepath.WalkDir(path, walkFn)
	}

	errorCount := 0
	for _, e := range entries {
		if e.Error != "" {
			errorCount++
		}
	}

	status := "success"
	if errorCount > 0 && errorCount == len(entries) {
		status = "error"
	}

	return encode(fileTypeResult{
		Status: status,
		Files:  entries,
		Total:  len(entries),
		Errors: errorCount,
	})
}

// DetectFileTypeInWorkspace resolves the requested path through the shared tool
// path guard before sniffing file content.
func DetectFileTypeInWorkspace(path string, recursive bool, cfg *config.Config) string {
	encode := func(r fileTypeResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}
	resolved, err := resolveToolPathForRead(path, cfg, true)
	if err != nil {
		return encode(fileTypeResult{
			Status: "error",
			Files:  []fileTypeEntry{{Path: path, Error: err.Error()}},
			Errors: 1,
		})
	}
	return DetectFileType(resolved, recursive)
}
