package outputcompress

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type substitutionCandidate struct {
	phrase  string
	count   int
	savings int
}

func normalizeRepetitiveSubstitutionConfig(cfg RepetitiveSubstitutionConfig) RepetitiveSubstitutionConfig {
	if cfg.MinPhraseChars <= 0 {
		cfg.MinPhraseChars = 15
	}
	if cfg.MinOccurrences <= 0 {
		cfg.MinOccurrences = 3
	}
	if cfg.MinSavingsPercent <= 0 {
		cfg.MinSavingsPercent = 15
	}
	if cfg.MaxInputChars <= 0 {
		cfg.MaxInputChars = 50000
	}
	if cfg.MaxDictionaryEntries <= 0 {
		cfg.MaxDictionaryEntries = 16
	}
	return cfg
}

func compressRepetitiveSubstitution(toolName, command, rawOutput, current string, cfg RepetitiveSubstitutionConfig) (string, bool) {
	cfg = normalizeRepetitiveSubstitutionConfig(cfg)
	if !cfg.Enabled || !cfg.LZWEnabled {
		return "", false
	}
	if len(current) == 0 || len(current) > cfg.MaxInputChars {
		return "", false
	}
	if isErrorOutput(rawOutput) || !isRepetitiveSubstitutionEligible(toolName, command, rawOutput, current) {
		return "", false
	}
	return applyRepetitiveSubstitution(current, cfg)
}

func isRepetitiveSubstitutionEligible(toolName, command, rawOutput, current string) bool {
	if isExactCopySensitiveTool(toolName) {
		return false
	}
	lowerCommand := strings.ToLower(strings.TrimSpace(command))
	lowerFilterText := strings.ToLower(rawOutput + "\n" + current)
	if strings.Contains(lowerFilterText, "<tool_call") || strings.Contains(lowerFilterText, "\"tool_calls\"") {
		return false
	}
	if isDiffCommand(lowerCommand) || looksLikeDiff(current) {
		return false
	}
	if looksLikeStructuredDocument(current) || looksLikeSourceCode(current) {
		return false
	}
	if toolName == "read_process_logs" {
		return true
	}
	if !isShellTool(toolName) {
		return false
	}
	return isLogCommand(lowerCommand) || isLogContent(current)
}

func isExactCopySensitiveTool(toolName string) bool {
	switch toolName {
	case "file_reader_advanced", "smart_file_read", "filesystem", "filesystem_op", "file_editor":
		return true
	default:
		return false
	}
}

func isDiffCommand(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	if fields[0] == "diff" {
		return true
	}
	return fields[0] == "git" && len(fields) > 1 && fields[1] == "diff"
}

func isLogCommand(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	if fields[0] == "journalctl" || fields[0] == "logcli" || strings.HasSuffix(fields[0], "log") {
		return true
	}
	if len(fields) > 1 && (fields[1] == "logs" || fields[1] == "log") {
		return true
	}
	if fields[0] == "tail" || fields[0] == "head" {
		return strings.Contains(command, ".log") || strings.Contains(command, "logs/")
	}
	return strings.Contains(command, " logs ")
}

func looksLikeDiff(input string) bool {
	return strings.Contains(input, "diff --git ") ||
		strings.Contains(input, "\n@@ -") ||
		strings.Contains(input, "\n--- ") && strings.Contains(input, "\n+++ ")
}

func looksLikeStructuredDocument(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return false
	}
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		return true
	}
	lines := strings.Split(trimmed, "\n")
	sample := len(lines)
	if sample > 20 {
		sample = 20
	}
	yamlish := 0
	for i := 0; i < sample; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, ": ") && !strings.Contains(line, " level=") && !strings.Contains(line, " msg=") {
			yamlish++
		}
	}
	return sample > 0 && yamlish > sample/2
}

func looksLikeSourceCode(input string) bool {
	lines := strings.Split(input, "\n")
	sample := len(lines)
	if sample > 80 {
		sample = 80
	}
	score := 0
	for i := 0; i < sample; i++ {
		line := strings.TrimSpace(lines[i])
		switch {
		case strings.HasPrefix(line, "package ") || strings.HasPrefix(line, "import "):
			score += 2
		case strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "type ") || strings.HasPrefix(line, "class "):
			score += 2
		case strings.HasPrefix(line, "const ") || strings.HasPrefix(line, "var ") || strings.HasPrefix(line, "let "):
			score++
		case strings.Contains(line, " := ") || strings.Contains(line, "=>") || strings.HasSuffix(line, "{"):
			score++
		}
	}
	return score >= 3
}

