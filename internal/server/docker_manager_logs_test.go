package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerstacks"
	"aurago/internal/dockerwatch"
)

// dmLines is a batch of two log lines.
func dmLines() dockerwatch.LogBatch {
	return dockerwatch.LogBatch{Lines: []dockerwatch.LogEntry{
		{T: time.Date(2026, 10, 28, 7, 33, 29, 123456789, time.UTC), S: "out", Text: "hello", Svc: "web", Ctr: "aaaaaaaaaaaa"},
		{T: time.Date(2026, 10, 28, 7, 33, 30, 0, time.UTC), S: "err", Text: "boom", Trunc: true},
	}}
}

func TestDockerManagerContainerLogsFollowStreamsLinesNoticesAndEnds(t *testing.T) {
	f := newDMLiveFixture(t)
	f.live.follow = func(_ context.Context, call dmFollowCall, out func(dockerwatch.LogBatch) error, notice func(dockerwatch.Notice) error) error {
		call.opt.Opened()
		if err := out(dmLines()); err != nil {
			return err
		}
		if err := notice(dockerwatch.Notice{Code: dockerwatch.NoticeNotFollowed, Message: "no slot", Services: []string{"db"}, Container: "bbbbbbbbbbbb"}); err != nil {
			return err
		}
		return out(dockerwatch.LogBatch{Lines: []dockerwatch.LogEntry{{T: time.Date(2026, 10, 28, 7, 33, 31, 0, time.UTC), S: "out", Text: "last"}}})
	}

	// The reconnect of the UI: the newest tail together with the time of the newest line.
	stream := f.open(http.MethodGet, "/hosts/local/containers/web1/logs?follow=1&tail=5000&since=2026-10-28T07:33:29.123456789Z", nil)
	if got := stream.resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	lines := stream.nextEvent()
	entries, _ := lines.json(t)["lines"].([]any)
	first, _ := entries[0].(map[string]any)
	second, _ := entries[1].(map[string]any)
	if lines.event != "lines" || len(entries) != 2 || first["t"] != "2026-10-28T07:33:29.123456789Z" || first["s"] != "out" || first["text"] != "hello" ||
		first["trunc"] != false || first["svc"] != "web" || first["ctr"] != "aaaaaaaaaaaa" || second["s"] != "err" || second["trunc"] != true || second["svc"] != nil {
		t.Fatalf("lines = %s %s", lines.event, lines.data)
	}
	notice := stream.nextEvent()
	if got := notice.json(t); notice.event != "notice" || got["code"] != "not_followed" || got["message"] != "no slot" || got["container"] != "bbbbbbbbbbbb" || len(got["services"].([]any)) != 1 {
		t.Fatalf("notice = %s %s", notice.event, notice.data)
	}
	if last := stream.nextEvent(); last.event != "lines" {
		t.Fatalf("frame = %s %s, want the last lines", last.event, last.data)
	}
	if end := stream.nextEvent(); end.event != "end" || end.json(t)["reason"] != "ended" || end.json(t)["code"] != nil {
		t.Fatalf("end = %s %s, want reason ended", end.event, end.data)
	}
	stream.expectEnd()

	call := f.live.lastCall(t)
	if call.host != "local" || call.ref != "web1" || call.opt.Tail != 5000 || !call.opt.Since.Equal(time.Date(2026, 10, 28, 7, 33, 29, 123456789, time.UTC)) {
		t.Fatalf("follower got %+v, want host local, web1, tail 5000 and the since time", call)
	}

	// Without a tail the container default is 200 and there is no since.
	f.open(http.MethodGet, "/hosts/local/containers/web1/logs?follow=1", nil).nextEvent()
	if call := f.live.lastCall(t); call.opt.Tail != 200 || !call.opt.Since.IsZero() {
		t.Fatalf("follower got %+v, want the default tail 200", call.opt)
	}
}

