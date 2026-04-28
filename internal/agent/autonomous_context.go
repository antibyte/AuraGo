package agent

import "aurago/internal/prompts"

func isAutonomousAgentRun(runCfg RunConfig, sessionID string) bool {
	return runCfg.MessageSource == "heartbeat" || sessionID == "heartbeat"
}

func shouldRunTurnSideEffects(runCfg RunConfig, sessionID string, flags prompts.ContextFlags) bool {
	if flags.IsMission || flags.IsCoAgent || runCfg.IsMission || runCfg.IsCoAgent {
		return false
	}
	return !isAutonomousAgentRun(runCfg, sessionID)
}