func applyRepetitiveSubstitution(input string, cfg RepetitiveSubstitutionConfig) (string, bool) {
	candidates := collectSubstitutionCandidates(input, cfg)
	if len(candidates) == 0 {
		return "", false
	}
	prefix := chooseSubstitutionMarkerPrefix(input, cfg.MaxDictionaryEntries)
	if prefix == "" {
		return "", false
	}

	result := input
	type entry struct {
		marker string
		phrase string
	}
	entries := make([]entry, 0, cfg.MaxDictionaryEntries)
	for _, cand := range candidates {
		if len(entries) >= cfg.MaxDictionaryEntries {
			break
		}
		count := strings.Count(result, cand.phrase)
		if count < cfg.MinOccurrences {
			continue
		}
		marker := fmt.Sprintf("%s%d@@", prefix, len(entries)+1)
		dictCost := len(marker) + len(cand.phrase) + 6
		if count*len(cand.phrase)-count*len(marker)-dictCost <= 0 {
			continue
		}
		result = strings.ReplaceAll(result, cand.phrase, marker)
		entries = append(entries, entry{marker: marker, phrase: cand.phrase})
	}
	if len(entries) == 0 {
		return "", false
	}

	var b strings.Builder
	b.WriteString("[repetitive-substitutions]\n")
	for _, item := range entries {
		fmt.Fprintf(&b, "%s = %q\n", item.marker, item.phrase)
	}
	b.WriteString("[/repetitive-substitutions]\n")
	b.WriteString(result)
	output := b.String()
	if !meetsSavingsThreshold(input, output, cfg.MinSavingsPercent) {
		return "", false
	}
	return output, true
}

func collectSubstitutionCandidates(input string, cfg RepetitiveSubstitutionConfig) []substitutionCandidate {
	counts := make(map[string]int)
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < cfg.MinPhraseChars {
			continue
		}
		addCandidatePhrase(counts, line, cfg.MinPhraseChars)
		fields := strings.Fields(line)
		for start := range fields {
			maxEnd := start + 8
			if maxEnd > len(fields) {
				maxEnd = len(fields)
			}
			for end := start + 3; end <= maxEnd; end++ {
				phrase := strings.Join(fields[start:end], " ")
				addCandidatePhrase(counts, phrase, cfg.MinPhraseChars)
			}
		}
	}

	candidates := make([]substitutionCandidate, 0)
	for phrase := range counts {
		actualCount := strings.Count(input, phrase)
		if actualCount < cfg.MinOccurrences {
			continue
		}
		markerLen := len("@@OTK1_99@@")
		savings := actualCount*len(phrase) - actualCount*markerLen - len(phrase)
		if savings <= 0 {
			continue
		}
		candidates = append(candidates, substitutionCandidate{phrase: phrase, count: actualCount, savings: savings})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].savings != candidates[j].savings {
			return candidates[i].savings > candidates[j].savings
		}
		return len(candidates[i].phrase) > len(candidates[j].phrase)
	})
	return candidates
}

func addCandidatePhrase(counts map[string]int, phrase string, minLen int) {
	phrase = strings.TrimSpace(phrase)
	if len(phrase) < minLen || len(phrase) > 180 {
		return
	}
	if strings.Contains(phrase, "@@OTK") || !containsLetter(phrase) {
		return
	}
	counts[phrase]++
}

func containsLetter(input string) bool {
	for _, r := range input {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func chooseSubstitutionMarkerPrefix(input string, maxEntries int) string {
	if maxEntries <= 0 {
		maxEntries = 16
	}
	for salt := 1; salt <= 99; salt++ {
		prefix := fmt.Sprintf("@@OTK%d_", salt)
		collides := false
		for i := 1; i <= maxEntries; i++ {
			if strings.Contains(input, fmt.Sprintf("%s%d@@", prefix, i)) {
				collides = true
				break
			}
		}
		if !collides {
			return prefix
		}
	}
	return ""
}

func meetsSavingsThreshold(input, output string, percent int) bool {
	if len(input) == 0 || len(output) >= len(input) {
		return false
	}
	if percent <= 0 {
		percent = 1
	}
	return (len(input)-len(output))*100 >= len(input)*percent
}
