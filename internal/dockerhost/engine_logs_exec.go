package dockerhost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/dockerutil"
)

// MaxLogLineBytes is the longest log line text FollowLogs delivers. A longer
// line is cut (on a character boundary) and flagged Truncated; the rest of that
// line is dropped without being buffered.
const MaxLogLineBytes = 16 << 10

// logTimestampMax bounds the timestamp prefix the Engine puts in front of a line
// (RFC 3339 with nanoseconds, "2026-10-28T07:33:29.123456789Z" is 30 bytes).
const logTimestampMax = 48

// LogOptions selects what FollowLogs reads.
type LogOptions struct {
	// Tail is the number of history lines (0..5000, anything else is clamped)
	// taken from the end of the log; 0 means no history. The Engine applies Since
	// to the lines Tail selects, and a Tail of 0 follows from the end of the log,
	// so Since brings lines older than "now" only together with a Tail.
	Tail int
	// Since limits the lines to the ones at or after this time; zero = not set.
	Since time.Time
	// Follow keeps the stream open for new lines until the container stops.
	Follow bool
	// Opened, when set, is called once when the Engine has accepted the request
	// and before the first line is delivered. An error that comes before it (an
	// unknown container, a refusal) is never followed by a call, so a caller
	// that streams the lines on can still answer such an error as a plain
	// response. Because of this field a LogOptions cannot be compared with ==.
	Opened func()
}

// LogLine is one line of a container log.
type LogLine struct {
	Stream    string    // "out" | "err" ("out" for TTY containers)
	Time      time.Time // from the Engine timestamp (timestamps=1 is always sent)
	Text      string    // without the timestamp and the trailing newline, at most MaxLogLineBytes
	Truncated bool      // the line was longer than MaxLogLineBytes
}

// logSink carries the callback of one FollowLogs call and the first error it
// returned, so the caller can tell a callback error from a read error.
type logSink struct {
	fn  func(LogLine) error
	err error
	now func() time.Time
}

// logLineSplitter is the io.Writer behind one stream (stdout or stderr) of a
// log. It cuts the bytes into lines, strips the Engine's timestamp and keeps at
// most one capped line in memory.
type logLineSplitter struct {
	stream string
	sink   *logSink
	buf    []byte
	over   bool // the current line exceeded the buffer; its remaining bytes are dropped
}

const logLineBufMax = logTimestampMax + 1 + MaxLogLineBytes

func (s *logLineSplitter) Write(p []byte) (int, error) {
	total := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			s.keep(p)
			return total, nil
		}
		s.keep(p[:i])
		p = p[i+1:]
		if err := s.emit(); err != nil {
			return total - len(p), err
		}
	}
	return total, nil
}

func (s *logLineSplitter) keep(chunk []byte) {
	room := logLineBufMax - len(s.buf)
	if room <= 0 {
		if len(chunk) > 0 {
			s.over = true
		}
		return
	}
	if len(chunk) > room {
		s.buf = append(s.buf, chunk[:room]...)
		s.over = true
		return
	}
	s.buf = append(s.buf, chunk...)
}

// finish emits a last line that has no trailing newline.
func (s *logLineSplitter) finish() error {
	if len(s.buf) == 0 && !s.over {
		return nil
	}
	return s.emit()
}

func (s *logLineSplitter) emit() error {
	line := s.buf
	truncated := s.over
	s.buf = s.buf[:0]
	s.over = false
	if s.sink.err != nil {
		return s.sink.err
	}
	line = bytes.TrimSuffix(line, []byte{'\r'})
	at, text := splitLogTimestamp(line, s.sink.now)
	if len(text) > MaxLogLineBytes {
		cut := MaxLogLineBytes
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
		text = text[:cut]
		truncated = true
	}
	out := LogLine{Stream: s.stream, Time: at, Text: strings.ToValidUTF8(string(text), string(utf8.RuneError)), Truncated: truncated}
	if err := s.sink.fn(out); err != nil {
		s.sink.err = err
		return err
	}
	return nil
}

// splitLogTimestamp separates the Engine's timestamp prefix from the text. A line
// without a parsable prefix keeps its whole text and gets the time of arrival.
func splitLogTimestamp(line []byte, now func() time.Time) (time.Time, []byte) {
	head, rest := line, []byte(nil)
	if sp := bytes.IndexByte(line, ' '); sp >= 0 {
		head, rest = line[:sp], line[sp+1:]
	}
	if len(head) >= len("2006-01-02T15:04:05Z") && len(head) <= logTimestampMax {
		if t, err := time.Parse(time.RFC3339Nano, string(head)); err == nil {
			return t.UTC(), rest
		}
	}
	return now().UTC(), line
}