func TestDockerManagerLogsFollowRefusesABadQueryBeforeStarting(t *testing.T) {
	f := newDMLiveFixture(t)
	f.stacks.listing = dockerstacks.Listing{Stacks: []dockerstacks.StackSummary{{Name: "shop", Dir: "/opt/stacks/shop"}}}
	for _, path := range []string{
		"/hosts/local/containers/web1/logs?follow=1&tail=abc",
		"/hosts/local/containers/web1/logs?follow=1&tail=-1",
		"/hosts/local/containers/web1/logs?follow=1&tail=5001",
		"/hosts/local/containers/web1/logs?follow=1&since=yesterday",
		"/hosts/local/stacks/shop/logs?follow=1&tail=9999",
		"/hosts/local/projects/shop/logs?follow=1&since=2026-10-28",
	} {
		f.open(http.MethodGet, path, nil).expectError(http.StatusBadRequest, "bad_request")
	}
	f.open(http.MethodPost, "/hosts/local/containers/web1/logs?follow=1", nil).expectError(http.StatusMethodNotAllowed, "method_not_allowed")
	f.live.mu.Lock()
	started := len(f.live.calls)
	f.live.mu.Unlock()
	if started != 0 {
		t.Fatalf("%d followers were started for refused requests", started)
	}
}

func TestDockerManagerLogsFollowAnswersEarlyFailuresAsJSON(t *testing.T) {
	f := newDMLiveFixture(t)
	early := func(err error) {
		f.live.follow = func(context.Context, dmFollowCall, func(dockerwatch.LogBatch) error, func(dockerwatch.Notice) error) error {
			return err
		}
	}
	path := "/hosts/local/containers/web1/logs?follow=1"

	early(dockerwatch.ErrTooManyStreams)
	stream := f.open(http.MethodGet, path, nil)
	stream.expectError(http.StatusTooManyRequests, "too_many_streams")
	if got := stream.resp.Header.Get("Content-Type"); strings.Contains(got, "event-stream") {
		t.Fatalf("Content-Type = %q: an early failure is not an event stream", got)
	}
	early(&dockerhost.StatusError{Status: 404, Err: errors.New("No such container: web1")})
	f.open(http.MethodGet, path, nil).expectError(http.StatusNotFound, "container_not_found")
	early(&dockerhost.StatusError{Status: 500, Err: errors.New("engine failure")})
	f.open(http.MethodGet, path, nil).expectError(http.StatusBadGateway, "docker_error")
	early(errors.New("dial tcp 10.0.0.9:2375: i/o timeout"))
	if body := f.open(http.MethodGet, path, nil).expectError(http.StatusBadGateway, "host_unreachable"); body["reason"] == nil || body["reason"] == "" {
		t.Fatalf("answer %v lacks the classified reason", body)
	}
	f.open(http.MethodGet, "/hosts/nowhere/containers/web1/logs?follow=1", nil).expectError(http.StatusNotFound, "host_not_found")
}

func TestDockerManagerLogsFollowEndsWithTheErrorOfAFailureAfterTheHeaders(t *testing.T) {
	f := newDMLiveFixture(t)
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{"connection lost", errors.New("read container logs: connection reset by peer"), "host_unreachable"},
		{"engine refused", &dockerhost.StatusError{Status: 500, Err: errors.New("log driver failed")}, "docker_error"},
		{"container gone", &dockerhost.StatusError{Status: 404, Err: errors.New("No such container: web1")}, "container_not_found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.live.follow = func(_ context.Context, call dmFollowCall, out func(dockerwatch.LogBatch) error, _ func(dockerwatch.Notice) error) error {
				call.opt.Opened()
				_ = out(dmLines())
				return tc.err
			}
			stream := f.open(http.MethodGet, "/hosts/local/containers/web1/logs?follow=1", nil)
			if first := stream.nextEvent(); first.event != "lines" {
				t.Fatalf("first frame = %s %s", first.event, first.data)
			}
			end := stream.nextEvent()
			got := end.json(t)
			if end.event != "end" || got["reason"] != "error" || got["code"] != tc.code || got["message"] == "" || got["message"] == nil {
				t.Fatalf("end = %s %s, want reason error with code %s and a message", end.event, end.data, tc.code)
			}
			stream.expectEnd()
		})
	}
}

