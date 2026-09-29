package gamemaker

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

type SourceMatch struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

type SourceSearch struct {
	Path      string        `json:"path"`
	SHA256    string        `json:"sha256"`
	Matches   []SourceMatch `json:"matches"`
	Truncated bool          `json:"truncated"`
}

// ProjectSearch reports literal matches across the editable text sources.
type ProjectSearch struct {
	Query         string         `json:"query"`
	Files         []SourceSearch `json:"files"`
	SearchedFiles int            `json:"searched_files"`
	Truncated     bool           `json:"truncated"`
}

const (
	maxSearchMatches = 12
	maxSearchedFiles = 64
	maxSearchedBytes = 512 * 1024
)

func validSearchQuery(query string) error {
	if strings.TrimSpace(query) == "" || utf8.RuneCountInString(query) > 120 || strings.ContainsAny(query, "\r\n\x00") {
		return fmt.Errorf("search requires a single-line literal query of 1–120 characters")
	}
	return nil
}

// searchLines appends at most limit matching lines of one text file.
func searchLines(content, query string, limit int) ([]SourceMatch, bool) {
	matches, truncated := []SourceMatch{}, false
	for i, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, query) {
			continue
		}
		if len(matches) >= limit {
			return matches, true
		}
		runes := []rune(line)
		if len(runes) > 400 {
			// Show the match, including when it occurs late in a long source line.
			start := max(0, utf8.RuneCountInString(line[:strings.Index(line, query)])-80)
			line = string(runes[start:min(start+400, len(runes))])
			truncated = true
		}
		matches = append(matches, SourceMatch{Line: i + 1, Text: line})
	}
	return matches, truncated
}

// Search one project file literally. Results and the digest refer to the same read.
func (s *Service) SearchJobFile(ctx context.Context, jobID, path, query string) (SourceSearch, error) {
	if err := validSearchQuery(query); err != nil {
		return SourceSearch{}, err
	}
	content, err := s.ReadJobFile(ctx, jobID, path)
	if err != nil {
		return SourceSearch{}, err
	}
	if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return SourceSearch{}, fmt.Errorf("search source text or JSON metadata, not binary assets")
	}
	result := SourceSearch{Path: path, SHA256: sourceHash(content)}
	result.Matches, result.Truncated = searchLines(content, query, maxSearchMatches)
	return result, nil
}

// SearchJobFiles searches the editable text sources (game.json and src/) when
// the agent does not yet know which file owns a behavior. Managed runtimes,
// compiled output, assets and internal state are never scanned.
func (s *Service) SearchJobFiles(ctx context.Context, jobID, query string) (ProjectSearch, error) {
	if err := validSearchQuery(query); err != nil {
		return ProjectSearch{}, err
	}
	files, err := s.ListJobFiles(ctx, jobID)
	if err != nil {
		return ProjectSearch{}, err
	}
	result := ProjectSearch{Query: query, Files: []SourceSearch{}}
	remaining := maxSearchMatches
	for _, file := range files {
		if file != "game.json" && !strings.HasPrefix(file, "src/") {
			continue
		}
		if !slices.Contains([]string{".ts", ".tsx", ".js", ".mjs", ".json", ".css", ".html", ".txt", ".md"}, strings.ToLower(path.Ext(file))) {
			continue
		}
		if result.SearchedFiles >= maxSearchedFiles || remaining == 0 {
			result.Truncated = true
			break
		}
		content, err := s.ReadJobFile(ctx, jobID, file)
		if err != nil || len(content) > maxSearchedBytes || !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
			continue
		}
		result.SearchedFiles++
		matches, truncated := searchLines(content, query, remaining)
		if len(matches) == 0 {
			continue
		}
		remaining -= len(matches)
		result.Truncated = result.Truncated || truncated
		result.Files = append(result.Files, SourceSearch{Path: file, SHA256: sourceHash(content), Matches: matches, Truncated: truncated})
	}
	return result, nil
}
