package gamemaker

import (
	"context"
	"fmt"
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

// Search one project file literally. Results and the digest refer to the same read.
func (s *Service) SearchJobFile(ctx context.Context, jobID, path, query string) (SourceSearch, error) {
	if strings.TrimSpace(query) == "" || utf8.RuneCountInString(query) > 120 || strings.ContainsAny(query, "\r\n\x00") {
		return SourceSearch{}, fmt.Errorf("search requires a single-line literal query of 1–120 characters")
	}
	content, err := s.ReadJobFile(ctx, jobID, path)
	if err != nil {
		return SourceSearch{}, err
	}
	if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return SourceSearch{}, fmt.Errorf("search source text or JSON metadata, not binary assets")
	}
	result := SourceSearch{Path: path, SHA256: sourceHash(content), Matches: []SourceMatch{}}
	for i, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, query) {
			continue
		}
		if len(result.Matches) == 12 {
			result.Truncated = true
			break
		}
		runes := []rune(line)
		if len(runes) > 400 {
			// Show the match, including when it occurs late in a long source line.
			start := max(0, utf8.RuneCountInString(line[:strings.Index(line, query)])-80)
			line = string(runes[start:min(start+400, len(runes))])
			result.Truncated = true
		}
		result.Matches = append(result.Matches, SourceMatch{Line: i + 1, Text: line})
	}
	return result, nil
}
