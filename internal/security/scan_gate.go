package security

import (
	"fmt"
	"log/slog"
	"sync"

	"aurago/internal/config"
)

// guardianOptionsFromConfig maps the guardian section of cfg (preset, scan windows,
// sanitizer, embedding, policy, taint, the judge's settings) into GuardianOptions. The
// server's Guardian, the agent loop's and the local gates' all take their options from
// here, so an operator's tuning reaches every scan; the judge client is not part of the
// options (AttachLLMJudge wires it). A nil cfg gives the zero options.
func guardianOptionsFromConfig(cfg *config.Config) GuardianOptions {
	if cfg == nil {
		return GuardianOptions{}
	}
	promptsec := cfg.Guardian.PromptSec
	return GuardianOptions{
		InjectionGuardEnabled: cfg.Guardian.InjectionGuard.Enabled,
		MaxScanBytes:          cfg.Guardian.MaxScanBytes,
		ScanEdgeBytes:         cfg.Guardian.ScanEdgeBytes,
		Preset:                promptsec.Preset,
		Sanitizer: PromptSecSanitizerOptions{
			Normalize:   promptsec.Sanitizer.Normalize,
			Dehomoglyph: promptsec.Sanitizer.Dehomoglyph,
			Decode:      promptsec.Sanitizer.Decode,
		},
		Embedding: PromptSecEmbeddingOptions{
			Enabled:   promptsec.Embedding.Enabled,
			Threshold: promptsec.Embedding.Threshold,
		},
		Policy:       promptsec.Policy,
		CustomPolicy: PromptSecCustomPolicyOptions{DisallowedTasks: promptsec.CustomPolicy.DisallowedTasks},
		Taint: PromptSecTaintOptions{
			Enabled:      promptsec.Taint.Enabled,
			DefaultLevel: promptsec.Taint.DefaultLevel,
		},
		LLMJudge:           LLMJudgeOptionsFromConfig(cfg),
		UseSanitizedOutput: promptsec.UseSanitizedOutput,
	}
}

// LLMJudgeOptionsFromConfig is the judge section of the guardian configuration, the options
// AttachLLMJudge takes. A nil cfg gives the zero (disabled) options.
func LLMJudgeOptionsFromConfig(cfg *config.Config) PromptSecLLMJudgeOptions {
	if cfg == nil {
		return PromptSecLLMJudgeOptions{}
	}
	judge := cfg.Guardian.PromptSec.LLMJudge
	return PromptSecLLMJudgeOptions{
		Enabled:     judge.Enabled,
		Mode:        judge.Mode,
		TimeoutSecs: judge.TimeoutSecs,
		Policy:      judge.Policy,
	}
}

// NewGuardianFromConfig builds a Guardian with the guardian options of cfg. The judge's
// settings are stored but no judge client is attached: callers that run the LLM judge
// (the server, the agent loop) call AttachLLMJudge with LLMJudgeOptionsFromConfig(cfg).
func NewGuardianFromConfig(logger *slog.Logger, cfg *config.Config) *Guardian {
	return NewGuardianWithOptions(logger, guardianOptionsFromConfig(cfg))
}

// guardianFromConfigCache keeps the last Guardian GuardianFromConfig built: building one
// compiles the rule set, and the options change only when the operator edits the guardian
// section.
var guardianFromConfigCache struct {
	sync.Mutex
	key string
	g   *Guardian
}

// defaultGateGuardian is the Guardian a gate scans with when it has no configuration: the
// package defaults, built once.
var defaultGateGuardian = sync.OnceValue(func() *Guardian { return NewGuardian(nil) })

// GuardianFromConfig returns the Guardian that the model-facing gates outside the agent loop
// (summary mode, the vision fallback of the chat upload path) scan with. It carries the
// guardian options of cfg (NewGuardianFromConfig), so an operator's tuning reaches those
// gates too, but no LLM judge: they scan locally. The Guardian is shared and cached per set
// of options, so it must not be given a system prompt or a judge. A nil cfg gets the package
// default.
func GuardianFromConfig(cfg *config.Config) *Guardian {
	if cfg == nil {
		return defaultGateGuardian()
	}
	opts := guardianOptionsFromConfig(cfg)
	opts.LLMJudge = PromptSecLLMJudgeOptions{}
	key := fmt.Sprintf("%#v", opts)
	c := &guardianFromConfigCache
	c.Lock()
	defer c.Unlock()
	if c.g == nil || c.key != key {
		c.g, c.key = NewGuardianWithOptions(nil, opts), key
	}
	return c.g
}

// GateScan is the local injection scan in front of a model that is about to read outside
// text (the summary helper, the AI step of a flow, the main model for a vision fallback). It
// reports whether the text must be withheld: the scan reached ThreatHigh, or it could not
// read all of the text, the verdict the dispatch gives external tool output. The returned
// scan is the one that decided, for logging.
//
// A text that scores High only through bare role or temporal phrases passes
// (relieveBarePhrases): a web page that says "You are now logged in" would otherwise fail a
// flow or lose its summary. The text is fenced as external data wherever it goes on, and its
// model is told not to obey it. SanitizeToolOutput applies the same rule to the output of
// external tools. A nil Guardian scans with the package default.
func GateScan(g *Guardian, text string) (scan ScanResult, withhold bool) {
	if g == nil {
		g = defaultGateGuardian()
	}
	scan = g.ScanForInjectionLocal(text)
	if scan.Incomplete {
		return scan, true
	}
	if scan.Level < ThreatHigh {
		return scan, false
	}
	return relieveBarePhrases(text, scan, ThreatHigh, g.ScanForInjectionLocal)
}

// RelieveBarePhrases is the relief rule of GateScan and SanitizeToolOutput (relieveBarePhrases)
// for a gate of another package whose bar is not ThreatHigh: the model-authored memory
// writes, which refuse a durable entry already at ThreatMedium. scan is the local scan of
// text, which reached threshold; the caller has dealt with an incomplete scan. It returns the
// scan that decides (the rescan of the text with its bare role and time phrases blanked
// when they were the only reason) and whether text stays hostile. rescan reads the blanked
// text; the Guardian's ScanForInjectionLocal is the usual one, a caller may wrap it.
func RelieveBarePhrases(text string, scan ScanResult, threshold ThreatLevel, rescan func(string) ScanResult) (ScanResult, bool) {
	return relieveBarePhrases(text, scan, threshold, rescan)
}
