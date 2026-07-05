package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/sandbox"
)

const (
	defaultSkillSpectorCommand        = "skillspector"
	defaultSkillSpectorTimeout        = 60 * time.Second
	defaultSkillSpectorMaxOutputBytes = 512 * 1024
)

// SkillSpectorConfig controls the optional external SkillSpector CLI scan.
type SkillSpectorConfig struct {
	Enabled        bool
	CommandPath    string
	Timeout        time.Duration
	MaxOutputBytes int64
}

// SkillSpectorReport is the AuraGo-normalized subset of SkillSpector JSON output.
type SkillSpectorReport struct {
	Score          float64                `json:"score"`
	Severity       string                 `json:"severity"`
	Recommendation string                 `json:"recommendation"`
	Findings       []SkillSpectorFinding  `json:"findings,omitempty"`
	Version        string                 `json:"version,omitempty"`
	ScanMode       string                 `json:"scan_mode,omitempty"`
	LLMUsed        bool                   `json:"llm_used"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Error          string                 `json:"error,omitempty"`
}

// SkillSpectorFinding is a single normalized SkillSpector issue.
type SkillSpectorFinding struct {
	ID         string  `json:"id,omitempty"`
	Category   string  `json:"category,omitempty"`
	Severity   string  `json:"severity,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	File       string  `json:"file,omitempty"`
	Line       int     `json:"line,omitempty"`
	Message    string  `json:"message,omitempty"`
}

type skillSpectorRawReport struct {
	RiskAssessment struct {
		Score          float64 `json:"score"`
		Severity       string  `json:"severity"`
		Recommendation string  `json:"recommendation"`
	} `json:"risk_assessment"`
	Issues []struct {
		ID         string  `json:"id"`
		Category   string  `json:"category"`
		Severity   string  `json:"severity"`
		Confidence float64 `json:"confidence"`
		Message    string  `json:"message"`
		Location   struct {
			File      string `json:"file"`
			StartLine int    `json:"start_line"`
		} `json:"location"`
	} `json:"issues"`
	Metadata map[string]interface{} `json:"metadata"`
}

func firstSkillSpectorConfig(configs []SkillSpectorConfig) (SkillSpectorConfig, bool) {
	if len(configs) == 0 {
		return SkillSpectorConfig{}, false
	}
	cfg := configs[0]
	if cfg.CommandPath == "" {
		cfg.CommandPath = defaultSkillSpectorCommand
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultSkillSpectorTimeout
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = defaultSkillSpectorMaxOutputBytes
	}
	return cfg, true
}

// RunSkillSpectorScan runs the SkillSpector CLI and parses its JSON report.
func RunSkillSpectorScan(ctx context.Context, target string, cfg SkillSpectorConfig) (*SkillSpectorReport, SecurityStatus, error) {
	cfg, _ = firstSkillSpectorConfig([]SkillSpectorConfig{cfg})
	if strings.TrimSpace(target) == "" {
		err := fmt.Errorf("skillspector target is required")
		return &SkillSpectorReport{Error: err.Error()}, SecurityError, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cfg.CommandPath, "scan", target, "--no-llm", "--format", "json")
	cmd.Env = sandbox.FilterEnv(os.Environ())
	cmd.Dir = target
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		cmd.Dir = filepath.Dir(target)
	}
	SetSkillLimits(cmd, 1024, int(cfg.Timeout.Seconds()))
	stdout := &limitWriter{limit: int(cfg.MaxOutputBytes)}
	stderrLimit := int(cfg.MaxOutputBytes)
	if stderrLimit > 64*1024 {
		stderrLimit = 64 * 1024
	}
	stderr := &limitWriter{limit: stderrLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	output := stdout.buf.Bytes()
	stderrText := strings.TrimSpace(stderr.buf.String())
	if stdout.overflow || stderr.overflow {
		limitErr := fmt.Errorf("skillspector output exceeded %d bytes", cfg.MaxOutputBytes)
		return &SkillSpectorReport{Error: limitErr.Error()}, SecurityError, limitErr
	}
	if ctx.Err() == context.DeadlineExceeded {
		timeoutErr := fmt.Errorf("skillspector scan timed out after %s", cfg.Timeout.Round(time.Second))
		return &SkillSpectorReport{Error: timeoutErr.Error()}, SecurityError, timeoutErr
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() != 1 {
				report := &SkillSpectorReport{Error: fmt.Sprintf("skillspector exited with code %d", exitErr.ExitCode())}
				if stderrText != "" {
					report.Error = fmt.Sprintf("%s: %s", report.Error, stderrText)
				}
				if len(output) > 0 {
					if parsed, _, parseErr := parseSkillSpectorReport(output); parseErr == nil {
						parsed.Error = report.Error
						return parsed, SecurityError, err
					} else {
						report.Error = fmt.Sprintf("%s: %v", report.Error, parseErr)
					}
				}
				return report, SecurityError, err
			}
		} else {
			wrapped := fmt.Errorf("running skillspector: %w", err)
			if stderrText != "" {
				wrapped = fmt.Errorf("%w: %s", wrapped, stderrText)
			}
			return &SkillSpectorReport{Error: wrapped.Error()}, SecurityError, wrapped
		}
	}

	report, status, parseErr := parseSkillSpectorReport(output)
	if parseErr != nil {
		return &SkillSpectorReport{Error: parseErr.Error()}, SecurityError, parseErr
	}
	return report, status, nil
}

