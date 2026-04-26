package heartbeat

import (
	"fmt"
	"strings"
	"time"

	"aurago/internal/config"
)

func buildHeartbeatPrompt(hb config.HeartbeatConfig, now time.Time) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("[SYSTEM HEARTBEAT] Automated wake-up check at %s on %s.", now.Format("15:04"), now.Format("Monday, January 2")))
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

	// Proactive guidance: time-based suggestions
	hour := now.Hour()
	switch {
	case hour >= 6 && hour < 9:
		parts = append(parts, "It's early morning — consider checking for overnight events, pending messages, or tasks due today.")
	case hour >= 11 && hour < 13:
		parts = append(parts, "It's around midday — a good time to check if morning tasks are on track and afternoon priorities are clear.")
	case hour >= 17 && hour < 19:
		parts = append(parts, "It's late afternoon — consider summarizing today's progress and checking for any remaining urgent items.")
	case hour >= 22 || hour < 5:
		parts = append(parts, "It's late — focus only on critical or time-sensitive items. Non-urgent matters can wait until morning.")
	}

	if hb.AdditionalPrompt != "" {
		parts = append(parts, "")
		parts = append(parts, "Additional user instructions:")
		parts = append(parts, hb.AdditionalPrompt)
		parts = append(parts, "")
	}

	parts = append(parts, "")
	parts = append(parts, "If nothing requires immediate action, simply confirm that all is well with a brief status summary.")
	parts = append(parts, "This heartbeat is a read-only status check by default. Do not edit homepage or project files, do not build or deploy websites, and do not change external systems unless the additional user instructions explicitly request that action.")
	parts = append(parts, "If you find items needing attention, report the issue instead of making broad changes. Create reminders, missions, or notifications only when they are clearly necessary and low risk.")
	parts = append(parts, "Do NOT ask the user questions — this is an autonomous background check.")
	parts = append(parts, "Be concise and efficient — avoid unnecessary tool calls if the situation is stable.")

	return strings.Join(parts, "\n")
}
