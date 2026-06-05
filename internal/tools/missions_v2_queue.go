package tools

import (
	"sort"
	"sync"
	"time"
)

type QueueItem struct {
	MissionID          string    `json:"mission_id"`
	Priority           int       `json:"priority"` // 3=high, 2=medium, 1=low
	EnqueuedAt         time.Time `json:"enqueued_at"`
	TriggerType        string    `json:"trigger_type,omitempty"`
	TriggerData        string    `json:"trigger_data,omitempty"`         // JSON data from trigger
	ExtraCheatsheetIDs []string  `json:"extra_cheatsheet_ids,omitempty"` // transient cheatsheet IDs injected by daemon
	ExtraPromptSuffix  string    `json:"extra_prompt_suffix,omitempty"`  // transient prompt augmentation from daemon
}

type missionQueueSnapshot struct {
	Items   []QueueItem `json:"items"`
	Running string      `json:"running,omitempty"`
}

// MissionQueue manages the execution queue with priority sorting
type MissionQueue struct {
	mu      sync.Mutex
	items   []QueueItem
	running string // ID of currently running mission
}

// NewMissionQueue creates a new mission queue
func NewMissionQueue() *MissionQueue {
	return &MissionQueue{
		items: []QueueItem{},
	}
}

// Enqueue adds a mission to the queue
func (q *MissionQueue) Enqueue(missionID string, priority string, triggerType, triggerData string) {
	q.enqueueItem(QueueItem{
		MissionID:   missionID,
		Priority:    prioFromString(priority),
		EnqueuedAt:  time.Now(),
		TriggerType: triggerType,
		TriggerData: triggerData,
	})
}

// EnqueueWithExtras adds a mission to the queue with transient daemon extras.
func (q *MissionQueue) EnqueueWithExtras(missionID string, priority int, triggerType, triggerData string, extraCheatsheetIDs []string, extraPromptSuffix string) {
	q.enqueueItem(QueueItem{
		MissionID:          missionID,
		Priority:           priority,
		EnqueuedAt:         time.Now(),
		TriggerType:        triggerType,
		TriggerData:        triggerData,
		ExtraCheatsheetIDs: extraCheatsheetIDs,
		ExtraPromptSuffix:  extraPromptSuffix,
	})
}

func (q *MissionQueue) enqueueItem(item QueueItem) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, existing := range q.items {
		if existing.MissionID == item.MissionID {
			return
		}
	}

	q.items = append(q.items, item)
	q.sort()
}

func prioFromString(p string) int {
	switch p {
	case "high":
		return 3
	case "low":
		return 1
	default:
		return 2
	}
}

// Dequeue removes and returns the next mission to run
func (q *MissionQueue) Dequeue() (QueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return QueueItem{}, false
	}

	item := q.items[0]
	q.items = q.items[1:]
	q.running = item.MissionID
	return item, true
}

// TryStartNext atomically claims the next queued mission if no mission is
// currently running. This avoids a race between GetRunning and Dequeue.
func (q *MissionQueue) TryStartNext() (QueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.running != "" || len(q.items) == 0 {
		return QueueItem{}, false
	}

	item := q.items[0]
	q.items = q.items[1:]
	q.running = item.MissionID
	return item, true
}

// sort orders items by priority (high first), then by enqueue time
func (q *MissionQueue) sort() {
	sort.Slice(q.items, func(i, j int) bool {
		if q.items[i].Priority != q.items[j].Priority {
			return q.items[i].Priority > q.items[j].Priority
		}
		return q.items[i].EnqueuedAt.Before(q.items[j].EnqueuedAt)
	})
}

// Done marks the current mission as finished
func (q *MissionQueue) Done() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.running = ""
}

// GetRunning returns the ID of the currently running mission
func (q *MissionQueue) GetRunning() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.running
}

// List returns all queued items
func (q *MissionQueue) List() []QueueItem {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]QueueItem, len(q.items))
	copy(out, q.items)
	return out
}

func (q *MissionQueue) Restore(items []QueueItem, running string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.items = q.items[:0]
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.MissionID == "" {
			continue
		}
		if _, ok := seen[item.MissionID]; ok {
			continue
		}
		seen[item.MissionID] = struct{}{}
		q.items = append(q.items, item)
	}
	q.running = running
	q.sort()
}

func (q *MissionQueue) Snapshot() ([]QueueItem, string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	out := make([]QueueItem, len(q.items))
	copy(out, q.items)
	return out, q.running
}

// Remove removes a specific mission from the queue
func (q *MissionQueue) Remove(missionID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, item := range q.items {
		if item.MissionID == missionID {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return true
		}
	}
	return false
}

// MissionManagerV2 provides enhanced mission management with triggers and queue
