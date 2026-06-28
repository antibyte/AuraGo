package security

import (
	"html"
	"log/slog"
	"regexp"
	"strings"
	"time"

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
	Level     ThreatLevel
	Patterns  []string // matched pattern names
	Message   string   // human-readable summary
	Sanitized string   // promptsec sanitized output, when enabled
}

// PromptSecSanitizerOptions mirrors the sanitizer configuration.
type PromptSecSanitizerOptions struct {
	Normalize   bool
	Dehomoglyph bool
	Decode      bool
}

// PromptSecEmbeddingOptions mirrors the embedding configuration.
type PromptSecEmbeddingOptions struct {
	Enabled   bool
	Threshold float64
}

// PromptSecCustomPolicyOptions mirrors custom policy configuration.
type PromptSecCustomPolicyOptions struct {
	DisallowedTasks []string
}

// PromptSecTaintOptions mirrors taint configuration.
type PromptSecTaintOptions struct {
	Enabled      bool
	DefaultLevel string
}

// PromptSecStructureOptions mirrors structure configuration.
type PromptSecStructureOptions struct {
	Enabled bool
	Mode    string
}

// PromptSecLLMJudgeOptions mirrors LLM judge configuration.
type PromptSecLLMJudgeOptions struct {
	Enabled     bool
	Mode        string
	TimeoutSecs int
	Policy      string
}

// GuardianOptions controls bounded regex scanning behavior and promptsec guard selection.
type GuardianOptions struct {
	MaxScanBytes       int
	ScanEdgeBytes      int
	Preset             string
	Spotlight          bool
	Canary             bool
	Sanitizer          PromptSecSanitizerOptions
	Embedding          PromptSecEmbeddingOptions
	Policy             string
	CustomPolicy       PromptSecCustomPolicyOptions
	Taint              PromptSecTaintOptions
	Structure          PromptSecStructureOptions
	LLMJudge           PromptSecLLMJudgeOptions
	LLMJudgeClient     promptsec.LLMJudge
	UseSanitizedOutput bool
	SystemPrompt       string
}

// Guardian provides multi-layer prompt injection defense.
// It scans text for known injection patterns, wraps external data for isolation,
// and strips dangerous role-impersonation markers from tool output.
type Guardian struct {
	logger            *slog.Logger
	maxScanBytes      int
	scanEdgeBytes     int
	protector         *promptsec.Protector
	useSanitized      bool
	psOpts            []promptsec.Guard
	llmJudgeOpts      PromptSecLLMJudgeOptions
	llmJudge          promptsec.LLMJudge
	structureOpts     PromptSecStructureOptions
	systemPrompt      string
	hasStructureGuard bool
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
		useSanitized:  opts.UseSanitizedOutput,
		llmJudge:      opts.LLMJudgeClient,
		llmJudgeOpts:  opts.LLMJudge,
		structureOpts: opts.Structure,
		systemPrompt:  opts.SystemPrompt,
	}

	g.psOpts = g.buildPromptSecGuards(opts)
	// Detect whether buildPromptSecGuards already added a structure guard.
	if opts.Structure.Enabled && opts.SystemPrompt != "" {
		g.hasStructureGuard = true
	}
	g.protector = promptsec.New(g.psOpts...)

	// If a judge client was supplied at construction time, wire it in now.
	if opts.LLMJudge.Enabled && g.llmJudge != nil {
		g.attachLLMJudge()
	}

	return g
}

// AttachLLMJudge wires an external classifier into the promptsec pipeline.
// This allows the existing security.LLMGuardian to be reused as promptsec's
// LLM-as-Judge escalation layer. If opts.Enabled is false the judge is stored
// but not activated.
func (g *Guardian) AttachLLMJudge(judge promptsec.LLMJudge, opts PromptSecLLMJudgeOptions) {
	if judge == nil {
		return
	}
	g.llmJudge = judge
	g.llmJudgeOpts = opts
	if opts.Enabled {
		g.attachLLMJudge()
	}
}

// SetSystemPrompt updates the trusted system prompt used by structure guards.
// It rebuilds the protector only when structure enforcement is enabled. This
// must be called before the first Analyze/ValidateOutput call in the agent loop.
func (g *Guardian) SetSystemPrompt(systemPrompt string) {
	if !g.structureOpts.Enabled || g.systemPrompt == systemPrompt {
		return
	}
	g.systemPrompt = systemPrompt

	mode := promptsec.Sandwich
	switch strings.ToLower(g.structureOpts.Mode) {
	case "xml":
		mode = promptsec.XMLTags
	case "random":
		mode = promptsec.RandomEnclosure
	case "post":
		mode = promptsec.PostPrompt
	}

	// Remove the previous structure guard if present. Structure is always the
	// last guard added by buildPromptSecGuards, so we can safely drop the tail.
	if g.hasStructureGuard && len(g.psOpts) > 0 {
		g.psOpts = g.psOpts[:len(g.psOpts)-1]
	}

	g.psOpts = append(g.psOpts, promptsec.WithStructure(mode, &promptsec.StructureOptions{
		SystemPrompt: g.systemPrompt,
	}))
	g.hasStructureGuard = true
	g.protector = promptsec.New(g.psOpts...)
	if g.llmJudge != nil {
		g.attachLLMJudge()
	}
}

