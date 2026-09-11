package gamemaker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode/utf8"
)

type SourceRead struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	SHA256     string `json:"sha256"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	TotalLines int    `json:"total_lines"`
}

type SourceWrite struct {
	Path    string      `json:"path"`
	Written bool        `json:"written"`
	SHA256  string      `json:"sha256"`
	Build   BuildResult `json:"build"`
}

func sourceHash(content string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(content))) }

// Read windows are bounded; the digest always covers the complete file.
func (s *Service) ReadJobFileRange(ctx context.Context, jobID, path string, start, end int) (SourceRead, error) {
	content, err := s.ReadJobFile(ctx, jobID, path)
	if err != nil {
		return SourceRead{}, err
	}
	if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return SourceRead{}, fmt.Errorf("read source text or JSON metadata, not binary assets")
	}
	lines := strings.Split(content, "\n")
	if start == 0 {
		start = 1
	}
	if end == 0 {
		end = min(start+119, len(lines))
	}
	if start < 1 || end < start || start > len(lines) || end > len(lines) || end-start >= 240 {
		return SourceRead{}, fmt.Errorf("choose 1-based start_line/end_line within 1–%d, at most 240 lines", len(lines))
	}
	selected := strings.Join(lines[start-1:end], "\n")
	if len(selected) > 24000 {
		return SourceRead{}, fmt.Errorf("range exceeds 24000 bytes; choose fewer lines or read source instead of minified vendor files")
	}
	return SourceRead{path, selected, sourceHash(content), start, end, len(lines)}, nil
}

func (s *Service) WriteJobFileChecked(ctx context.Context, jobID, path, content, expected string) (SourceWrite, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if expected != "" {
		old, err := s.ReadJobFile(ctx, jobID, path)
		if err != nil {
			return SourceWrite{}, err
		}
		if sourceHash(old) != expected {
			return SourceWrite{}, fmt.Errorf("source_conflict: file changed; read the affected range again before editing")
		}
	}
	return s.saveSource(ctx, jobID, path, content)
}

// Exact replacement never guesses a location or rewrites multiple matches.
func (s *Service) ReplaceJobFile(ctx context.Context, jobID, path, old, replacement, expected string) (SourceWrite, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return SourceWrite{}, err
	}
	if old == "" || expected == "" {
		return SourceWrite{}, fmt.Errorf("replace requires nonempty old_text and expected_sha256 from read")
	}
	content, err := s.ReadJobFile(ctx, jobID, path)
	if err != nil {
		return SourceWrite{}, err
	}
	if sourceHash(content) != expected {
		return SourceWrite{}, fmt.Errorf("source_conflict: file changed; read the affected range again before editing")
	}
	if n := strings.Count(content, old); n != 1 {
		return SourceWrite{}, fmt.Errorf("old_text matches %d locations; supply one unique exact block", n)
	}
	return s.saveSource(ctx, jobID, path, strings.Replace(content, old, replacement, 1))
}

func (s *Service) saveSource(ctx context.Context, jobID, path, content string) (SourceWrite, error) {
	if !utf8.ValidString(content) {
		return SourceWrite{}, fmt.Errorf("source must be valid UTF-8")
	}
	if err := s.writeJobFile(ctx, jobID, path, content); err != nil {
		return SourceWrite{}, err
	}
	result := SourceWrite{Path: path, Written: true, Build: s.BuildJob(ctx, jobID)}
	// A build may inject the diagnostic prelude into main.ts.
	current, err := s.ReadJobFile(ctx, jobID, path)
	if err != nil {
		return result, err
	}
	result.SHA256 = sourceHash(current)
	result.Build.Diagnostics = result.Build.Diagnostics[:min(12, len(result.Build.Diagnostics))]
	return result, nil
}
