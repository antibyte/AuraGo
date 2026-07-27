// Command import_training_traces explicitly stages sanitized AuraGo traces for
// human review. It never scans runtime directories and never marks rows as
// approved training data.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"aurago/internal/security"
)

var (
	emailPattern       = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	ipv4Pattern        = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	windowsPathPattern = regexp.MustCompile(`(?i)\b[A-Z]:\\(?:[^\\\s]+\\)*[^\\\s]*`)
	unixHomePattern    = regexp.MustCompile(`/(?:home|Users)/[^/\s]+(?:/[^\s]*)?`)
	hostPattern        = regexp.MustCompile(`(?i)\b(?:host|hostname|server|device)\s*[=:]\s*[A-Z0-9._\-]+`)
)

type cliOptions struct {
	input   string
	staging string
}

type importReport struct {
	SchemaVersion string `json:"schema_version"`
	InputSHA256   string `json:"input_sha256"`
	RowsRead      int    `json:"rows_read"`
	RowsStaged    int    `json:"rows_staged"`
	RowsRejected  int    `json:"rows_rejected"`
	Replacements  int    `json:"replacements"`
	ReviewStatus  string `json:"review_status"`
	GeneratedAt   string `json:"generated_at"`
}

func parseOptions() cliOptions {
	var opts cliOptions
	flag.StringVar(&opts.input, "input", "", "explicit source JSONL file")
	flag.StringVar(&opts.staging, "staging", "", "staging directory for sanitized rows and report")
	flag.Parse()
	return opts
}

func main() {
	opts := parseOptions()
	if opts.input == "" || opts.staging == "" {
		fmt.Fprintln(os.Stderr, "--input and --staging are required")
		os.Exit(2)
	}
	if err := importTraces(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func importTraces(opts cliOptions) error {
	inputPath, err := filepath.Abs(opts.input)
	if err != nil {
		return fmt.Errorf("resolve input path: %w", err)
	}
	stagingPath, err := filepath.Abs(opts.staging)
	if err != nil {
		return fmt.Errorf("resolve staging path: %w", err)
	}
	if inputPath == stagingPath {
		return fmt.Errorf("input file and staging directory must differ")
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read explicit trace input: %w", err)
	}
	if err := os.MkdirAll(stagingPath, 0o700); err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}

	outputPath := filepath.Join(stagingPath, "traces_staged.jsonl")
	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create staged trace file: %w", err)
	}
	writer := bufio.NewWriter(output)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	report := importReport{
		SchemaVersion: "2.0",
		ReviewStatus:  "pending_human_review",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	inputSum := sha256.Sum256(data)
	report.InputSHA256 = hex.EncodeToString(inputSum[:])

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		report.RowsRead++
		var row map[string]interface{}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			report.RowsRejected++
			continue
		}
		if _, ok := row["messages"].([]interface{}); !ok {
			report.RowsRejected++
			continue
		}
		lineSum := sha256.Sum256([]byte(line))
		sanitized, replacements := sanitizeValue(row)
		cleanRow, _ := sanitized.(map[string]interface{})
		cleanRow["schema_version"] = "2.0"
		cleanRow["source"] = "trace_curated"
		cleanRow["review_status"] = "pending"
		cleanRow["trace_import"] = map[string]interface{}{
			"input_line":      report.RowsRead,
			"original_sha256": hex.EncodeToString(lineSum[:]),
		}
		report.Replacements += replacements
		if err := encoder.Encode(cleanRow); err != nil {
			_ = output.Close()
			return fmt.Errorf("write staged row: %w", err)
		}
		report.RowsStaged++
	}
	if err := scanner.Err(); err != nil {
		_ = output.Close()
		return fmt.Errorf("scan trace input: %w", err)
	}
	if err := writer.Flush(); err != nil {
		_ = output.Close()
		return fmt.Errorf("flush staged traces: %w", err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close staged traces: %w", err)
	}
	reportPath := filepath.Join(stagingPath, "trace_import_report.json")
	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	reportJSON = append(reportJSON, '\n')
	if err := os.WriteFile(reportPath, reportJSON, 0o600); err != nil {
		return fmt.Errorf("write trace import report: %w", err)
	}
	fmt.Printf("Staged %d/%d rows for human review in %s\n", report.RowsStaged, report.RowsRead, stagingPath)
	return nil
}

func sanitizeValue(value interface{}) (interface{}, int) {
	switch typed := value.(type) {
	case string:
		return sanitizeText(typed)
	case []interface{}:
		out := make([]interface{}, len(typed))
		replacements := 0
		for i, item := range typed {
			out[i], replacements = mergeSanitized(item, replacements)
		}
		return out, replacements
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		replacements := 0
		for key, item := range typed {
			out[key], replacements = mergeSanitized(item, replacements)
		}
		return out, replacements
	default:
		return value, 0
	}
}

func mergeSanitized(value interface{}, previous int) (interface{}, int) {
	cleaned, count := sanitizeValue(value)
	return cleaned, previous + count
}

func sanitizeText(text string) (string, int) {
	original := text
	text = security.RedactSensitiveInfo(security.Scrub(text))
	replacements := 0
	replace := func(pattern *regexp.Regexp, placeholder string) {
		before := text
		text = pattern.ReplaceAllString(text, placeholder)
		if before != text {
			replacements++
		}
	}
	replace(emailPattern, "[redacted-email]")
	replace(ipv4Pattern, "192.0.2.10")
	replace(windowsPathPattern, "[redacted-path]")
	replace(unixHomePattern, "[redacted-path]")
	replace(hostPattern, "host=[redacted-host]")
	if text != original && replacements == 0 {
		replacements = 1
	}
	return text, replacements
}
