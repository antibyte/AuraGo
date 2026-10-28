package security

import (
	"regexp"
	"strings"
)

var aiReader = regexp.MustCompile(`(?i)(?:\b(?:ai|ki)[ -]?(?:assistants?|assistenten?|agents?|agenten|models?|modelle|summarizers?)\b|\b(?:chatgpt|claude|gemini|llm|sprachmodell|language model)\b|(?:^|[\n.!?])\s*(?:assistant|assistent|systemanweisung)\s*[:,]|\bliebe ki\b|\b(?:when an ai|wenn eine ki|automated assistants?|automatisierte[nr]? agenten)\b|\b(?:modèle de langage|assistant ia)\b)`)
var explicitOverride = regexp.MustCompile(`(?i)\b(?:disregard the instructions you were given|ignorez (?:toutes )?(?:les|vos) (?:instructions|consignes) (?:précédentes|antérieures)|ignoriere (?:alle )?(?:vorherigen|bisherigen) (?:anweisungen|regeln))\b`)
var aiDirective = regexp.MustCompile(`(?i)\b(?:must|should|always|never|recommend|tell|rank|stop|instead|call|open|append|end with|before answering|delete|do this|do not|dürfen|sollen?|muss|müssen|empfiehl|sage?|schreib|rufe?|öffne|lade|führe|hör|unabhängig|zuerst|hänge|endet|doit|doivent|ignorez|répondez|envoyez|recommandez)\b`)
var aiAddress = regexp.MustCompile(`(?i)(?:\b(?:note to|instructions? (?:for|to)|attention|hey|hinweis für|anweisung an|achtung)\s+(?:(?:automated|automatisierte[nr]?|ai|ki)[ -]?)?(?:assistants?|assistenten?|agents?|agenten|language model|sprachmodell|chatgpt|claude|gemini|llm)|\b(?:if you are an ai|when an ai reads|wenn du ein sprachmodell|wenn eine ki diesen text|liebe ki)\b|(?:^|[.!?])\s*(?:assistant(?: ia)?|assistent|systemanweisung)\s*[:,])`)
var aiResponseOrder = regexp.MustCompile(`(?i)\b(?:ai models?|ki[ -]modelle)\s+(?:must|should|dürfen|sollen|müssen)\b[^.!?]{0,300}\b(?:describe|recommend|mention|rank|summarize|summarise|darstellen|empfehlen|erwähnen|einstufen)\b`)
var aiResponseTarget = regexp.MustCompile(`(?i)\b(?:this (?:page|article)|the user|your (?:answer|summary)|diese[rn]? (?:seite|artikel)|deine (?:antwort|zusammenfassung)|de[mn] nutzer)\b`)
var responseLink = regexp.MustCompile(`(?i)(?:your answer|when you answer|every summary|jede antwort|jede zusammenfassung|ta réponse|votre réponse)`)
var responseAppend = regexp.MustCompile(`(?i)\b(?:append|add|end|finish|anhängen|ergänzen|endet|beenden|am ende|ajoutez|terminez)\b`)

// externalDirective is intentionally limited to instructions addressing an AI reader.
// Ordinary citations/publisher requirements are left to the optional classifier.
func externalDirective(text string) bool {
	view := scanView(text)
	if explicitOverride.MatchString(view) {
		return true
	}
	for _, line := range strings.Split(view, "\n") {
		for start := 0; start < len(line); start += 768 {
			part := line[start:min(len(line), start+1024)]
			if aiReader.MatchString(part) && aiDirective.MatchString(part) && (aiAddress.MatchString(part) || (aiResponseOrder.MatchString(part) && aiResponseTarget.MatchString(part))) {
				return true
			}
			if responseLink.MatchString(part) && responseAppend.MatchString(part) && strings.Contains(part, "https://") {
				return true
			}
		}
	}
	return false
}
