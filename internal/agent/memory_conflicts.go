package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"aurago/internal/memory"
)

type conflictSignal struct {
	Key   string
	Value string
}

type conflictTextFormat uint8

const (
	conflictRawFact conflictTextFormat = iota
	conflictStoredDocument
	conflictSearchResult
)

var conflictSignalPatterns = []struct {
	predicate string
	re        *regexp.Regexp
}{
	{predicate: "preference", re: regexp.MustCompile(`(?i)^(.+?)\s+(?:prefers?|preferred)\s+(.+)$`)},
	{predicate: "primary_platform", re: regexp.MustCompile(`(?i)^(.+?)\s+primary platform is\s+(.+)$`)},
	{predicate: "primary_location", re: regexp.MustCompile(`(?i)^(.+?)\s+primary (?:location|residence) is\s+(.+)$`)},
	{predicate: "language", re: regexp.MustCompile(`(?i)^(.+?)\s+(?:default language is|response language is)\s+(.+)$`)},
	{predicate: "default_port", re: regexp.MustCompile(`(?i)^(.+?)\s+default port is\s+([0-9]{1,5})$`)},
}

func detectMemoryConflictsForDocIDs(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, docIDs []string, fallbackText string) (resultErr error) {
	return detectMemoryConflictsForDocIDsWithContext(context.Background(), logger, stm, ltm, docIDs, fallbackText, conflictRawFact)
}

func detectMemoryConflictsForDocIDsWithContext(ctx context.Context, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, docIDs []string, fallbackText string, format conflictTextFormat) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if stm == nil || ltm == nil || len(docIDs) == 0 {
		return
	}
	for _, docID := range docIDs {
		if err := ctx.Err(); err != nil {
			return errors.Join(resultErr, err)
		}
		// Curation may have archived this document since a caller's metadata snapshot.
		meta, err := stm.GetMemoryMeta(docID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			resultErr = errors.Join(resultErr, fmt.Errorf("read memory conflict metadata: %w", err))
			continue
		}
		if memory.IsMemoryArchived(meta) {
			continue
		}
		text := strings.TrimSpace(fallbackText)
		textFormat := format
		if text == "" {
			stored, err := ltm.GetByID(docID)
			if err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("read memory conflict document: %w", err))
				continue
			}
			text = stored
			textFormat = conflictStoredDocument
		}
		signals := deriveConflictSignals(text, textFormat)
		for _, signal := range signals {
			if err := ctx.Err(); err != nil {
				return errors.Join(resultErr, err)
			}
			ranked, err := searchRankedMemoriesOnly(ctx, ltm, stm, signal.Key, 8, nil, time.Now())
			if err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("search memory conflicts: %w", err))
			}
			for _, match := range ranked {
				if err := ctx.Err(); err != nil {
					return errors.Join(resultErr, err)
				}
				if match.docID == "" || match.docID == docID {
					continue
				}
				for _, other := range deriveConflictSignals(match.text, conflictSearchResult) {
					if other.Key != signal.Key || other.Value == "" || signal.Value == "" || other.Value == signal.Value {
						continue
					}
					reason := fmt.Sprintf("conflicting values for %s", signal.Key)
					if err := stm.RegisterMemoryConflict(docID, match.docID, signal.Key, signal.Value, other.Value, reason); err != nil {
						resultErr = errors.Join(resultErr, err)
						if logger != nil {
							logger.Warn("failed to register memory conflict", "doc_id", docID, "other_doc_id", match.docID, "error", err)
						}
					}
				}
			}
		}
	}
	return errors.Join(resultErr, ctx.Err())
}

func deriveConflictSignals(text string, format conflictTextFormat) []conflictSignal {
	cleaned := normalizeConflictText(text, format)
	if cleaned == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]conflictSignal, 0, 2)
	for _, pattern := range conflictSignalPatterns {
		matches := pattern.re.FindStringSubmatch(cleaned)
		if len(matches) != 3 {
			continue
		}
		subject := canonicalConflictPart(matches[1])
		value := canonicalConflictPart(matches[2])
		if subject == "" || value == "" {
			continue
		}
		predicate := pattern.predicate
		if predicate == "preference" {
			predicate = singletonPreferenceType(value)
			if predicate == "" {
				continue
			}
		}
		signal := conflictSignal{Key: subject + "|" + predicate, Value: value}
		if _, ok := seen[signal.Key+"|"+signal.Value]; ok {
			continue
		}
		seen[signal.Key+"|"+signal.Value] = struct{}{}
		out = append(out, signal)
	}
	return out
}

// Only known single-choice preferences provide evidence of mutual exclusion.
func singletonPreferenceType(value string) string {
	switch value {
	case "german", "english", "french", "spanish", "italian", "japanese", "chinese", "dutch", "polish", "portuguese", "swedish", "danish", "norwegian", "czech", "greek", "hindi":
		return "language"
	case "dark theme", "light theme", "system theme":
		return "theme"
	case "vim", "emacs", "vscode", "vs code", "neovim":
		return "editor"
	}
	return ""
}

func normalizeConflictText(text string, format conflictTextFormat) string {
	text = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"))
	for format == conflictSearchResult && strings.HasPrefix(text, "[") {
		if idx := strings.Index(text, "]"); idx >= 0 && idx < len(text)-1 {
			tag := strings.ToLower(strings.TrimSpace(text[1:idx]))
			if strings.HasPrefix(tag, "similarity:") || strings.HasPrefix(tag, "domain:") ||
				tag == "aurago_memories" || tag == "tool_guides" || tag == "documentation" || tag == "file_index" {
				text = strings.TrimSpace(text[idx+1:])
				continue
			}
		}
		break
	}
	if format != conflictRawFact {
		if concept, _, analysis := memory.AnalysisDocumentParts(text); analysis {
			// A proven provenance body makes its preceding category an envelope.
			text = strings.TrimSpace(concept[strings.Index(concept, "]")+1:])
		} else if _, content, stored := strings.Cut(text, "\n\n"); stored {
			text = strings.TrimSpace(content)
		}
	}
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ")
	text = strings.Trim(text, " .,!?:;")
	return text
}

func canonicalConflictPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, " .,!?:;")
	value = strings.TrimPrefix(value, "the ")
	value = strings.TrimPrefix(value, "a ")
	value = strings.TrimPrefix(value, "an ")
	return value
}
