package server

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Code Studio LSP bridge. GET /api/code-studio/lsp?lang=go|typescript|python|cpp
// upgrades to a WebSocket and pipes it to one language server, a non-TTY exec
// in the Code Studio container. The page only names a language; the command
// comes from a fixed table. Every text frame carries one JSON-RPC message in
// both directions: the bridge strips the LSP Content-Length headers from the
// server's stdout and adds them on the way to its stdin.

const (
	// codeStudioLSPSessionEnv tags every process of one session: the server and
	// the children that inherit its environment, so the cleanup finds them all.
	codeStudioLSPSessionEnv = "AURAGO_LSP_SESSION"

	// codeStudioLSPMaxSessions bounds the language servers of the process.
	codeStudioLSPMaxSessions = 8

	// codeStudioLSPHeaderLimit bounds the header block of one server message.
	codeStudioLSPHeaderLimit = 8 << 10
	// codeStudioLSPMessageLimit bounds the body of one server message.
	codeStudioLSPMessageLimit = 32 << 20
	// codeStudioLSPReadBuffer is larger than the header limit, so a header
	// line that fits the limit always fits the read buffer.
	codeStudioLSPReadBuffer = codeStudioLSPHeaderLimit + 1024
	// codeStudioLSPFrameSlack is added to the max file size for page frames:
	// a didOpen carries a whole file plus its JSON envelope.
	codeStudioLSPFrameSlack = 64 << 10
	// codeStudioLSPStderrTail is how much stderr the debug log keeps.
	codeStudioLSPStderrTail = 2 << 10

	// codeStudioLSPExitWait is how long the cleanup waits for the server to
	// exit after its stdin closed.
	codeStudioLSPExitWait = 2 * time.Second
	// codeStudioLSPCheckTimeout bounds the `command -v` probe.
	codeStudioLSPCheckTimeout = 15 * time.Second
	// codeStudioLSPCloseWriteWait bounds the write of one close frame.
	codeStudioLSPCloseWriteWait = time.Second

	// Close codes of the browser contract (plan 2b). A server process that
	// exits closes with websocket.CloseInternalServerErr (1011).
	codeStudioLSPCloseNotInstalled = 4404
	codeStudioLSPCloseTooMany      = 4429
	codeStudioLSPCloseStartFailed  = 4500
)

// readCodeStudioLSPMessage reads one base-protocol message: header lines up
// to an empty line (at most codeStudioLSPHeaderLimit bytes in total), then
// exactly Content-Length body bytes (at most codeStudioLSPMessageLimit). It
// returns io.EOF only when the stream ends between two messages. r must
// buffer at least codeStudioLSPReadBuffer bytes.
func readCodeStudioLSPMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	headerBytes := 0
	for {
		line, err := r.ReadSlice('\n')
		headerBytes += len(line)
		if headerBytes > codeStudioLSPHeaderLimit || errors.Is(err, bufio.ErrBufferFull) {
			return nil, fmt.Errorf("language server header exceeds %d bytes", codeStudioLSPHeaderLimit)
		}
		if err != nil {
			if errors.Is(err, io.EOF) && headerBytes > 0 {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		text := strings.TrimRight(string(line), "\r\n")
		if text == "" {
			break
		}
		name, value, found := strings.Cut(text, ":")
		if !found {
			return nil, fmt.Errorf("malformed language server header %q", text)
		}
		if !strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid language server Content-Length %q", strings.TrimSpace(value))
		}
		if n > codeStudioLSPMessageLimit {
			return nil, fmt.Errorf("language server message exceeds %d bytes", codeStudioLSPMessageLimit)
		}
		length = n
	}
	if length < 0 {
		return nil, errors.New("language server message has no Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, codeStudioLSPUnexpectedEOF(err)
	}
	return body, nil
}

// writeCodeStudioLSPMessage writes body with its Content-Length header in one write.
func writeCodeStudioLSPMessage(w io.Writer, body []byte) error {
	frame := make([]byte, 0, len(body)+32)
	frame = append(frame, "Content-Length: "...)
	frame = strconv.AppendInt(frame, int64(len(body)), 10)
	frame = append(frame, "\r\n\r\n"...)
	frame = append(frame, body...)
	_, err := w.Write(frame)
	return err
}

// codeStudioLSPUnexpectedEOF reports an end of stream inside a message as
// io.ErrUnexpectedEOF.
func codeStudioLSPUnexpectedEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}

// codeStudioLSPDemux turns a non-TTY Docker attach stream (8-byte frame
// headers: stream type, three zero bytes, big-endian payload size) into the
// server's stdout. Payloads of any other stream go to stderr.
type codeStudioLSPDemux struct {
	src       io.Reader
	stderr    io.Writer
	header    [8]byte
	remaining uint32
}

func (d *codeStudioLSPDemux) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for d.remaining == 0 {
		if _, err := io.ReadFull(d.src, d.header[:]); err != nil {
			return 0, err
		}
		size := binary.BigEndian.Uint32(d.header[4:8])
		if d.header[0] == 1 {
			d.remaining = size
			continue
		}
		dst := d.stderr
		if dst == nil {
			dst = io.Discard
		}
		if _, err := io.CopyN(dst, d.src, int64(size)); err != nil {
			return 0, codeStudioLSPUnexpectedEOF(err)
		}
	}
	if uint32(len(p)) > d.remaining {
		p = p[:d.remaining]
	}
	n, err := d.src.Read(p)
	d.remaining -= uint32(n)
	if errors.Is(err, io.EOF) && d.remaining > 0 {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

// codeStudioLSPTail keeps the last codeStudioLSPStderrTail bytes written to
// it. The bridge discards stderr and logs only this tail at debug level.
type codeStudioLSPTail struct {
	mu  sync.Mutex
	buf []byte
}

func (t *codeStudioLSPTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - codeStudioLSPStderrTail; over > 0 {
		t.buf = append(t.buf[:0], t.buf[over:]...)
	}
	return len(p), nil
}

func (t *codeStudioLSPTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.ToValidUTF8(string(t.buf), "")
}
