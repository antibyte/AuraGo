package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerstacks"
	"aurago/internal/dockerwatch"
)

// ── Docker Manager log follow ───────────────────────────────────────────────
// The three `logs?follow=1` routes answer a server-sent event stream:
//
//   - "event: lines":  {"lines":[{"t","s","text","trunc","svc","ctr"}, ...]}
//   - "event: notice": {"code","message","services","container"}
//   - "event: end":    {"reason":"ended"} or {"reason":"error","code","message"}
//   - ": heartbeat" comments every dmSSEHeartbeat while nothing happens.
//
// Anything that goes wrong before the stream is up (unknown host, container,
// stack or project, no free stream slot, an unreachable host, a bad query) is an
// ordinary JSON error answer; the headers of the stream go out only when the
// follower reports that it is established (FollowOptions.Opened). A client that
// leaves ends the follow and gets nothing more. docker.readonly allows it.

const (
	dmLogTailContainer = 200 // default `tail` of a container
	dmLogTailProject   = 100 // default `tail` per container of a stack or project
)

// logOptions reads `tail` (0 to 5000, default def) and `since` (RFC 3339) of a
// follow request. A bad value answers 400.
func (dm *dmRequest) logOptions(def int) (dockerwatch.FollowOptions, bool) {
	query := dm.r.URL.Query()
	opt := dockerwatch.FollowOptions{Tail: def}
	if raw := query.Get("tail"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > dockerwatch.MaxLogTail {
			dmWriteError(dm.w, http.StatusBadRequest, dmCodeBadRequest, "tail must be a number from 0 to "+strconv.Itoa(dockerwatch.MaxLogTail), nil)
			return opt, false
		}
		opt.Tail = n
	}
	if raw := query.Get("since"); raw != "" {
		since, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			dmWriteError(dm.w, http.StatusBadRequest, dmCodeBadRequest, "since must be an RFC 3339 time, for example 2026-10-28T07:33:29Z", nil)
			return opt, false
		}
		opt.Since = since
	}
	return opt, true
}

// dmFollowFunc starts one follower with the options, the callbacks of the
// stream and the context that ends it.
type dmFollowFunc func(ctx context.Context, opt dockerwatch.FollowOptions, out func(dockerwatch.LogBatch) error, notice func(dockerwatch.Notice) error) error

// containerLogsFollow serves GET /hosts/{id}/containers/{ref}/logs?follow=1.
func (dm *dmRequest) containerLogsFollow(hostID, ref string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	live, ok := dm.liveService()
	if !ok {
		return
	}
	opt, ok := dm.logOptions(dmLogTailContainer)
	if !ok {
		return
	}
	ctx, cancel := dm.context(dmRegistryTimeout)
	_, known := dm.host(ctx, hostID)
	cancel()
	if !known {
		return
	}
	dm.streamLogs(opt, dmCodeContainerNotFound, func(ctx context.Context, opt dockerwatch.FollowOptions, out func(dockerwatch.LogBatch) error, notice func(dockerwatch.Notice) error) error {
		return live.FollowContainer(ctx, hostID, ref, opt, out, notice)
	})
}

// stackFolder finds a managed stack of the host in the stacks listing (the
// cached folder scan, the containers read live) and returns its folder on the
// host. found is false when the host has no stack of that name.
func (dm *dmRequest) stackFolder(ctx context.Context, svc dmStackService, hostID, name string) (dir string, found bool, err error) {
	listing, err := svc.List(ctx, hostID, false)
	if err != nil {
		return "", false, err
	}
	for _, stack := range listing.Stacks {
		if stack.Name == name {
			return stack.Dir, true, nil
		}
	}
	return "", false, nil
}

// followRequired answers 400 unless the request carries follow=1: stack and
// project logs exist only as a stream.
func (dm *dmRequest) followRequired() bool {
	if dmTruthy(dm.r.URL.Query().Get("follow")) {
		return true
	}
	dmWriteError(dm.w, http.StatusBadRequest, dmCodeBadRequest, "logs of a stack or project are a live stream: add follow=1", nil)
	return false
}

