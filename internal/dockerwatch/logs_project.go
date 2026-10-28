package dockerwatch

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// FollowProject follows the logs of every container of a compose project that
// the selector picks, merged into one stream: first the newest opt.Tail lines
// of each container, merged by timestamp (at most MaxLogHistory in all), then
// the live lines in the order they arrive. A container that starts later (after
// `up`, `update`, a restart that recreates it) is attached when the host's
// events watcher reports its `start`; after a watcher resync (reconnect or
// poll) the project is listed again. Every followed container holds one stream
// slot of the host; the services that find no slot are reported once per change
// as a not_followed notice and attached when a slot frees up. It returns when
// ctx ends (ctx.Err()), when out or notice fails (that error) or when the
// project cannot be read at the start (the Engine or transport error unchanged).
// A project without containers is followed too: nothing is sent until one
// starts. out and notice are never called concurrently.
func (h *Hub) FollowProject(ctx context.Context, host string, sel ProjectSelector, opt FollowOptions, out func(LogBatch) error, notice func(Notice) error) error {
	return h.logEnv().followProject(ctx, host, sel, opt, out, notice)
}

// logCtr is one container of the followed project.
type logCtr struct {
	full, short, name, svc string
	last                   time.Time // time of the newest line seen
	since                  time.Time // Since of the first live read, when catchUp
	catchUp                bool      // the first live read starts at the log's tail instead of at its end
	active                 bool      // a stream is open
	failed                 bool      // the last read failed; only an event or a resync tries again
}

func newLogCtr(s dockerhost.ContainerSummary, catchUp bool, since time.Time) *logCtr {
	c := &logCtr{full: s.FullID, short: s.ID, catchUp: catchUp, since: since}
	if len(s.Names) > 0 {
		c.name = s.Names[0]
	}
	c.svc = s.ComposeService
	if c.svc == "" {
		c.svc = c.name
	}
	if c.svc == "" {
		c.svc = c.short
	}
	return c
}

// liveOptions are the options of the next live read: right behind the newest
// line seen; or, for a container nobody has read yet, from its log's tail (a
// container that appeared after the follower started has all of its lines to
// show); or just the new lines.
func (c *logCtr) liveOptions() dockerhost.LogOptions {
	switch {
	case !c.last.IsZero():
		return dockerhost.LogOptions{Tail: MaxLogTail, Since: c.last.Add(time.Nanosecond), Follow: true}
	case c.catchUp:
		return dockerhost.LogOptions{Tail: MaxLogTail, Since: c.since, Follow: true}
	}
	return dockerhost.LogOptions{Follow: true}
}

func logFollowable(state string) bool {
	return state == "running" || state == "paused" || state == "restarting"
}

type projectFollower struct {
	env    logEnv
	host   string
	sel    ProjectSelector
	opt    FollowOptions
	c      *dockerutil.Client
	b      *logBatcher
	ctx    context.Context
	cancel context.CancelFunc

	evTrigger  chan struct{} // an event asks for a re-check of the project
	endTrigger chan struct{} // a stream ended: a slot is free, a container may have restarted

	mu       sync.Mutex
	ctrs     map[string]*logCtr // by full container id
	reported string             // services of the last not_followed notice
	firstErr error
	wg       sync.WaitGroup
}

func (e logEnv) followProject(ctx context.Context, host string, sel ProjectSelector, opt FollowOptions, out func(LogBatch) error, notice func(Notice) error) error {
	if sel.Project == "" {
		return errors.New("project name is empty")
	}
	// Before anything else, so a refused follow does no work: no list, no history.
	if e.acquireProject != nil {
		release, err := e.acquireProject(host)
		if err != nil {
			return err
		}
		defer release()
	}
	c, err := e.hosts.Client(ctx, host)
	if err != nil {
		return err
	}
	inner, cancel := context.WithCancel(ctx)
	defer cancel()
	p := &projectFollower{
		env: e, host: host, sel: sel, opt: opt, c: c,
		ctx: inner, cancel: cancel,
		evTrigger:  make(chan struct{}, 1),
		endTrigger: make(chan struct{}, 1),
		ctrs:       map[string]*logCtr{},
	}
	p.b = e.batcher(out, notice, p.fail)
	return p.run(ctx)
}

