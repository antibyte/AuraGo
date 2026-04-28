package security

import (
	"html"
	"log/slog"
	"regexp"
	"strings"

	"github.com/danielthedm/promptsec"
)

const (
	defaultGuardianMaxScanBytes  = 16 * 1024
	defaultGuardianScanEdgeBytes = 6 * 1024
	guardianScanOmittedMark      = "\n[... guardian scan truncated ...]\n"
	missionAdvisoryStartMarker   = "<!-- aurago:mission-advisory:v1:start -->"
	missionAdvisoryEndMarker     = "<!-- aurago:mission-advisory:v1:end -->"
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

// GuardianOptions controls bounded regex scanning behavior.
type GuardianOptions struct {
	MaxScanBytes  int
	ScanEdgeBytes int
	Preset        string
	Spotlight     bool
	Canary        bool
}

// Guardian provides multi-layer prompt injection defense.
// It scans text for known injection patterns, wraps external data for isolation,
// and strips dangerous role-impersonation markers from tool output.
type Guardian struct {
	logger        *slog.Logger
	maxScanBytes  int
	scanEdgeBytes int
	protector     *promptsec.Protector
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

	preset := promptsec.PresetStrict
	switch strings.ToLower(opts.Preset) {
	case "moderate":
		preset = promptsec.PresetModerate
	case "lenient":
		preset = promptsec.PresetLenient
	}

	psOpts := []promptsec.Guard{
		promptsec.WithHeuristics(&promptsec.HeuristicOptions{Preset: preset}),
		promptsec.WithOutputValidator(&promptsec.OutputOptions{}),
	}

	if opts.Spotlight {
		psOpts = append(psOpts, promptsec.WithSpotlighting(promptsec.Datamark, &promptsec.DatamarkOptions{Token: "^"}))
	}
	if opts.Canary {
		psOpts = append(psOpts, promptsec.WithCanary(&promptsec.CanaryOptions{Format: promptsec.CanaryHex, Length: 16}))
	}

	g := &Guardian{
		logger:        logger,
		maxScanBytes:  maxScanBytes,
		scanEdgeBytes: scanEdgeBytes,
		protector:     promptsec.New(psOpts...),
	}

	return g
}

// ScanForInjection analyzes text for prompt injection patterns.
// Returns a ScanResult with the highest threat level found and all matched patterns.
func (g *Guardian) ScanForInjection(text string) ScanResult {
	if text == "" {
		return ScanResult{Level: ThreatNone}
	}

	scanText, truncated := prepareGuardianScanText(text, g.maxScanBytes, g.scanEdgeBytes)

	analysis := g.protector.Analyze(scanText)
	result := ScanResult{Level: ThreatNone}

	if !analysis.Safe || len(analysis.Threats) > 0 {
		var msgs []string
		for _, thr := range analysis.Threats {
			// Find max ThreatLevel effectively
			lvl := ThreatLow
			if thr.Severity >= 0.8 {
				lvl = ThreatCritical
			} else if thr.Severity >= 0.5 {
				lvl = ThreatHigh
			} else if thr.Severity >= 0.3 {
				lvl = ThreatMedium
			}

			if lvl > result.Level {
				result.Level = lvl
			}

			result.Patterns = append(result.Patterns, string(thr.Type))
			msgs = append(msgs, thr.Message)
		}

		// If promptsec marks unsafe, ensure at least ThreatMedium
		if !analysis.Safe && result.Level < ThreatMedium {
			result.Level = ThreatMedium
		}

		result.Message = strings.Join(msgs, "; ")
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
// All HTML special characters in the content are escaped so that no nested or
// pre-encoded tags can break out of the isolation boundary.  This prevents
// double-encoding bypass attacks where pre-encoded entities like
// &lt;/external_data&gt; would pass through a partial escaper unchanged and
// potentially be decoded by the downstream LLM.
func IsolateExternalData(content string) string {
	if content == "" {
		return ""
	}
	safe := html.EscapeString(content)
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
		"google_workspace":  true,
		"gworkspace":        true,
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
		"yepapi_seo":        true,
		"yepapi_serp":       true,
		"yepapi_scrape":     true,
		"yepapi_youtube":    true,
		"yepapi_tiktok":     true,
		"yepapi_instagram":  true,
		"yepapi_amazon":     true,
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

	validation := g.protector.ValidateOutput(output, nil)
	if !validation.Safe {
		var msgs []string
		for _, thr := range validation.Threats {
			msgs = append(msgs, thr.Message)
		}
		recommendation := strings.Join(msgs, "; ")
		if g.logger != nil {
			g.logger.Warn("[Guardian] Promptsec validation failed", "tool", toolName, "recommendation", recommendation)
		}
		output += "\n[SECURITY WARNING: " + recommendation + "]"
	}

	return output
}

// ScanUserInput analyzes a user message for injection attempts.
// Logs the result but does NOT block — the user is the operator.
// Returns the scan result for upstream decision-making.
func (g *Guardian) ScanUserInput(text string) ScanResult {
	scanText := StripInternalMissionAdvisoryForScan(text)
	scan := g.ScanForInjection(scanText)
	if scan.Level >= ThreatHigh && g.logger != nil {
		g.logger.Warn("[Guardian] Suspicious user input detected",
			"threat", scan.Level.String(), "patterns", scan.Patterns, "preview", truncateForLog(scanText, 200))
	}
	return scan
}

// StripInternalMissionAdvisoryForScan removes scheduler-generated advisory
// blocks before user-input injection scanning. These blocks are internal
// context, not user-authored instructions, and contain planning language that
// can resemble instruction-override attacks.
func StripInternalMissionAdvisoryForScan(text string) string {
	if !strings.Contains(text, missionAdvisoryStartMarker) {
		return text
	}
	for {
		start := strings.Index(text, missionAdvisoryStartMarker)
		if start < 0 {
			return text
		}
		endRel := strings.Index(text[start:], missionAdvisoryEndMarker)
		if endRel < 0 {
			return strings.TrimSpace(text[:start])
		}
		end := start + endRel + len(missionAdvisoryEndMarker)
		text = strings.TrimSpace(text[:start]) + "\n" + strings.TrimSpace(text[end:])
	}
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
