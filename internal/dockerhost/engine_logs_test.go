package dockerhost

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

const logTS = "2026-10-28T07:33:29.123456789Z"

// logsEngine registers the container "web" with the given Tty flag and serves
// its log with body.
func logsEngine(t *testing.T, tty bool, body func(w http.ResponseWriter, r *http.Request)) *fakeEngine {
	t.Helper()
	fe := newFakeEngine(t)
	if tty {
		fe.json("GET", "/containers/web/json", http.StatusOK, `{"Config":{"Tty":true}}`)
	} else {
		fe.json("GET", "/containers/web/json", http.StatusOK, `{"Config":{"Tty":false}}`)
	}
	fe.on("GET", "/containers/web/logs", body)
	return fe
}

func collectLogs(t *testing.T, fe *fakeEngine, opt LogOptions) ([]LogLine, error) {
	t.Helper()
	var lines []LogLine
	err := FollowLogs(context.Background(), fe.client(t), "web", opt, func(l LogLine) error {
		lines = append(lines, l)
		return nil
	})
	return lines, err
}

func TestFollowLogsDemultiplexesAndStripsTimestamps(t *testing.T) {
	var query url.Values
	fe := logsEngine(t, false, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		var b bytes.Buffer
		b.Write(muxFrame(1, logTS+" hello\n"))
		b.Write(muxFrame(2, "2026-10-28T07:33:30.000000001Z boom\n"))
		// one line split across two frames, then two lines in one frame
		b.Write(muxFrame(1, "2026-10-28T07:33:31.5Z part"))
		b.Write(muxFrame(1, "ial\n2026-10-28T07:33:32Z a\n2026-10-28T07:33:33Z b\n"))
		b.Write(muxFrame(1, "2026-10-28T07:33:34Z no newline at the end"))
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
		_, _ = w.Write(b.Bytes())
	})
	since := time.Unix(1730000000, 123).UTC()
	opened := 0
	lines, err := collectLogs(t, fe, LogOptions{Tail: 50, Since: since, Follow: true, Opened: func() { opened++ }})
	if err != nil {
		t.Fatalf("FollowLogs: %v", err)
	}
	if opened != 1 {
		t.Fatalf("Opened was called %d times, want once", opened)
	}
	want := []LogLine{
		{Stream: "out", Time: time.Date(2026, 10, 28, 7, 33, 29, 123456789, time.UTC), Text: "hello"},
		{Stream: "err", Time: time.Date(2026, 10, 28, 7, 33, 30, 1, time.UTC), Text: "boom"},
		{Stream: "out", Time: time.Date(2026, 10, 28, 7, 33, 31, 500000000, time.UTC), Text: "partial"},
		{Stream: "out", Time: time.Date(2026, 10, 28, 7, 33, 32, 0, time.UTC), Text: "a"},
		{Stream: "out", Time: time.Date(2026, 10, 28, 7, 33, 33, 0, time.UTC), Text: "b"},
		{Stream: "out", Time: time.Date(2026, 10, 28, 7, 33, 34, 0, time.UTC), Text: "no newline at the end"},
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("lines = %+v\nwant %+v", lines, want)
	}
	for key, value := range map[string]string{"follow": "1", "tail": "50", "timestamps": "1", "stdout": "1", "stderr": "1", "since": "1730000000.000000123"} {
		if got := query.Get(key); got != value {
			t.Errorf("query %s = %q, want %q (query %v)", key, got, value, query)
		}
	}
}

func TestFollowLogsReadsATTYStreamRaw(t *testing.T) {
	long := strings.Repeat("a", MaxLogLineBytes+5000)
	euro := strings.Repeat("€", MaxLogLineBytes) // 3 bytes each: the cut falls inside a character
	fe := logsEngine(t, true, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, logTS+" first\r\n")
		_, _ = io.WriteString(w, logTS+" "+long+"\r\n")
		_, _ = io.WriteString(w, logTS+" "+euro+"\n")
		_, _ = io.WriteString(w, logTS+" \x1b[31mred\x1b[0m\n")
		_, _ = io.WriteString(w, "no timestamp here\n")
	})
	before := time.Now().Add(-time.Second)
	lines, err := collectLogs(t, fe, LogOptions{})
	if err != nil {
		t.Fatalf("FollowLogs: %v", err)
	}
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5: %+v", len(lines), lines)
	}
	if lines[0].Text != "first" || lines[0].Stream != "out" || lines[0].Truncated {
		t.Fatalf("line 0 = %+v, want the text without the carriage return", lines[0])
	}
	if !lines[1].Truncated || lines[1].Text != long[:MaxLogLineBytes] {
		t.Fatalf("long line: truncated=%v len=%d, want a cut at %d", lines[1].Truncated, len(lines[1].Text), MaxLogLineBytes)
	}
	if !lines[2].Truncated || !utf8.ValidString(lines[2].Text) || len(lines[2].Text) != MaxLogLineBytes-1 {
		t.Fatalf("multi-byte line: truncated=%v valid=%v len=%d, want a cut on a character boundary at %d",
			lines[2].Truncated, utf8.ValidString(lines[2].Text), len(lines[2].Text), MaxLogLineBytes-1)
	}
	if lines[3].Text != "\x1b[31mred\x1b[0m" || lines[3].Truncated {
		t.Fatalf("ANSI line = %+v, want the escape sequences kept", lines[3])
	}
	if lines[4].Text != "no timestamp here" || lines[4].Time.Before(before) {
		t.Fatalf("line without a timestamp = %+v, want the whole text and the arrival time", lines[4])
	}
}