// stackLogs serves GET /hosts/{id}/stacks/{name}/logs?follow=1: the merged logs
// of the containers the stack's folder started. A stack without containers is a
// valid stream (nothing is sent until one starts); an unknown stack is 404
// stack_not_found.
func (dm *dmRequest) stackLogs(hostID, name string) {
	if !dm.allow(http.MethodGet) || !dm.followRequired() {
		return
	}
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	live, ok := dm.liveService()
	if !ok {
		return
	}
	opt, ok := dm.logOptions(dmLogTailProject)
	if !ok {
		return
	}
	if err := dockerstacks.ValidateName(name); err != nil {
		dm.stackFailure(err)
		return
	}
	ctx, cancel := dm.context(dmStackReadTimeout)
	dir, found, err := dm.stackFolder(ctx, svc, hostID, name)
	cancel()
	switch {
	case err != nil:
		dm.stackFailure(err)
		return
	case !found:
		dmWriteError(dm.w, http.StatusNotFound, dockerstacks.CodeStackNotFound, "there is no stack "+strconv.Quote(name)+" on this host", nil)
		return
	}
	sel := dockerwatch.ProjectSelector{Project: name, WorkingDir: dir}
	dm.streamLogs(opt, "", func(ctx context.Context, opt dockerwatch.FollowOptions, out func(dockerwatch.LogBatch) error, notice func(dockerwatch.Notice) error) error {
		return live.FollowProject(ctx, hostID, sel, opt, out, notice)
	})
}

// projectSub serves /hosts/{id}/projects/{project}/{logs|start|stop|restart}.
func (dm *dmRequest) projectSub(hostID, project, sub string) {
	if sub == "logs" {
		dm.projectLogs(hostID, project)
		return
	}
	dm.projectAction(hostID, project, sub)
}

// projectLogs serves GET /hosts/{id}/projects/{project}/logs?follow=1: the merged
// logs of an external compose project. When the host has a stack of the same
// name, only the containers that do not come from that stack's folder belong to
// the project (the same split the stack list makes). A project without any
// container is 404 project_not_found.
func (dm *dmRequest) projectLogs(hostID, project string) {
	if !dm.allow(http.MethodGet) || !dm.followRequired() {
		return
	}
	live, ok := dm.liveService()
	if !ok {
		return
	}
	opt, ok := dm.logOptions(dmLogTailProject)
	if !ok {
		return
	}
	ctx, cancel := dm.context(dmStackReadTimeout)
	defer cancel()
	_, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	sel := dockerwatch.ProjectSelector{Project: project}
	if dm.stacks != nil {
		dir, found, err := dm.stackFolder(ctx, dm.stacks, hostID, project)
		var coded *dockerstacks.Error
		switch {
		case err == nil && found:
			sel.WorkingDir, sel.Exclude = dir, true
		case err == nil, errors.As(err, &coded):
			// No stack of that name, or a host without the stack helpers (it has no
			// stacks): the whole project is external.
		default:
			dm.stackFailure(err)
			return
		}
	}
	list, err := dockerhost.ListContainers(ctx, client)
	if err != nil {
		dm.engineFailure(err, "")
		return
	}
	members := 0
	for _, container := range list {
		if sel.Matches(container) {
			members++
		}
	}
	if members == 0 {
		dmWriteError(dm.w, http.StatusNotFound, dmCodeProjectNotFound, "there is no compose project "+strconv.Quote(project)+" with containers on this host", nil)
		return
	}
	cancel()
	dm.streamLogs(opt, "", func(ctx context.Context, opt dockerwatch.FollowOptions, out func(dockerwatch.LogBatch) error, notice func(dockerwatch.Notice) error) error {
		return live.FollowProject(ctx, hostID, sel, opt, out, notice)
	})
}

// dmLogStream is the event stream of one follow. Lines, notices and the
// heartbeat are written by different goroutines under one mutex.
type dmLogStream struct {
	dm     *dmRequest
	cancel context.CancelFunc // ends the follow when a write fails

	once   sync.Once
	stop   chan struct{} // ends the heartbeat
	done   chan struct{} // closed when the heartbeat goroutine has returned
	mu     sync.Mutex
	stream *dmEventStream // nil until open
	failed bool           // a write failed: the client is gone or too slow
}

