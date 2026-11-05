package flows

import "context"

// requireFlow and requireMissionFlow keep ordinary flow operations from taking
// ownership of a reusable block, even through direct service calls.
func (s *Service) requireFlow(ctx context.Context, id string) (*FlowRecord, error) {
	rec, err := s.store.GetFlow(ctx, id)
	if err != nil {
		return nil, err
	}
	if rec.Kind != KindFlow {
		return nil, ErrWrongFlowKind
	}
	return rec, nil
}

func (s *Service) requireMissionFlow(ctx context.Context, missionID string) (*FlowRecord, error) {
	rec, err := s.store.GetFlowByMission(ctx, missionID)
	if err != nil {
		return nil, err
	}
	if rec.Kind != KindFlow {
		return nil, ErrWrongFlowKind
	}
	return rec, nil
}
