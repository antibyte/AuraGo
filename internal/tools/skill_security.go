package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"aurago/internal/security"
)

// SecurityReport contains the results of all security scans for a skill.
type SecurityReport struct {
	StaticAnalysis   []Finding           `json:"static_analysis,omitempty"`
	VirusTotalScore  float64             `json:"virustotal_score,omitempty"`
	VirusTotalReport string              `json:"virustotal_report,omitempty"`
	GuardianScore    float64             `json:"guardian_score,omitempty"`
	GuardianVerdict  string              `json:"guardian_verdict,omitempty"`
	GuardianReason   string              `json:"guardian_reason,omitempty"`
	SkillSpector     *SkillSpectorReport `json:"skillspector,omitempty"`
	OverallScore     float64             `json:"overall_score"`
	OverallStatus    string              `json:"overall_status"`
	ScannedAt        time.Time           `json:"scanned_at"`
}

// Finding represents a single issue found during static code analysis.
type Finding struct {
	Severity string `json:"severity"` // "info", "warning", "critical"
	Category string `json:"category"` // "exec", "import", "network", "file", "injection"
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
}

// ValidationResult from upload validation checks.
type ValidationResult struct {
	Passed   bool      `json:"passed"`
	Findings []Finding `json:"findings,omitempty"`
	Message  string    `json:"message,omitempty"`
}

// dangerousPattern defines a regex-based detection rule for static analysis.
type dangerousPattern struct {
	Name           string
	Regex          *regexp.Regexp
	Severity       string
	Category       string
	Message        string
	SkipIfContains string // if non-empty, skip match when line contains this substring
}

