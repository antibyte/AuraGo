package tools

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// errFlowMissionNotOwnable refuses a flow mission in QueueOwnedMission. The text contains
// "not supported" so the mission API answers 400.
var errFlowMissionNotOwnable = fmt.Errorf("queueing a flow mission for the agent is not supported; start it with RunNow or TriggerMission")

// QueueOwnedMission transfers a cancellable invocation to the queue. Its owner
// is runtime-only: persisted invocations without that owner cannot be replayed.
// release is called on rejection, removal or after the invocation returns.
func (m *MissionManagerV2) QueueOwnedMission(ctx context.Context, release context.CancelFunc, id, trigger, data string) (err error) {
	ownerContext, cancelOwner := context.WithCancel(ctx)
	stopShutdown := context.AfterFunc(m.ctx, cancelOwner)
	originalRelease := release
	var ownerMu sync.Mutex
	owners := 0
	retain := func() context.CancelFunc {
		ownerMu.Lock()
		owners++
		ownerMu.Unlock()
		var once sync.Once
		return func() {
			once.Do(func() {
				ownerMu.Lock()
				owners--
				last := owners == 0
				ownerMu.Unlock()
				if !last {
					return
				}
				stopShutdown()
				cancelOwner()
				if originalRelease != nil {
					originalRelease()
				}
			})
		}
	}
	release = retain()
	ctx = ownerContext
	accepted := false
	defer func() {
		if !accepted {
			release()
		}
	}()
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = requireMissionMutationPermission(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx.Err() != nil {
		return fmt.Errorf("mission manager is stopped")
	}
	mission, ok := m.missions[id]
	if !ok {
		return fmt.Errorf("mission not found")
	}
	if !mission.Enabled {
		return fmt.Errorf("mission is disabled")
	}
	// A flow mission never enters the agent queue (the dispatcher would drop it): its runs
	// start in the flow service through RunNow or TriggerMission.
	if isFlowMission(mission) {
		return errFlowMissionNotOwnable
	}
	// The remote protocol currently has no cancellation acknowledgment. Never
	// launch detached remote execution on behalf of a revocable Desktop owner.
	if isRemoteMission(mission) {
		return fmt.Errorf("remote missions do not support revocable desktop execution")
	}
	if trigger != "manual" && mission.ExecutionType == ExecutionTriggered && !m.shouldFireTriggerLocked(mission, trigger, time.Now()) {
		return &MissionTriggerSkippedError{Reason: "rate_limited"}
	}
	priority := prioFromString(mission.Priority)
	if trigger == "manual" {
		priority = prioFromString("high")
	}
	item := QueueItem{MissionID: id, Priority: priority, EnqueuedAt: time.Now(), TriggerType: trigger, TriggerData: data, RequiresOwner: true, ownerContext: ctx, releaseOwner: release, retainOwner: retain}
	if !m.queue.enqueueItem(item) {
		return fmt.Errorf("mission is already queued or running")
	}
	previousStatus := mission.Status
	mission.Status = MissionStatusQueued
	// Persist the non-replayable owner marker before any recoverable queued status.
	if err = m.saveQueueLocked(); err == nil {
		err = m.save()
	}
	if err != nil {
		m.queue.Remove(id)
		mission.Status = previousStatus
		_ = m.saveQueueLocked()
		return err
	}
	accepted = true
	return nil
}

// ActiveOwnerContext is only set by server-owned queue admission, never by
// mission prompt text, request headers or a client-supplied origin field.
func (m *MissionManagerV2) ActiveOwnerContext(id string) (context.Context, bool) {
	m.queue.mu.Lock()
	defer m.queue.mu.Unlock()
	item := m.queue.runningItem
	return item.ownerContext, m.queue.running == id && item.RequiresOwner
}

func (q *MissionQueue) persistedSnapshot() missionQueueSnapshot {
	q.mu.Lock()
	defer q.mu.Unlock()
	ids := make([]string, 0, len(q.nonReplayable))
	for id := range q.nonReplayable {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return missionQueueSnapshot{Items: append([]QueueItem{}, q.items...), Running: q.running, RunningRequiresOwner: q.runningItem.RequiresOwner, NonReplayableIDs: ids}
}

// Keep markers after completion until a deliberate independent invocation
// supersedes them. A crash between queue and mission-status saves cannot replay
// an ownerless invocation from an older queued/running status.
func (q *MissionQueue) restoreNonReplayable(ids []string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.nonReplayable = make(map[string]bool, len(ids))
	for _, id := range ids {
		q.nonReplayable[id] = true
	}
}

func (q *MissionQueue) activeItem(id string) QueueItem {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.running == id {
		return q.runningItem
	}
	return QueueItem{}
}
