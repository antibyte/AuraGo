package agent

import (
	"regexp"
	"strings"
)

var announcementPhrases = []string{
	"lass mich", "ich starte", "ich werde", "ich führe", "ich teste",
	"ich versuche", "versuche ich", "ich probiere", "probiere ich",
	"nochmal", "noch einmal", "erneut", "wieder ",
	"let me", "i will", "i'll", "i am going to", "i'm going to",
	"let's start", "starting", "launching", "i'll start", "i'll run",
	"i try", "i'll try", "trying", "retrying",
	"alles klar", "okay, let", "sure, let", "sure, i",
	"ich suche nach", "ich schaue nach", "ich prüfe", "ich überprüfe",
	"ich sehe mir", "lass mich sehen", "ich werde nachschauen",
	"i'll check", "let me check", "checking", "searching", "looking",
	"i am looking", "i will look", "i'll search", "i will search",
	"ich frage ab", "ich lade", "i'll load", "i am loading",
}

var postToolForwardCues = []string{
	"jetzt ", "als nächstes", "danach", "anschließend", "nun ",
	"now ", "next ", "next,", "then ", "after that",
}

var postToolActionCues = []string{
	"ich baue", "baue ich", "ich deploye", "deploye ich", "ich starte", "starte ich",
	"ich prüfe", "prüfe ich", "ich installiere", "installiere ich", "ich führe", "führe ich",
	"ich werde", "werde ich", "i will", "i'll", "let me", "starting", "launching",
}

var genericForwardCues = []string{
	" next ", " now ", " then ", " after ", " continue ", " continuing ",
	" weiter ", " danach ", " anschließend ", " nun ", " luego ", " despues ", " después ",
	" ensuite ", " puis ", " maintenant ", " ora ", " poi ", " adesso ",
}

var operationalTerms = []string{
	"build", "deploy", "test", "run", "restart", "install", "search", "read", "write",
	"edit", "update", "modify", "analy", "inspect", "create", "delete", "move", "list",
	"lint", "screenshot", "summar", "publish", "commit", "push", "pull", "grep", "tail",
	"head", "open", "browse", "render", "compile", "generate", "filesystem", "homepage", "netlify",
	"file_reader_advanced", "file_search", "execute_shell", "execute_skill", "smart_file_read",
	"analyze_image", "docker", "git", ".go", ".ts", ".tsx", ".js", ".css", ".html", ".json",
	".yaml", ".yml", ".md", ".log",
}

var completionEvidenceTerms = []string{
	"completed", "finished", "done", "successful", "successfully", "succeeded", "failed", "error",
	"updated", "written", "wrote", "saved", "created", "deleted", "modified", "changed", "built",
	"deployed", "installed", "rendered", "generated", "found", "listed", "read", "loaded", "analyzed",
	"analysed", "verified", "published", "committed", "pushed", "pulled", "restarted",
	"abgeschlossen", "fertig", "erfolgreich", "fehlgeschlagen", "aktualisiert", "geschrieben",
	"gespeichert", "erstellt", "geändert", "gebaut", "deployt", "installiert", "gefunden",
	"gelesen", "geladen", "analysiert", "verifiziert", "veröffentlicht", "neu gestartet",
	// German result/presentation verbs
	"präsentiert", "gesendet", "gespielt", "abgespielt", "vorgeführt", "demonstriert",
	"hier ist", "hier sind", "hier hast du", "hier haben wir", "schau mal", "siehe oben",
	"tipp gelernt", "problem erkannt", "lösung gefunden",
}

var planLinePattern = regexp.MustCompile(`(?m)^\s*(?:[-*]|\d+[.)])\s+\S`)
var pathLikePattern = regexp.MustCompile(`(?i)(?:[A-Za-z]:\\|/|\.{1,2}/|[A-Za-z0-9_-]+\.(?:go|ts|tsx|js|jsx|css|html|json|yaml|yml|md|log|txt|png|jpg|jpeg|webp|svg))`)
var urlLikePattern = regexp.MustCompile(`(?i)\bhttps?://`)
var resultMetricPattern = regexp.MustCompile(`(?i)\b\d+\s+(?:bytes?|files?|lines?|matches?|entries?|tests?|warnings?|errors?|items?|records?|results?)\b`)
var statusEvidencePattern = regexp.MustCompile(`(?i)\b(?:status|exit code|http)\s*[:=]?\s*(?:ok|success|successful|error|failed|200|201|204|400|401|403|404|409|422|429|500)\b`)

func isAnnouncementOnlyResponse(content string, tc ToolCall, useNativePath, lastResponseWasTool bool, lastUserMsg string) bool {
	if tc.IsTool || useNativePath || tc.RawCodeDetected || len(content) > 1000 {
		return false
	}

	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		return false
	}
	if strings.HasSuffix(strings.TrimRight(trimmedContent, "\"'"), "?") {
		return false
	}

	lc := strings.ToLower(trimmedContent)
	leadIn := lc
	if len(leadIn) > 250 {
		leadIn = leadIn[:250]
	}

	containsAnnouncementPhrase := containsAnySubstring(leadIn, announcementPhrases)
	containsForwardCue := containsAnySubstring(leadIn, postToolForwardCues) || containsAnySubstring(leadIn, genericForwardCues)
	containsActionCue := containsAnySubstring(leadIn, postToolActionCues)
	hasPlanStructure := looksLikePlanStructure(trimmedContent, leadIn)
	hasActionIntent := containsActionIntent(leadIn)
	hasCompletionEvidence := containsCompletionEvidence(lc)

	// When the user asked a question, only skip announcement detection if the
	// response looks like a genuine answer (contains completion evidence) and
	// does NOT simultaneously announce a next action.
	userAskedQuestion := lastUserMsg != "" && strings.HasSuffix(strings.TrimSpace(lastUserMsg), "?")
	if userAskedQuestion && hasCompletionEvidence && !(hasActionIntent && (containsForwardCue || containsActionCue || hasPlanStructure)) {
		return false
	}

	if !lastResponseWasTool {
		return containsAnnouncementPhrase || (hasActionIntent && (containsForwardCue || containsActionCue || hasPlanStructure))
	}

	if hasCompletionEvidence && !(hasActionIntent && (containsForwardCue || containsActionCue || hasPlanStructure)) {
		return false
	}

	return hasActionIntent && (containsForwardCue || containsActionCue || hasPlanStructure)
}

func containsAnySubstring(s string, needles []string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(s, needle) {
			return true
		}
	}
	return false
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

func containsActionIntent(leadIn string) bool {
	if containsAnySubstring(leadIn, operationalTerms) {
		return true
	}
	if pathLikePattern.MatchString(leadIn) || urlLikePattern.MatchString(leadIn) {
		return true
	}
	return false
}

func containsCompletionEvidence(lc string) bool {
	if containsAnySubstring(lc, completionEvidenceTerms) {
		return true
	}
	if resultMetricPattern.MatchString(lc) {
		return true
	}
	if statusEvidencePattern.MatchString(lc) {
		return true
	}
	if strings.Contains(lc, "saved to ") || strings.Contains(lc, "written to ") || strings.Contains(lc, "live at ") {
		return true
	}
	return false
}