// Compiled dangerous patterns for static Python code analysis.
var dangerousPatterns = []dangerousPattern{
	{
		Name:     "eval_usage",
		Regex:    regexp.MustCompile(`\beval\s*\(`),
		Severity: "warning",
		Category: "exec",
		Message:  "eval() can execute arbitrary code",
	},
	{
		Name:     "exec_usage",
		Regex:    regexp.MustCompile(`\bexec\s*\(`),
		Severity: "warning",
		Category: "exec",
		Message:  "exec() can execute arbitrary code",
	},
	{
		Name:     "subprocess_shell",
		Regex:    regexp.MustCompile(`subprocess\.\w+.*shell\s*=\s*True`),
		Severity: "critical",
		Category: "exec",
		Message:  "subprocess with shell=True allows shell injection",
	},
	{
		Name:     "os_system",
		Regex:    regexp.MustCompile(`\bos\.system\s*\(`),
		Severity: "warning",
		Category: "exec",
		Message:  "os.system() executes shell commands",
	},
	{
		Name:     "os_popen",
		Regex:    regexp.MustCompile(`\bos\.popen\s*\(`),
		Severity: "warning",
		Category: "exec",
		Message:  "os.popen() executes shell commands",
	},
	{
		Name:     "compile_exec",
		Regex:    regexp.MustCompile(`\bcompile\s*\(.*\bexec\b`),
		Severity: "warning",
		Category: "exec",
		Message:  "compile() with exec mode can execute arbitrary code",
	},
	{
		Name:     "import_ctypes",
		Regex:    regexp.MustCompile(`(?m)^\s*(?:import\s+ctypes|from\s+ctypes\s+import)`),
		Severity: "warning",
		Category: "import",
		Message:  "ctypes allows direct memory manipulation",
	},
	{
		Name:     "import_subprocess",
		Regex:    regexp.MustCompile(`(?m)^\s*import\s+subprocess`),
		Severity: "info",
		Category: "import",
		Message:  "Uses subprocess module for command execution",
	},
	{
		Name:     "import_socket",
		Regex:    regexp.MustCompile(`(?m)^\s*import\s+socket`),
		Severity: "info",
		Category: "network",
		Message:  "Uses raw socket connections",
	},
	{
		Name:     "private_ip_access",
		Regex:    regexp.MustCompile(`(192\.168\.|10\.\d+\.\d+\.\d+|172\.(?:1[6-9]|2\d|3[01])\.)`),
		Severity: "info",
		Category: "network",
		Message:  "References private/internal IP ranges",
	},
	{
		Name:     "file_delete",
		Regex:    regexp.MustCompile(`(?:os\.remove|os\.unlink|shutil\.rmtree)\s*\(`),
		Severity: "warning",
		Category: "file",
		Message:  "Deletes files or directories",
	},
	{
		Name:     "pickle_load",
		Regex:    regexp.MustCompile(`pickle\.loads?\s*\(`),
		Severity: "critical",
		Category: "injection",
		Message:  "pickle.load() can execute arbitrary code during deserialization",
	},
	{
		Name:     "marshal_loads",
		Regex:    regexp.MustCompile(`marshal\.loads?\s*\(`),
		Severity: "critical",
		Category: "injection",
		Message:  "marshal.load() can execute arbitrary code",
	},
	{
		Name:     "getattr_dynamic",
		Regex:    regexp.MustCompile(`__import__\s*\(`),
		Severity: "warning",
		Category: "injection",
		Message:  "__import__() allows dynamic module loading",
	},
	{
		Name:     "base64_decode_exec",
		Regex:    regexp.MustCompile(`base64\.b64decode.*exec|exec.*base64\.b64decode`),
		Severity: "critical",
		Category: "injection",
		Message:  "Possible obfuscated code execution via base64",
	},
	// ── Daemon-specific patterns ──────────────────────────────────────
	{
		Name:     "os_fork",
		Regex:    regexp.MustCompile(`\bos\.fork\s*\(`),
		Severity: "critical",
		Category: "exec",
		Message:  "os.fork() can create fork bombs in long-running daemons",
	},
	{
		Name:     "socket_bind_all",
		Regex:    regexp.MustCompile(`\.bind\s*\(\s*["'(]\s*["']0\.0\.0\.0["']`),
		Severity: "warning",
		Category: "network",
		Message:  "Binding to 0.0.0.0 exposes service to all network interfaces",
	},
	{
		Name:     "import_http_server",
		Regex:    regexp.MustCompile(`(?m)^\s*(?:import\s+http\.server|from\s+http\.server\s+import|import\s+socketserver)`),
		Severity: "warning",
		Category: "network",
		Message:  "Running an HTTP server inside a daemon skill — ensure this is intentional",
	},
	{
		Name:     "signal_sigkill",
		Regex:    regexp.MustCompile(`signal\.(?:SIGKILL|SIGSTOP)`),
		Severity: "warning",
		Category: "exec",
		Message:  "Overriding SIGKILL/SIGSTOP handling can prevent graceful shutdown",
	},
	{
		Name:     "multiprocessing_process",
		Regex:    regexp.MustCompile(`(?m)^\s*(?:from\s+multiprocessing\s+import|import\s+multiprocessing)`),
		Severity: "info",
		Category: "exec",
		Message:  "Multiprocessing in daemon skills may spawn untracked child processes",
	},
	{
		Name:           "requests_no_timeout",
		Regex:          regexp.MustCompile(`requests\.(get|post|put|delete|request|patch)\s*\(`),
		Severity:       "warning",
		Category:       "network",
		Message:        "HTTP request without timeout can hang indefinitely",
		SkipIfContains: "timeout",
	},
}

// StaticCodeAnalysis scans Python source code for dangerous patterns.
func StaticCodeAnalysis(code string) []Finding {
	var findings []Finding
	lines := strings.Split(code, "\n")

	for _, pattern := range dangerousPatterns {
		for lineNum, line := range lines {
			if pattern.Regex.MatchString(line) {
				if shouldSkipDangerousPattern(pattern, line) {
					continue
				}
				findings = append(findings, Finding{
					Severity: pattern.Severity,
					Category: pattern.Category,
					Message:  pattern.Message,
					Line:     lineNum + 1,
					Pattern:  pattern.Name,
				})
			}
		}
	}

	return findings
}

