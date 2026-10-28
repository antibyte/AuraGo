package dockerwatch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

const logsStackDir = "/opt/stacks/shop"

// startLogsProject runs FollowProject against the fake and returns what it
// returns once the test cancels it.
func startLogsProject(t *testing.T, fe *logsFakeEngine, sel ProjectSelector, opt FollowOptions, sink *logsSink) (cancel func(), done <-chan error) {
	t.Helper()
	ctx, cancelCtx := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- fe.env().followProject(ctx, "h1", sel, opt, sink.out, sink.notice) }()
	t.Cleanup(cancelCtx)
	return cancelCtx, result
}

func stopLogsProject(t *testing.T, cancel func(), done <-chan error, fe *logsFakeEngine) {
	t.Helper()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("FollowProject after cancel = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("FollowProject did not return after the context ended")
	}
	if n := fe.slotsInUse(); n != 0 {
		t.Fatalf("%d stream slots still held after FollowProject returned", n)
	}
}

func TestFollowProjectMergesTheHistoryByTimestampThenFollowsLive(t *testing.T) {
	fe := newLogsFakeEngine()
	web := logsCtrOf("a", "shop", "web", logsStackDir, "running")
	db := logsCtrOf("b", "shop", "db", logsStackDir, "running")
	job := logsCtrOf("c", "shop", "job", logsStackDir, "exited")
	otherProject := logsCtrOf("d", "blog", "web", logsStackDir, "running")
	externalSameName := logsCtrOf("e", "shop", "web", "/home/me/shop", "running")
	fe.containers = []dockerhost.ContainerSummary{web, db, job, otherProject, externalSameName}
	fe.history[web.FullID] = []dockerhost.LogLine{logsLine(1, "out", "w1"), logsLine(4, "out", "w4")}
	fe.history[db.FullID] = []dockerhost.LogLine{logsLine(2, "out", "d2"), logsLine(3, "err", "d3")}
	fe.history[job.FullID] = []dockerhost.LogLine{logsLine(0, "out", "j0")}
	fe.history[otherProject.FullID] = []dockerhost.LogLine{logsLine(1, "out", "never")}
	fe.history[externalSameName.FullID] = []dockerhost.LogLine{logsLine(1, "out", "never")}

	sink := &logsSink{}
	opened := make(chan int, 4)
	opt := FollowOptions{Tail: 100, Opened: func() { opened <- len(sink.lines()) }}
	cancel, done := startLogsProject(t, fe, ProjectSelector{Project: "shop", WorkingDir: logsStackDir}, opt, sink)
	sink.waitLines(t, 5)
	if n := <-opened; n != 0 || len(opened) != 0 {
		t.Fatalf("Opened came after %d lines or more than once; it must come once, before the history", n)
	}

	got := sink.lines()
	if texts := sink.texts(); !slices.Equal(texts, []string{"j0", "w1", "d2", "d3", "w4"}) {
		t.Fatalf("history = %v, want the three containers merged by timestamp", texts)
	}
	if got[1].Svc != "web" || got[1].Ctr != web.ID || got[3].S != "err" || got[3].Svc != "db" || got[0].Svc != "job" {
		t.Fatalf("entries = %+v, want service, container and stream on every line", got)
	}
	if len(sink.batches) != 1 {
		t.Fatalf("%d batches, want the merged history in one", len(sink.batches))
	}

	// The exited container only has history; the running ones continue right behind their newest line.
	logsWaitFor(t, "the live reads", func() bool { return len(fe.liveCalls(web.FullID)) == 1 && len(fe.liveCalls(db.FullID)) == 1 })
	if n := len(fe.liveCalls(job.FullID)) + len(fe.liveCalls(otherProject.FullID)) + len(fe.liveCalls(externalSameName.FullID)); n != 0 {
		t.Fatalf("%d live reads of containers that are stopped or not part of the stack", n)
	}
	wantWeb := logsRead{Tail: MaxLogTail, Since: logsBase.Add(4*time.Second + time.Nanosecond), Follow: true}
	wantDB := logsRead{Tail: MaxLogTail, Since: logsBase.Add(3*time.Second + time.Nanosecond), Follow: true}
	if c := fe.liveCalls(web.FullID)[0].read; c != wantWeb {
		t.Fatalf("web live options = %+v, want %+v", c, wantWeb)
	}
	if c := fe.liveCalls(db.FullID)[0].read; c != wantDB {
		t.Fatalf("db live options = %+v, want %+v", c, wantDB)
	}
	for _, c := range fe.callList() {
		if !c.read.Follow && c.read.Tail != 100 {
			t.Fatalf("history read %+v, want tail 100", c)
		}
	}

	fe.push(web.FullID, logsLine(5, "out", "w5"))
	fe.push(db.FullID, logsLine(6, "out", "d6"))
	sink.waitLines(t, 7)
	if texts := sink.texts()[5:]; !slices.Equal(texts, []string{"w5", "d6"}) && !slices.Equal(texts, []string{"d6", "w5"}) {
		t.Fatalf("live lines = %v", texts)
	}
	stopLogsProject(t, cancel, done, fe)
}

