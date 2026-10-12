package server

import (
	"context"
	"sync"

	"aurago/internal/agent"
	"aurago/internal/persondetect"
)

const (
	// personAlertAgentQueueSize bounds the waiting agent wake-ups of person detection (spec
	// "Alerts": bounded queue of 16, single consumer).
	personAlertAgentQueueSize = 16
	// personAlertSessionID is the session and the message source of camera detection runs.
	personAlertSessionID = "camera_detection"
)

// personAlertAgentQueue holds the agent wake-ups of person detection alerts. At most
// personAlertAgentQueueSize prompts wait; one consumer hands them to the agent loop one
// after another, like the Uptime Kuma relay, so a burst of detections never becomes a burst
// of agent runs. The zero value is ready to use: prompts enqueued before start wait for it.
type personAlertAgentQueue struct {
	mu      sync.Mutex
	queue   chan string
	cancel  context.CancelFunc
	done    chan struct{}
	started bool
	closed  bool
}

func (q *personAlertAgentQueue) channelLocked() chan string {
	if q.queue == nil {
		q.queue = make(chan string, personAlertAgentQueueSize)
	}
	return q.queue
}

// enqueue is the Agent sink of the dispatcher: persondetect.ErrAgentQueueFull when the queue
// is full, skipped "agent unavailable" after stop.
func (q *personAlertAgentQueue) enqueue(prompt string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return persondetect.Skipped("agent unavailable")
	}
	select {
	case q.channelLocked() <- prompt:
		return nil
	default:
		return persondetect.ErrAgentQueueFull
	}
}

// start runs the single consumer until parent ends or stop is called. Every prompt gets a
// turn of at most five minutes (consumeUptimeKumaNotifications). A second start, or a start
// after stop, does nothing.
func (q *personAlertAgentQueue) start(parent context.Context, deliver func(context.Context, string)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || q.started {
		return
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	q.started, q.cancel, q.done = true, cancel, make(chan struct{})
	queue, done := q.channelLocked(), q.done
	go func() {
		defer close(done)
		consumeUptimeKumaNotifications(ctx, queue, deliver)
	}()
}

// stop ends the consumer and waits for it, the running turn included (its context is
// cancelled). Waiting prompts are dropped; later enqueues are skipped. stop is idempotent.
func (q *personAlertAgentQueue) stop() {
	q.mu.Lock()
	q.closed = true
	cancel, done := q.cancel, q.done
	q.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

// buildCameraDetectionRunConfig is the run config of camera detection wake-ups: the Uptime
// Kuma relay's (same dependencies, read when the wake-up runs) in session and source
// camera_detection, with turn side effects (memory, journal, personality) suppressed like
// the Frigate relay.
func (s *Server) buildCameraDetectionRunConfig() agent.RunConfig {
	run := s.buildUptimeKumaRunConfig()
	run.SessionID = personAlertSessionID
	run.MessageSource = personAlertSessionID
	run.SuppressTurnSideEffects = true
	return run
}

// deliverPersonAlertPrompt runs one wake-up through the agent loop.
func (s *Server) deliverPersonAlertPrompt(ctx context.Context, prompt string) {
	agent.LoopbackContext(ctx, s.buildCameraDetectionRunConfig(), prompt, agent.NoopBroker{})
}
