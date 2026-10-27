package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerstacks"
)

// dmSSEFrame is one block of an event stream: an event with its data, or a comment.
type dmSSEFrame struct {
	event, data, comment string
}

// dmParseSSE splits a finished stream into frames.
func dmParseSSE(t *testing.T, stream string) []dmSSEFrame {
	t.Helper()
	var frames []dmSSEFrame
	reader := bufio.NewReader(strings.NewReader(stream))
	for {
		frame, err := dmReadSSEFrame(reader)
		if err != nil {
			return frames
		}
		frames = append(frames, frame)
	}
}

// dmReadSSEFrame reads lines up to the blank line that ends a frame.
func dmReadSSEFrame(r *bufio.Reader) (dmSSEFrame, error) {
	var frame dmSSEFrame
	seen := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return frame, err
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if seen {
				return frame, nil
			}
		case strings.HasPrefix(line, ": "):
			frame.comment, seen = strings.TrimPrefix(line, ": "), true
		case strings.HasPrefix(line, "event: "):
			frame.event, seen = strings.TrimPrefix(line, "event: "), true
		case strings.HasPrefix(line, "data: "):
			frame.data, seen = strings.TrimPrefix(line, "data: "), true
		}
	}
}

func dmOp(state, output string) dockerstacks.Operation {
	return dockerstacks.Operation{ID: "op_0123456789abcdef", HostID: "local", Stack: "web", Kind: dockerstacks.KindUp, State: state, Output: output}
}