func TestFollowProjectAttachesContainersThatStartLater(t *testing.T) {
	fe := newLogsFakeEngine()
	web := logsCtrOf("a", "shop", "web", logsStackDir, "running")
	fe.containers = []dockerhost.ContainerSummary{web}
	sink := &logsSink{}
	cancel, done := startLogsProject(t, fe, ProjectSelector{Project: "shop", WorkingDir: logsStackDir}, FollowOptions{}, sink)

	// Without a history the first live read only takes new lines.
	logsWaitFor(t, "the first live read", func() bool { return len(fe.liveCalls(web.FullID)) == 1 })
	if c := fe.liveCalls(web.FullID)[0].read; c != (logsRead{Follow: true}) {
		t.Fatalf("first live options = %+v, want only new lines", c)
	}
	if n := sink.countNotices(NoticeAttached); n != 0 {
		t.Fatalf("%d attached notices for the containers found at the start", n)
	}
	lists := fe.listCount()

	// Events that are not a start of this project's containers cause no re-check.
	fe.emit(Event{Type: "container", Action: "die", Project: "shop"})
	fe.emit(Event{Type: "container", Action: "start", Project: "blog"})
	fe.emit(Event{Type: "image", Action: "pull"})
	time.Sleep(50 * time.Millisecond)
	if fe.listCount() != lists {
		t.Fatalf("irrelevant events listed the project %d more times", fe.listCount()-lists)
	}

	api := logsCtrOf("b", "shop", "api", logsStackDir, "running")
	fe.mu.Lock()
	fe.containers = append(fe.containers, api)
	fe.mu.Unlock()
	fe.emit(Event{Type: "container", Action: "start", Project: "shop", Service: "api", ID: api.FullID})

	n := sink.waitNotice(t, NoticeAttached, 1)
	if n.Container != api.ID || !slices.Equal(n.Services, []string{"api"}) {
		t.Fatalf("attached notice = %+v, want the new api container", n)
	}
	// A container that appeared after the start has all of its lines to show.
	if c := fe.liveCalls(api.FullID)[0].read; c != (logsRead{Tail: MaxLogTail, Follow: true}) {
		t.Fatalf("new container options = %+v, want its whole tail", c)
	}
	fe.push(api.FullID, logsLine(1, "out", "listening"))
	sink.waitLines(t, 1)
	if l := sink.lines()[0]; l.Svc != "api" || l.Ctr != api.ID || l.Text != "listening" {
		t.Fatalf("line = %+v", l)
	}
	if n := len(fe.liveCalls(web.FullID)); n != 1 {
		t.Fatalf("web was read %d times; the running container keeps its stream", n)
	}
	stopLogsProject(t, cancel, done, fe)
}

func TestFollowProjectContinuesBehindTheLastLineWhenAContainerRestarts(t *testing.T) {
	fe := newLogsFakeEngine()
	web := logsCtrOf("a", "shop", "web", logsStackDir, "running")
	fe.containers = []dockerhost.ContainerSummary{web}
	fe.history[web.FullID] = []dockerhost.LogLine{logsLine(5, "out", "old")}
	sink := &logsSink{}
	cancel, done := startLogsProject(t, fe, ProjectSelector{Project: "shop", WorkingDir: logsStackDir}, FollowOptions{Tail: 10}, sink)
	sink.waitLines(t, 1)
	logsWaitFor(t, "the live read", func() bool { return len(fe.liveCalls(web.FullID)) == 1 })
	fe.push(web.FullID, logsLine(6, "out", "serving"))
	sink.waitLines(t, 2)

	// The container stops: its stream ends and the follower says so, but does not read it again.
	fe.setState("a", "exited")
	fe.end(web.FullID)
	d := sink.waitNotice(t, NoticeDetached, 1)
	if d.Container != web.ID || !slices.Equal(d.Services, []string{"web"}) {
		t.Fatalf("detached notice = %+v", d)
	}
	time.Sleep(60 * time.Millisecond)
	if n := len(fe.liveCalls(web.FullID)); n != 1 {
		t.Fatalf("a stopped container was read again (%d live reads)", n)
	}

	// It starts again: the next read begins right behind the last line, so nothing repeats.
	fe.setState("a", "running")
	fe.emit(Event{Type: "container", Action: "start", Project: "shop", Service: "web"})
	sink.waitNotice(t, NoticeAttached, 1)
	logsWaitFor(t, "the second live read", func() bool { return len(fe.liveCalls(web.FullID)) == 2 })
	want := logsRead{Tail: MaxLogTail, Since: logsBase.Add(6*time.Second + time.Nanosecond), Follow: true}
	if c := fe.liveCalls(web.FullID)[1].read; c != want {
		t.Fatalf("restart options = %+v, want %+v", c, want)
	}
	fe.push(web.FullID, logsLine(7, "out", "serving again"))
	sink.waitLines(t, 3)
	if texts := sink.texts(); !slices.Equal(texts, []string{"old", "serving", "serving again"}) {
		t.Fatalf("lines = %v", texts)
	}
	stopLogsProject(t, cancel, done, fe)
}

