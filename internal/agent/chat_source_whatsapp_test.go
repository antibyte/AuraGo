package agent

import (
	"database/sql"
	"testing"
)

// WhatsApp owner chats are a chat source exactly like Telegram: every switch that lists
// "telegram" must list "whatsapp" next to it.
func TestWhatsAppIsAChatSourceWhereverTelegramIs(t *testing.T) {
	for _, source := range []string{"telegram", "whatsapp"} {
		t.Run(source, func(t *testing.T) {
			if !shouldUseSupervisorToolRoute(RunConfig{MessageSource: source}) {
				t.Error("shouldUseSupervisorToolRoute = false, want true")
			}
			if !taskRouterEligible(RunConfig{MessageSource: source}) {
				t.Error("taskRouterEligible = false, want true")
			}
			if !shouldConsiderOperationalIssueReminder(RunConfig{MessageSource: source, PlannerDB: &sql.DB{}}, "status?") {
				t.Error("shouldConsiderOperationalIssueReminder = false, want true")
			}
			if !isShortChannel(source) {
				t.Error("isShortChannel = false, want true")
			}
		})
	}
	if got, want := channelToneHint("whatsapp"), channelToneHint("telegram"); got == "" || got != want {
		t.Errorf("channelToneHint(whatsapp) = %q, want the Telegram hint %q", got, want)
	}
}