// buildPromptSecGuards assembles the core guard chain from configuration.
func (g *Guardian) buildPromptSecGuards(opts GuardianOptions) []promptsec.Guard {
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

	// Sanitizer runs early so later guards operate on canonical input.
	if opts.Sanitizer.Normalize || opts.Sanitizer.Dehomoglyph || opts.Sanitizer.Decode {
		psOpts = append(psOpts, promptsec.WithSanitizer(&promptsec.SanitizerOptions{
			Normalize:      opts.Sanitizer.Normalize,
			Dehomoglyph:    opts.Sanitizer.Dehomoglyph,
			DecodePayloads: opts.Sanitizer.Decode,
		}))
	}

	// Embedding-based similarity guard against known attack vectors.
	if opts.Embedding.Enabled {
		threshold := opts.Embedding.Threshold
		if threshold <= 0 || threshold > 1 {
			threshold = 0.65
		}
		psOpts = append(psOpts, promptsec.WithEmbedding(&promptsec.EmbeddingOptions{
			Threshold: threshold,
		}))
	}

	// Context-aware task policy.
	if policyOpt := buildPromptSecPolicy(opts.Policy, opts.CustomPolicy); policyOpt != nil {
		psOpts = append(psOpts, promptsec.WithPolicy(policyOpt))
	}

	if opts.Spotlight {
		psOpts = append(psOpts, promptsec.WithSpotlighting(promptsec.Datamark, &promptsec.DatamarkOptions{Token: "^"}))
	}
	if opts.Canary {
		psOpts = append(psOpts, promptsec.WithCanary(&promptsec.CanaryOptions{Format: promptsec.CanaryHex, Length: 16}))
	}

	// Prompt structure enforcement (sandwich defense etc.).
	// Only add the guard when a system prompt is available; it will be rebuilt
	// later via SetSystemPrompt if the prompt changes.
	if opts.Structure.Enabled && opts.SystemPrompt != "" {
		mode := promptsec.Sandwich
		switch strings.ToLower(opts.Structure.Mode) {
		case "xml":
			mode = promptsec.XMLTags
		case "random":
			mode = promptsec.RandomEnclosure
		case "post":
			mode = promptsec.PostPrompt
		}
		psOpts = append(psOpts, promptsec.WithStructure(mode, &promptsec.StructureOptions{
			SystemPrompt: opts.SystemPrompt,
		}))
	}

	return psOpts
}

func (g *Guardian) attachLLMJudge() {
	if g.llmJudge == nil {
		return
	}
	mode := promptsec.LLMJudgeModeUncertain
	switch strings.ToLower(g.llmJudgeOpts.Mode) {
	case "always":
		mode = promptsec.LLMJudgeModeAlways
	case "threat_detected":
		mode = promptsec.LLMJudgeModeThreatDetected
	case "no_threat":
		mode = promptsec.LLMJudgeModeNoThreat
	}
	timeout := time.Duration(g.llmJudgeOpts.TimeoutSecs) * time.Second
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	judgeGuard := promptsec.WithLLMJudge(&promptsec.LLMJudgeOptions{
		Mode:       mode,
		Timeout:    timeout,
		Policy:     g.llmJudgeOpts.Policy,
		Judge:      g.llmJudge,
		Cache:      true,
		FailClosed: false,
		Model:      "llm_guardian",
	})

	g.protector = promptsec.New(append(g.psOpts, judgeGuard)...)
}

// buildPromptSecPolicy returns a policy options value for the configured policy name.
func buildPromptSecPolicy(name string, custom PromptSecCustomPolicyOptions) *promptsec.PolicyOptions {
	switch strings.ToLower(name) {
	case "rag":
		return promptsec.PolicyRAG()
	case "support":
		return promptsec.PolicySupportBot()
	case "coding":
		return promptsec.PolicyCodingAssistant()
	case "translation":
		return promptsec.PolicyTranslationApp()
	case "custom":
		if len(custom.DisallowedTasks) == 0 {
			return nil
		}
		return &promptsec.PolicyOptions{
			Name:            "custom",
			DisallowedTasks: parsePolicyTasks(custom.DisallowedTasks),
		}
	default:
		return nil
	}
}