func TestFollowProjectReportsServicesWithoutAStreamSlotOnce(t *testing.T) {
	fe := newLogsFakeEngine()
	fe.slots = 2
	var ctrs []dockerhost.ContainerSummary
	for _, svc := range []struct{ id, name string }{{"a", "web"}, {"b", "db"}, {"c", "cache"}} {
		ctrs = append(ctrs, logsCtrOf(svc.id, "shop", svc.name, logsStackDir, "running"))
	}
	fe.containers = ctrs
	sink := &logsSink{}
	cancel, done := startLogsProject(t, fe, ProjectSelector{Project: "shop", WorkingDir: logsStackDir}, FollowOptions{}, sink)

	n := sink.waitNotice(t, NoticeNotFollowed, 1)
	if !slices.Equal(n.Services, []string{"cache"}) || n.Message == "" {
		t.Fatalf("not_followed notice = %+v, want the third service", n)
	}
	logsWaitFor(t, "two streams", func() bool { return fe.slotsInUse() == 2 })

	// The same shortage is not reported again by a re-check.
	fe.emit(Event{Type: "resync", Action: "poll"})
	logsWaitFor(t, "the re-check", func() bool { return fe.listCount() >= 2 })
	time.Sleep(30 * time.Millisecond)
	if c := sink.countNotices(NoticeNotFollowed); c != 1 {
		t.Fatalf("%d not_followed notices, want one for an unchanged shortage", c)
	}

	// A container stops: its slot goes to the service that had none.
	fe.setState("a", "exited")
	fe.end(ctrs[0].FullID)
	a := sink.waitNotice(t, NoticeAttached, 1)
	if !slices.Equal(a.Services, []string{"cache"}) {
		t.Fatalf("attached notice = %+v, want cache to get the free slot", a)
	}
	if c := sink.countNotices(NoticeNotFollowed); c != 1 {
		t.Fatalf("%d not_followed notices after the slot was handed over, want still one", c)
	}
	stopLogsProject(t, cancel, done, fe)
}

func TestFollowProjectCutsALongHistoryAndSaysSo(t *testing.T) {
	fe := newLogsFakeEngine()
	web := logsCtrOf("a", "shop", "web", logsStackDir, "exited")
	db := logsCtrOf("b", "shop", "db", logsStackDir, "exited")
	fe.containers = []dockerhost.ContainerSummary{web, db}
	for i := 0; i < 3000; i++ {
		fe.history[web.FullID] = append(fe.history[web.FullID], logsLine(2*i, "out", fmt.Sprintf("web %d", i)))
		fe.history[db.FullID] = append(fe.history[db.FullID], logsLine(2*i+1, "out", fmt.Sprintf("db %d", i)))
	}
	sink := &logsSink{}
	cancel, done := startLogsProject(t, fe, ProjectSelector{Project: "shop", WorkingDir: logsStackDir}, FollowOptions{Tail: 5000}, sink)

	sink.waitNotice(t, NoticeTruncatedHistory, 1)
	sink.waitLines(t, MaxLogHistory)
	lines := sink.lines()
	if len(lines) != MaxLogHistory {
		t.Fatalf("%d history lines, want the newest %d", len(lines), MaxLogHistory)
	}
	if lines[0].Text != "web 500" || lines[len(lines)-1].Text != "db 2999" {
		t.Fatalf("history runs from %q to %q, want the newest 5000 of both containers", lines[0].Text, lines[len(lines)-1].Text)
	}
	for _, b := range sink.batches {
		if len(b.Lines) > logFlushLines {
			t.Fatalf("a batch of %d lines, want at most %d", len(b.Lines), logFlushLines)
		}
	}
	stopLogsProject(t, cancel, done, fe)
}