func parseSkillSpectorReport(data []byte) (*SkillSpectorReport, SecurityStatus, error) {
	var raw skillSpectorRawReport
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, SecurityError, fmt.Errorf("parsing skillspector JSON: %w", err)
	}
	report := &SkillSpectorReport{
		Score:          raw.RiskAssessment.Score,
		Severity:       strings.ToUpper(strings.TrimSpace(raw.RiskAssessment.Severity)),
		Recommendation: strings.ToUpper(strings.TrimSpace(raw.RiskAssessment.Recommendation)),
		Metadata:       raw.Metadata,
	}
	if report.Metadata == nil {
		report.Metadata = map[string]interface{}{}
	}
	report.Version = metadataString(report.Metadata, "skillspector_version")
	report.ScanMode = metadataString(report.Metadata, "scan_mode")
	report.LLMUsed = false
	for _, issue := range raw.Issues {
		report.Findings = append(report.Findings, SkillSpectorFinding{
			ID:         issue.ID,
			Category:   issue.Category,
			Severity:   issue.Severity,
			Confidence: issue.Confidence,
			File:       issue.Location.File,
			Line:       issue.Location.StartLine,
			Message:    issue.Message,
		})
	}
	return report, skillSpectorStatus(report), nil
}

func skillSpectorStatus(report *SkillSpectorReport) SecurityStatus {
	if report == nil {
		return SecurityError
	}
	if strings.TrimSpace(report.Error) != "" {
		return SecurityError
	}
	switch strings.ToUpper(strings.TrimSpace(report.Recommendation)) {
	case "SAFE":
		return SecurityClean
	case "CAUTION":
		return SecurityWarning
	case "DO_NOT_INSTALL":
		return SecurityDangerous
	}
	switch strings.ToUpper(strings.TrimSpace(report.Severity)) {
	case "LOW":
		return SecurityClean
	case "MEDIUM":
		return SecurityWarning
	case "HIGH", "CRITICAL":
		return SecurityDangerous
	default:
		return SecurityError
	}
}

func metadataString(metadata map[string]interface{}, key string) string {
	if v, ok := metadata[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (m *SkillManager) prepareSkillSpectorBundle(skill *SkillRegistryEntry) (string, func(), error) {
	if m == nil || skill == nil {
		return "", func() {}, fmt.Errorf("skill is required for skillspector scan")
	}
	if err := validateSkillExecutable(skill.Executable); err != nil {
		return "", func() {}, err
	}
	tempDir, err := os.MkdirTemp("", "aurago-skillspector-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("creating skillspector bundle: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	copyRequired := func(src, dstName string) error {
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(tempDir, dstName), data, 0o600)
	}
	execPath := filepath.Join(m.skillsDir, skill.Executable)
	if err := copyRequired(execPath, filepath.Base(skill.Executable)); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("copying skill code for skillspector: %w", err)
	}
	base := strings.TrimSuffix(skill.Executable, filepath.Ext(skill.Executable))
	manifestPath := filepath.Join(m.skillsDir, base+".json")
	if err := copyRequired(manifestPath, filepath.Base(base)+".json"); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("copying skill manifest for skillspector: %w", err)
	}
	if docName := SkillDocumentationFilename(skill.Executable); docName != "" {
		docPath := filepath.Join(m.skillsDir, docName)
		if _, err := os.Stat(docPath); err == nil {
			if err := copyRequired(docPath, filepath.Base(docName)); err != nil {
				cleanup()
				return "", func() {}, fmt.Errorf("copying skill documentation for skillspector: %w", err)
			}
		}
	}
	return tempDir, cleanup, nil
}
