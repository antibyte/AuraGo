package agent

import "github.com/sashabaranov/go-openai"

// A duplicate block already follows repeated identical calls. Allow a bounded
// opportunity to use a different operation, without spending the entire job on
// repeated circuit-breaker feedback or switching to textual pseudo-tool calls.
const maxGameMakerDuplicateBlocks = 3

func (s *agentLoopState) blockDuplicateToolCall(tc ToolCall) bool {
	if !s.recoveryState.handleDuplicateToolCall(tc, &s.req, s.currentLogger, s.telemetryScope) {
		return false
	}
	if s.runCfg.MessageSource == "game_maker" {
		s.gameMakerDuplicateBlocks++
		if n := len(s.req.Messages); n > 0 && s.req.Messages[n-1].Role == openai.ChatMessageRoleSystem {
			s.req.Messages[n-1].Content = "CIRCUIT BREAKER: This identical request was already attempted; it was not executed again. Use the previous result and take a different concrete step with the supplied Game Maker tools. Re-read files only after changing them. Do not restart planning or repeat this request. Further blocked repetitions will stop this run with its progress saved."
		}
	}
	return true
}

func (s *agentLoopState) restrictToolsAfterCircuitBreaker() {
	if s.runCfg.MessageSource == "game_maker" {
		// The duplicate signature remains blocked by toolRecoveryState. Other
		// operations must stay usable through the same native tool catalog.
		s.req.ToolChoice = nil
		return
	}
	s.req.Tools = nil
	s.req.ToolChoice = "none"
}

func (s *agentLoopState) noteGameMakerToolProgress(failed, blocked bool) {
	if !failed && !blocked {
		// A successful different step is progress. Failed or skipped calls must
		// not buy another series of duplicate-feedback/model rounds.
		s.gameMakerDuplicateBlocks = 0
	}
}