// logQuery builds the query of GET /containers/{id}/logs.
func logQuery(opt LogOptions) string {
	q := url.Values{}
	q.Set("stdout", "1")
	q.Set("stderr", "1")
	q.Set("timestamps", "1")
	if opt.Follow {
		q.Set("follow", "1")
	}
	q.Set("tail", strconv.Itoa(min(max(opt.Tail, 0), logTailMax)))
	if !opt.Since.IsZero() {
		since := opt.Since.UTC()
		q.Set("since", fmt.Sprintf("%d.%09d", since.Unix(), since.Nanosecond()))
	}
	return q.Encode()
}

// FollowLogs streams a container's log lines to fn until ctx ends, the log ends
// (the container stopped, or the log was not followed) or fn fails. It reads
// Config.Tty once and demultiplexes non-TTY streams; the Engine's timestamps
// are always requested and removed from the text. It returns nil when the log
// ended, ctx.Err() when ctx ended, the error of fn when fn failed and a
// *StatusError (404 gone, 409 and others) when the Engine refused the request.
// The request uses the streaming client of c, which has no total timeout.
func FollowLogs(ctx context.Context, c *dockerutil.Client, ref string, opt LogOptions, fn func(LogLine) error) error {
	if err := validateContainerRef(ref); err != nil {
		return err
	}
	var inspect struct {
		Config struct {
			Tty bool `json:"Tty"`
		} `json:"Config"`
	}
	esc := url.PathEscape(ref)
	if err := engineJSON(ctx, c, http.MethodGet, "containers/"+esc+"/json", &inspect); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dockerutil.Endpoint("containers/"+esc+"/logs?"+logQuery(opt)), nil)
	if err != nil {
		return fmt.Errorf("create Docker request: %w", err)
	}
	resp, err := c.StreamingHTTPClient().Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("Docker request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message := dockerutil.SanitizeOneLine(dockerutil.EngineErrorMessage(dockerutil.ReadErrorBody(resp.Body)), 512)
		return &StatusError{Status: resp.StatusCode, Err: fmt.Errorf("Docker API returned %d: %s", resp.StatusCode, message)}
	}

	if opt.Opened != nil {
		opt.Opened()
	}
	sink := &logSink{fn: fn, now: time.Now}
	stdout := &logLineSplitter{stream: "out", sink: sink}
	stderr := &logLineSplitter{stream: "err", sink: sink}
	var readErr error
	if inspect.Config.Tty {
		_, readErr = io.Copy(stdout, resp.Body)
	} else {
		readErr = DemuxStreams(resp.Body, stdout, stderr)
	}
	switch {
	case sink.err != nil:
		return sink.err
	case ctx.Err() != nil:
		return ctx.Err()
	case readErr != nil:
		return fmt.Errorf("read container logs: %w", readErr)
	}
	if err := stdout.finish(); err != nil {
		return err
	}
	return stderr.finish()
}

// execStartBody is the body of POST /exec/{id}/start.
type execStartBody struct {
	Detach bool `json:"Detach"`
	Tty    bool `json:"Tty"`
}

type execCreateBody struct {
	AttachStdin  bool     `json:"AttachStdin"`
	AttachStdout bool     `json:"AttachStdout"`
	AttachStderr bool     `json:"AttachStderr"`
	Tty          bool     `json:"Tty"`
	Cmd          []string `json:"Cmd"`
	Env          []string `json:"Env,omitempty"`
}

// ErrInvalidExecID is returned for an exec id that is unsafe in an API path.
var ErrInvalidExecID = errors.New("invalid exec id")

func validateExecID(id string) error {
	if validateContainerRef(id) != nil {
		return ErrInvalidExecID
	}
	return nil
}

// execJSON is Client.DoJSON with a request body that wraps a non-2xx answer in
// a *StatusError.
func execJSON(ctx context.Context, c *dockerutil.Client, method, path string, body, out any) error {
	status, err := c.DoJSON(ctx, method, path, body, out)
	if err != nil && status != 0 && (status < 200 || status >= 300) {
		return &StatusError{Status: status, Err: err}
	}
	return err
}