var bashDangerousPatterns = []dangerousPattern{
	{
		Name:     "bash_eval",
		Regex:    regexp.MustCompile(`\beval\s+`),
		Severity: "critical",
		Category: "exec",
		Message:  "eval in shell scripts can execute arbitrary code",
	},
	{
		Name:     "bash_curl_pipe",
		Regex:    regexp.MustCompile(`curl\s.*\|\s*(bash|sh|zsh)`),
		Severity: "critical",
		Category: "exec",
		Message:  "Piping curl output to shell executes remote code",
	},
	{
		Name:     "bash_wget_pipe",
		Regex:    regexp.MustCompile(`wget\s.*\|\s*(bash|sh|zsh)`),
		Severity: "critical",
		Category: "exec",
		Message:  "Piping wget output to shell executes remote code",
	},
	{
		Name:     "bash_rm_rf",
		Regex:    regexp.MustCompile(`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r)\s`),
		Severity: "critical",
		Category: "file",
		Message:  "rm -rf can recursively delete files",
	},
	{
		Name:     "bash_base64_decode_exec",
		Regex:    regexp.MustCompile(`base64\s+(-d|--decode)\s*.*\|\s*(bash|sh)`),
		Severity: "critical",
		Category: "injection",
		Message:  "Base64-decoded content piped to shell — possible obfuscated execution",
	},
	{
		Name:     "bash_nc_listener",
		Regex:    regexp.MustCompile(`nc\s+(-l|-p)\s`),
		Severity: "warning",
		Category: "network",
		Message:  "Netcat listener can expose shell access",
	},
	{
		Name:     "bash_dev_tcp",
		Regex:    regexp.MustCompile(`/dev/tcp/`),
		Severity: "warning",
		Category: "network",
		Message:  "Bash /dev/tcp creates raw network connections",
	},
	{
		Name:     "bash_cron_modify",
		Regex:    regexp.MustCompile(`crontab\s+(-[a-zA-Z]*[eil])`),
		Severity: "warning",
		Category: "exec",
		Message:  "Modifying crontab can schedule persistent tasks",
	},
	{
		Name:     "bash_passwd_access",
		Regex:    regexp.MustCompile(`/etc/(passwd|shadow)`),
		Severity: "warning",
		Category: "file",
		Message:  "Accessing system credential files",
	},
}

var jsDangerousPatterns = []dangerousPattern{
	{
		Name:     "js_eval",
		Regex:    regexp.MustCompile(`\beval\s*\(`),
		Severity: "critical",
		Category: "exec",
		Message:  "eval() can execute arbitrary code",
	},
	{
		Name:     "js_function_constructor",
		Regex:    regexp.MustCompile(`new\s+Function\s*\(`),
		Severity: "critical",
		Category: "exec",
		Message:  "Function constructor can execute arbitrary code",
	},
	{
		Name:     "js_child_process_exec",
		Regex:    regexp.MustCompile(`child_process\.\w*(exec|spawn|execSync|execFile)\s*\(`),
		Severity: "warning",
		Category: "exec",
		Message:  "child_process execution spawns system commands",
	},
	{
		Name:     "js_require_child_process",
		Regex:    regexp.MustCompile(`require\s*\(\s*['"]child_process['"]\s*\)`),
		Severity: "info",
		Category: "import",
		Message:  "Imports child_process module for command execution",
	},
	{
		Name:     "js_require_fs",
		Regex:    regexp.MustCompile(`require\s*\(\s*['"]fs['"]\s*\)`),
		Severity: "info",
		Category: "file",
		Message:  "Imports filesystem module",
	},
	{
		Name:     "js_process_env",
		Regex:    regexp.MustCompile(`process\.env\b`),
		Severity: "info",
		Category: "injection",
		Message:  "Accesses process environment variables",
	},
	{
		Name:     "js_net_connect",
		Regex:    regexp.MustCompile(`(?:require\s*\(\s*['"]net['"]\s*\)|\.connect\s*\()`),
		Severity: "info",
		Category: "network",
		Message:  "Creates network connections",
	},
	{
		Name:     "js_http_request",
		Regex:    regexp.MustCompile(`(?:https?\.get|https?\.request|fetch\s*\()`),
		Severity: "info",
		Category: "network",
		Message:  "Makes HTTP requests",
	},
	{
		Name:     "js_write_file",
		Regex:    regexp.MustCompile(`fs\.\w*(writeFile|writeFileSync|unlink|unlinkSync|rmdir|rm)\s*\(`),
		Severity: "warning",
		Category: "file",
		Message:  "Writes or deletes files on the filesystem",
	},
}

