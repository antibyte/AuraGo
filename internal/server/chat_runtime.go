package server

import (
	"aurago/internal/config"
	"aurago/internal/llm"
)

// chatRuntimeSnapshot supplies long-lived bots with a matching config/client
// pair for each new turn. Accepted turns keep that detached config throughout.
func (s *Server) chatRuntimeSnapshot() (*config.Config, llm.ChatClient) {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	current := s.ConfigSnapshot()
	if current == nil {
		return nil, nil
	}
	return current.Clone(), s.LLMClient
}
