package security

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// QuarantineReason is a local classification, never a model-written explanation.
type QuarantineReason string

const (
	QuarantineSuspicious  QuarantineReason = "suspicious"
	QuarantineIncomplete  QuarantineReason = "incomplete"
	QuarantineUnavailable QuarantineReason = "unavailable"
)

func quarantineReasonText(reason QuarantineReason) string {
	switch reason {
	case QuarantineIncomplete:
		return "security inspection was incomplete or exceeded its size limit"
	case QuarantineUnavailable:
		return "the security scanner was unavailable"
	default:
		return "the security inspection identified suspicious content"
	}
}

// ContentScanQuarantine never inherits the permissive tool-execution fail-safe.
func ContentScanQuarantine(reason QuarantineReason) GuardianResult {
	if reason != QuarantineIncomplete && reason != QuarantineUnavailable {
		reason = QuarantineSuspicious
	}
	return GuardianResult{Decision: DecisionQuarantine, RiskScore: 0.5,
		Reason: quarantineReasonText(reason), QuarantineReason: reason}
}

// QuarantineNotice replaces content only at an already-authorized delivery target.
// References are identifiers, not a way to echo attacker-authored prompt text.
func QuarantineNotice(source, reference string, result GuardianResult) string {
	safeReference := reference != "" && len(reference) <= 160
	for _, r := range reference {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.:/@", r)) {
			safeReference = false
			break
		}
	}
	if !safeReference {
		sum := sha256.Sum256([]byte(reference))
		reference = fmt.Sprintf("sha256:%x", sum[:12])
	}
	return "[QUARANTINE NOTICE]\nAn incoming item intended for you was quarantined. " +
		"Reason: " + quarantineReasonText(result.QuarantineReason) + ". " +
		"Its original content was withheld. This notice does not authorize bypassing quarantine.\n" +
		IsolateExternalData("Source: "+truncateUTF8Prefix(source, 80)+"\nReference: "+reference)
}
