package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"

	"github.com/gorilla/websocket"
)

// The Docker Manager's terminal is the second backend of the container
// terminal protocol (binary stdin/stdout, text {"type":"resize"} and
// {"type":"end"}): where the Containers page talks to the Engine named in
// tools.DockerConfig, this one runs on the *dockerutil.Client of any registered
// host, so it works on TLS, mutual TLS and SSH hosts too. The browser side is
// serveContainerTerminalSessionWith with the options below.

// dockerManagerTerminalCommand starts bash as a login shell when the image has
// it, else sh.
var dockerManagerTerminalCommand = []string{"/bin/sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash -l; fi; exec sh -l"}

// dockerManagerTerminalServeOptions bound the browser side of the Docker
// Manager terminal: frames up to 1 MiB, a ping every 30 s and a session that
// ends when nothing (not even a pong) arrives for 75 s.
var dockerManagerTerminalServeOptions = containerTerminalServeOptions{
	ReadLimit:    1 << 20,
	PingInterval: 30 * time.Second,
	PongWait:     75 * time.Second,
}

// serveDockerManagerTerminal serves an upgraded WebSocket with the session until
// either side ends it.
func serveDockerManagerTerminal(ctx context.Context, conn *websocket.Conn, session containerTerminalSession) {
	serveContainerTerminalSessionWith(ctx, conn, session, dockerManagerTerminalServeOptions)
}

// dockerManagerTerminalEnv is the environment of a terminal exec: the terminal
// type and the tag End finds the shell by (the same as containerTerminalExecPayload).
func dockerManagerTerminalEnv(nonce string) []string {
	return []string{"TERM=xterm-256color", containerTerminalSessionEnv + "=" + nonce}
}

// dockerManagerTerminalSession is one interactive exec in a container of a
// registered host.
type dockerManagerTerminalSession struct {
	c      *dockerutil.Client
	ref    string // the container
	execID string
	nonce  string // the AURAGO_TERMINAL_SESSION value of this exec
	stream io.ReadWriteCloser
}

var (
	_ containerTerminalSession        = (*dockerManagerTerminalSession)(nil)
	_ containerTerminalWriteDeadliner = (*dockerManagerTerminalSession)(nil)
	_ containerTerminalEnder          = (*dockerManagerTerminalSession)(nil)
)

// newDockerManagerTerminalSession creates the shell exec in the container ref
// (a running one; the caller has checked that and the container's protection),
// attaches to it and sizes the terminal. Engine refusals come back as a
// *dockerhost.StatusError (404 gone, 409 not running).
func newDockerManagerTerminalSession(ctx context.Context, c *dockerutil.Client, ref string, cols, rows int) (containerTerminalSession, error) {
	nonce, err := newContainerTerminalNonce()
	if err != nil {
		return nil, fmt.Errorf("create terminal session tag: %w", err)
	}
	execID, err := dockerhost.ExecCreate(ctx, c, ref, dockerManagerTerminalCommand, dockerManagerTerminalEnv(nonce), true)
	if err != nil {
		return nil, err
	}
	stream, err := dockerhost.ExecStart(ctx, c, execID)
	if err != nil {
		return nil, err
	}
	session := &dockerManagerTerminalSession{c: c, ref: ref, execID: execID, nonce: nonce, stream: stream}
	_ = session.Resize(ctx, cols, rows) // the first resize may race the shell's start; the browser sends another
	return session, nil
}

func (s *dockerManagerTerminalSession) Read(p []byte) (int, error)  { return s.stream.Read(p) }
func (s *dockerManagerTerminalSession) Write(p []byte) (int, error) { return s.stream.Write(p) }
func (s *dockerManagerTerminalSession) Close() error                { return s.stream.Close() }

// SetWriteDeadline bounds the next writes to the exec stream.
func (s *dockerManagerTerminalSession) SetWriteDeadline(t time.Time) error {
	if deadliner, ok := s.stream.(containerTerminalWriteDeadliner); ok {
		return deadliner.SetWriteDeadline(t)
	}
	return nil
}

// Resize sets the terminal size; sizes are clamped like the Containers page's.
func (s *dockerManagerTerminalSession) Resize(ctx context.Context, cols, rows int) error {
	return dockerhost.ExecResize(ctx, s.c, s.execID,
		clampTerminalDimension(cols, 120, 2, 500), clampTerminalDimension(rows, 30, 1, 200))
}

// End sends SIGHUP to this session's shell through a detached helper exec
// (containerTerminalEndScript, the tag passed as an argument); it types nothing
// into the terminal. The stream ends when the shell exits.
func (s *dockerManagerTerminalSession) End(ctx context.Context) error {
	if s.ref == "" || s.nonce == "" {
		return errors.New("terminal session has no end tag")
	}
	id, err := dockerhost.ExecCreate(ctx, s.c, s.ref,
		[]string{"/bin/sh", "-c", containerTerminalEndScript, "aurago-end-session", s.nonce}, nil, false)
	if err != nil {
		return err
	}
	return dockerhost.ExecStartDetached(ctx, s.c, id)
}
