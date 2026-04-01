package security

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

const (
	defaultGuardianMaxScanBytes  = 16 * 1024
	defaultGuardianScanEdgeBytes = 6 * 1024
	guardianScanOmittedMark      = "\n[... guardian scan truncated ...]\n"
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

// GuardianOptions controls bounded regex scanning behavior.
type GuardianOptions struct {
	MaxScanBytes  int
	ScanEdgeBytes int
}

// Guardian provides multi-layer prompt injection defense.
// It scans text for known injection patterns, wraps external data for isolation,
// and strips dangerous role-impersonation markers from tool output.
type Guardian struct {
	logger        *slog.Logger
	patterns      []injectionPattern
	maxScanBytes  int
	scanEdgeBytes int
}

// NewGuardian creates a Guardian with pre-compiled injection detection patterns.
// Patterns cover English, German, and common multilingual injection techniques.
func NewGuardian(logger *slog.Logger) *Guardian {
	return NewGuardianWithOptions(logger, GuardianOptions{})
}

// NewGuardianWithOptions creates a Guardian with optional scan window overrides.
func NewGuardianWithOptions(logger *slog.Logger, opts GuardianOptions) *Guardian {
	maxScanBytes := opts.MaxScanBytes
	if maxScanBytes <= 0 {
		maxScanBytes = defaultGuardianMaxScanBytes
	}
	scanEdgeBytes := opts.ScanEdgeBytes
	if scanEdgeBytes <= 0 {
		scanEdgeBytes = defaultGuardianScanEdgeBytes
	}
	if scanEdgeBytes*2 > maxScanBytes {
		scanEdgeBytes = maxScanBytes / 2
	}
	if scanEdgeBytes <= 0 {
		scanEdgeBytes = maxScanBytes
	}

	g := &Guardian{
		logger:        logger,
		maxScanBytes:  maxScanBytes,
		scanEdgeBytes: scanEdgeBytes,
	}
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
		{"role_hijack_cjk", `(?i)(你现在是|从现在开始你是|请扮演|作为.{0,8}(助手|系统|管理员|开发者)|あなたは今|今からあなたは|として振る舞って|지금부터 너는|이제 너는|처럼 행동해)`, ThreatCritical},

		// ── Instruction override ────────────────────────────────────
		{"override_en", `(?i)\b(ignore (all |your )?(previous|prior|above|earlier|original|system) (instructions?|prompts?|rules?|guidelines?))\b`, ThreatCritical},
		{"override_de", `(?i)\b(ignoriere? (alle |deine )?(vorherigen?|bisherigen?|obigen?|urspr.nglichen?) (anweisungen?|instruktionen?|regeln?|prompts?))\b`, ThreatCritical},
		{"override_cjk", `(?i)(忽略.{0,8}(之前|以上|原来|系统).{0,8}(指令|提示|规则)|无视.{0,8}(之前|系统).{0,8}(指示|规则)|前の(指示|命令|プロンプト)を無視|これまでの(指示|ルール)を無視|이전 (지시|명령|규칙)을 무시|시스템 (지침|프롬프트)을 무시)`, ThreatCritical},
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

		// ── Shell with external network tools ─────────────────────
		// curl/wget/etc. to public internet via shell is unsafe; web_scraper must be used instead.
		{"curl_external", `(?i)\b(curl|wget|fetch)\s+.*https?://`, ThreatHigh},
		{"powershell_web", `(?i)(Invoke-WebRequest|Invoke-RestMethod|iwr|irm)\s+.*https?://`, ThreatHigh},

		// ── Encoded / obfuscated payloads ───────────────────────────
		{"base64_payload", `(?i)\b(decode|eval|exec)\s*\(\s*(base64|atob|b64)\b`, ThreatHigh},
		{"base64_literal_payload", `(?i)\b(base64_decode|frombase64string|atob)\s*\(\s*["'][A-Za-z0-9+/=]{16,}["']\s*\)`, ThreatHigh},
		{"hex_literal_payload", `(?i)\b(hex\.decodestring|fromhex|unhexlify|decode_hex|hexdecode|xxd\s+-r\s+-p)\b.{0,24}["']?[A-Fa-f0-9]{16,}["']?`, ThreatHigh},
		{"rot13_literal_payload", `(?i)(\b(rot13|caesar)\b.{0,20}\b(decode|decoded|decoder|transform|translated?)\b)|(\bcodecs\.decode\b.{0,80}["']rot_13["'])|(\brot_13\b.{0,80}["'][A-Za-z\s]{12,}["'])`, ThreatMedium},
		{"charcode_literal_payload", `(?i)\b(fromcharcode|charcodeat|string\.fromcharcode|chr\()\b.{0,40}(?:\d{2,3}\s*,\s*){4,}\d{2,3}`, ThreatHigh},
		{"morse_literal_payload", `(?i)\b(morse(\.|_)?decode|decode(\.|_)?morse|translate.{0,10}morse|decode\s+morse)\b.{0,32}["']?[.\-/\s]{12,}["']?`, ThreatMedium},
		{"uu_literal_payload", `(?i)\b(uudecode|uu\.decode|decode\s+uu(?:encode|encoded)?)\b.{0,80}(begin\s+[0-7]{3}\s+\S+|[ !-` + "`" + `]{20,})`, ThreatMedium},
		{"unicode_escape", `(?i)(\\u00[0-9a-f]{2}){4,}`, ThreatMedium},
		{"html_entity_injection", `(?i)(&#x?[0-9a-f]+;|&(lt|gt|amp|quot|apos);){3,}`, ThreatMedium},
		{"zero_width_injection", `[\x{200B}\x{200C}\x{200D}\x{FEFF}\x{2060}]+`, ThreatMedium},
		{"markdown_js_link", `(?i)\[[^\]]{0,120}\]\(\s*javascript\s*:`, ThreatHigh},
		{"markdown_injection_heading", `(?im)^\s{0,3}#{1,6}\s*(ignore|system prompt|developer mode|new instructions?|override|jailbreak|bypass)\b`, ThreatMedium},

		// ── Application secret extraction ─────────────────────────
		{"aurago_env_read", `(?i)(printenv|echo\s+\$\{?|get-item\s+env:|getenvironmentvariable|\$env:|export\s+.*=.*)\s*AURAGO_`, ThreatCritical},
		{"env_master_key", `(?i)(master.?key|masterkey|vault.?key|AURAGO_MASTER|AURAGO_SECRET)`, ThreatHigh},

		// ── Repetition / flooding (token waste attack) ──────────────
		{"repeat_attack", `(?i)\b(repeat (this|the following|after me) (\d+|a thousand|forever|infinitely) times)\b`, ThreatMedium},

		// ── Credential exfiltration via encoding / transformation ──
		// Detect requests to convert/encode credential-related variables to numbers, ASCII, hex, etc.
		{"cred_encode_en", `(?i)(password|username|credential|secret|token|api.?key).{0,80}(\bord\b|ascii|ordinal|convert.{0,20}(number|integer|digit)|as.{0,5}numbers?|to.{0,5}(numbers?|digit)|int\(.*char|chr\()`, ThreatHigh},
		{"cred_encode_de", `(?i)(passwort|benutzername|zugangsdaten|anmeldedaten|geheimnis|token).{0,80}(\bord\b|ascii|ordinal|in.{0,5}zahlen?|als.{0,5}zahlen?|umwandeln|konvertieren|umgewandelt|in.{0,5}ziffern?)`, ThreatHigh},
		// Detect false claim that security masked data, combined with credential-related requests
		{"false_mask_bypass_de", `(?i)(sicherheits?.?(system|guard|filter|schutz)|guardian|filter|maskier).{0,80}(maskiert|verborgen|gesperrt|geblockt|zensiert|versteckt).{0,120}(username|passwort|zugangsdaten|variable|credential)`, ThreatHigh},
		{"false_mask_bypass_en", `(?i)(security.?(system|guard|filter)|guardian|filter).{0,80}(masked|hidden|blocked|censored|redacted).{0,120}(username|password|credentials?|variable|token)`, ThreatHigh},
		// Detect hardcoded credential-like test variables in code (medium — could be legitimately in test code)
		{"cred_hardcode_in_code", `(?i)(TEST_USERNAME|TEST_PASSWORD|test.?user(name)?|test.?pass(word)?)\s*=\s*['"][^'"]{3,}`, ThreatMedium},
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

	scanText, truncated := prepareGuardianScanText(text, g.maxScanBytes, g.scanEdgeBytes)
	result := ScanResult{Level: ThreatNone}
	for _, p := range g.patterns {
		if p.re.MatchString(scanText) {
			result.Patterns = append(result.Patterns, p.name)
			if p.level > result.Level {
				result.Level = p.level
			}
		}
	}

	if len(result.Patterns) > 0 {
		result.Message = fmt.Sprintf("Detected %d injection pattern(s): %s [threat=%s]",
			len(result.Patterns), strings.Join(result.Patterns, ", "), result.Level)
		if truncated {
			result.Message += " [scan_window=truncated]"
		}
	} else if truncated {
		result.Message = "No injection patterns detected in truncated scan window"
	}

	return result
}

func prepareGuardianScanText(text string, maxScanBytes, scanEdgeBytes int) (string, bool) {
	if maxScanBytes <= 0 {
		maxScanBytes = defaultGuardianMaxScanBytes
	}
	if scanEdgeBytes <= 0 {
		scanEdgeBytes = defaultGuardianScanEdgeBytes
	}
	if len(text) <= maxScanBytes {
		return text, false
	}
	if scanEdgeBytes*2 > maxScanBytes {
		scanEdgeBytes = maxScanBytes / 2
	}
	if scanEdgeBytes <= 0 {
		scanEdgeBytes = maxScanBytes
	}
	head := text[:scanEdgeBytes]
	tail := text[len(text)-scanEdgeBytes:]
	return head + guardianScanOmittedMark + tail, true
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
		// Core execution tools that return third-party output
		"execute_skill":        true,
		"api_request":          true,
		"execute_remote_shell": true,
		"remote_execution":     true,
		// Communication — email and messaging content is external data
		"email":         true,
		"fetch_email":   true,
		"check_email":   true,
		"discord":       true,
		"fetch_discord": true,
		// Network/web content
		"web_scraper":       true,
		"fetch_url":         true,
		"call_webhook":      true,
		"mqtt_get_messages": true,
		// External integrations — return data from third-party systems
		"fritzbox":          true,
		"mcp_call":          true,
		"sql_query":         true,
		"docker":            true,
		"meshcentral":       true,
		"proxmox":           true,
		"proxmox_ve":        true,
		"github":            true,
		"netlify":           true,
		"home_assistant":    true,
		"tailscale":         true,
		"webdav":            true,
		"webdav_storage":    true,
		"s3_storage":        true,
		"s3":                true,
		"paperless":         true,
		"paperless_ngx":     true,
		"adguard":           true,
		"adguard_home":      true,
		"truenas":           true,
		"co_agent":          true,
		"co_agents":         true,
		"ansible":           true,
		"jellyfin":          true,
		"cloudflare_tunnel": true,
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
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