func parsePolicyTasks(names []string) []promptsec.PolicyTask {
	var tasks []promptsec.PolicyTask
	for _, n := range names {
		switch strings.ToLower(n) {
		case "code_generation":
			tasks = append(tasks, promptsec.PolicyTaskCodeGeneration)
		case "sql_access":
			tasks = append(tasks, promptsec.PolicyTaskSQLAccess)
		case "terminal_simulation":
			tasks = append(tasks, promptsec.PolicyTaskTerminalSimulation)
		case "roleplay":
			tasks = append(tasks, promptsec.PolicyTaskRoleplay)
		case "external_persona":
			tasks = append(tasks, promptsec.PolicyTaskExternalPersona)
		case "translation":
			tasks = append(tasks, promptsec.PolicyTaskTranslation)
		case "creative_writing":
			tasks = append(tasks, promptsec.PolicyTaskCreativeWriting)
		case "opinion_persuasion":
			tasks = append(tasks, promptsec.PolicyTaskOpinionPersuasion)
		}
	}
	return tasks
}

// ScanForInjection analyzes text for prompt injection patterns.
// Returns a ScanResult with the highest threat level found and all matched patterns.
func (g *Guardian) ScanForInjection(text string) ScanResult {
	return g.scanWithOptions(text, scanOptions{})
}

// ScanForInjectionWithSource analyzes text while tracking its provenance.
func (g *Guardian) ScanForInjectionWithSource(text, source string, taintLevel promptsec.TrustLevel) ScanResult {
	return g.scanWithOptions(text, scanOptions{source: source, taintLevel: taintLevel})
}

// SanitizeForLLM runs the full promptsec pipeline and returns the sanitized output
// along with the threat analysis. It is useful for pre-processing external content
// before it enters the LLM context.
func (g *Guardian) SanitizeForLLM(text, source string) ScanResult {
	lvl := promptsec.Untrusted
	if source == "system" {
		lvl = promptsec.System
	}
	return g.scanWithOptions(text, scanOptions{source: source, taintLevel: lvl, returnSanitized: true})
}

type scanOptions struct {
	source          string
	taintLevel      promptsec.TrustLevel
	returnSanitized bool
}

func (g *Guardian) scanWithOptions(text string, opts scanOptions) ScanResult {
	if text == "" {
		return ScanResult{Level: ThreatNone}
	}

	scanWindows, chunked := prepareGuardianScanTexts(text, g.maxScanBytes, g.scanEdgeBytes)
	result := ScanResult{Level: ThreatNone}
	var msgs []string

	for _, scanText := range scanWindows {
		analysis := g.protector.Analyze(scanText)

		// Collect sanitized output from the first window when requested.
		// Some guards (e.g. structure) rewrite the output without adding threats.
		if (g.useSanitized || opts.returnSanitized) && result.Sanitized == "" {
			result.Sanitized = analysis.Output
		}

		if analysis.Safe && len(analysis.Threats) == 0 {
			continue
		}
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

			result.Patterns = appendUniqueString(result.Patterns, string(thr.Type))
			msgs = appendUniqueString(msgs, thr.Message)
		}

		// If promptsec marks unsafe, ensure at least ThreatMedium
		if !analysis.Safe && result.Level < ThreatMedium {
			result.Level = ThreatMedium
		}
	}

	if result.Level > ThreatNone {
		result.Message = strings.Join(msgs, "; ")
		if result.Message == "" {
			result.Message = "Promptsec marked content unsafe"
		}
		if chunked {
			result.Message += " [scan_window=chunked]"
		}
	} else if chunked {
		result.Message = "No injection patterns detected in chunked scan windows"
	}

	return result
}

func prepareGuardianScanTexts(text string, maxScanBytes, scanEdgeBytes int) ([]string, bool) {
	if maxScanBytes <= 0 {
		maxScanBytes = defaultGuardianMaxScanBytes
	}
	if scanEdgeBytes <= 0 {
		scanEdgeBytes = defaultGuardianScanEdgeBytes
	}
	if len(text) <= maxScanBytes {
		return []string{text}, false
	}
	if scanEdgeBytes >= maxScanBytes {
		scanEdgeBytes = maxScanBytes / 4
	}
	if scanEdgeBytes < 0 {
		scanEdgeBytes = 0
	}
	stride := maxScanBytes - scanEdgeBytes
	if stride <= 0 {
		stride = maxScanBytes
	}

	windows := make([]string, 0, (len(text)/stride)+1)
	for start := 0; start < len(text); {
		end := start + maxScanBytes
		if end > len(text) {
			end = len(text)
		}
		windows = append(windows, text[start:end])
		if end == len(text) {
			break
		}
		start += stride
	}
	return windows, true
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

func appendUniqueString(items []string, value string) []string {
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
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
// Execution tools remain heuristic because their output can be local operator diagnostics.
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
		"agentmail":     true,
		"fetch_email":   true,
		"check_email":   true,
		"discord":       true,
		"fetch_discord": true,
		// File and memory reads can include attacker-controlled project or user content
		"filesystem":           true,
		"filesystem_op":        true,
		"file_reader_advanced": true,
		"smart_file_read":      true,
		"file_search":          true,
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
	scan := g.ScanForInjectionWithSource(content, source, promptsec.Untrusted)
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