func TestFollowLogsReturnsTheCallbackErrorAndStopsOnContext(t *testing.T) {
	handlerDone := make(chan struct{})
	fe := logsEngine(t, true, func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		_, _ = io.WriteString(w, logTS+" one\n"+logTS+" two\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done() // a followed log that stays quiet
	})
	c := fe.client(t)

	stop := errors.New("stop here")
	calls := 0
	err := FollowLogs(context.Background(), c, "web", LogOptions{Follow: true}, func(LogLine) error {
		calls++
		return stop
	})
	if !errors.Is(err, stop) || calls != 1 {
		t.Fatalf("FollowLogs = %v after %d calls, want the callback error after the first line", err, calls)
	}
	<-handlerDone

	fe2 := logsEngine(t, true, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, logTS+" one\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan error, 1)
	first := make(chan struct{}, 1)
	go func() {
		got <- FollowLogs(ctx, fe2.client(t), "web", LogOptions{Follow: true}, func(LogLine) error {
			select {
			case first <- struct{}{}:
			default:
			}
			return nil
		})
	}()
	select {
	case <-first:
	case <-time.After(5 * time.Second):
		t.Fatal("no line arrived")
	}
	cancel()
	select {
	case err := <-got:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("FollowLogs after cancel = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("FollowLogs did not return after the context ended")
	}
}

// A stalled Engine or socket proxy must not hold the request (and the caller's
// stream slot) for ever: no headers within logHeaderTimeout ends the call, while
// a followed log that has started may stay quiet for longer than that.
func TestFollowLogsGivesUpWhenNoHeadersArriveButNotOnAQuietFollow(t *testing.T) {
	prev := logHeaderTimeout
	logHeaderTimeout = 200 * time.Millisecond
	t.Cleanup(func() { logHeaderTimeout = prev })

	stalled := logsEngine(t, false, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // accepts the request and never answers
	})
	began := time.Now()
	_, err := collectLogs(t, stalled, LogOptions{Follow: true})
	if !errors.Is(err, context.DeadlineExceeded) || ClassifyError(err) != CodeTimeout {
		t.Fatalf("stalled Engine: %v, want a timeout error", err)
	}
	if took := time.Since(began); took > 5*time.Second {
		t.Fatalf("FollowLogs held a stalled request for %v", took)
	}

	quiet := logsEngine(t, false, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush() // headers in, then a pause longer than the header wait
		time.Sleep(3 * logHeaderTimeout)
		_, _ = w.Write(muxFrame(1, logTS+" late line\n"))
	})
	lines, err := collectLogs(t, quiet, LogOptions{Follow: true})
	if err != nil || len(lines) != 1 || lines[0].Text != "late line" {
		t.Fatalf("quiet follow = %v, %v; the header wait must stop once the headers are in", lines, err)
	}
}

func TestFollowLogsReportsEngineAnswers(t *testing.T) {
	fe := newFakeEngine(t)
	c := fe.client(t)
	opened := func() { t.Error("Opened was called although the Engine refused the request") }
	err := FollowLogs(context.Background(), c, "gone", LogOptions{Opened: opened}, func(LogLine) error { return nil })
	if StatusOf(err) != http.StatusNotFound {
		t.Fatalf("unknown container: %v (status %d), want 404", err, StatusOf(err))
	}
	fe.json("GET", "/containers/web/json", http.StatusOK, `{"Config":{"Tty":false}}`)
	fe.json("GET", "/containers/web/logs", http.StatusNotImplemented, `{"message":"configured logging driver does not support reading"}`)
	err = FollowLogs(context.Background(), c, "web", LogOptions{Opened: opened}, func(LogLine) error { return nil })
	if StatusOf(err) != http.StatusNotImplemented || !strings.Contains(err.Error(), "does not support reading") {
		t.Fatalf("log driver without reads: %v (status %d), want the 501 answer", err, StatusOf(err))
	}
	if err := FollowLogs(context.Background(), c, "../x", LogOptions{}, func(LogLine) error { return nil }); !errors.Is(err, ErrInvalidContainerRef) {
		t.Fatalf("unsafe reference: %v, want ErrInvalidContainerRef", err)
	}
}

func TestLogQueryClampsTailAndFormatsSince(t *testing.T) {
	cases := []struct {
		opt  LogOptions
		want string
	}{
		{LogOptions{}, "stderr=1&stdout=1&tail=0&timestamps=1"},
		{LogOptions{Tail: 200, Follow: true}, "follow=1&stderr=1&stdout=1&tail=200&timestamps=1"},
		{LogOptions{Tail: 99999}, "stderr=1&stdout=1&tail=5000&timestamps=1"},
		{LogOptions{Tail: -3}, "stderr=1&stdout=1&tail=0&timestamps=1"},
		{LogOptions{Tail: 100, Follow: true, Since: time.Unix(1730000000, 5).UTC()}, "follow=1&since=1730000000.000000005&stderr=1&stdout=1&tail=100&timestamps=1"},
	}
	for _, tc := range cases {
		if got := logQuery(tc.opt); got != tc.want {
			t.Errorf("logQuery(%+v) = %q, want %q", tc.opt, got, tc.want)
		}
	}
}
