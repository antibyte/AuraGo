package dockerstacks

import (
	"context"
	"sync"
	"time"
	"unicode/utf8"
)

// Event types of an operation's live stream.
const (
	EventOutput   = "output"
	EventProgress = "progress"
	EventState    = "state"
	EventDone     = "done"
)

const (
	// subscriberBuffer is how many events a subscriber may fall behind before it
	// is dropped; the client then reconnects and gets a fresh snapshot.
	subscriberBuffer = 256
	// flushInterval is how often the output of a running operation is written to its row.
	flushInterval = 2 * time.Second
	journalWrite  = 10 * time.Second
)

type subscriber struct{ ch chan Event }

// outcome is how an operation ended.
type outcome struct {
	state     string // StateSucceeded | StateFailed | StateInterrupted
	code      string
	message   string
	exit      *int
	errorLine int
	warning   string
	ports     []PortLink
	abandoned bool   // the service was closed: leave the operation as it is
	cleanup   func() // runs after the final state is recorded (removes the runner)
}

// opRun is the live state of one operation: the journal row it mirrors, the
// output tail, and the subscribers of its event stream.
type opRun struct {
	s *Service

	mu        sync.Mutex
	op        Operation
	tail      []byte
	seq       int64
	subs      map[*subscriber]struct{}
	closed    bool // finished or abandoned: no more events
	lastFlush time.Time
	lastLine  map[string]string // last output line per progress item id
	scan      errorScan
}

func newOpRun(s *Service, op Operation) *opRun {
	r := &opRun{s: s, op: op, subs: map[*subscriber]struct{}{}, lastLine: map[string]string{}, lastFlush: s.opts.Now()}
	r.tail = []byte(op.Output)
	r.trimTail(OutputTailBytes)
	r.op.Output = ""
	return r
}

// trimTail keeps at most limit bytes, cutting at a character boundary.
func (r *opRun) trimTail(limit int) {
	if len(r.tail) <= limit {
		return
	}
	cut := len(r.tail) - limit
	for cut < len(r.tail) && !utf8.RuneStart(r.tail[cut]) {
		cut++
	}
	r.tail = append(r.tail[:0], r.tail[cut:]...)
}

func (r *opRun) tailText() string {
	tail := r.tail
	if len(tail) > OutputTailBytes {
		cut := len(tail) - OutputTailBytes
		for cut < len(tail) && !utf8.RuneStart(tail[cut]) {
			cut++
		}
		tail = tail[cut:]
	}
	return string(tail)
}

// snapshot is a copy of the operation with the output so far.
func (r *opRun) snapshot() Operation {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked(true)
}

func (r *opRun) snapshotLocked(withOutput bool) Operation {
	op := r.op
	if withOutput {
		op.Output = r.tailText()
	} else {
		op.Output = ""
	}
	if op.ExitCode != nil {
		code := *op.ExitCode
		op.ExitCode = &code
	}
	op.Ports = append([]PortLink(nil), op.Ports...)
	return op
}

// publishLocked numbers an event and hands it to every subscriber without
// blocking. A subscriber whose buffer is full is dropped.
func (r *opRun) publishLocked(ev Event) {
	r.seq++
	ev.Seq = r.seq
	for sub := range r.subs {
		select {
		case sub.ch <- ev:
		default:
			delete(r.subs, sub)
			close(sub.ch)
		}
	}
}

// persistLocked writes the row; a failing journal is logged, never fatal.
func (r *opRun) persistLocked() {
	op := r.snapshotLocked(true)
	ctx, cancel := context.WithTimeout(context.Background(), journalWrite)
	defer cancel()
	if err := r.s.journal.save(ctx, op); err != nil {
		r.s.log.Warn("docker operation could not be saved", "operation", op.ID, "error", err)
	}
	r.lastFlush = r.s.opts.Now()
}

// addOutput appends text to the output and publishes it. The row is rewritten
// at most every flushInterval.
func (r *opRun) addOutput(text string) {
	if text == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addOutputLocked(text)
}

func (r *opRun) addOutputLocked(text string) {
	if r.closed {
		return
	}
	r.tail = append(r.tail, text...)
	r.trimTail(2 * OutputTailBytes)
	r.publishLocked(Event{Type: EventOutput, Text: text})
	if r.s.opts.Now().Sub(r.lastFlush) >= flushInterval {
		r.persistLocked()
	}
}

// line appends one line of AuraGo's own commentary to the output.
func (r *opRun) line(text string) { r.addOutput(text + "\n") }

// progress publishes one progress item.
func (r *opRun) progress(p Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progressLocked(p)
}

func (r *opRun) progressLocked(p Progress) {
	if r.closed {
		return
	}
	r.publishLocked(Event{Type: EventProgress, Progress: &p})
}

