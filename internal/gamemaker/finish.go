package gamemaker

import "context"

// FinishJob records an explicit request to publish this exact build.
func (s *Service) FinishJob(ctx context.Context, jobID string) BuildResult {
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	result := s.BuildJob(ctx, jobID)
	if result.OK {
		s.mu.Lock()
		s.finishedBuilds[jobID] = result
		s.mu.Unlock()
	}
	return result
}

func (s *Service) JobFinished(jobID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.finishedBuilds[jobID]
	return ok
}