// open sends the headers of the event stream and starts the heartbeat. It is the
// follower's Opened callback and runs once.
func (l *dmLogStream) open() {
	l.once.Do(func() {
		w := l.dm.w
		flusher := w.(http.Flusher) // checked by streamLogs
		header := w.Header()
		header.Set("Content-Type", "text/event-stream")
		header.Set("Cache-Control", "no-cache")
		header.Set("X-Accel-Buffering", "no")
		l.mu.Lock()
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		l.stream = &dmEventStream{w: w, flusher: flusher, rc: http.NewResponseController(w)}
		l.mu.Unlock()
		go l.heartbeat()
	})
}

func (l *dmLogStream) opened() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stream != nil
}

// heartbeat writes a comment line whenever the stream has been quiet for
// dmSSEHeartbeat, and ends the follow when that write fails.
func (l *dmLogStream) heartbeat() {
	defer close(l.done)
	tick := time.NewTicker(dmSSEHeartbeat)
	defer tick.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-tick.C:
			if l.write(func(st *dmEventStream) error { return st.comment("heartbeat") }) != nil {
				l.cancel()
				return
			}
		}
	}
}

// write runs one write on the stream under the mutex. After a failed write it
// refuses all further ones.
func (l *dmLogStream) write(fn func(*dmEventStream) error) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stream == nil {
		return errors.New("the log stream is not open")
	}
	if l.failed {
		return errors.New("the log stream is closed")
	}
	if err := fn(l.stream); err != nil {
		l.failed = true
		return err
	}
	return nil
}

func (l *dmLogStream) lines(batch dockerwatch.LogBatch) error {
	return l.write(func(st *dmEventStream) error { return st.send("lines", batch) })
}

func (l *dmLogStream) notice(n dockerwatch.Notice) error {
	return l.write(func(st *dmEventStream) error { return st.send("notice", n) })
}

// dmLogEnd is the data of the closing `end` event.
type dmLogEnd struct {
	Reason  string `json:"reason"` // ended | error
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// finish stops the heartbeat and writes the closing event for the follower's
// result: err nil is a log that ended; an error is reported with the code the
// route would have answered with before the stream was up. A follow that the
// client cancelled, or one whose writes failed, gets nothing.
func (l *dmLogStream) finish(err error, notFoundCode string) {
	close(l.stop)
	<-l.done
	if l.dm.r.Context().Err() != nil {
		return
	}
	end := dmLogEnd{Reason: "ended"}
	if err != nil {
		failure := dmLiveFailureOf(err, notFoundCode)
		end = dmLogEnd{Reason: "error", Code: failure.code, Message: failure.message}
	}
	_ = l.write(func(st *dmEventStream) error { return st.send("end", end) })
}

// streamLogs runs a follower and turns its callbacks into the event stream.
// Errors that precede the stream's start are answered as JSON.
func (dm *dmRequest) streamLogs(opt dockerwatch.FollowOptions, notFoundCode string, run dmFollowFunc) {
	if _, ok := dm.w.(http.Flusher); !ok {
		dmWriteError(dm.w, http.StatusInternalServerError, dmCodeInternal, "streaming is not supported", nil)
		return
	}
	ctx, cancel := context.WithCancel(dm.r.Context())
	defer cancel()
	ls := &dmLogStream{dm: dm, cancel: cancel, stop: make(chan struct{}), done: make(chan struct{})}
	opt.Opened = ls.open
	err := run(ctx, opt, ls.lines, ls.notice)

	if !ls.opened() {
		if dm.r.Context().Err() != nil {
			return // the client left before the stream was up
		}
		if err != nil {
			dm.liveFailure(err, notFoundCode)
			return
		}
		ls.open() // a follower that ended without ever opening: an empty stream that ended
	}
	ls.finish(err, notFoundCode)
}