// composeLine takes one raw line of the runner's output. A line that is a
// JSON progress item becomes a progress event plus a readable output line
// (byte ticks and repeats of the same text for the same item stay out of the
// output); every other line goes to the output as it is.
func (r *opRun) composeLine(raw string) {
	if hook := r.s.rawLine; hook != nil {
		hook(r.op.ID, raw)
	}
	p, isProgress := parseProgressLine(raw)
	r.mu.Lock()
	defer r.mu.Unlock()
	if !isProgress {
		r.scan.observe(raw, nil)
		r.addOutputLocked(raw + "\n")
		return
	}
	r.scan.observe(raw, &p)
	r.progressLocked(p)
	if isByteTick(p) {
		return
	}
	text := progressText(p)
	if r.lastLine[p.ID] == text {
		return
	}
	r.lastLine[p.ID] = text
	r.addOutputLocked(text + "\n")
}

// pullMessage turns one message of an image pull into a progress item and, for
// the messages that are not byte ticks, an output line.
func (r *opRun) pullMessage(m jsonMessage) {
	id := m.ID
	if id == "" {
		id = RunnerImageTag
	}
	p := Progress{ID: id, Parent: RunnerImageTag, Status: m.Status, Current: m.Progress.Current, Total: m.Progress.Total}
	if p.Total > 0 && p.Current > 0 {
		p.Percent = float64(p.Current*1000/p.Total) / 10
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progressLocked(p)
	if isByteTick(p) || m.Status == "" {
		return
	}
	text := m.Status
	if m.ID != "" {
		text = m.ID + ": " + m.Status
	}
	if r.lastLine[id] == text {
		return
	}
	r.lastLine[id] = text
	r.addOutputLocked(text + "\n")
}

// mutate changes the operation under the lock and writes the row.
func (r *opRun) mutate(fn func(op *Operation)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	fn(&r.op)
	r.persistLocked()
}

// setState records a state change and publishes it (without the output).
func (r *opRun) setState(state string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.op.State == state {
		return
	}
	r.op.State = state
	if state == StateRunning && r.op.StartedAt == nil {
		now := r.s.opts.Now().UTC()
		r.op.StartedAt = &now
	}
	r.persistLocked()
	snap := r.snapshotLocked(false)
	r.publishLocked(Event{Type: EventState, Operation: &snap})
}

// finish records the outcome, publishes the done event, ends every subscriber's
// stream and releases the stack's lock: its place among the active operations
// and the cached folder scan of its host (a stack operation may have changed the
// folders; prepare leaves the scan it just made). The service lock is taken
// before the run's, the order begin uses, so that the final state and the free
// stack become visible together: whoever sees the operation done can start the
// next one.
func (r *opRun) finish(out outcome) {
	s := r.s
	s.mu.Lock()
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		s.mu.Unlock()
		return
	}
	now := s.opts.Now().UTC()
	r.op.State = out.state
	r.op.Code, r.op.Error, r.op.ErrorLine, r.op.Warning = out.code, out.message, out.errorLine, out.warning
	if out.exit != nil {
		code := *out.exit
		r.op.ExitCode = &code
	}
	if out.ports != nil {
		r.op.Ports = out.ports
	}
	r.op.FinishedAt = &now
	r.persistLocked()
	done := r.snapshotLocked(true)
	r.closed = true
	r.publishLocked(Event{Type: EventDone, Operation: &done})
	r.closeSubscribersLocked()

	delete(s.runs, done.ID)
	if s.locks[lockKey{done.HostID, done.Stack}] == done.ID {
		delete(s.locks, lockKey{done.HostID, done.Stack})
	}
	if done.Stack != "" {
		delete(s.scans, done.HostID)
	}
	r.mu.Unlock()
	s.mu.Unlock()
	s.pruneJournal(done.HostID, done.Stack)
}

// abandon ends the run without a result: the service is closing and the next
// Open reattaches. The output so far is saved.
func (r *opRun) abandon() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.persistLocked()
	r.closeSubscribersLocked()
	r.closed = true
	r.mu.Unlock()
}

func (r *opRun) closeSubscribersLocked() {
	for sub := range r.subs {
		close(sub.ch)
		delete(r.subs, sub)
	}
}

// subscribe returns the current state with the output so far and a channel of
// the events that follow. ok is false when the run has ended.
func (r *opRun) subscribe() (op Operation, events <-chan Event, cancel func(), ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return Operation{}, nil, nil, false
	}
	sub := &subscriber{ch: make(chan Event, subscriberBuffer)}
	r.subs[sub] = struct{}{}
	cancel = func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if _, found := r.subs[sub]; found {
			delete(r.subs, sub)
			close(sub.ch)
		}
	}
	return r.snapshotLocked(true), sub.ch, cancel, true
}

// pruneJournal keeps the newest journalKeep finished operations of a stack.
func (s *Service) pruneJournal(hostID, stack string) {
	ctx, cancel := context.WithTimeout(context.Background(), journalWrite)
	defer cancel()
	if err := s.journal.prune(ctx, hostID, stack, journalKeep); err != nil {
		s.log.Warn("docker operation journal could not be pruned", "error", err)
	}
}