func (p *projectFollower) run(parent context.Context) (result error) {
	// Subscribe before listing: a start between the two is not lost.
	stopWatch := p.env.watch(p.host, p.onEvent)
	var startErr error
	defer func() {
		stopWatch()
		p.shutdown()
		result = p.result(parent, startErr)
	}()

	list, err := p.env.list(p.ctx, p.c)
	if err != nil {
		startErr = err
		return nil
	}
	if p.opt.Opened != nil {
		p.opt.Opened()
	}
	selected := p.selected(list)
	p.mu.Lock()
	for _, s := range selected {
		p.ctrs[s.FullID] = newLogCtr(s, p.opt.tail() > 0, p.opt.Since)
	}
	p.mu.Unlock()
	if p.opt.tail() > 0 {
		p.readHistory(selected)
	}
	if p.ctx.Err() != nil {
		return nil
	}
	p.attach(selected, false)

	for {
		select {
		case <-p.ctx.Done():
			return nil
		case <-p.evTrigger:
			p.reconcile(true)
		case <-p.endTrigger:
			timer := time.NewTimer(p.env.settle)
			select {
			case <-timer.C:
			case <-p.ctx.Done():
				timer.Stop()
				return nil
			}
			p.reconcile(false)
		}
	}
}

// shutdown stops the container streams and sends what is left.
func (p *projectFollower) shutdown() {
	p.cancel()
	p.wg.Wait()
	if err := p.b.close(); err != nil {
		p.fail(err)
	}
}

