package server

import (
	"context"
	"hash/fnv"
	"sync"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

// Fixed lock striping bounds memory independently of the number of retained runs.
var durableProjectionLocks [64]sync.Mutex

var _ flows.DurableHistoryProjector = flowMissionBridge{}

// ProjectDurableRun rereads the committed source while serializing this run's
// projections. It never invokes the ordinary mission completion bridge.
func (b flowMissionBridge) ProjectDurableRun(ctx context.Context, runID string) error {
	if b.s == nil || b.s.Flows == nil || b.s.MissionManagerV2 == nil {
		return flows.ErrDurableUnavailable
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(runID))
	mu := &durableProjectionLocks[h.Sum64()%uint64(len(durableProjectionLocks))]
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	projection, err := b.s.Flows.DurableProjection(ctx, runID)
	if err != nil {
		return err
	}
	if projection == nil {
		return flows.ErrDurableUnavailable
	}
	return b.s.MissionManagerV2.ProjectDurableFlowRun(ctx, tools.DurableMissionHistory{
		RunID: projection.RunID, FlowID: projection.FlowID, MissionID: projection.MissionID,
		Status: string(projection.Status), StartedAt: projection.StartedAt,
		FinishedAt: projection.FinishedAt, ErrorCode: projection.ErrorCode,
	})
}
