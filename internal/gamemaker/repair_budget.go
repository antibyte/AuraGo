package gamemaker

// Repair accounting lives in one place so the repair loop, the mutation gate,
// the visual repair and the disclosed remaining budget agree. A job may override
// the default; the override is clamped to a bounded range.
const (
	defaultRepairPasses = 3
	maxRepairPasses     = 6
)

func clampRepairPasses(value int) int {
	if value <= 0 {
		return defaultRepairPasses
	}
	if value < 1 {
		return 1
	}
	if value > maxRepairPasses {
		return maxRepairPasses
	}
	return value
}

// repairLimitLocked requires s.mu to be held.
func (s *Service) repairLimitLocked(jobID string) int {
	if limit, ok := s.repairLimits[jobID]; ok && limit > 0 {
		return limit
	}
	return defaultRepairPasses
}

func (s *Service) repairLimit(jobID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.repairLimitLocked(jobID)
}