func TestFollowProjectDoesNotRetryAFailedReadUntilAnEvent(t *testing.T) {
	fe := newLogsFakeEngine()
	web := logsCtrOf("a", "shop", "web", logsStackDir, "running")
	fe.containers = []dockerhost.ContainerSummary{web}
	fe.refused[web.FullID] = &dockerhost.StatusError{Status: http.StatusNotImplemented, Err: errors.New("logging driver does not support reading")}
	sink := &logsSink{}
	cancel, done := startLogsProject(t, fe, ProjectSelector{Project: "shop"}, FollowOptions{}, sink)

	n := sink.waitNotice(t, NoticeNotFollowed, 1)
	if !slices.Equal(n.Services, []string{"web"}) || n.Container != web.ID {
		t.Fatalf("notice = %+v, want web reported as not followed", n)
	}
	time.Sleep(60 * time.Millisecond) // the ended stream asks for a re-check; it must not read web again
	if c := len(fe.callList()); c != 1 {
		t.Fatalf("%d reads of a container whose log cannot be read, want 1", c)
	}

	// An event is a reason to try again.
	fe.mu.Lock()
	delete(fe.refused, web.FullID)
	fe.mu.Unlock()
	fe.emit(Event{Type: "resync", Action: "reconnect"})
	logsWaitFor(t, "the second read", func() bool { return len(fe.callList()) == 2 })
	stopLogsProject(t, cancel, done, fe)
}

func TestFollowProjectGivesUpOnAHistoryReadThatHangs(t *testing.T) {
	fe := newLogsFakeEngine()
	web := logsCtrOf("a", "shop", "web", logsStackDir, "running")
	db := logsCtrOf("b", "shop", "db", logsStackDir, "running")
	fe.containers = []dockerhost.ContainerSummary{web, db}
	fe.history[web.FullID] = []dockerhost.LogLine{logsLine(1, "out", "web is up")}
	fe.hang[db.FullID] = true
	env := fe.env()
	env.historyTimeout = 100 * time.Millisecond
	sink := &logsSink{}
	ctx, cancelCtx := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- env.followProject(ctx, "h1", ProjectSelector{Project: "shop", WorkingDir: logsStackDir}, FollowOptions{Tail: 10}, sink.out, sink.notice)
	}()
	t.Cleanup(cancelCtx)

	// The healthy container's history is not held back by the hung read, and the hung one is reported.
	sink.waitLines(t, 1)
	n := sink.waitNotice(t, NoticeNotFollowed, 1)
	if !slices.Equal(n.Services, []string{"db"}) || n.Container != db.ID {
		t.Fatalf("notice = %+v, want db reported as not followed", n)
	}
	logsWaitFor(t, "web to be followed", func() bool { return len(fe.liveCalls(web.FullID)) == 1 })
	time.Sleep(30 * time.Millisecond)
	if n := len(fe.liveCalls(db.FullID)); n != 0 {
		t.Fatalf("db was followed live %d times after its history read hung", n)
	}
	stopLogsProject(t, cancelCtx, done, fe)
}

func TestFollowProjectEndsWithTheReceiversError(t *testing.T) {
	fe := newLogsFakeEngine()
	web := logsCtrOf("a", "shop", "web", logsStackDir, "running")
	fe.containers = []dockerhost.ContainerSummary{web}
	fe.history[web.FullID] = []dockerhost.LogLine{logsLine(1, "out", "one")}
	boom := errors.New("client gone")
	sink := &logsSink{outErr: boom}
	_, done := startLogsProject(t, fe, ProjectSelector{Project: "shop"}, FollowOptions{Tail: 10}, sink)
	select {
	case err := <-done:
		if !errors.Is(err, boom) {
			t.Fatalf("FollowProject = %v, want the receiver's error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("FollowProject kept running after its receiver failed")
	}
	if n := fe.slotsInUse(); n != 0 {
		t.Fatalf("%d stream slots held after the failure", n)
	}
}

func TestFollowProjectReportsAnUnreadableProjectAtTheStart(t *testing.T) {
	fe := newLogsFakeEngine()
	env := fe.env()
	boom := &dockerhost.StatusError{Status: http.StatusBadGateway, Err: errors.New("engine down")}
	env.list = func(context.Context, *dockerutil.Client) ([]dockerhost.ContainerSummary, error) { return nil, boom }
	opt := FollowOptions{Opened: func() { t.Error("Opened was called for a project that could not be listed") }}
	err := env.followProject(context.Background(), "h1", ProjectSelector{Project: "shop"}, opt, (&logsSink{}).out, nil)
	if dockerhost.StatusOf(err) != http.StatusBadGateway {
		t.Fatalf("FollowProject = %v, want the Engine error unchanged", err)
	}
	if err := env.followProject(context.Background(), "h1", ProjectSelector{}, FollowOptions{}, (&logsSink{}).out, nil); err == nil {
		t.Fatal("a selector without a project must be refused")
	}
}
