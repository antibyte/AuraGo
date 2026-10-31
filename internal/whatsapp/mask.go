package whatsapp

import (
	"regexp"

	"go.mau.fi/whatsmeow/types"
)

var longDigitRun = regexp.MustCompile(`\d{5,}`)

// maskDigits keeps the last four digits of every run of five or more digits,
// so phone numbers and JID users never reach logs or status in full.
func maskDigits(s string) string {
	return longDigitRun.ReplaceAllStringFunc(s, func(run string) string {
		return "***" + run[len(run)-4:]
	})
}

// maskJID returns the masked user part of a phone or LID JID, or "".
func maskJID(jid types.JID) string {
	if jid.IsEmpty() {
		return ""
	}
	return maskDigits(jid.User)
}
