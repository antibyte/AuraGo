package embeddings

import (
	"sync"
	"time"
)

const (
	// maxProcessRestarts is how many times a supervised embedding process
	// (ONNX worker, llama.cpp server or sidecar) may be restarted inside
	// processRestartWindow. A lifetime cap would turn one bad hour into a
	// permanent outage, while a sliding window still stops a crash loop.
	maxProcessRestarts   = 3
	processRestartWindow = 10 * time.Minute
)

// restartBudget is a sliding-window limiter for process restarts. The zero
// value is not usable; construct it with newRestartBudget.
type restartBudget struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	taken  []time.Time
}

func newRestartBudget(limit int, window time.Duration) *restartBudget {
	return newRestartBudgetWithClock(limit, window, time.Now)
}

// newRestartBudgetWithClock lets tests drive the window without sleeping.
func newRestartBudgetWithClock(limit int, window time.Duration, now func() time.Time) *restartBudget {
	if now == nil {
		now = time.Now
	}
	return &restartBudget{limit: limit, window: window, now: now}
}

// take consumes one restart and reports whether the budget allowed it.
func (budget *restartBudget) take() bool {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	now := budget.now()
	budget.pruneLocked(now)
	if len(budget.taken) >= budget.limit {
		return false
	}
	budget.taken = append(budget.taken, now)
	return true
}

// release gives back the most recent restart. It is for restarts that were cut
// short by the caller's own context, which say nothing about the process.
func (budget *restartBudget) release() {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if len(budget.taken) > 0 {
		budget.taken = budget.taken[:len(budget.taken)-1]
	}
}

// remaining returns how many restarts the window still allows.
func (budget *restartBudget) remaining() int {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	budget.pruneLocked(budget.now())
	return budget.limit - len(budget.taken)
}

func (budget *restartBudget) pruneLocked(now time.Time) {
	kept := budget.taken[:0]
	for _, taken := range budget.taken {
		if now.Sub(taken) < budget.window {
			kept = append(kept, taken)
		}
	}
	budget.taken = kept
}
