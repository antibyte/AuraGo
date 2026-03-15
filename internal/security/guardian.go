package security

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// ThreatLevel indicates the severity of a detected injection attempt.
type ThreatLevel int

const (
	ThreatNone     ThreatLevel = iota
	ThreatLow                  // Suspicious but likely benign
	ThreatMedium               // Pattern matches but could be legitimate
	ThreatHigh                 // Strong injection signature
	ThreatCritical             // High-confidence injection attempt
)

func (t ThreatLevel) String() string {
	switch t {
	case ThreatNone:
		return "none"
	case ThreatLow:
		return "low"
	case ThreatMedium:
		return "medium"
	case ThreatHigh:
		return "high"
	case ThreatCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ScanResult contains the analysis of a text for injection patterns.
type ScanResult struct {
	Level    ThreatLevel
	Patterns []string // matched pattern names
	Message  string   // human-readable summary
}

// injectionPattern holds a compiled regex and metadata for detection.
type injectionPattern struct {
	name  string
	re    *regexp.Regexp
	level ThreatLevel
}

// Guardian provides multi-layer prompt injection defense.
// It scans text for known injection patterns, wraps external data for isolation,
// and strips dangerous role-impersonation markers from tool output.
type Guardian struct {
	logger   *slog.Logger
	patterns []injectionPattern
}

// NewGuardian creates a Guardian with pre-compiled injection detection patterns.
// Patterns cover English, German, and common multilingual injection techniques.
func NewGuardian(logger *slog.Logger) *Guardian {
	g := &Guardian{logger: logger}
	g.compilePatterns()
	return g
}

func (g *Guardian) compilePatterns() {
	raw := []struct {
		name  string
		regex string
		level ThreatLevel
	}{
		// ── Role hijacking / identity override ──────────────────────
		{"role_hijack_en", `(?i)\b(you are now|act as|pretend (to be|you'?re)|from now on you|your new (role|identity|instructions?)|assume the (role|persona|identity))\b`, ThreatCritical},
		{"role_hijack_de", `(?i)\b(du bist (jetzt|nun|ab sofort)|ab jetzt bist du|verhalte dich als|deine neue (rolle|identität|anweisung|aufgabe)|nimm die rolle)\b`, ThreatCritical},

		// ── Instruction override ────────────────────────────────────
		{"override_en", `(?i)\b(ignore (all |your )?(previous|prior|above|earlier|original|system) (instructions?|prompts?|rules?|guidelines?))\b`, ThreatCritical},
		{"override_de", `(?i)\b(ignoriere? (alle |deine )?(vorherigen?|bisherigen?|obigen?|urspr.nglichen?) (anweisungen?|instruktionen?|regeln?|prompts?))\b`, ThreatCritical},
		{"override_new", `(?i)\b(new instructions?|new system prompt|override (system|instructions?)|replace (your|the) (prompt|instructions?))\b`, ThreatHigh},
		{"override_new_de", `(?i)\b(neue (anweisungen?|instruktionen?|system.?prompt)|ersetze? (deine?|die) (anweisungen?|instruktionen?))\b`, ThreatHigh},

		// ── System prompt extraction ────────────────────────────────
		{"extract_prompt_en", `(?i)(show|reveal|print|repeat|output|display|tell me|what (is|are)|give me).{0,20}(system|initial|original|full|complete) (prompt|instructions?|message|rules?)`, ThreatHigh},
		{"extract_prompt_de", `(?i)(zeig|gib|nenn|wiederhole|ausgib).{0,20}(system.?prompt|anweisungen?|instruktionen?|regeln?)`, ThreatHigh},

		// ── Developer/debug mode tricks ─────────────────────────────
		{"devmode", `(?i)\b(enter (developer|debug|admin|maintenance|god|test) mode|enable (dev|debug|admin|sudo|root) mode|DAN mode|jailbreak|bypass (safety|filter|restriction|guard))\b`, ThreatCritical},
		{"devmode_de", `(?i)\b(aktiviere? (entwickler|debug|admin|wartungs|test).?modus|schalte? (sicherheit|filter|schutz|einschr.nkung).{0,5} (ab|aus))\b`, ThreatHigh},

		// ── Delimiter / context escape ──────────────────────────────
		{"delimiter_escape", `(?i)(<<\s*SYS(TEM)?\s*>>|<\|im_start\|>|<\|im_end\|>|\[INST\]|\[\/INST\]|<\|system\|>|<\|user\|>|<\|assistant\|>)`, ThreatCritical},
		{"role_tag_inject", `(?i)(###\s*(system|user|assistant|human|ai)\s*:)`, ThreatHigh},
		{"xml_role_inject", `(?i)(<(system|assistant|user|human|ai)>)`, ThreatMedium},

		// ── Dangerous action coercion ───────────────────────────────
		{"action_coerce", `(?i)\b(execute this (tool|command|code)|call this function|run the following (command|code|script)|you must (run|execute|call))\b`, ThreatMedium},
		{"tool_json_inject", `(?i)\{\s*"(action|tool)"\s*:\s*"(execute_shell|execute_python|set_secret|save_tool|api_request|filesystem)"`, ThreatHigh},

		// ── Encoded / obfuscated payloads ───────────────────────────
		{"base64_payload", `(?i)\b(decode|eval|exec)\s*\(\s*(base64|atob|b64)\b`, ThreatHigh},
		{"unicode_escape", `(?i)(\\u00[0-9a-f]{2}){4,}`, ThreatMedium},

		// ── Application secret extraction ─────────────────────────
		{"aurago_env_read", `(?i)(printenv|echo\s+\$\{?|get-item\s+env:|getenvironmentvariable|\$env:|export\s+.*=.*)\s*AURAGO_`, ThreatCritical},
		{"env_master_key", `(?i)(master.?key|masterkey|vault.?key|AURAGO_MASTER|AURAGO_SECRET)`, ThreatHigh},

		// ── Repetition / flooding (token waste attack) ──────────────
		{"repeat_attack", `(?i)\b(repeat (this|the following|after me) (\d+|a thousand|forever|infinitely) times)\b`, ThreatMedium},
	}

	for _, r := range raw {
		compiled, err := regexp.Compile(r.regex)
		if err != nil {
			if g.logger != nil {
				g.logger.Warn("[Guardian] Failed to compile pattern", "name", r.name, "error", err)
			}
			continue
		}
		g.patterns = append(g.patterns, injectionPattern{
			name:  r.name,
			re:    compiled,
			level: r.level,
		})
	}
}

// ScanForInjection analyzes text for prompt injection patterns.
// Returns a ScanResult with the highest threat level found and all matched patterns.
func (g *Guardian) ScanForInjection(text string) ScanResult {
	if text == "" {
		return ScanResult{Level: ThreatNone}
	}

	result := ScanResult{Level: ThreatNone}
	for _, p := range g.patterns {
		if p.re.MatchString(text) {
			result.Patterns = append(result.Patterns, p.name)
			if p.level > result.Level {
				result.Level = p.level
			}
		}
	}

	if len(result.Patterns) > 0 {
		result.Message = fmt.Sprintf("Detected %d injection pattern(s): %s [threat=%s]",
			len(result.Patterns), strings.Join(result.Patterns, ", "), result.Level)
	}

	return result
}

// ── External Data Isolation ─────────────────────────────────────────────────

// IsolateExternalData wraps content in <external_data> tags for safe LLM ingestion.
// Any existing <external_data> tags in the content are escaped to prevent nesting attacks.
func IsolateExternalData(content string) string {
	if content == "" {
		return ""
	}
	// Escape any existing tags to prevent premature tag closure
	safe := strings.ReplaceAll(content, "</external_data>", "&lt;/external_data&gt;")
	safe = strings.ReplaceAll(safe, "<external_data>", "&lt;external_data&gt;")
	return "<external_data>\n" + safe + "\n</external_data>"
}

// ── Tool Output Sanitization ────────────────────────────────────────────────

// roleMarkers are patterns that could trick the LLM into treating external data
// as a system or user message boundary.
var roleMarkers = regexp.MustCompile(`(?im)^(system|user|assistant|human|ai)\s*:`)

// SanitizeToolOutput processes tool output to prevent injection.
// It strips role impersonation markers and wraps output from external-facing tools in isolation tags.
// External tools: execute_skill, api_request, execute_remote_shell, execute_shell, execute_python, run_tool, filesystem (read_file only).
func (g *Guardian) SanitizeToolOutput(toolName, output string) string {
	if output == "" {
		return output
	}

	// 1. Strip role impersonation markers (e.g. "system:" at line start)
	output = roleMarkers.ReplaceAllStringFunc(output, func(match string) string {
		return "[" + strings.TrimSuffix(match, ":") + "]:"
	})

	// 2. Determine if this tool returns external/untrusted data
	externalTools := map[string]bool{
		"execute_skill":        true,
		"api_request":          true,
		"execute_remote_shell": true,
		"remote_execution":     true,
	}

	// Tools that may contain external data depending on usage
	semiTrustedTools := map[string]bool{
		"execute_shell":  true,
		"execute_python": true,
		"run_tool":       true,
		"filesystem":     true,
	}

	if externalTools[toolName] {
		// Always isolate: these tools inherently return third-party content
		output = IsolateExternalData(output)
	} else if semiTrustedTools[toolName] {
		// Scan for injection patterns — isolate if suspicious
		scan := g.ScanForInjection(output)
		if scan.Level >= ThreatMedium {
			if g.logger != nil {
				g.logger.Warn("[Guardian] Injection patterns in tool output, isolating",
					"tool", toolName, "threat", scan.Level.String(), "patterns", scan.Patterns)
			}
			output = IsolateExternalData(output)
		}
	}

	return output
}

// ScanUserInput analyzes a user message for injection attempts.
// Logs the result but does NOT block — the user is the operator.
// Returns the scan result for upstream decision-making.
func (g *Guardian) ScanUserInput(text string) ScanResult {
	scan := g.ScanForInjection(text)
	if scan.Level >= ThreatMedium && g.logger != nil {
		g.logger.Warn("[Guardian] Suspicious user input detected",
			"threat", scan.Level.String(), "patterns", scan.Patterns, "preview", truncateForLog(text, 200))
	}
	return scan
}

// ScanExternalContent scans content from external sources (web, API, files) for injection.
// Always isolates the content regardless of scan result, but logs threats.
func (g *Guardian) ScanExternalContent(source, content string) string {
	scan := g.ScanForInjection(content)
	if scan.Level >= ThreatLow && g.logger != nil {
		g.logger.Warn("[Guardian] Injection patterns in external content",
			"source", source, "threat", scan.Level.String(), "patterns", scan.Patterns)
	}
	return IsolateExternalData(content)
}

func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
