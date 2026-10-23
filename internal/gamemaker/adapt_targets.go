package gamemaker

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Adapt mode gives a small model a short list of deterministic anchors in the
// installed starter instead of a free rewrite of src/main.ts. The anchors are
// derived from the real template text that the accepted plan installs, so their
// snippets can be copied verbatim into one exact `replace`.
const (
	// adaptTargetLimit bounds the model-visible anchor list.
	adaptTargetLimit = 6
	// adaptSnippetMaxBytes keeps every anchor snippet inside a bounded edit block.
	adaptSnippetMaxBytes = 400
	// adaptTargetPath is the entry source the plan-bound starter installs.
	adaptTargetPath = "src/main.ts"
)

// CodeModeRequiresReplace is the stable error code for a rejected full write in
// adapt mode. It never consumes a repair pass and never ends the agent round.
const CodeModeRequiresReplace = "mode_requires_replace"

// EditTarget is one deterministic anchor of the installed starter.
type EditTarget struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Snippet   string `json:"snippet"`
	Purpose   string `json:"purpose"`
}

// adaptAnchorSpec names one stable location in a starter. Patterns run against
// one line of the real template text; the first unused matching line wins, so a
// reordered template changes the line numbers but never invents a snippet.
type adaptAnchorSpec struct {
	Purpose string
	Pattern *regexp.Regexp
}

// The order is the priority order of the bounded anchor list: level data, spawn,
// tuning, opposition, planned asset roles, then the entry call. The objective and
// instruction text reaches the game through playerUIOptions in the plan-bound
// src/common.ts, which adapt mode never edits; the player_ui anchor only appears
// when the entry source already carries one (for example after a first edit).
var adaptAnchorSpecs = []adaptAnchorSpec{
	{"stage list: configureLevels ids, titles and order", regexp.MustCompile(`configureLevels\(`)},
	{"player spawn position, body size and role", regexp.MustCompile(`this\.player\s*=\s*this\.(body|add\.rectangle)\(`)},
	{"life and run tuning constants", regexp.MustCompile(`this\.(state\.lives|lives)\s*=`)},
	{"enemy or wave spawn and its tuning", regexp.MustCompile(`this\.(enemy|spawn)\(`)},
	{"stage layout and enemy tuning data", regexp.MustCompile(`^const (STAGES|AREAS|LINES|BOARD)\b`)},
	{"planned asset role binding", regexp.MustCompile(`this\.body\([^)]*["'](item|goal|powerup|hazard|obstacle|enemy|ball|key|cell|block)["']|paintAsset\(`)},
	{"objective and instruction text passed to playerUIOptions", regexp.MustCompile(`playerUIOptions|playerOptions|instructions\s*:`)},
	{"entry call and engine gravity", regexp.MustCompile(`^(start|startGame)\(`)},
}

// AdaptTargets derives the bounded anchors of one installed starter from its
// actual bytes. It is pure: the same content always yields the same targets.
func AdaptTargets(content []byte) []EditTarget {
	if len(content) == 0 {
		return nil
	}
	// Compare and slice LF text; the digest always covers the complete file, so
	// a CRLF checkout still verifies through the same sha256.
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	digest := sourceHash(string(content))
	used := make([]bool, len(lines))
	targets := make([]EditTarget, 0, adaptTargetLimit)
	for _, spec := range adaptAnchorSpecs {
		if len(targets) >= adaptTargetLimit {
			break
		}
		for index, line := range lines {
			if used[index] || line == "" {
				continue
			}
			if !spec.Pattern.MatchString(line) {
				continue
			}
			// A longer line cannot be copied as a bounded block; a later match of
			// the same kind still can.
			if len(line) > adaptSnippetMaxBytes || !utf8.ValidString(line) {
				continue
			}
			used[index] = true
			targets = append(targets, EditTarget{
				Path:      adaptTargetPath,
				SHA256:    digest,
				StartLine: index + 1,
				EndLine:   index + 1,
				Snippet:   line,
				Purpose:   spec.Purpose,
			})
			break
		}
	}
	return targets
}

// AdaptTargetsJSON is the bounded JSON form used by the building context and by
// a refused write answer. Both bounds are structural: at most adaptTargetLimit
// entries with snippets of at most adaptSnippetMaxBytes.
func AdaptTargetsJSON(content []byte) []map[string]any {
	targets := AdaptTargets(content)
	out := make([]map[string]any, 0, len(targets))
	for _, target := range targets {
		out = append(out, map[string]any{
			"path":       target.Path,
			"sha256":     target.SHA256,
			"start_line": target.StartLine,
			"end_line":   target.EndLine,
			"snippet":    target.Snippet,
			"purpose":    target.Purpose,
		})
	}
	return out
}

// AdaptJobTargets returns the anchors of the job's current entry source. A
// missing or unreadable file yields no anchors instead of an error: adapt mode
// then behaves like a rewrite.
func (s *Service) AdaptJobTargets(ctx context.Context, jobID string) []EditTarget {
	content, err := s.ReadJobFile(ctx, jobID, adaptTargetPath)
	if err != nil {
		return nil
	}
	return AdaptTargets([]byte(content))
}

// ModeRequiresReplaceError rejects a full-content write of the plan-bound entry
// source in adapt mode. The targets are the allowed anchors.
type ModeRequiresReplaceError struct {
	Path    string
	Targets []EditTarget
}

func (e *ModeRequiresReplaceError) Error() string {
	return "mode_requires_replace: this adapt-mode job edits the installed starter. Use operation=replace with the exact snippet and expected_sha256 of one edit_targets entry, or replace_many for several non-overlapping snippets, instead of writing " + e.Path + " with complete content."
}

// ModeRequiresReplaceMatches reports the stable adapt-mode write rejection,
// including a wrapped error.
func ModeRequiresReplaceMatches(err error) bool {
	var rejected *ModeRequiresReplaceError
	return errors.As(err, &rejected)
}

// adaptProtectedPath reports the entry source regardless of the separator or a
// leading "./" the model used.
func adaptProtectedPath(rawPath string) bool {
	normalized := strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(rawPath), "\\", "/"), "./")
	return normalized == adaptTargetPath
}

// adaptWriteRejected enforces the adapt-mode edit rule for a complete-content
// write. It stays a plain tool error: the orchestrator's repair budget and the
// agent round are untouched, and replace/replace_many remain available.
func (s *Service) adaptWriteRejected(ctx context.Context, jobID, rawPath string) error {
	if !adaptProtectedPath(rawPath) {
		return nil
	}
	if s.jobBuildingMode(ctx, jobID) != BuildingModeAdapt {
		return nil
	}
	targets := s.AdaptJobTargets(ctx, jobID)
	if len(targets) == 0 {
		// Without a bounded anchor there is nothing to replace; keep the starter
		// writable instead of trapping the run.
		return nil
	}
	return &ModeRequiresReplaceError{Path: adaptTargetPath, Targets: targets}
}
