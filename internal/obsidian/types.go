package obsidian

import "encoding/json"

// ServerStatus represents the Obsidian Local REST API server status.
type ServerStatus struct {
	Online        bool              `json:"online"`
	Status        string            `json:"status"`
	Service       string            `json:"service,omitempty"`
	Authenticated bool              `json:"authenticated"`
	Versions      map[string]string `json:"versions,omitempty"`
}

// NoteJSON represents a note with metadata as returned by the API.
type NoteJSON struct {
	Content     string                 `json:"content"`
	Frontmatter map[string]interface{} `json:"frontmatter,omitempty"`
	Path        string                 `json:"path"`
	Tags        []string               `json:"tags,omitempty"`
	Stat        *FileStat              `json:"stat,omitempty"`
}

// FileStat represents file statistics.
type FileStat struct {
	CTime int64 `json:"ctime"`
	MTime int64 `json:"mtime"`
	Size  int64 `json:"size"`
}

// FileEntry represents a file or directory in the vault listing.
type FileEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "directory"
}

// SearchResult represents a single search result.
type SearchResult struct {
	Filename string          `json:"filename"`
	Result   json.RawMessage `json:"result,omitempty"` // varies by search type
	Score    float64         `json:"score,omitempty"`
	Matches  []SearchMatch   `json:"matches,omitempty"`
}

// SearchMatch represents a match within a search result.
type SearchMatch struct {
	Match   string `json:"match"`
	Context string `json:"context,omitempty"`
}

// SimpleSearchResult represents a result from simple search.
type SimpleSearchResult struct {
	Filename string        `json:"filename"`
	Score    float64       `json:"score,omitempty"`
	Matches  []SimpleMatch `json:"matches,omitempty"`
}

// SimpleMatch represents a match in simple search.
type SimpleMatch struct {
	Match   MatchRange `json:"match"`
	Context string     `json:"context,omitempty"`
}

// MatchRange represents the character range of a match.
type MatchRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Command represents an Obsidian command.
type Command struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Tag represents a tag with usage count.
type Tag struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// DocumentMapEntry represents an entry in a document map.
type DocumentMapEntry struct {
	Type  string `json:"type"` // "heading", "block", "frontmatter-key"
	Key   string `json:"key"`
	Level int    `json:"level,omitempty"` // heading level (1-6)
}
