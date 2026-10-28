package dockerhost

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestExecCreateResizeAndDetachedStart(t *testing.T) {
	fe := newFakeEngine(t)
	var bodies []string
	fe.on("POST", "/containers/web/exec", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"Id":"e1e1e1"}`)
	})
	fe.on("POST", "/containers/stopped/exec", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"message":"Container stopped is not running"}`)
	})
	var resized, started string
	fe.on("POST", "/exec/e1e1e1/resize", func(w http.ResponseWriter, r *http.Request) {
		resized = r.URL.RawQuery
	})
	fe.on("POST", "/exec/e1e1e1/start", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		started = string(b)
	})
	c := fe.client(t)
	ctx := context.Background()

	id, err := ExecCreate(ctx, c, "web", []string{"/bin/sh", "-c", "echo hi"}, []string{"TERM=xterm-256color"}, true)
	if err != nil || id != "e1e1e1" {
		t.Fatalf("ExecCreate = %q, %v", id, err)
	}
	if _, err := ExecCreate(ctx, c, "web", []string{"/bin/true"}, nil, false); err != nil {
		t.Fatalf("ExecCreate (no tty): %v", err)
	}
	var tty, plain struct {
		AttachStdin, AttachStdout, AttachStderr, Tty bool
		Cmd, Env                                     []string
	}
	if err := json.Unmarshal([]byte(bodies[0]), &tty); err != nil || !tty.Tty || !tty.AttachStdin || !tty.AttachStdout || !tty.AttachStderr ||
		!slices.Equal(tty.Cmd, []string{"/bin/sh", "-c", "echo hi"}) || !slices.Equal(tty.Env, []string{"TERM=xterm-256color"}) {
		t.Fatalf("interactive exec body = %s (%v)", bodies[0], err)
	}
	if err := json.Unmarshal([]byte(bodies[1]), &plain); err != nil || plain.Tty || plain.AttachStdin || plain.AttachStdout || plain.AttachStderr || len(plain.Env) != 0 {
		t.Fatalf("helper exec body = %s (%v), want nothing attached", bodies[1], err)
	}
	if _, err := ExecCreate(ctx, c, "stopped", []string{"sh"}, nil, true); StatusOf(err) != http.StatusConflict {
		t.Fatalf("exec in a stopped container: %v (status %d), want 409", err, StatusOf(err))
	}
	if _, err := ExecCreate(ctx, c, "web", nil, nil, true); err == nil {
		t.Fatal("an exec without a command must be refused before any request")
	}

	if err := ExecResize(ctx, c, "e1e1e1", 132, 43); err != nil || resized != "h=43&w=132" {
		t.Fatalf("ExecResize = %v with query %q, want h=43&w=132", err, resized)
	}
	if err := ExecResize(ctx, c, "e1e1e1", 0, 24); err == nil {
		t.Fatal("a zero width must be refused")
	}
	if err := ExecStartDetached(ctx, c, "e1e1e1"); err != nil || started != `{"Detach":true,"Tty":false}` {
		t.Fatalf("ExecStartDetached = %v with body %q", err, started)
	}
	if err := ExecStartDetached(ctx, c, "bad/id"); !errors.Is(err, ErrInvalidExecID) {
		t.Fatalf("unsafe exec id: %v, want ErrInvalidExecID", err)
	}
}

// seenExec is what the hijacking fake saw of the exec start request.
type seenExec struct {
	method, path, body string
	header             http.Header
}

// hijackExec answers POST /exec/<id>/start like dockerd: 101 and then a raw
// duplex stream, here an echo of whatever the client writes.
func hijackExec(t *testing.T, fe *fakeEngine, route string, seen *seenExec, firstBytes string) {
	t.Helper()
	fe.on("POST", route, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen.method, seen.path, seen.header, seen.body = r.Method, r.URL.Path, r.Header.Clone(), string(body)
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		// The header and the first stream bytes leave in one write.
		_, _ = buf.WriteString("HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n" + firstBytes)
		_ = buf.Flush()
		chunk := make([]byte, 64)
		for {
			n, err := buf.Read(chunk)
			if n > 0 {
				_, _ = conn.Write(chunk[:n])
			}
			if err != nil {
				return
			}
		}
	})
}

func TestExecStartHijacksTheConnectionWithTheNegotiatedVersion(t *testing.T) {
	fe := newFakeEngine(t)
	fe.setVersion("1.43", "24.0.0")
	var seen seenExec
	hijackExec(t, fe, "/exec/e1e1e1/start", &seen, "prompt$ ")
	c := fe.client(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := ExecStart(ctx, c, "e1e1e1")
	if err != nil {
		t.Fatalf("ExecStart: %v", err)
	}
	defer stream.Close()

	got := make([]byte, len("prompt$ "))
	if _, err := io.ReadFull(stream, got); err != nil || string(got) != "prompt$ " {
		t.Fatalf("first stream bytes = %q, %v; the bytes behind the response header must not be lost", got, err)
	}
	if _, err := stream.Write([]byte("ls\r")); err != nil {
		t.Fatalf("write: %v", err)
	}
	echo := make([]byte, 3)
	if _, err := io.ReadFull(stream, echo); err != nil || string(echo) != "ls\r" {
		t.Fatalf("echo = %q, %v", echo, err)
	}
	if _, ok := stream.(interface{ SetWriteDeadline(time.Time) error }); !ok {
		t.Fatal("the stream must expose SetWriteDeadline for the terminal's write bound")
	}

	if seen.path != "/v1.43/exec/e1e1e1/start" || seen.method != http.MethodPost {
		t.Fatalf("request = %s %s, want POST /v1.43/exec/e1e1e1/start (the negotiated version)", seen.method, seen.path)
	}
	if !strings.EqualFold(seen.header.Get("Connection"), "Upgrade") || seen.header.Get("Upgrade") != "tcp" || seen.header.Get("Content-Type") != "application/json" {
		t.Fatalf("headers = %v, want an upgrade to tcp with a JSON body", seen.header)
	}
	if seen.body != `{"Detach":false,"Tty":true}` {
		t.Fatalf("body = %q", seen.body)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := stream.Read(make([]byte, 1)); err == nil {
		t.Fatal("Read after Close must fail")
	}
}

func TestExecStartReportsRefusalsAndHonoursTheContext(t *testing.T) {
	fe := newFakeEngine(t)
	fe.on("POST", "/exec/dead/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"message":"Container abc is paused"}`)
	})
	silent := make(chan struct{})
	t.Cleanup(func() { close(silent) })
	fe.on("POST", "/exec/silent/start", func(w http.ResponseWriter, r *http.Request) {
		select { // a peer that accepts the request and never answers
		case <-silent:
		case <-r.Context().Done():
		}
	})
	c := fe.client(t)

	_, err := ExecStart(context.Background(), c, "dead")
	if StatusOf(err) != http.StatusConflict || !strings.Contains(err.Error(), "is paused") {
		t.Fatalf("refused start: %v (status %d), want the 409 answer", err, StatusOf(err))
	}
	if _, err := ExecStart(context.Background(), c, "unknown"); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("unknown exec: %v (status %d), want 404", err, StatusOf(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	began := time.Now()
	if _, err := ExecStart(ctx, c, "silent"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("silent peer: %v, want the context deadline", err)
	}
	if time.Since(began) > 5*time.Second {
		t.Fatal("ExecStart held the handshake past the context")
	}
	if _, err := ExecStart(context.Background(), c, "../x"); !errors.Is(err, ErrInvalidExecID) {
		t.Fatalf("unsafe exec id: %v, want ErrInvalidExecID", err)
	}
}