// StaticCodeAnalysisBash scans Bash/shell source code for dangerous patterns.
func StaticCodeAnalysisBash(code string) []Finding {
	return runPatternAnalysis(code, bashDangerousPatterns)
}

// StaticCodeAnalysisJS scans JavaScript source code for dangerous patterns.
func StaticCodeAnalysisJS(code string) []Finding {
	return runPatternAnalysis(code, jsDangerousPatterns)
}

func runPatternAnalysis(code string, patterns []dangerousPattern) []Finding {
	var findings []Finding
	lines := strings.Split(code, "\n")
	for _, pattern := range patterns {
		for lineNum, line := range lines {
			if pattern.Regex.MatchString(line) {
				if shouldSkipDangerousPattern(pattern, line) {
					continue
				}
				findings = append(findings, Finding{
					Severity: pattern.Severity,
					Category: pattern.Category,
					Message:  pattern.Message,
					Line:     lineNum + 1,
					Pattern:  pattern.Name,
				})
			}
		}
	}
	return findings
}

var pythonTimeoutArgRe = regexp.MustCompile(`\btimeout\s*=`)

func shouldSkipDangerousPattern(pattern dangerousPattern, line string) bool {
	if pattern.Name == "requests_no_timeout" {
		return pythonTimeoutArgRe.MatchString(stripPythonLineComment(line))
	}
	return pattern.SkipIfContains != "" && strings.Contains(line, pattern.SkipIfContains)
}

func stripPythonLineComment(line string) string {
	inSingle := false
	inDouble := false
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && (inSingle || inDouble) {
			escaped = true
			continue
		}
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:i]
			}
		}
	}
	return line
}

// ValidateSkillUpload checks an uploaded file for basic validity.
func ValidateSkillUpload(fileData []byte, filename string, maxSizeMB int) *ValidationResult {
	result := &ValidationResult{Passed: true}

	// Check file extension
	if !strings.HasSuffix(strings.ToLower(filename), ".py") {
		result.Passed = false
		result.Findings = append(result.Findings, Finding{
			Severity: "critical",
			Category: "file",
			Message:  "Only .py files are allowed",
		})
		return result
	}

	// Check file size
	maxBytes := maxSizeMB * 1024 * 1024
	if maxBytes <= 0 {
		maxBytes = 1 * 1024 * 1024 // Default 1MB
	}
	if len(fileData) > maxBytes {
		result.Passed = false
		result.Findings = append(result.Findings, Finding{
			Severity: "critical",
			Category: "file",
			Message:  fmt.Sprintf("File too large (max %d MB)", maxSizeMB),
		})
		return result
	}

	// Check for null bytes (binary content)
	if strings.ContainsRune(string(fileData), 0) {
		result.Passed = false
		result.Findings = append(result.Findings, Finding{
			Severity: "critical",
			Category: "file",
			Message:  "File contains binary content",
		})
		return result
	}

	// Run static analysis
	staticFindings := StaticCodeAnalysis(string(fileData))
	result.Findings = append(result.Findings, staticFindings...)

	// Check for critical findings
	for _, f := range staticFindings {
		if f.Severity == "critical" {
			result.Passed = false
		}
	}

	return result
}

