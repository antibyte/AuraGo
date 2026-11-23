package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// codeStudioExecOutputLimit caps the output one /api/code-studio/exec call returns to the page.
const codeStudioExecOutputLimit = 4 << 20

// codeStudioStdinExecer runs a command with a payload on its standard input.
// The Docker adapter implements it; handleExec asks for it only when a
// request carries stdin.
type codeStudioStdinExecer interface {
	ExecWithStdin(ctx context.Context, containerID string, cmd []string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error)
}

var _ codeStudioStdinExecer = codeStudioDockerAdapter{}

// capCodeStudioExecOutput cuts output to at most limit bytes without
// splitting a UTF-8 sequence and reports whether it cut anything.
func capCodeStudioExecOutput(output string, limit int) (string, bool) {
	if len(output) <= limit {
		return output, false
	}
	cut := limit
	for cut > 0 && output[cut]&0xC0 == 0x80 {
		cut--
	}
	return output[:cut], true
}

// ExecWithStdin runs cmd like Exec, with stdin attached: the start call is
// upgraded to a raw stream, stdin is written, the write side is closed so the
// process reads EOF, and stdout and stderr are read until the process ends.
// At most codeStudioExecOutputLimit+1 bytes are kept; the rest is drained so
// the process can finish and its exit code stays real.
func (a codeStudioDockerAdapter) ExecWithStdin(ctx context.Context, containerID string, cmd []string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error) {
	if timeout <= 0 || timeout > codeStudioMaxExecTime {
		timeout = codeStudioMaxExecTime
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body, err := json.Marshal(map[string]interface{}{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          cmd,
		"Tty":          false,
	})
	if err != nil {
		return codeStudioExecResult{}, fmt.Errorf("encode docker exec: %w", err)
	}
	data, code, err := tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/exec", string(body))
	if err != nil {
		return codeStudioExecResult{}, err
	}
	if code != http.StatusCreated {
		return codeStudioExecResult{}, fmt.Errorf("docker exec create failed: %s", strings.TrimSpace(string(data)))
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil {
		return codeStudioExecResult{}, fmt.Errorf("parse docker exec id: %w", err)
	}
	if created.ID == "" {
		return codeStudioExecResult{}, errors.New("docker exec create returned no id")
	}
	stream, err := openCodeStudioExecStdinStream(ctx, a.cfg, created.ID)
	if err != nil {
		return codeStudioExecResult{}, err
	}
	defer stream.Close()
	stop := context.AfterFunc(ctx, func() { _ = stream.Close() })
	defer stop()
	go func() {
		_, _ = stream.Write(stdin)
		if closer, ok := stream.Conn.(interface{ CloseWrite() error }); ok {
			_ = closer.CloseWrite()
		}
	}()
	output, readErr := readCodeStudioExecFrames(stream, codeStudioExecOutputLimit+1)
	if readErr != nil && ctx.Err() != nil {
		return codeStudioExecResult{}, fmt.Errorf("code studio exec: %w", ctx.Err())
	}
	return codeStudioExecResult{ExitCode: a.execExitCode(created.ID), Output: output}, nil
}

// openCodeStudioExecStdinStream starts an exec with stdin attached and no
// TTY, so the output arrives multiplexed. It refuses connections that cannot
// half-close, because the process would never see the end of its input.
func openCodeStudioExecStdinStream(ctx context.Context, cfg tools.DockerConfig, execID string) (*bufferedDockerRawConn, error) {
	version, err := dockerutil.NegotiatedAPIVersion(ctx, cfg.Host)
	if err != nil {
		return nil, err
	}
	conn, err := dockerutil.DialContext(ctx, cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("connect to Docker engine: %w", err)
	}
	if _, ok := conn.(interface{ CloseWrite() error }); !ok {
		_ = conn.Close()
		return nil, errors.New("docker connection cannot close stdin")
	}
	body := []byte(`{"Detach":false,"Tty":false}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/"+version+"/exec/"+url.PathEscape(execID)+"/start", bytes.NewReader(body))
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("build docker exec start request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")
	req.ContentLength = int64(len(body))
	if err := req.Write(conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send docker exec start request: %w", err)
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read docker exec start response: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusSwitchingProtocols {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("docker exec start failed: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return &bufferedDockerRawConn{Conn: conn, reader: reader}, nil
}

// readCodeStudioExecFrames demultiplexes a non-TTY Docker attach stream
// (8-byte header, then the payload). It keeps at most keep payload bytes and
// discards the rest until the stream ends.
func readCodeStudioExecFrames(r io.Reader, keep int) (string, error) {
	var out bytes.Buffer
	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(r, header); err != nil {
			if errors.Is(err, io.EOF) {
				return out.String(), nil
			}
			return out.String(), err
		}
		size := int64(binary.BigEndian.Uint32(header[4:8]))
		room := int64(keep - out.Len())
		if room < 0 {
			room = 0
		}
		if room > size {
			room = size
		}
		if room > 0 {
			if _, err := io.CopyN(&out, r, room); err != nil {
				return out.String(), err
			}
		}
		if rest := size - room; rest > 0 {
			if _, err := io.CopyN(io.Discard, r, rest); err != nil {
				return out.String(), err
			}
		}
	}
}

// execExitCode waits until Docker reports the exec as finished (at most about
// two seconds) and returns its exit code, or -1 when that cannot be read.
func (a codeStudioDockerAdapter) execExitCode(execID string) int {
	for attempt := 0; attempt < 40; attempt++ {
		data, code, err := tools.DockerRequestContext(context.Background(), a.cfg, http.MethodGet, "/exec/"+url.PathEscape(execID)+"/json", "")
		if err != nil || code != http.StatusOK {
			return -1
		}
		var inspect struct {
			Running  bool `json:"Running"`
			ExitCode int  `json:"ExitCode"`
		}
		if json.Unmarshal(data, &inspect) != nil {
			return -1
		}
		if !inspect.Running {
			return inspect.ExitCode
		}
		time.Sleep(50 * time.Millisecond)
	}
	return -1
}
