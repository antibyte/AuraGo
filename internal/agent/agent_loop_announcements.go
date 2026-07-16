package agent

import (
	"regexp"
	"strings"
)

var planLinePattern = regexp.MustCompile(`(?m)^\s*(?:[-*]|\d+[.)])\s+\S`)
var pathLikePattern = regexp.MustCompile(`(?i)(?:[A-Za-z]:\\|/|\.{1,2}/|[A-Za-z0-9_-]+\.(?:go|ts|tsx|js|jsx|css|html|json|yaml|yml|md|log|txt|png|jpg|jpeg|webp|svg))`)
var urlLikePattern = regexp.MustCompile(`(?i)\bhttps?://`)
var resultMetricPattern = regexp.MustCompile(`(?i)\b\d+\s+(?:bytes?|files?|lines?|matches?|entries?|tests?|warnings?|errors?|items?|records?|results?|seconds?|minutes?|hours?|ms|kb|mb|gb)\b`)
var statusEvidencePattern = regexp.MustCompile(`(?i)\b(?:status|exit code|http)\s*[:=]?\s*(?:ok|success|successful|stabil|erfolgreich|unauff[äa]llig|error|failed|200|201|204|400|401|403|404|409|422|429|500)\b`)
var completionPhrasePattern = regexp.MustCompile(`(?i)\b(?:ist\s+abgeschlossen|wurde\s+abgeschlossen|keine\s+(?:weitere\s+)?aktion\s+erforderlich|keine\s+benachrichtigung\s+n[öo]tig)\b`)
var actionPromisePattern = regexp.MustCompile(`(?i)\b(?:ich\s+(?:werde|pr[üu]fe|schaue|checke|mache|starte|f[üu]hre|erstelle|sende|aktualisiere|behebe|repariere|k[üu]mmere)|i\s+(?:will|am going to)|i'll|i'm going to|let me)\b`)
var actionableUserIntentPattern = regexp.MustCompile(`(?i)\b(?:ja|ok|okay|weiter|mach|go ahead|do it|check|fix|run|repair|pr[üu]f|sende|erstelle|beheb|reparier|starte|f[üu]hre)\b`)
var refusalPattern = regexp.MustCompile(`(?i)\b(?:cannot|can't|can not|unable|nicht|kann\s+nicht|keine\s+berechtigung|not allowed)\b`)

func isAnnouncementOnlyResponse(content string, tc ToolCall, useNativePath, lastResponseWasTool bool, lastUserMsg string) bool {
	if tc.IsTool || tc.RawCodeDetected || len(content) > 1000 {
		return false
	}

	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" || asksUserForInput(trimmedContent) {
		return false
	}

	leadIn := strings.ToLower(trimmedContent)
	if len(leadIn) > 250 {
		leadIn = leadIn[:250]
	}

	if containsCompletionEvidence(trimmedContent) {
		return false
	}

	return looksLikePlanStructure(trimmedContent, leadIn)
}

func shouldRecoverAnnouncementOnlyResponse(parsedToolResp ParsedToolResponse, tc ToolCall, useNativePath, lastResponseWasTool bool, lastUserMsg string) bool {
	announcementContent := parsedToolResp.SanitizedContent
	if announcementContent == "" || tc.IsTool {
		return false
	}
	if !isAnnouncementOnlyResponse(announcementContent, tc, useNativePath, lastResponseWasTool, lastUserMsg) {
		return false
	}
	return true
}

func shouldRecoverActionPromiseWithoutTool(content string, tc ToolCall, lastUserMsg string) bool {
	if tc.IsTool || tc.RawCodeDetected || len(content) > 600 {
		return false
	}
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" || asksUserForInput(trimmedContent) || containsCompletionEvidence(trimmedContent) {
		return false
	}
	if refusalPattern.MatchString(trimmedContent) {
		return false
	}
	return actionPromisePattern.MatchString(trimmedContent) && actionableUserIntentPattern.MatchString(lastUserMsg)
}

func looksLikePlanStructure(trimmedContent, leadIn string) bool {
	if strings.HasSuffix(strings.TrimSpace(trimmedContent), ":") {
		return true
	}
	if strings.Contains(leadIn, "->") || strings.Contains(leadIn, "=>") {
		return true
	}
	return planLinePattern.MatchString(trimmedContent)
}

func containsStructuralReference(content string) bool {
	return pathLikePattern.MatchString(content) || urlLikePattern.MatchString(content)
}

func containsCompletionEvidence(content string) bool {
	if strings.ContainsAny(content, "✅✓☑✔") {
		return true
	}
	return resultMetricPattern.MatchString(content) ||
		statusEvidencePattern.MatchString(content) ||
		completionPhrasePattern.MatchString(content)
}

func asksUserForInput(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "?")
}
