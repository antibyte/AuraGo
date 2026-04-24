package server

import "aurago/internal/services"

func (s *Server) attachFileKGSyncer() {
	if s == nil || s.FileIndexer == nil {
		return
	}
	if s.Cfg == nil || s.ShortTermMem == nil || s.LongTermMem == nil || s.KG == nil {
		return
	}
	s.FileIndexer.SetKGSyncer(services.NewFileKGSyncer(
		s.Cfg,
		s.Logger,
		s.LLMClient,
		s.LongTermMem,
		s.ShortTermMem,
		s.KG,
	))
}
