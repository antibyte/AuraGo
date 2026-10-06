package agent

import (
	"encoding/json"
	"strings"

	"aurago/internal/security"
	"aurago/internal/tools"
)

// questionUserBlockedMessage tells the model why an answer was withheld and
// what to do next, without echoing the answer.
const questionUserBlockedMessage = "answer withheld by the guardian: the free-text reply matched prompt-injection patterns; " +
	"ask the user to rephrase without instruction-like wording, or to pick an option"

// questionAnswerFromAdminSurface reports whether a question_user answer was
// completed from an owner-only surface. Those surfaces only log high-threat
// chat text, so their answers are isolated but never withheld. Every other
// source, including an empty one, is untrusted.
func questionAnswerFromAdminSurface(source tools.QuestionSource) bool {
	switch source {
	case tools.QuestionSourceWeb, tools.QuestionSourceDesktop:
		return true
	default:
		return false
	}
}

// screenQuestionFreeText formats a completed question_user answer as tool
// output and reports whether the answer was withheld.
//
// The free text is chat from the web or desktop chat, Telegram, Discord or SMS,
// or text relayed by an internal loopback turn, and none of those paths ran it
// through the per-channel guardian, so it is scanned and isolated here. A
// high-threat answer is withheld unless an admin surface sent it, where the
// scan only logs as ScanUserInput does. For SMS (ordinary chat is only
// isolated) and internal relays (follow-up turns are only logged) this is
// deliberately stricter than their ordinary chat path. Selected options are
// model-authored values and pass through unchanged.
func screenQuestionFreeText(dc *DispatchContext, sessionID string, response tools.QuestionResponse) (string, bool) {
	if strings.TrimSpace(response.FreeText) != "" {
		if dc.Guardian != nil {
			if scan := dc.Guardian.ScanForInjection(response.FreeText); scan.Level >= security.ThreatHigh {
				admin := questionAnswerFromAdminSurface(response.Source)
				if dc.Logger != nil {
					msg := "[Guardian] Blocked free-text answer to question_user"
					if admin {
						msg = "[Guardian] High-threat free-text answer to question_user from an admin surface, isolating without blocking"
					}
					dc.Logger.Warn(msg, "session_id", sessionID, "source", string(response.Source),
						"threat", scan.Level.String(), "patterns", scan.Patterns)
				}
				if !admin {
					b, _ := json.Marshal(struct {
						tools.QuestionResponse
						Message string `json:"message"`
					}{tools.QuestionResponse{Status: "blocked"}, questionUserBlockedMessage})
					return "Tool Output: " + string(b), true
				}
			}
		}
		// json.Marshal HTML-escapes the angle brackets of the boundary
		// tags, so the StripThinkingTags pass on tool output keeps them.
		response.FreeText = security.IsolateExternalData(response.FreeText)
	}
	b, _ := json.Marshal(response)
	return "Tool Output: " + string(b), false
}
