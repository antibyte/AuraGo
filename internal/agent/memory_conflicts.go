package agent

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"aurago/internal/memory"
)

type conflictSignal struct {
	Key   string
	Value string
}

var conflictSignalPatterns = []struct {
	predicate string
	re        *regexp.Regexp
}{
	{predicate: "preference", re: regexp.MustCompile(`(?i)^(.+?)\s+(?:prefers?|preferred)\s+(.+)$`)},
	{predicate: "usage", re: regexp.MustCompile(`(?i)^(.+?)\s+(?:uses?|used|switched to|switches to)\s+(.+)$`)},
	{predicate: "platform", re: regexp.MustCompile(`(?i)^(.+?)\s+(?:runs on|run on|hosted on|is on)\s+(.+)$`)},
	{predicate: "location", re: regexp.MustCompile(`(?i)^(.+?)\s+(?:located in|lives in|moved to|is in)\s+(.+)$`)},
	{predicate: "language", re: regexp.MustCompile(`(?i)^(.+?)\s+(?:language is|responds in|speaks?)\s+(.+)$`)},
	{predicate: "port", re: regexp.MustCompile(`(?i)^(.+?)\s+(?:runs on port|listens on port|port is|port)\s+([0-9]{2,5})\b`)},
}

func detectMemoryConflictsForDocIDs(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, docIDs []string, fallbackText string) {
	if stm == nil || ltm == nil || len(docIDs) == 0 {
		return
	}
	for _, docID := range docIDs {
		text := strings.TrimSpace(fallbackText)
		if text == "" {
			stored, err := ltm.GetByID(docID)
			if err != nil {
				continue
			}
			text = stored
		}
		signals := deriveConflictSignals(text)
		for _, signal := range signals {
			matches, matchIDs, err := ltm.SearchMemoriesOnly(signal.Key, 8)
			if err != nil {
				continue
			}
			for idx, matchText := range matches {
				if idx >= len(matchIDs) || matchIDs[idx] == "" || matchIDs[idx] == docID {
					continue
				}
				for _, other := range deriveConflictSignals(matchText) {
					if other.Key != signal.Key || other.Value == "" || signal.Value == "" || other.Value == signal.Value {
						continue
					}
					reason := fmt.Sprintf("conflicting values for %s", signal.Key)
					if err := stm.RegisterMemoryConflict(docID, matchIDs[idx], signal.Key, signal.Value, other.Value, reason); err != nil && logger != nil {
						logger.Warn("failed to register memory conflict", "doc_id", docID, "other_doc_id", matchIDs[idx], "error", err)
					}
				}
			}
		}
	}
}

func deriveConflictSignals(text string) []conflictSignal {
	cleaned := normalizeConflictText(text)
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
		signal := conflictSignal{Key: subject + "|" + pattern.predicate, Value: value}
		if _, ok := seen[signal.Key+"|"+signal.Value]; ok {
			continue
		}
		seen[signal.Key+"|"+signal.Value] = struct{}{}
		out = append(out, signal)
	}
	return out
}

func normalizeConflictText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "[")
	if idx := strings.Index(text, "]"); idx >= 0 && idx < len(text)-1 {
		text = text[idx+1:]
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
