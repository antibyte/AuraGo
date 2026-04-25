package tools

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	directDestructiveShellPatterns = []shellPolicyPattern{
		{reason: "direct recursive delete command", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*rm\s+(-[^\n\r;|&]*[rf]|-[^\n\r;|&]*[fr]|--recursive)\b`)},
		{reason: "direct recursive delete command", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*(del|erase|rmdir|rd)\b[^\r\n;|&]*\s(/s|/q)\b`)},
		{reason: "direct recursive delete command", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*remove-item\b[^\r\n;|&]*-(recurse|force)\b`)},
		{reason: "direct destructive find/delete command", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*find\b[^\r\n;|&]*-delete\b`)},
		{reason: "disk formatting or partitioning command", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*(format|mkfs(\.[a-z0-9_+-]+)?|fdisk|sfdisk|parted|diskpart)\b`)},
		{reason: "raw disk write command", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*dd\b[^\r\n;|&]*\bof=(/dev/|\\\\\.\\physicaldrive)`)},
		{reason: "secure wipe command", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*shred\b`)},
	}
	privilegeEscalationPatterns = []shellPolicyPattern{
		{reason: "raw privilege escalation wrapper", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*(sudo|su|doas|pkexec|runas)\b`)},
		{reason: "raw privilege escalation wrapper", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*start-process\b[^\r\n;|&]*-verb\s+runas\b`)},
	}
	interpreterBypassPatterns = []shellPolicyPattern{
		{reason: "interpreter command chain that can bypass shell policy", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*(bash|sh|zsh|cmd|powershell|pwsh)\b[^\r\n;|&]*\s(-c|-lc|/c|-command)\b`)},
		{reason: "encoded PowerShell command that can bypass shell policy", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*(powershell|pwsh)\b[^\r\n;|&]*\s(-enc|-encodedcommand)\b`)},
		{reason: "interpreter command chain that can bypass shell policy", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*(python|python3|node|perl|ruby)\b[^\r\n;|&]*\s(-c|-e)\b`)},
		{reason: "interpreter command chain that can bypass shell policy", pattern: regexp.MustCompile(`(?i)\b(os\.system|subprocess\.(run|popen|call)|invoke-expression|iex\b|child_process\.(exec|spawn)|system\()`)},
	}
	downloadExecutePatterns = []shellPolicyPattern{
		{reason: "downloaded script piped to interpreter", pattern: regexp.MustCompile(`(?i)\b(curl|wget|iwr|invoke-webrequest)\b[^\r\n;|&]*\|\s*(sh|bash|zsh|powershell|pwsh|iex|invoke-expression)\b`)},
	}
	sensitiveEnvReadPatterns = []shellPolicyPattern{
		{reason: "environment variable dump could expose secrets", pattern: regexp.MustCompile(`(?i)(^|[;&|\r\n])\s*(get-childitem|dir|ls)\s+env:`)},
		{reason: "environment variable dump could expose secrets", pattern: regexp.MustCompile(`(?i)\[environment\]::getenvironmentvariables\s*\(`)},
	}
	shellControlOperatorPattern = regexp.MustCompile(`(&&|\|\||;|\r|\n)`)
)

type shellPolicyPattern struct {
	reason  string
	pattern *regexp.Regexp
}

// ValidateShellCommandPolicy rejects shell commands that fall into high-risk classes.
func ValidateShellCommandPolicy(command string) error {
	if reason := blockedShellCommandReason(command); reason != "" {
		return fmt.Errorf("command blocked: %s", reason)
	}
	return nil
}

func blockedShellCommandReason(command string) string {
	normalized := strings.TrimSpace(command)
	if normalized == "" {
		return ""
	}

	if reason := matchShellPolicyPatterns(normalized, privilegeEscalationPatterns); reason != "" {
		return reason
	}
	if reason := matchShellPolicyPatterns(normalized, interpreterBypassPatterns); reason != "" {
		return reason
	}
	if reason := matchShellPolicyPatterns(normalized, downloadExecutePatterns); reason != "" {
		return reason
	}
	if reason := matchShellPolicyPatterns(normalized, sensitiveEnvReadPatterns); reason != "" {
		return reason
	}
	if reason := matchShellPolicyPatterns(normalized, directDestructiveShellPatterns); reason != "" {
		if shellControlOperatorPattern.MatchString(normalized) {
			return "destructive command chained through shell control operators"
		}
		return reason
	}
	return ""
}

func matchShellPolicyPatterns(command string, patterns []shellPolicyPattern) string {
	for _, candidate := range patterns {
		if candidate.pattern.MatchString(command) {
			return candidate.reason
		}
	}
	return ""
}
