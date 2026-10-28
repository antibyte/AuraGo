package dockerwatch

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// logsLine is a log line `sec` seconds after logsBase.
func logsLine(sec int, stream, text string) dockerhost.LogLine {
	return dockerhost.LogLine{Stream: stream, Time: logsBase.Add(time.Duration(sec) * time.Second), Text: text}
}

// logsCtrOf is a container of a compose project. id is one character that
// fills the 64-character id.
func logsCtrOf(id, project, service, workingDir, state string) dockerhost.ContainerSummary {
	full := strings.Repeat(id, 64)
	return dockerhost.ContainerSummary{
		ID: full[:12], FullID: full, Names: []string{project + "-" + service + "-1"},
		State: state, ComposeProject: project, ComposeService: service, ComposeWorkingDir: workingDir,
	}
}

// logsWaitFor polls cond until it holds.
func logsWaitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// logsSink receives what a follower sends.
type logsSink struct {
	mu      sync.Mutex
	batches []LogBatch
	notices []Notice
	outErr  error // returned by out once set
}

func (s *logsSink) out(b LogBatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.outErr != nil {
		return s.outErr
	}
	s.batches = append(s.batches, b)
	return nil
}

func (s *logsSink) notice(n Notice) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notices = append(s.notices, n)
	return nil
}

func (s *logsSink) lines() []LogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []LogEntry
	for _, b := range s.batches {
		all = append(all, b.Lines...)
	}
	return all
}

func (s *logsSink) texts() []string {
	var out []string
	for _, l := range s.lines() {
		out = append(out, l.Text)
	}
	return out
}

func (s *logsSink) noticeList() []Notice {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Notice(nil), s.notices...)
}

func (s *logsSink) waitLines(t *testing.T, n int) {
	t.Helper()
	logsWaitFor(t, "log lines", func() bool { return len(s.lines()) >= n })
}

// waitNotice waits for the n-th (1-based) notice with the code and returns it.
func (s *logsSink) waitNotice(t *testing.T, code string, n int) Notice {
	t.Helper()
	var found Notice
	logsWaitFor(t, "notice "+code, func() bool {
		count := 0
		for _, got := range s.noticeList() {
			if got.Code == code {
				count++
				if count == n {
					found = got
					return true
				}
			}
		}
		return false
	})
	return found
}

func (s *logsSink) countNotices(code string) int {
	count := 0
	for _, got := range s.noticeList() {
		if got.Code == code {
			count++
		}
	}
	return count
}

// logsRead is what a log read asked for (LogOptions without its callback, so it can be compared).
type logsRead struct {
	Tail   int
	Since  time.Time
	Follow bool
}

// logsCall is one log read the fake Engine served.
type logsCall struct {
	ref  string
	read logsRead
}

// logsFakeEngine replaces the Engine calls, the event feed and the stream
// budget of a logEnv. A read serves the container's history (the last Tail
// lines at or after Since) and, when it follows, then waits for lines pushed
// with push until end, the context or a refusal set with refuse.
type logsFakeEngine struct {
	mu         sync.Mutex
	containers []dockerhost.ContainerSummary
	history    map[string][]dockerhost.LogLine
	feeds      map[string]chan dockerhost.LogLine
	refused    map[string]error
	hang       map[string]bool // history reads of these containers never answer
	calls      []logsCall
	lists      int
	slots      int // stream slots of the host
	used       int
	watcher    func(Event)
}

func newLogsFakeEngine() *logsFakeEngine {
	return &logsFakeEngine{
		history: map[string][]dockerhost.LogLine{},
		feeds:   map[string]chan dockerhost.LogLine{},
		refused: map[string]error{},
		hang:    map[string]bool{},
		slots:   MaxStreamsPerHost,
	}
}

