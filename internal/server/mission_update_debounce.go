package server

import (
	"sync"
	"time"
)

// missionUpdateDebounce is the window of broadcastMissionState: the first call sends at once,
// later calls inside the window fold into one send at its end, which reads the state then.
const missionUpdateDebounce = 250 * time.Millisecond

// missionUpdateDebouncer folds mission_update broadcasts (leading and trailing edge). The zero
// value is ready to use.
type missionUpdateDebouncer struct {
	mu      sync.Mutex
	open    bool // a window is running
	pending bool // a call arrived inside it
	// schedule runs fn after d; nil means time.AfterFunc. Tests replace it.
	schedule func(d time.Duration, fn func())
}

// fire sends now when no window is open, else marks the window's trailing send.
func (d *missionUpdateDebouncer) fire(send func()) {
	d.mu.Lock()
	if d.open {
		d.pending = true
		d.mu.Unlock()
		return
	}
	d.open = true
	d.mu.Unlock()
	send()
	d.arm(send)
}

// arm starts a window; at its end a pending call sends and starts the next window.
func (d *missionUpdateDebouncer) arm(send func()) {
	schedule := d.schedule
	if schedule == nil {
		schedule = func(w time.Duration, fn func()) { time.AfterFunc(w, fn) }
	}
	schedule(missionUpdateDebounce, func() {
		d.mu.Lock()
		if !d.pending {
			d.open = false
			d.mu.Unlock()
			return
		}
		d.pending = false
		d.mu.Unlock()
		send()
		d.arm(send)
	})
}
