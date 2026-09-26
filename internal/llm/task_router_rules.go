package llm

import (
	"regexp"
	"strings"
	"unicode"
)

// TaskClassification contains only allowlisted labels, never executable advice.
type TaskClassification struct {
	Domain     string `json:"domain"`
	Complexity string `json:"complexity"`
	Uncertain  bool   `json:"uncertain"`
}

func (c TaskClassification) Area() string {
	if c.Uncertain {
		return ""
	}
	switch c.Domain {
	case "coding", "research", "creativity", "security", "writing":
		return c.Domain
	case "general":
		switch c.Complexity {
		case "easy", "normal", "complex":
			return c.Complexity
		case "unknown":
			return "general"
		}
	}
	return ""
}

var routerQuoted = regexp.MustCompile("(?s)```.*?(?:```|$)|`[^`]*`|\"[^\"]*\"|[„“][^“”]*[“”]|https?://[^\\s]+|<external_data[^>]*>.*?(?:</external_data>|$)")
var routerQuoteLines = regexp.MustCompile(`(?m)^\s*>.*$`)
var routerPolite = regexp.MustCompile(`^(?:please|bitte|can you|could you|kannst du|könntest du)\s+`)
var routerModelSelection = regexp.MustCompile(`^(?:use|switch|choose|select|nutze|nimm|wechsle|wähle)\b.{0,120}\b(?:model\w*|modell\w*|provider)\b`)
var routerWritingOutcome = regexp.MustCompile(`^(?:write|draft|edit|schreibe|verfasse|überarbeite)\s+(?:(?:a|an|the|this|eine[nrms]?|den|diesen|meinen)\s+)?(?:short |brief |kurzen |kurze |freundlichen )?(?:email|e-mail|letter|brief|guide|anleitung|article|artikel|report|bericht|documentation|dokumentation|text|blog)\b`)
var routerRules = []struct {
	domain  string
	pattern *regexp.Regexp
}{
	{"security", regexp.MustCompile(`\b(audit|assess|review|check|prüfe|prüfen|analysiere|untersuche|überprüfe).{0,100}\b(security|vulnerabilit\w*|threat\w*|permission\w*|sicherheit\w*|schwachstell\w*|berechtigung\w*|login|anmeldung)\b|\b(threat model|bedrohungsmodell|security audit|sicherheitsaudit|hardening|härte|absichern)\b`)},
	{"coding", regexp.MustCompile(`\b(implement\w*|debug\w*|refactor\w*|implementiere|implementieren|programmiere|programming|programmieren)\b|\b(fix|repair|test|write|build|create|corrige|behebe|korrigiere|teste|schreibe|erstelle|repariere).{0,80}\b(code|bug\w*|function\w*|funktion\w*|script\w*|skript\w*|unit test\w*|regression\w*|compiler|api|sql|python|golang|javascript|typescript|go|authenticat\w*|authentifizier\w*)\b`)},
	{"research", regexp.MustCompile(`\b(research|recherchier\w*|investigate|find sources|find evidence|look up|suche quellen|finde quellen|verifiziere)\b|\b(compare|vergleiche|prüfe|verify).{0,100}\b(source\w*|quelle\w*|evidence|beleg\w*|studie\w*|studies|approach\w*|ansätze|anbieter|provider|literature|literatur)\b`)},
	{"creativity", regexp.MustCompile(`\b(brainstorm\w*|invent|erfinde|erfinden|ideen|ideas|konzepte|concepts)\b|\b(write|create|schreibe|entwickle|entwirf).{0,80}\b(story|stories|poem|gedicht|geschichte|roman|haiku|spielidee|game concept|kreativ\w*|creative)\b`)},
	{"writing", regexp.MustCompile(`\b(rewrite|translate|proofread|summariz\w*|summaris\w*|übersetze|übersetzen|überarbeite|kürze|formuliere|zusammenfassen|fasse zusammen|korrigiere die grammatik|correct the grammar)\b|\b(write|draft|edit|structure|schreibe|verfasse|strukturiere).{0,80}\b(text|guide|anleitung|email|e-mail|brief|letter|bericht|report|article|artikel|blog|documentation|dokumentation)\b`)},
}

