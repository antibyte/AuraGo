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
	"added", "removed", "enabled", "disabled", "activated", "configured", "inserted", "appended",
	"abgeschlossen", "fertig", "erfolgreich", "fehlgeschlagen", "aktualisiert", "geschrieben",
	"gespeichert", "erstellt", "geändert", "gebaut", "deployt", "installiert", "gefunden",
	"gelesen", "geladen", "analysiert", "verifiziert", "veröffentlicht", "neu gestartet",
	"generiert", "erzeugt", "konvertiert", "übertragen", "heruntergeladen", "hochgeladen",
	// German past participles for creative/functional work
	"hinzugefügt", "eingebaut", "ergänzt", "aktiviert", "deaktiviert", "konfiguriert",
	"eingefügt", "entfernt", "angepasst", "integriert", "implementiert", "ausgeführt",
	// German result/presentation verbs
	"präsentiert", "gesendet", "gespielt", "abgespielt", "vorgeführt", "demonstriert",
	"hier ist", "hier sind", "hier hast du", "hier haben wir", "schau mal", "siehe oben",
	"tipp gelernt", "problem erkannt", "lösung gefunden",
	// Unicode success indicators — must be checked in the original content (before lowercasing)
	"✅", "✓", "☑", "✔",
}

var planLinePattern = regexp.MustCompile(`(?m)^\s*(?:[-*]|\d+[.)])\s+\S`)
var pathLikePattern = regexp.MustCompile(`(?i)(?:[A-Za-z]:\\|/|\.{1,2}/|[A-Za-z0-9_-]+\.(?:go|ts|tsx|js|jsx|css|html|json|yaml|yml|md|log|txt|png|jpg|jpeg|webp|svg))`)
var urlLikePattern = regexp.MustCompile(`(?i)\bhttps?://`)
var resultMetricPattern = regexp.MustCompile(`(?i)\b\d+\s+(?:bytes?|files?|lines?|matches?|entries?|tests?|warnings?|errors?|items?|records?|results?|seconds?|minutes?|hours?|sekunden|minuten|stunden|ms|kb|mb|gb)\b`)
var statusEvidencePattern = regexp.MustCompile(`(?i)\b(?:status|exit code|http)\s*[:=]?\s*(?:ok|success|successful|error|failed|200|201|204|400|401|403|404|409|422|429|500)\b`)

func isAnnouncementOnlyResponse(content string, tc ToolCall, useNativePath, lastResponseWasTool bool, lastUserMsg string) bool {
	if tc.IsTool || tc.RawCodeDetected || len(content) > 1000 {
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
	containsActionCue := containsAnyWordPhrase(leadIn, postToolActionCues)
	hasPlanStructure := looksLikePlanStructure(trimmedContent, leadIn)
	hasActionIntent := containsActionIntent(leadIn)
	hasCompletionEvidence := containsCompletionEvidence(lc)

	// When the user asked a question, only skip announcement detection if the
	// response looks like a genuine answer (contains completion evidence) and
	// does NOT simultaneously announce a next action.
	userAskedQuestion := lastUserMsg != "" && strings.HasSuffix(strings.TrimSpace(lastUserMsg), "?")
	if userAskedQuestion && hasCompletionEvidence && !(hasActionIntent && (containsForwardCue || containsActionCue)) {
		return false
	}

	if !lastResponseWasTool {
		// Also exempt completion summaries in the pre-tool path to avoid false
		// positives when the agent responds to a stall-guard or follow-up prompt
		// after finishing all work (lastResponseWasTool is false because the stall
		// guard injected a fake user message). Only an explicit forward cue AND an
		// action cue together can override this (mixed completion+next-action case).
		// A URL or "jetzt" (current state) alone must NOT suppress completion evidence.
		// Exception: strong forward signal (action cue + plan structure like ":") always
		// wins — e.g. "Todo erstellt! Jetzt baue ich X:" is an announcement even though
		// "erstellt" looks like completion evidence.
		strongForwardSignal := containsActionCue && hasPlanStructure
		if hasCompletionEvidence && !strongForwardSignal && !(hasActionIntent && containsForwardCue && containsActionCue) {
			return false
		}
		return containsAnnouncementPhrase || strongForwardSignal || (hasActionIntent && (containsForwardCue || containsActionCue || hasPlanStructure))
	}

	// Post-tool path: if completion evidence is present, only override it when
	// there are BOTH an explicit forward cue ("next", "als nächstes") AND an
	// action cue ("let me", "ich werde"). Requiring both prevents false positives
	// where a completion URL + "jetzt" (current state) suppresses the exemption:
	//   "Fertig! läuft jetzt lokal auf http://..."  ← must NOT trigger
	//   "Done! Now I will deploy to Netlify."        ← must still trigger
	// Exception: strong forward signal (action cue + plan structure like trailing ":")
	// overrides completion evidence — e.g. "Todo erstellt! Jetzt baue ich X ein:" must
	// trigger even though "erstellt" is a completion term.
	strongForwardSignal := containsActionCue && hasPlanStructure
	if hasCompletionEvidence && !strongForwardSignal && !(hasActionIntent && containsForwardCue && containsActionCue) {
		return false
	}

	// A clear announcement phrase (e.g. "lass mich", "ich werde", "let me") without
	// any completion evidence is a sufficient trigger in the post-tool path.
	// This catches responses like "Ich mach das selbst! Lass mich zuerst die
	// Code-Struktur anschauen." where hasActionIntent is false (no file paths or
	// operational terms) but the intent to act is unambiguous.
	if containsAnnouncementPhrase && !hasCompletionEvidence {
		return true
	}

	return (hasActionIntent || strongForwardSignal) && (containsForwardCue || containsActionCue || hasPlanStructure)
}

func containsAnySubstring(s string, needles []string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// containsAnyWordPhrase checks whether any of the needles appears in s with a
// word-start boundary: the character immediately before the match must not be an
// ASCII letter.  This prevents cross-word false positives like "ich deploye"
// matching inside "erfolgreich deployed".
func containsAnyWordPhrase(s string, needles []string) bool {
	for _, needle := range needles {
		if needle == "" {
			continue
		}
		idx := strings.Index(s, needle)
		if idx < 0 {
			continue
		}
		// Require that the byte before the match is not an ASCII letter.
		if idx > 0 {
			b := s[idx-1]
			if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') {
				continue
			}
		}
		return true
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
	// Check Unicode checkmark symbols in the original downcased (but not ASCII-only) string first.
	// These are multi-byte UTF-8 runes that survive ToLower unchanged.
	if strings.ContainsAny(lc, "✅✓☑✔") {
		return true
	}
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
