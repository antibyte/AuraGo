package agent

import (
	"strings"

	"aurago/internal/prompts"
)

func isAutonomousMessageSource(source string) bool {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "heartbeat", "planner_notification", "uptime_kuma", "space_agent_bridge", "follow_up", "cron", "cron_prompt":
		return true
	default:
		return false
	}
}

func isAutonomousAgentRun(runCfg RunConfig, sessionID string) bool {
	if runCfg.IsMission || strings.EqualFold(strings.TrimSpace(runCfg.MessageSource), "mission") {
		return true
	}
	return isAutonomousMessageSource(runCfg.MessageSource) || sessionID == "heartbeat"
}

func shouldRunTurnSideEffects(runCfg RunConfig, sessionID string, flags prompts.ContextFlags) bool {
	if runCfg.SuppressTurnSideEffects {
		return false
	}
	if flags.IsMission || flags.IsCoAgent || runCfg.IsMission || runCfg.IsCoAgent {
		return false
	}
	if runCfg.IsMaintenance || sessionID == "maintenance" {
		return false
	}
	return !isAutonomousAgentRun(runCfg, sessionID)
}