func TestDockerManagerOperationListAndGet(t *testing.T) {
	f := newDMStackFixture(t)

	// No operations: an empty array, never null.
	code, body := f.do(http.MethodGet, "/operations", nil)
	if code != http.StatusOK || len(dmArray(t, body, "operations")) != 0 || strings.Contains(f.lastRaw, "null") {
		t.Fatalf("empty list = %d %s, want an empty operations array", code, f.lastRaw)
	}

	// The filter reaches the service, and the list carries no output.
	f.fake.history = []dockerstacks.Operation{dmOp(dockerstacks.StateRunning, "secret output tail")}
	code, body = f.do(http.MethodGet, "/operations?host=local&stack=web&active=1&limit=5", nil)
	want := dockerstacks.OperationFilter{HostID: "local", Stack: "web", Active: true, Limit: 5}
	if got := dmArray(t, body, "operations"); code != http.StatusOK || len(got) != 1 || got[0]["output"] != "" || f.fake.filter != want {
		t.Fatalf("filtered list = %d %s (filter %+v), want one operation without output and filter %+v", code, f.lastRaw, f.fake.filter, want)
	}
	f.do(http.MethodGet, "/operations?limit=abc", nil)
	if f.fake.filter.Limit != 0 {
		t.Fatalf("filter = %+v, want no limit for a malformed ?limit", f.fake.filter)
	}

	// One operation carries its output; an unknown id is a 404.
	f.fake.operation = dmOp(dockerstacks.StateSucceeded, "all done")
	code, body = f.do(http.MethodGet, "/operations/op_0123456789abcdef", nil)
	if op := dmObject(t, body, "operation"); code != http.StatusOK || op["output"] != "all done" || op["state"] != "succeeded" {
		t.Fatalf("get = %d %s, want the operation with its output", code, f.lastRaw)
	}
	f.fake.errs["Operation"] = &dockerstacks.Error{Code: dockerstacks.CodeOperationNotFound, Message: "no such operation"}
	code, body = f.do(http.MethodGet, "/operations/op_ffffffffffffffff", nil)
	f.expectError(code, body, http.StatusNotFound, "operation_not_found")
	code, body = f.do(http.MethodPost, "/operations", nil)
	f.expectError(code, body, http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestDockerManagerOperationStreamOfAFinishedOperation(t *testing.T) {
	f := newDMStackFixture(t)
	f.fake.snapshot = dmOp(dockerstacks.StateSucceeded, "all done")

	rec := f.serve(http.MethodGet, "/operations/op_0123456789abcdef/stream", nil)
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" || rec.Header().Get("Cache-Control") != "no-cache" || rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("headers = %v, want the event stream headers", rec.Header())
	}
	frames := dmParseSSE(t, rec.Body.String())
	if len(frames) != 2 || frames[0].event != "snapshot" || frames[1].event != "done" {
		t.Fatalf("frames = %+v, want snapshot then done", frames)
	}
	var snapshot dockerstacks.Operation
	if err := json.Unmarshal([]byte(frames[0].data), &snapshot); err != nil || snapshot.Output != "all done" {
		t.Fatalf("snapshot data = %q (%v), want the operation with its output", frames[0].data, err)
	}
	if !f.fake.called("Unsubscribe op_0123456789abcdef") {
		t.Fatal("the subscription was not cancelled")
	}

	f.fake.errs["Subscribe"] = &dockerstacks.Error{Code: dockerstacks.CodeOperationNotFound, Message: "no such operation"}
	code, body := f.do(http.MethodGet, "/operations/op_ffffffffffffffff/stream", nil)
	f.expectError(code, body, http.StatusNotFound, "operation_not_found")
}

func TestDockerManagerOperationStreamEndsWhenTheChannelClosesWithoutDone(t *testing.T) {
	f := newDMStackFixture(t)
	f.fake.snapshot = dmOp(dockerstacks.StateRunning, "pulling")
	f.fake.operation = dmOp(dockerstacks.StateRunning, "pulling") // what Operation answers afterwards

	// The subscriber was dropped (or AuraGo is closing) and the operation goes on: the stream just ends.
	rec := f.serve(http.MethodGet, "/operations/op_0123456789abcdef/stream", nil)
	frames := dmParseSSE(t, rec.Body.String())
	if len(frames) != 1 || frames[0].event != "snapshot" {
		t.Fatalf("frames = %+v, want only the snapshot so that the client reconnects", frames)
	}

	// The operation finished while the subscriber was away: the client still gets done.
	f.fake.operation = dmOp(dockerstacks.StateFailed, "boom")
	rec = f.serve(http.MethodGet, "/operations/op_0123456789abcdef/stream", nil)
	frames = dmParseSSE(t, rec.Body.String())
	if len(frames) != 2 || frames[1].event != "done" || !strings.Contains(frames[1].data, `"state":"failed"`) {
		t.Fatalf("frames = %+v, want snapshot then done with the final state", frames)
	}
}

func TestDockerManagerOperationStreamFollowsALiveOperation(t *testing.T) {
	old := dmSSEHeartbeat
	dmSSEHeartbeat = 20 * time.Millisecond
	t.Cleanup(func() { dmSSEHeartbeat = old })

	f := newDMStackFixture(t)
	f.fake.snapshot = dmOp(dockerstacks.StateRunning, "pulling")
	f.fake.events = make(chan dockerstacks.Event, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dm := &dmRequest{s: f.s, w: w, r: r, stacks: f.fake}
		dm.route(dockerManagerSegments(r.URL.Path))
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/docker-manager/operations/op_0123456789abcdef/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	next := func() dmSSEFrame {
		t.Helper()
		frame, err := dmReadSSEFrame(reader)
		if err != nil {
			t.Fatalf("stream ended early: %v", err)
		}
		return frame
	}

	if frame := next(); frame.event != "snapshot" {
		t.Fatalf("first frame = %+v, want the snapshot", frame)
	}
	// A quiet stream writes comments.
	if frame := next(); frame.comment != "heartbeat" {
		t.Fatalf("second frame = %+v, want a heartbeat comment", frame)
	}

	progress := &dockerstacks.Progress{ID: "nginx", Status: "Downloading", Current: 5, Total: 10, Percent: 50}
	running := dmOp(dockerstacks.StateHealthWait, "")
	finished := dmOp(dockerstacks.StateSucceeded, "all done")
	f.fake.events <- dockerstacks.Event{Type: dockerstacks.EventOutput, Seq: 1, Text: "Container web Started\n"}
	f.fake.events <- dockerstacks.Event{Type: dockerstacks.EventProgress, Seq: 2, Progress: progress}
	f.fake.events <- dockerstacks.Event{Type: dockerstacks.EventState, Seq: 3, Operation: &running}
	f.fake.events <- dockerstacks.Event{Type: dockerstacks.EventDone, Seq: 4, Operation: &finished}

	var order []string
	for {
		frame, err := dmReadSSEFrame(reader)
		if err != nil {
			t.Fatalf("stream ended before done: %v (frames so far %v)", err, order)
		}
		if frame.event == "" {
			continue // heartbeats may interleave
		}
		order = append(order, frame.event)
		if frame.event == "output" && !strings.Contains(frame.data, "Container web Started") {
			t.Fatalf("output frame = %q, want the chunk", frame.data)
		}
		if frame.event == "progress" && !strings.Contains(frame.data, `"percent":50`) {
			t.Fatalf("progress frame = %q, want the parsed progress", frame.data)
		}
		if frame.event == "done" {
			if !strings.Contains(frame.data, `"output":"all done"`) {
				t.Fatalf("done frame = %q, want the full final operation", frame.data)
			}
			break
		}
	}
	if strings.Join(order, ",") != "output,progress,state,done" {
		t.Fatalf("events = %v, want output, progress, state, done in order", order)
	}
	// After done the handler returns: the connection reaches EOF and the subscription is cancelled.
	if _, err := io.ReadAll(reader); err != nil {
		t.Fatalf("reading to the end of the stream: %v", err)
	}
	dmWaitFor(t, "the subscription to be cancelled", func() bool { return f.fake.called("Unsubscribe op_0123456789abcdef") })
}

func TestDockerManagerOperationStreamEndsWhenTheClientGoesAway(t *testing.T) {
	f := newDMStackFixture(t)
	f.fake.snapshot = dmOp(dockerstacks.StateRunning, "")
	f.fake.events = make(chan dockerstacks.Event) // never delivers
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dm := &dmRequest{s: f.s, w: w, r: r, stacks: f.fake}
		dm.route(dockerManagerSegments(r.URL.Path))
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/docker-manager/operations/op_0123456789abcdef/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if frame, err := dmReadSSEFrame(bufio.NewReader(resp.Body)); err != nil || frame.event != "snapshot" {
		t.Fatalf("first frame = %+v, %v, want the snapshot", frame, err)
	}
	cancel()
	dmWaitFor(t, "the handler to cancel its subscription after the client left", func() bool { return f.fake.called("Unsubscribe op_0123456789abcdef") })
}

// dmWaitFor polls cond for up to five seconds.
func dmWaitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