// ExecCreate creates an exec in a running container and returns its id. With
// tty it is an interactive session: a pseudo terminal with stdin, stdout and
// stderr attached. Without tty nothing is attached (a helper to be started
// with ExecStartDetached). Env entries are "NAME=value".
func ExecCreate(ctx context.Context, c *dockerutil.Client, ref string, cmd, env []string, tty bool) (string, error) {
	if err := validateContainerRef(ref); err != nil {
		return "", err
	}
	if len(cmd) == 0 {
		return "", errors.New("exec command is empty")
	}
	body := execCreateBody{AttachStdin: tty, AttachStdout: tty, AttachStderr: tty, Tty: tty, Cmd: cmd, Env: env}
	var created struct {
		ID string `json:"Id"`
	}
	if err := execJSON(ctx, c, http.MethodPost, "containers/"+url.PathEscape(ref)+"/exec", body, &created); err != nil {
		return "", err
	}
	if created.ID == "" {
		return "", errors.New("Docker exec create returned no id")
	}
	return created.ID, nil
}

// ExecStart hijacks POST /exec/{id}/start ({"Detach":false,"Tty":true}). The
// connection is dialled with c.Dial and the request is written with the API
// version c.APIVersion negotiated, so TLS, mutual TLS and SSH hosts work like
// the local socket. The result is the raw terminal stream; Read is buffered (it
// returns bytes the Engine sent right behind the response header) and Close
// closes the connection. ctx bounds the handshake only: the stream outlives it.
// An Engine refusal is a *StatusError (404 unknown exec, 409 container not
// running or paused).
func ExecStart(ctx context.Context, c *dockerutil.Client, execID string) (io.ReadWriteCloser, error) {
	if err := validateExecID(execID); err != nil {
		return nil, err
	}
	version, err := c.APIVersion(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := c.Dial(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to Docker engine: %w", err)
	}
	// A peer that never answers must not hold the handshake past ctx.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	stream, err := execUpgrade(ctx, conn, version, execID)
	if err != nil {
		stop()
		_ = conn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	if !stop() { // ctx ended while the handshake finished: the connection is closed
		_ = conn.Close()
		return nil, ctx.Err()
	}
	return stream, nil
}

func execUpgrade(ctx context.Context, conn net.Conn, version, execID string) (*execStream, error) {
	body, _ := json.Marshal(execStartBody{Detach: false, Tty: true})
	target := "http://docker/v" + strings.TrimPrefix(version, "v") + "/exec/" + url.PathEscape(execID) + "/start"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build docker exec start request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")
	if err := req.Write(conn); err != nil {
		return nil, fmt.Errorf("send docker exec start request: %w", err)
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		return nil, fmt.Errorf("read docker exec start response: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusSwitchingProtocols {
		defer resp.Body.Close()
		message := dockerutil.SanitizeOneLine(dockerutil.EngineErrorMessage(dockerutil.ReadErrorBody(resp.Body)), 512)
		return nil, &StatusError{Status: resp.StatusCode, Err: fmt.Errorf("Docker API returned %d: %s", resp.StatusCode, message)}
	}
	return &execStream{Conn: conn, reader: reader}, nil
}

// execStream is a hijacked exec connection. Reads go through the buffer that
// parsed the response header; everything else (Write, Close, the deadlines)
// is the connection's.
type execStream struct {
	net.Conn
	reader *bufio.Reader
}

func (s *execStream) Read(p []byte) (int, error) { return s.reader.Read(p) }

// ExecResize sets the size of an exec's terminal.
func ExecResize(ctx context.Context, c *dockerutil.Client, execID string, cols, rows int) error {
	if err := validateExecID(execID); err != nil {
		return err
	}
	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("invalid terminal size %dx%d", cols, rows)
	}
	return engineJSON(ctx, c, http.MethodPost, fmt.Sprintf("exec/%s/resize?h=%d&w=%d", url.PathEscape(execID), rows, cols), nil)
}

// ExecStartDetached starts an exec in the background ({"Detach":true,"Tty":false});
// the Engine answers when the process has started.
func ExecStartDetached(ctx context.Context, c *dockerutil.Client, execID string) error {
	if err := validateExecID(execID); err != nil {
		return err
	}
	return execJSON(ctx, c, http.MethodPost, "exec/"+url.PathEscape(execID)+"/start", execStartBody{Detach: true, Tty: false}, nil)
}