var routerComplex = regexp.MustCompile(`\b(plan|plane|design|entwirf|analy[sz]e|analysiere|entwickle).{0,140}\b(migration|architecture|architektur|strategy|strategie|dependencies|abhängigkeiten|rollback|trade.?offs|risks|risiken)\b`)
var routerNormal = regexp.MustCompile(`\b(sort|organize|organise|schedule|arrange|sortiere|organisiere|ordne|plane|vergleiche|compare).{0,100}\b(appointments|termine|dates|daten|tasks|aufgaben|list|liste|files|dateien|options|optionen|meeting|meetings|besprechung|woche|week|einkauf|shopping|urlaub|trip|reise)\b`)
var routerEasy = regexp.MustCompile(`\b(convert|calculate|count|rechne|berechne|zähle|wieviel|wie viel).{0,70}\d|\b(what is|was ist)\s+\d|\b(wandle|umrechnen).{0,80}\d|^\d+\s*[+*/-]\s*\d+[?\s]*$|\b(what time|wie spät|hauptstadt von|capital of)\b`)
var routerGeneral = regexp.MustCompile(`^(hi|hello|hey|hallo|guten morgen|guten abend|guten tag|good morning|good evening|danke|danke schön|vielen dank|thanks|thank you|how are you|how are you doing|wie geht es dir|wie geht.s|tell me about yourself|erzähle mir von dir|wer bist du|who are you|was kannst du|what can you do|bis später|see you|goodbye|tschüss)([,!?.\s]*(aura|aurago|dir|so much|nochmal))?[!?.\s]*$`)

// ClassifyTask deliberately abstains on conflicting or weak evidence.
func ClassifyTask(intent string) TaskClassification {
	runes := []rune(intent)
	if len(runes) > 8000 {
		runes = runes[:8000]
	}
	text := strings.TrimSpace(routerQuoteLines.ReplaceAllString(routerQuoted.ReplaceAllString(strings.ToLower(string(runes)), " "), " "))
	text = routerPolite.ReplaceAllString(text, "")
	unknown := TaskClassification{Domain: "general", Complexity: "unknown", Uncertain: true}
	if text == "" {
		return unknown
	}
	// Model-selection instructions describe routing preferences, not a task domain.
	if routerModelSelection.MatchString(text) {
		return unknown
	}
	// A requested transformation of supplied text wins over the subject of that text.
	for _, prefix := range []string{"rewrite ", "translate ", "proofread ", "übersetze ", "überarbeite den text", "korrigiere die grammatik", "correct the grammar", "fasse zusammen"} {
		if strings.HasPrefix(text, prefix) && !regexpRouterSecondAction.MatchString(text) {
			return TaskClassification{Domain: "writing", Complexity: "normal"}
		}
	}
	if routerWritingOutcome.MatchString(text) && !regexpRouterSecondAction.MatchString(text) {
		return TaskClassification{Domain: "writing", Complexity: "normal"}
	}
	matched := ""
	for _, rule := range routerRules {
		if rule.pattern.MatchString(text) {
			if matched != "" {
				return unknown
			}
			matched = rule.domain
		}
	}
	if matched != "" {
		return TaskClassification{Domain: matched, Complexity: "normal"}
	}
	if routerComplex.MatchString(text) {
		return TaskClassification{Domain: "general", Complexity: "complex"}
	}
	if routerNormal.MatchString(text) {
		return TaskClassification{Domain: "general", Complexity: "normal"}
	}
	if routerEasy.MatchString(text) {
		return TaskClassification{Domain: "general", Complexity: "easy"}
	}
	if routerGeneral.MatchString(text) {
		return TaskClassification{Domain: "general", Complexity: "unknown"}
	}
	return unknown
}

var regexpRouterSecondAction = regexp.MustCompile(`\b(and|und|then|danach)\s+(implement\w*|debug\w*|recherchier\w*|research|audit)\b`)

func IsTaskContinuation(intent string) bool {
	s := strings.TrimFunc(strings.ToLower(intent), func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) })
	switch s {
	case "continue", "go on", "weiter", "mach weiter", "fortsetzen":
		return true
	}
	return false
}
