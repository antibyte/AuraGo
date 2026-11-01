package memory

import "testing"

// TestCoreMemoryPolicyTreatsWhatsAppLikeTelegram keeps WhatsApp on the same footing as
// Telegram in the two core-memory tool rules: a tool-state claim and an "always call" order.
func TestCoreMemoryPolicyTreatsWhatsAppLikeTelegram(t *testing.T) {
	for _, channel := range []string{"telegram", "whatsapp"} {
		if !isCoreMemoryToolStateClaim(channel + " integration is unavailable") {
			t.Errorf("%s state claim must be recognized", channel)
		}
		if got := coreMemoryInstructionRule("always call " + channel + " for every answer"); got != "always_call" {
			t.Errorf("%s always-call order = %q, want always_call", channel, got)
		}
	}
	// The claim needs a state term, so a plain mention stays a durable fact.
	if isCoreMemoryToolStateClaim("the owner chats on whatsapp every evening") {
		t.Error("a plain whatsapp mention must not read as a tool-state claim")
	}
}