func (f *logsFakeEngine) env() logEnv {
	return logEnv{
		hosts: logsFakeHosts{},
		acquire: func(string) (func(), error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.used >= f.slots {
				return nil, ErrTooManyStreams
			}
			f.used++
			return func() { f.mu.Lock(); f.used--; f.mu.Unlock() }, nil
		},
		watch: func(_ string, fn func(Event)) func() {
			f.mu.Lock()
			f.watcher = fn
			f.mu.Unlock()
			return func() {}
		},
		follow:         f.follow,
		list:           f.list,
		after:          logAfterFunc,
		flushEvery:     5 * time.Millisecond,
		flushLines:     logFlushLines,
		settle:         5 * time.Millisecond,
		historyTimeout: time.Second,
	}
}

func (f *logsFakeEngine) list(context.Context, *dockerutil.Client) ([]dockerhost.ContainerSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lists++
	return append([]dockerhost.ContainerSummary(nil), f.containers...), nil
}

func (f *logsFakeEngine) feed(ref string) chan dockerhost.LogLine {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch := f.feeds[ref]
	if ch == nil {
		ch = make(chan dockerhost.LogLine, 64)
		f.feeds[ref] = ch
	}
	return ch
}

func (f *logsFakeEngine) follow(ctx context.Context, _ *dockerutil.Client, ref string, opt dockerhost.LogOptions, fn func(dockerhost.LogLine) error) error {
	f.mu.Lock()
	f.calls = append(f.calls, logsCall{ref: ref, read: logsRead{Tail: opt.Tail, Since: opt.Since, Follow: opt.Follow}})
	refusal := f.refused[ref]
	lines := f.history[ref]
	hang := f.hang[ref] && !opt.Follow
	f.mu.Unlock()
	if refusal != nil {
		return refusal
	}
	if hang {
		<-ctx.Done()
		return ctx.Err()
	}
	if opt.Opened != nil {
		opt.Opened()
	}
	var served []dockerhost.LogLine
	for _, l := range lines {
		if opt.Since.IsZero() || !l.Time.Before(opt.Since) {
			served = append(served, l)
		}
	}
	if len(served) > opt.Tail {
		served = served[len(served)-opt.Tail:]
	}
	for _, l := range served {
		if err := fn(l); err != nil {
			return err
		}
	}
	if !opt.Follow {
		return nil
	}
	feed := f.feed(ref)
	for {
		select {
		case l, ok := <-feed:
			if !ok {
				return nil
			}
			if err := fn(l); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// push sends a live line of a container.
func (f *logsFakeEngine) push(ref string, l dockerhost.LogLine) { f.feed(ref) <- l }

// end ends the container's live stream (it stopped); the next read starts a new one.
func (f *logsFakeEngine) end(ref string) {
	f.mu.Lock()
	ch := f.feeds[ref]
	delete(f.feeds, ref)
	f.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (f *logsFakeEngine) setState(id, state string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.containers {
		if f.containers[i].ID[:1] == id {
			f.containers[i].State = state
		}
	}
}

func (f *logsFakeEngine) callList() []logsCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]logsCall(nil), f.calls...)
}

// liveCalls are the reads that follow.
func (f *logsFakeEngine) liveCalls(ref string) []logsCall {
	var out []logsCall
	for _, c := range f.callList() {
		if c.ref == ref && c.read.Follow {
			out = append(out, c)
		}
	}
	return out
}

func (f *logsFakeEngine) listCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lists
}

func (f *logsFakeEngine) slotsInUse() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.used
}

// emit delivers an event to the follower's watcher, like the hub's events watcher.
func (f *logsFakeEngine) emit(ev Event) {
	f.mu.Lock()
	fn := f.watcher
	f.mu.Unlock()
	if fn != nil {
		fn(ev)
	}
}

type logsFakeHosts struct{}

func (logsFakeHosts) Host(context.Context, string) (dockerhost.Host, error) {
	return dockerhost.Host{}, nil
}

func (logsFakeHosts) Client(context.Context, string) (*dockerutil.Client, error) { return nil, nil }
