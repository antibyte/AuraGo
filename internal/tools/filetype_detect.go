// Package tools – filetype_detect: magic-byte file-type detection via h2non/filetype.
package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/h2non/filetype"
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

// detectOne reads up to 261 bytes of a file and returns the detected type.
// 261 bytes is sufficient for all filetype matchers.
func detectOne(path string) fileTypeEntry {
	entry := fileTypeEntry{Path: path}
	f, err := os.Open(path) // #nosec G304 – path comes from an OS walk, not user HTTP input
	if err != nil {
		entry.Error = fmt.Sprintf("open: %v", err)
		return entry
	}
	defer f.Close()

	buf := make([]byte, 261)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		entry.Error = fmt.Sprintf("read: %v", err)
		return entry
	}

	kind, err := filetype.Match(buf[:n])
	if err != nil || kind == filetype.Unknown {
		entry.MIME = "application/octet-stream"
		entry.Extension = strings.TrimPrefix(filepath.Ext(path), ".")
		entry.Group = "unknown"
		return entry
	}

	entry.MIME = kind.MIME.Value
	entry.Extension = kind.Extension
	if idx := strings.Index(kind.MIME.Type, "/"); idx >= 0 {
		entry.Group = kind.MIME.Type[:idx]
	} else {
		entry.Group = kind.MIME.Type
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
