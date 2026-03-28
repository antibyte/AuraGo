package tools

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

type SmartFileReadResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var smartFileReadSummariseFunc = SummariseContent

const (
	smartFileReadDefaultLineCount = 20
	smartFileReadMaxLineCount     = 120
	smartFileReadDefaultMaxTokens = 2500
	smartFileReadMaxProbeBytes    = 64 * 1024
)

func ExecuteSmartFileRead(ctx context.Context, llmCfg SummaryLLMConfig, logger *slog.Logger, operation, filePath, query, samplingStrategy string, maxTokens, lineCount int, workspaceDir string) string {
	encode := func(r SmartFileReadResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if filePath == "" {
		return encode(SmartFileReadResult{Status: "error", Message: "'file_path' is required"})
	}

	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return encode(SmartFileReadResult{Status: "error", Message: err.Error()})
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return encode(SmartFileReadResult{Status: "error", Message: fmt.Sprintf("Failed to stat file: %v", err)})
	}
	if info.IsDir() {
		return encode(SmartFileReadResult{Status: "error", Message: "smart_file_read only supports files, not directories"})
	}

	probe, err := readProbeBytes(resolved, smartFileReadMaxProbeBytes)
	if err != nil {
		return encode(SmartFileReadResult{Status: "error", Message: fmt.Sprintf("Failed to inspect file: %v", err)})
	}
	detected := detectOne(resolved)
	textLike := smartFileReadIsTextLike(filePath, detected.MIME, probe)
	strategy, strategyNote := normalizeSamplingStrategy(samplingStrategy)
	if lineCount <= 0 {
		lineCount = smartFileReadDefaultLineCount
	}
	if lineCount > smartFileReadMaxLineCount {
		lineCount = smartFileReadMaxLineCount
	}
	if maxTokens <= 0 {
		maxTokens = smartFileReadDefaultMaxTokens
	}

	switch operation {
	case "analyze":
		return encode(buildSmartFileAnalyzeResult(filePath, info, detected, probe, textLike, resolved, strategy))
	case "sample":
		if !textLike {
			return encode(SmartFileReadResult{
				Status:  "error",
				Message: fmt.Sprintf("%s looks like a non-text file (%s). Use detect_file_type or a specialized tool instead.", filePath, detected.MIME),
			})
		}
		sample, meta, err := buildSmartFileSample(resolved, strategy, lineCount)
		if err != nil {
			return encode(SmartFileReadResult{Status: "error", Message: err.Error()})
		}
		msg := fmt.Sprintf("Built %s sample from %s", meta["strategy"], filePath)
		if strategyNote != "" {
			msg += ". " + strategyNote
		}
		return encode(SmartFileReadResult{
			Status:  "success",
			Message: msg,
			Data: map[string]interface{}{
				"path":              filePath,
				"size_bytes":        info.Size(),
				"sampling_strategy": meta["strategy"],
				"sample_sections":   meta["sections"],
				"content":           sample,
				"next_steps": []string{
					"Use file_reader_advanced read_lines/head/tail to zoom into an interesting section.",
					"Use file_reader_advanced search_context to inspect recurring errors or symbols with local context.",
				},
			},
		})
	case "structure":
		return encode(buildSmartFileStructureResult(filePath, info, detected, probe, textLike))
	case "summarize":
		if !textLike {
			return encode(SmartFileReadResult{
				Status:  "error",
				Message: fmt.Sprintf("%s looks like a non-text file (%s). Use a specialized extractor before summarizing.", filePath, detected.MIME),
			})
		}
		summarySource, meta, err := buildSmartFileSummarySource(filePath, resolved, info.Size(), strategy, lineCount, maxTokens)
		if err != nil {
			return encode(SmartFileReadResult{Status: "error", Message: err.Error()})
		}
		if query == "" {
			query = "Summarize the important content, key findings, and suspicious or noteworthy details in this file."
		}
		summaryJSON, err := smartFileReadSummariseFunc(ctx, llmCfg, logger, summarySource, query, "file contents")
		if err != nil {
			fallback := heuristicSmartFileSummary(meta, query)
			return encode(SmartFileReadResult{
				Status:  "success",
				Message: fmt.Sprintf("Summary LLM unavailable, returning heuristic summary for %s", filePath),
				Data: map[string]interface{}{
					"path":              filePath,
					"sampling_strategy": meta["sampling_strategy"],
					"summary":           fallback,
					"summary_source":    meta["summary_source"],
				},
			})
		}

		var envelope struct {
			Status  string `json:"status"`
			Content string `json:"content"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(summaryJSON), &envelope); err != nil {
			return encode(SmartFileReadResult{Status: "error", Message: fmt.Sprintf("Failed to decode summary result: %v", err)})
		}
		if envelope.Status == "error" {
			return encode(SmartFileReadResult{Status: "error", Message: envelope.Message})
		}
		return encode(SmartFileReadResult{
			Status:  "success",
			Message: fmt.Sprintf("Summary generated for %s", filePath),
			Data: map[string]interface{}{
				"path":              filePath,
				"sampling_strategy": meta["sampling_strategy"],
				"summary_source":    meta["summary_source"],
				"content":           envelope.Content,
			},
		})
	default:
		return encode(SmartFileReadResult{Status: "error", Message: fmt.Sprintf("Unknown smart_file_read operation '%s'. Valid: analyze, sample, structure, summarize", operation)})
	}
}

func buildSmartFileAnalyzeResult(filePath string, info os.FileInfo, detected fileTypeEntry, probe []byte, textLike bool, resolved string, strategy string) SmartFileReadResult {
	lineCount := 0
	if textLike {
		lineCount, _ = countFileLinesDetailed(resolved)
	}
	return SmartFileReadResult{
		Status:  "success",
		Message: fmt.Sprintf("Analyzed %s", filePath),
		Data: map[string]interface{}{
			"path":              filePath,
			"size_bytes":        info.Size(),
			"line_count":        lineCount,
			"mime":              detected.MIME,
			"extension":         detected.Extension,
			"group":             detected.Group,
			"is_text_like":      textLike,
			"is_large":          info.Size() > 32*1024,
			"recommended_tool":  smartFileReadRecommendedTool(filePath, detected.MIME, textLike),
			"default_strategy":  strategy,
			"next_steps":        smartFileReadNextSteps(filePath, detected.MIME, textLike),
			"detected_encoding": smartFileReadEncodingHint(probe),
		},
	}
}

func buildSmartFileStructureResult(filePath string, info os.FileInfo, detected fileTypeEntry, probe []byte, textLike bool) SmartFileReadResult {
	structure := map[string]interface{}{
		"path":         filePath,
		"size_bytes":   info.Size(),
		"mime":         detected.MIME,
		"extension":    detected.Extension,
		"is_text_like": textLike,
		"format":       "unknown",
	}

	text := strings.TrimSpace(string(probe))
	lowerExt := strings.ToLower(filepath.Ext(filePath))
	switch {
	case lowerExt == ".json" || strings.HasPrefix(text, "{") || strings.HasPrefix(text, "["):
		structure["format"] = "json"
		structure["root_type"] = smartJSONRootType(text)
		if keys := extractJSONPreviewKeys(text, 20); len(keys) > 0 {
			structure["top_level_keys"] = keys
		}
	case lowerExt == ".xml" || strings.HasPrefix(text, "<"):
		structure["format"] = "xml"
		if root := detectXMLRootElement(probe); root != "" {
			structure["root_element"] = root
		}
	case lowerExt == ".csv" || looksLikeCSV(text):
		structure["format"] = "csv"
		headers, cols := detectCSVStructure(text)
		if len(headers) > 0 {
			structure["headers"] = headers
		}
		structure["column_count"] = cols
	default:
		structure["format"] = "text"
		structure["preview"] = truncateForSmartFile(text, 800)
	}

	return SmartFileReadResult{
		Status:  "success",
		Message: fmt.Sprintf("Detected structure for %s", filePath),
		Data:    structure,
	}
}

func buildSmartFileSummarySource(filePath, resolved string, sizeBytes int64, strategy string, lineCount, maxTokens int) (string, map[string]interface{}, error) {
	maxChars := approxCharsForTokens(maxTokens)
	meta := map[string]interface{}{
		"sampling_strategy": strategy,
	}
	if sizeBytes <= int64(maxChars) {
		data, err := os.ReadFile(resolved)
		if err != nil {
			return "", nil, fmt.Errorf("failed to read file for summary: %w", err)
		}
		text := string(data)
		meta["summary_source"] = "full_file"
		meta["chars_used"] = len(text)
		return fmt.Sprintf("FILE: %s\nMODE: full_file\n\n%s", filePath, truncateForSmartFile(text, maxChars)), meta, nil
	}

	sample, sampleMeta, err := buildSmartFileSample(resolved, strategy, lineCount)
	if err != nil {
		return "", nil, err
	}
	meta["summary_source"] = "sampled_sections"
	meta["sample_sections"] = sampleMeta["sections"]
	meta["chars_used"] = len(sample)
	return fmt.Sprintf(
		"FILE: %s\nMODE: sampled_sections\nSIZE_BYTES: %d\nNOTE: The original file is larger than the summary budget, so this summary is based on representative samples.\n\n%s",
		filePath, sizeBytes, truncateForSmartFile(sample, maxChars),
	), meta, nil
}

func buildSmartFileSample(resolved, strategy string, lineCount int) (string, map[string]interface{}, error) {
	switch strategy {
	case "head":
		lines, err := readHeadLinesDetailed(resolved, lineCount)
		if err != nil {
			return "", nil, err
		}
		return formatSmartFileSection("HEAD SAMPLE", lines), map[string]interface{}{
			"strategy": "head",
			"sections": 1,
		}, nil
	case "tail":
		lines, err := readTailLinesDetailed(resolved, lineCount)
		if err != nil {
			return "", nil, err
		}
		return formatSmartFileSection("TAIL SAMPLE", lines), map[string]interface{}{
			"strategy": "tail",
			"sections": 1,
		}, nil
	default:
		totalLines, err := countFileLinesDetailed(resolved)
		if err != nil {
			return "", nil, err
		}
		head, err := readLineRangeDetailed(resolved, 1, lineCount)
		if err != nil {
			return "", nil, err
		}
		midStart := 1
		if totalLines > lineCount {
			midStart = totalLines/2 - lineCount/2
			if midStart < 1 {
				midStart = 1
			}
		}
		midEnd := midStart + lineCount - 1
		middle, err := readLineRangeDetailed(resolved, midStart, midEnd)
		if err != nil {
			return "", nil, err
		}
		tail, err := readTailLinesDetailed(resolved, lineCount)
		if err != nil {
			return "", nil, err
		}
		content := strings.Join([]string{
			formatSmartFileSection("HEAD SAMPLE", head),
			formatSmartFileSection(fmt.Sprintf("MIDDLE SAMPLE (around line %d)", midStart), middle),
			formatSmartFileSection("TAIL SAMPLE", tail),
		}, "\n\n")
		return content, map[string]interface{}{
			"strategy": "distributed",
			"sections": 3,
		}, nil
	}
}

func smartFileReadRecommendedTool(filePath, mime string, textLike bool) string {
	if !textLike {
		switch strings.ToLower(filepath.Ext(filePath)) {
		case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
			return "analyze_image"
		case ".pdf":
			return "pdf_extractor"
		}
		return "detect_file_type"
	}
	if strings.HasSuffix(strings.ToLower(filePath), ".log") {
		return "file_reader_advanced"
	}
	return "smart_file_read"
}

func smartFileReadNextSteps(filePath, mime string, textLike bool) []string {
	if !textLike {
		switch strings.ToLower(filepath.Ext(filePath)) {
		case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
			return []string{"Use analyze_image for screenshots and image files.", "Use detect_file_type if the extension looks suspicious."}
		case ".pdf":
			return []string{"Use pdf_extractor to read PDF text.", "Use pdf_operations for PDF-specific manipulation."}
		}
		return []string{"Use detect_file_type to verify the real file type.", "Use a specialized tool instead of loading raw binary data."}
	}
	return []string{
		"Use smart_file_read sample to inspect representative sections without dumping the whole file.",
		"Use file_reader_advanced head/tail/read_lines/search_context for precise follow-up reads.",
	}
}

func smartFileReadIsTextLike(filePath, mime string, probe []byte) bool {
	if len(probe) == 0 {
		return true
	}
	if looksLikeBinaryFile(filePath, probe) {
		return false
	}
	if strings.HasPrefix(mime, "text/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".txt", ".md", ".log", ".json", ".yaml", ".yml", ".xml", ".csv", ".tsv", ".ini", ".cfg", ".conf", ".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".html", ".css", ".sql":
		return true
	}
	return utf8.Valid(probe)
}

func smartFileReadEncodingHint(probe []byte) string {
	if utf8.Valid(probe) {
		return "utf-8 or ASCII"
	}
	contentType := http.DetectContentType(probe)
	return contentType
}

func normalizeSamplingStrategy(strategy string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "", "distributed":
		return "distributed", ""
	case "head":
		return "head", ""
	case "tail":
		return "tail", ""
	case "semantic":
		return "distributed", "semantic sampling is not yet implemented; used distributed sampling instead"
	default:
		return "distributed", fmt.Sprintf("unknown sampling_strategy %q; used distributed sampling instead", strategy)
	}
}

func approxCharsForTokens(maxTokens int) int {
	if maxTokens <= 0 {
		maxTokens = smartFileReadDefaultMaxTokens
	}
	if maxTokens < 200 {
		maxTokens = 200
	}
	if maxTokens > 4000 {
		maxTokens = 4000
	}
	return maxTokens * 4
}

func truncateForSmartFile(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n\n[...truncated...]"
}

func readProbeBytes(path string, limit int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, limit)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf[:n], nil
}

func formatSmartFileSection(title string, lines []string) string {
	return fmt.Sprintf("[%s]\n%s", title, strings.Join(lines, "\n"))
}

func countFileLinesDetailed(resolved string) (int, error) {
	f, err := os.Open(resolved)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()
	scanner := newLargeFileScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("failed to scan file: %w", err)
	}
	return count, nil
}

func readHeadLinesDetailed(resolved string, count int) ([]string, error) {
	return readLineRangeDetailed(resolved, 1, count)
}

func readTailLinesDetailed(resolved string, count int) ([]string, error) {
	f, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()
	scanner := newLargeFileScanner(f)
	var allLines []string
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan file: %w", err)
	}
	if count <= 0 || count >= len(allLines) {
		return allLines, nil
	}
	return allLines[len(allLines)-count:], nil
}

func readLineRangeDetailed(resolved string, start, end int) ([]string, error) {
	if start < 1 {
		start = 1
	}
	if end < start {
		end = start
	}
	f, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()
	scanner := newLargeFileScanner(f)
	lineNum := 0
	var lines []string
	for scanner.Scan() {
		lineNum++
		if lineNum >= start && lineNum <= end {
			lines = append(lines, scanner.Text())
		}
		if lineNum > end {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan file: %w", err)
	}
	return lines, nil
}

func smartJSONRootType(text string) string {
	trimmed := strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(trimmed, "{"):
		return "object"
	case strings.HasPrefix(trimmed, "["):
		return "array"
	default:
		return "unknown"
	}
}

func extractJSONPreviewKeys(text string, max int) []string {
	if max <= 0 {
		max = 20
	}
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "{") {
		return nil
	}
	re := regexp.MustCompile(`"([^"\\]+)"\s*:`)
	matches := re.FindAllStringSubmatch(trimmed, max)
	var keys []string
	seen := map[string]bool{}
	for _, m := range matches {
		if len(m) < 2 || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		keys = append(keys, m[1])
	}
	return keys
}

func detectXMLRootElement(data []byte) string {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		tok, err := decoder.Token()
		if err != nil {
			return ""
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start.Name.Local
		}
	}
}

func looksLikeCSV(text string) bool {
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	record, err := reader.Read()
	return err == nil && len(record) >= 2
}

func detectCSVStructure(text string) ([]string, int) {
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	record, err := reader.Read()
	if err != nil {
		return nil, 0
	}
	return record, len(record)
}

func heuristicSmartFileSummary(meta map[string]interface{}, query string) string {
	var b strings.Builder
	b.WriteString("Heuristic file summary.\n")
	if query != "" {
		b.WriteString("Focus: ")
		b.WriteString(query)
		b.WriteString("\n")
	}
	if v, ok := meta["summary_source"].(string); ok && v != "" {
		b.WriteString("Source mode: ")
		b.WriteString(v)
		b.WriteString(". ")
	}
	if v, ok := meta["sampling_strategy"].(string); ok && v != "" {
		b.WriteString("Sampling strategy: ")
		b.WriteString(v)
		b.WriteString(". ")
	}
	b.WriteString("Use smart_file_read sample or file_reader_advanced for more precise follow-up reads.")
	return b.String()
}

func newLargeFileScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return scanner
}