func TestDockerManagerLogsFollowHeartbeatsAndStopsWhenTheClientLeaves(t *testing.T) {
	withHeartbeat(t, 20*time.Millisecond)
	f := newDMLiveFixture(t)
	returned := make(chan error, 1)
	f.live.follow = func(ctx context.Context, call dmFollowCall, _ func(dockerwatch.LogBatch) error, _ func(dockerwatch.Notice) error) error {
		call.opt.Opened()
		<-ctx.Done()
		returned <- ctx.Err()
		return ctx.Err()
	}
	stream := f.open(http.MethodGet, "/hosts/local/containers/web1/logs?follow=1", nil)
	if frame := stream.next(); frame.comment != "heartbeat" {
		t.Fatalf("frame = %+v, want a heartbeat comment on a quiet log", frame)
	}
	stream.close()
	select {
	case err := <-returned:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("follower ended with %v, want a cancelled context", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the follower kept running after the client left")
	}
}

func TestDockerManagerContainerLogsWithoutFollowStayTheOneShotJSON(t *testing.T) {
	f := newDMLiveFixture(t)
	body := f.open(http.MethodGet, "/hosts/local/containers/web1/logs?tail=10", nil).body()
	if body["status"] != "ok" || !strings.Contains(body["logs"].(string), "hello from web1") {
		t.Fatalf("one-shot logs = %v", body)
	}
	f.live.mu.Lock()
	started := len(f.live.calls)
	f.live.mu.Unlock()
	if started != 0 {
		t.Fatal("a request without follow started a follower")
	}
}

func TestDockerManagerStackLogsFollowTheStacksFolder(t *testing.T) {
	f := newDMLiveFixture(t)
	f.stacks.listing = dockerstacks.Listing{Stacks: []dockerstacks.StackSummary{{Name: "shop", HostID: "local", Dir: "/opt/stacks/shop"}}}
	f.live.follow = func(_ context.Context, call dmFollowCall, out func(dockerwatch.LogBatch) error, _ func(dockerwatch.Notice) error) error {
		call.opt.Opened()
		return out(dmLines())
	}

	stream := f.open(http.MethodGet, "/hosts/local/stacks/shop/logs?follow=1", nil)
	if lines := stream.nextEvent(); lines.event != "lines" {
		t.Fatalf("frame = %s %s", lines.event, lines.data)
	}
	call := f.live.lastCall(t)
	if call.host != "local" || call.sel != (dockerwatch.ProjectSelector{Project: "shop", WorkingDir: "/opt/stacks/shop"}) || call.opt.Tail != 100 {
		t.Fatalf("follower got %+v, want the project shop from its folder with the default tail 100", call)
	}
	if !f.stacks.called("List local") {
		t.Fatalf("stack calls = %v, want the cached listing", f.stacks.calls)
	}

	f.open(http.MethodGet, "/hosts/local/stacks/shop/logs", nil).expectError(http.StatusBadRequest, "bad_request")
	f.open(http.MethodGet, "/hosts/local/stacks/blog/logs?follow=1", nil).expectError(http.StatusNotFound, "stack_not_found")
	f.open(http.MethodGet, "/hosts/local/stacks/Bad_Name/logs?follow=1", nil).expectError(http.StatusBadRequest, "stack_name_invalid")
	f.stacks.errs["List"] = &dockerstacks.Error{Code: dockerstacks.CodeHostNotPrepared, Message: "the host is not prepared"}
	f.open(http.MethodGet, "/hosts/local/stacks/shop/logs?follow=1", nil).expectError(http.StatusConflict, "host_not_prepared")
	f.noStacks = true
	f.open(http.MethodGet, "/hosts/local/stacks/shop/logs?follow=1", nil).expectError(http.StatusServiceUnavailable, "stacks_unavailable")
}

func TestDockerManagerProjectLogsFollowAnExternalProject(t *testing.T) {
	f := newDMLiveFixture(t)
	f.live.follow = func(_ context.Context, call dmFollowCall, out func(dockerwatch.LogBatch) error, _ func(dockerwatch.Notice) error) error {
		call.opt.Opened()
		return out(dmLines())
	}
	follow := func(project string) *dmStream {
		return f.open(http.MethodGet, "/hosts/local/projects/"+project+"/logs?follow=1&tail=20", nil)
	}

	// The local fake Engine has the compose project "shop" (container web1).
	if lines := follow("shop").nextEvent(); lines.event != "lines" {
		t.Fatalf("frame = %s %s", lines.event, lines.data)
	}
	if call := f.live.lastCall(t); call.sel != (dockerwatch.ProjectSelector{Project: "shop"}) || call.opt.Tail != 20 {
		t.Fatalf("follower got %+v, want the whole project shop with tail 20", call)
	}

	// A project without containers.
	follow("ghost").expectError(http.StatusNotFound, "project_not_found")
	f.open(http.MethodGet, "/hosts/local/projects/shop/logs", nil).expectError(http.StatusBadRequest, "bad_request")

	// A stack of the same name takes only the containers of its folder: the project keeps the rest.
	f.stacks.listing = dockerstacks.Listing{Stacks: []dockerstacks.StackSummary{{Name: "shop", Dir: "/opt/stacks/shop"}}}
	follow("shop").nextEvent()
	if call := f.live.lastCall(t); call.sel != (dockerwatch.ProjectSelector{Project: "shop", WorkingDir: "/opt/stacks/shop", Exclude: true}) {
		t.Fatalf("follower got %+v, want the stack's folder excluded", call.sel)
	}
	f.local.setContainers(dmContainersJSON(t, dmContainerJSON('a', "web1", "sha256:img1", "running", "bridge",
		map[string]string{"com.docker.compose.project": "shop", "com.docker.compose.project.working_dir": "/opt/stacks/shop"}, nil)))
	follow("shop").expectError(http.StatusNotFound, "project_not_found")

	// A host without the stack helpers has no stacks: the project is external as a whole.
	f.stacks.errs["List"] = &dockerstacks.Error{Code: dockerstacks.CodeHostNotPrepared, Message: "the host is not prepared"}
	follow("shop").nextEvent()
	if call := f.live.lastCall(t); call.sel != (dockerwatch.ProjectSelector{Project: "shop"}) {
		t.Fatalf("follower got %+v, want the whole project when the host has no stacks", call.sel)
	}
	f.noStacks = true
	follow("shop").nextEvent()

	// An Engine that does not answer the list is a plain failure.
	f.stacks.errs["List"] = nil
	f.local.force("GET /containers/json", http.StatusInternalServerError)
	follow("shop").expectError(http.StatusBadGateway, "docker_error")
}

// TestDockerManagerContainerLogsFollowReadsTheEnginesLog runs the whole path: the
// real hub follows a container of a fake Engine.
func TestDockerManagerContainerLogsFollowReadsTheEnginesLog(t *testing.T) {
	ls := newDMLiveServer(t, "[]")
	stream := ls.open(http.MethodGet, "/hosts/local/containers/web/logs?follow=1&tail=5000&since=2026-10-28T07:33:00Z", nil, nil)
	lines := stream.nextEvent()
	entries, _ := lines.json(t)["lines"].([]any)
	if lines.event != "lines" || len(entries) != 2 {
		t.Fatalf("frame = %s %s, want the two lines of the Engine's log", lines.event, lines.data)
	}
	out, _ := entries[0].(map[string]any)
	errLine, _ := entries[1].(map[string]any)
	if out["s"] != "out" || out["text"] != "hello from the engine" || out["t"] != "2026-10-28T07:33:29.000000001Z" || errLine["s"] != "err" || errLine["text"] != "something went wrong" {
		t.Fatalf("lines = %v", entries)
	}
	if q := ls.engine.logRequests(); len(q) != 1 || !strings.Contains(q[0], "follow=1") || !strings.Contains(q[0], "tail=5000") || !strings.Contains(q[0], "since=") || !strings.Contains(q[0], "timestamps=1") {
		t.Fatalf("log requests = %v, want follow, tail 5000, since and timestamps", q)
	}

	ls.engine.release() // the container's log ends
	if end := stream.nextEvent(); end.event != "end" || end.json(t)["reason"] != "ended" {
		t.Fatalf("end = %s %s", end.event, end.data)
	}
	stream.expectEnd()

	ls.open(http.MethodGet, "/hosts/local/containers/ghost/logs?follow=1", nil, nil).expectError(http.StatusNotFound, "container_not_found")
	// With every stream slot of the host taken, a follow is refused as JSON.
	for i := 0; i < dockerwatch.MaxStreamsPerHost; i++ {
		release, err := ls.s.DockerWatch.Acquire("local")
		if err != nil {
			t.Fatalf("Acquire %d: %v", i, err)
		}
		t.Cleanup(release)
	}
	ls.open(http.MethodGet, "/hosts/local/containers/web/logs?follow=1", nil, nil).expectError(http.StatusTooManyRequests, "too_many_streams")
}