// result is what FollowProject returns: the caller's cancellation first, then
// the error that stopped the follower.
func (p *projectFollower) result(parent context.Context, startErr error) error {
	if err := parent.Err(); err != nil {
		return err
	}
	if startErr != nil {
		return startErr
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.firstErr
}

// fail records the first error and ends the follower.
func (p *projectFollower) fail(err error) {
	p.mu.Lock()
	if p.firstErr == nil {
		p.firstErr = err
	}
	p.mu.Unlock()
	p.cancel()
}

func (p *projectFollower) onEvent(ev Event) {
	relevant := ev.Type == "resync" || (ev.Type == "container" && ev.Action == "start" && ev.Project == p.sel.Project)
	if !relevant {
		return
	}
	select {
	case p.evTrigger <- struct{}{}:
	default:
	}
}

func (p *projectFollower) selected(list []dockerhost.ContainerSummary) []dockerhost.ContainerSummary {
	var out []dockerhost.ContainerSummary
	for _, s := range list {
		if p.sel.Matches(s) {
			out = append(out, s)
		}
	}
	return out
}

func (p *projectFollower) entry(c *logCtr, l dockerhost.LogLine) LogEntry {
	e := logEntryOf(l)
	e.Svc, e.Ctr = c.svc, c.short
	return e
}

// logHistory is the history of one container while it is being merged.
type logHistory struct {
	lines []LogEntry
	bytes int
	last  time.Time
	err   error
}

func (h *logHistory) add(e LogEntry) {
	if e.T.After(h.last) {
		h.last = e.T
	}
	h.lines = append(h.lines, e)
	h.bytes += len(e.Text) + 64
	for h.bytes > logHistoryBytes && len(h.lines) > 1 { // drop the oldest lines of a flood
		h.bytes -= len(h.lines[0].Text) + 64
		h.lines = h.lines[1:]
	}
}

// historyItem is one history line while it is being merged: its entry and the
// place it comes from, which together give a total order (time, then the
// container's position in the selection, then the line's position in its log).
type historyItem struct {
	e       LogEntry
	ci, seq int
}

func (a historyItem) before(b historyItem) bool {
	if c := a.e.T.Compare(b.e.T); c != 0 {
		return c < 0
	}
	if a.ci != b.ci {
		return a.ci < b.ci
	}
	return a.seq < b.seq
}

// historyHeap is a min-heap of history lines: the oldest is at the root.
type historyHeap []historyItem

func (h historyHeap) Len() int           { return len(h) }
func (h historyHeap) Less(i, j int) bool { return h[i].before(h[j]) }
func (h historyHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *historyHeap) Push(x any)        { *h = append(*h, x.(historyItem)) }
func (h *historyHeap) Pop() any {
	old := *h
	last := old[len(old)-1]
	old[len(old)-1] = historyItem{}
	*h = old[:len(old)-1]
	return last
}

// historyMerge merges the histories of the containers of a project and keeps
// only the newest MaxLogHistory lines of them, so what a project follow holds is
// bounded by that cap and not by the number of containers times what each log
// holds (up to logHistoryBytes). A container's lines go in with add as soon as
// its read has ended, in log order; the result is the lines in time order, a
// tie broken by the container's position and then the line's, which is the order
// a stable sort of the concatenated histories gives. Safe for concurrent use.
type historyMerge struct {
	mu    sync.Mutex
	kept  historyHeap
	total int // every line added, kept or not
}

func (m *historyMerge) add(ci int, lines []LogEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for seq, e := range lines {
		m.total++
		item := historyItem{e: e, ci: ci, seq: seq}
		switch {
		case len(m.kept) < MaxLogHistory:
			heap.Push(&m.kept, item)
		case m.kept[0].before(item): // newer than the oldest line kept: it takes its place
			m.kept[0] = item
			heap.Fix(&m.kept, 0)
		}
	}
}

// result returns the kept lines oldest first, and whether older ones were dropped.
func (m *historyMerge) result() (entries []LogEntry, truncated bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := slices.Clone(m.kept)
	slices.SortFunc(items, func(a, b historyItem) int {
		switch {
		case a.before(b):
			return -1
		case b.before(a):
			return 1
		}
		return 0
	})
	entries = make([]LogEntry, len(items))
	for i, item := range items {
		entries[i] = item.e
	}
	return entries, m.total > len(m.kept)
}

// readHistory reads the tail of every container (a few at a time, without a
// stream slot: the reads end by themselves, or after historyTimeout), merges the
// lines by timestamp and sends the newest MaxLogHistory of them. A container's
// lines are merged (historyMerge) and dropped as soon as its read has ended, so
// at most logHistoryParallel container histories plus the newest MaxLogHistory
// lines are held at any time. A container whose read fails or hangs is reported
// as not followed.
func (p *projectFollower) readHistory(selected []dockerhost.ContainerSummary) {
	opt := dockerhost.LogOptions{Tail: p.opt.tail(), Since: p.opt.Since}
	results := make([]*logHistory, len(selected))
	var merge historyMerge
	sem := make(chan struct{}, logHistoryParallel)
	var wg sync.WaitGroup
	for i, s := range selected {
		select {
		case sem <- struct{}{}:
		case <-p.ctx.Done():
		}
		if p.ctx.Err() != nil {
			break
		}
		p.mu.Lock()
		ctr := p.ctrs[s.FullID]
		p.mu.Unlock()
		wg.Add(1)
		go func(i int, ctr *logCtr) {
			defer wg.Done()
			defer func() { <-sem }()
			h := &logHistory{}
			ctx, cancel := context.WithTimeout(p.ctx, p.env.historyTimeout)
			defer cancel()
			h.err = p.env.follow(ctx, p.c, ctr.full, opt, func(l dockerhost.LogLine) error {
				h.add(p.entry(ctr, l))
				return nil
			})
			merge.add(i, h.lines)
			h.lines = nil // merged: only the newest MaxLogHistory lines of all are kept
			results[i] = h
		}(i, ctr)
	}
	wg.Wait()
	if p.ctx.Err() != nil {
		return
	}

	var problems []Notice
	for i, h := range results {
		if h == nil {
			continue
		}
		p.mu.Lock()
		ctr := p.ctrs[selected[i].FullID]
		ctr.last = h.last
		p.mu.Unlock()
		if h.err == nil {
			continue
		}
		p.mu.Lock()
		if dockerhost.StatusOf(h.err) == http.StatusNotFound {
			delete(p.ctrs, ctr.full) // gone since the list
		} else {
			ctr.failed = true
			problems = append(problems, Notice{Code: NoticeNotFollowed, Message: fmt.Sprintf("The log of %s could not be read.", ctr.svc), Services: []string{ctr.svc}, Container: ctr.short})
		}
		p.mu.Unlock()
	}
	entries, truncated := merge.result()
	if err := p.b.addAll(entries); err != nil {
		p.fail(err)
		return
	}
	if truncated {
		p.notify(Notice{Code: NoticeTruncatedHistory, Message: fmt.Sprintf("Only the newest %d history lines are shown.", MaxLogHistory)})
	}
	for _, n := range problems {
		p.notify(n)
	}
}

// notify sends a notice; a failing receiver ends the follower.
func (p *projectFollower) notify(n Notice) {
	if err := p.b.send(n); err != nil {
		p.fail(err)
	}
}

// reconcile lists the project again and attaches the running containers that
// have no stream. After an event or a resync it also lets containers whose last
// read failed try again.
func (p *projectFollower) reconcile(fromEvent bool) {
	list, err := p.env.list(p.ctx, p.c)
	if err != nil {
		return // the next event, resync or ended stream asks again
	}
	selected := p.selected(list)
	present := make(map[string]bool, len(selected))
	p.mu.Lock()
	for _, s := range selected {
		present[s.FullID] = true
		if ctr := p.ctrs[s.FullID]; ctr == nil {
			p.ctrs[s.FullID] = newLogCtr(s, true, time.Time{})
		} else if fromEvent {
			ctr.failed = false
		}
	}
	for id, ctr := range p.ctrs {
		if !present[id] && !ctr.active {
			delete(p.ctrs, id)
		}
	}
	p.mu.Unlock()
	p.attach(selected, true)
}

// attach opens a stream for every running container without one, as far as
// the host's stream slots go, and reports the services that found no slot.
// announce adds an attached notice per container.
func (p *projectFollower) attach(selected []dockerhost.ContainerSummary, announce bool) {
	blocked := map[string]bool{}
	var attached []*logCtr
	p.mu.Lock()
	for _, s := range selected {
		ctr := p.ctrs[s.FullID]
		if ctr == nil || ctr.active || ctr.failed || !logFollowable(s.State) {
			continue
		}
		release, err := p.env.acquire(p.host)
		if err != nil {
			blocked[ctr.svc] = true
			continue
		}
		ctr.active = true
		p.wg.Add(1)
		go p.followCtr(ctr, release, ctr.liveOptions())
		attached = append(attached, ctr)
	}
	p.mu.Unlock()

	if announce {
		for _, ctr := range attached {
			p.notify(Notice{Code: NoticeAttached, Message: fmt.Sprintf("Following %s.", ctr.svc), Services: []string{ctr.svc}, Container: ctr.short})
		}
	}
	services := make([]string, 0, len(blocked))
	for svc := range blocked {
		services = append(services, svc)
	}
	slices.Sort(services)
	key := strings.Join(services, "\x00")
	p.mu.Lock()
	changed := key != p.reported
	p.reported = key
	p.mu.Unlock()
	if changed && len(services) > 0 {
		p.notify(Notice{
			Code:     NoticeNotFollowed,
			Message:  fmt.Sprintf("The host has no free log stream for %s; they are followed as soon as one is free.", strings.Join(services, ", ")),
			Services: services,
		})
	}
}

// followCtr reads one container's live log until the stream ends.
func (p *projectFollower) followCtr(ctr *logCtr, release func(), opt dockerhost.LogOptions) {
	defer p.wg.Done()
	var once sync.Once
	free := func() { once.Do(release) }
	defer free()
	err := p.env.follow(p.ctx, p.c, ctr.full, opt, func(l dockerhost.LogLine) error {
		p.mu.Lock()
		if l.Time.After(ctr.last) {
			ctr.last = l.Time
		}
		p.mu.Unlock()
		return p.b.add(p.entry(ctr, l))
	})
	free()
	p.mu.Lock()
	ctr.active = false
	p.mu.Unlock()
	if p.ctx.Err() != nil {
		return
	}
	if berr := p.b.failure(); berr != nil {
		p.fail(berr)
		return
	}
	switch {
	case err == nil:
		p.notify(Notice{Code: NoticeDetached, Message: fmt.Sprintf("%s stopped; its log is followed again when it starts.", ctr.svc), Services: []string{ctr.svc}, Container: ctr.short})
	case dockerhost.StatusOf(err) == http.StatusNotFound:
		p.mu.Lock()
		delete(p.ctrs, ctr.full)
		p.mu.Unlock()
		p.notify(Notice{Code: NoticeDetached, Message: fmt.Sprintf("%s was removed.", ctr.svc), Services: []string{ctr.svc}, Container: ctr.short})
	default:
		p.mu.Lock()
		ctr.failed = true
		p.mu.Unlock()
		p.notify(Notice{Code: NoticeNotFollowed, Message: fmt.Sprintf("The log of %s is no longer followed.", ctr.svc), Services: []string{ctr.svc}, Container: ctr.short})
	}
	select {
	case p.endTrigger <- struct{}{}:
	default:
	}
}