// DetermineSecurityStatus computes the overall security status from a report.
func DetermineSecurityStatus(report *SecurityReport) SecurityStatus {
	if report == nil {
		return SecurityPending
	}

	hasCritical := false
	hasWarning := false
	hasError := false

	for _, f := range report.StaticAnalysis {
		switch f.Severity {
		case "critical":
			hasCritical = true
		case "warning":
			hasWarning = true
		}
	}

	// VirusTotal flagged
	if report.VirusTotalScore > 0 {
		hasCritical = true
	}

	// LLM Guardian verdict
	if report.GuardianVerdict == "dangerous" || report.GuardianVerdict == "block" {
		hasCritical = true
	}
	if report.GuardianVerdict == "suspicious" || report.GuardianVerdict == "quarantine" {
		hasWarning = true
	}
	if report.SkillSpector != nil {
		switch strings.ToUpper(strings.TrimSpace(report.SkillSpector.Recommendation)) {
		case "DO_NOT_INSTALL":
			hasCritical = true
		case "CAUTION":
			hasWarning = true
		case "SAFE":
			// no-op
		default:
			switch strings.ToUpper(strings.TrimSpace(report.SkillSpector.Severity)) {
			case "CRITICAL", "HIGH":
				hasCritical = true
			case "MEDIUM":
				hasWarning = true
			}
		}
		if strings.TrimSpace(report.SkillSpector.Error) != "" {
			hasError = true
		}
	}

	if hasError {
		report.OverallStatus = string(SecurityError)
		report.OverallScore = 1.0
		return SecurityError
	}

	if hasCritical {
		report.OverallStatus = string(SecurityDangerous)
		report.OverallScore = 1.0
		return SecurityDangerous
	}
	if hasWarning {
		report.OverallStatus = string(SecurityWarning)
		report.OverallScore = 0.5
		return SecurityWarning
	}

	report.OverallStatus = string(SecurityClean)
	report.OverallScore = 0.0
	return SecurityClean
}

// ScanSkill runs all configured security scans on a skill.
func (m *SkillManager) ScanSkill(ctx context.Context, id string, vtAPIKey string, guardian *security.LLMGuardian, useVT, useGuardian bool, skillSpector ...SkillSpectorConfig) (*SecurityReport, SecurityStatus, error) {
	skill, err := m.GetSkill(id)
	if err != nil {
		return nil, SecurityError, fmt.Errorf("loading skill: %w", err)
	}
	code, err := m.GetSkillCode(id)
	if err != nil {
		return nil, SecurityError, fmt.Errorf("reading skill code: %w", err)
	}

	report := &SecurityReport{
		ScannedAt: time.Now().UTC(),
	}

	// 1. Static analysis (always runs)
	report.StaticAnalysis = StaticCodeAnalysis(code)

	// 2. VirusTotal scan (if enabled and API key configured)
	if useVT && vtAPIKey != "" {
		h := sha256.Sum256([]byte(code))
		fileHash := hex.EncodeToString(h[:])
		vtResult := ExecuteVirusTotalScan(vtAPIKey, fileHash)
		if strings.Contains(strings.ToLower(vtResult), "positives") || strings.Contains(vtResult, "detected") {
			report.VirusTotalScore = 1.0
			report.VirusTotalReport = vtResult
		} else {
			report.VirusTotalReport = vtResult
		}
	}

	// 3. LLM Guardian code review (Further Consideration #1 — opt-in)
	if useGuardian && guardian != nil {
		guardianResult := guardian.EvaluateContent(ctx, "python_skill_upload", code)
		report.GuardianScore = guardianResult.RiskScore
		report.GuardianVerdict = string(guardianResult.Decision)
		report.GuardianReason = guardianResult.Reason
	}

	var scanErr error
	if cfg, ok := firstSkillSpectorConfig(skillSpector); ok && cfg.Enabled {
		target, cleanup, prepErr := m.prepareSkillSpectorBundle(skill)
		if prepErr != nil {
			report.SkillSpector = &SkillSpectorReport{Error: prepErr.Error()}
			scanErr = prepErr
		} else {
			defer cleanup()
			var ssStatus SecurityStatus
			report.SkillSpector, ssStatus, scanErr = RunSkillSpectorScan(ctx, target, cfg)
			if ssStatus == SecurityError && scanErr != nil && report.SkillSpector == nil {
				report.SkillSpector = &SkillSpectorReport{Error: scanErr.Error()}
			}
		}
	}

	// Determine overall status
	status := DetermineSecurityStatus(report)

	// Update in database
	if err := m.UpdateSkillSecurity(id, status, report); err != nil {
		m.logger.Warn("Failed to update skill security status", "id", id, "error", err)
	}

	return report, status, scanErr
}
