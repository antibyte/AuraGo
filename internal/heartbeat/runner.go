package heartbeat

import (
	"fmt"
	"strings"
	"time"

	"aurago/internal/config"
)

func buildHeartbeatPrompt(hb config.HeartbeatConfig, now time.Time) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("[SYSTEM HEARTBEAT] Automated wake-up check at %s.", now.Format("15:04")))
	parts = append(parts, "")
	parts = append(parts, "You are being woken up by the Heartbeat scheduler to perform a background status check.")
	parts = append(parts, "")

	var checks []string
	if hb.CheckTasks {
		checks = append(checks, "- Check for any open tasks, missions, or to-dos that need attention.")
	}
	if hb.CheckAppointments {
		checks = append(checks, "- Check for any upcoming appointments, calendar events, or reminders.")
	}
	if hb.CheckEmails {
		checks = append(checks, "- Check configured email accounts for new messages.")
	}

	if len(checks) > 0 {
		parts = append(parts, "Please perform the following checks:")
		parts = append(parts, checks...)
		parts = append(parts, "")
	}

	if hb.AdditionalPrompt != "" {
		parts = append(parts, "Additional user instructions:")
		parts = append(parts, hb.AdditionalPrompt)
		parts = append(parts, "")
	}

	parts = append(parts, "If nothing requires immediate action, simply confirm that all is well with a brief status summary.")
	parts = append(parts, "If you find items needing attention, create appropriate reminders, missions, or notifications.")
	parts = append(parts, "Do NOT ask the user questions — this is an autonomous background check.")

	return strings.Join(parts, "\n")
}
