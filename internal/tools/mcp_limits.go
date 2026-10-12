package tools

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
)

const (
	mcpMessageLimit        = 32 << 20
	mcpDiscoveryPageLimit  = 16 << 20
	mcpDiscoveryTotalLimit = 64 << 20
	mcpStderrLimit         = 64 << 10
)

var errMCPMessageTooLarge = errors.New("MCP message exceeds size limit")
var errMCPRequestNotSent = errors.New("MCP request was not sent")

// A zero-value lock whose waiters can leave without leaking a goroutine.
type mcpContextMutex struct {
	once  sync.Once
	token chan struct{}
}

func (m *mcpContextMutex) Lock(ctx context.Context) error {
	m.once.Do(func() { m.token = make(chan struct{}, 1) })
	if err := ctx.Err(); err != nil {
		return errors.Join(errMCPRequestNotSent, err)
	}
	select {
	case m.token <- struct{}{}:
		if err := ctx.Err(); err != nil {
			m.Unlock()
			return errors.Join(errMCPRequestNotSent, err)
		}
		return nil
	case <-ctx.Done():
		return errors.Join(errMCPRequestNotSent, ctx.Err())
	}
}
func (m *mcpContextMutex) Unlock() { <-m.token }

func readMCPBounded(r io.Reader, limit int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if len(data) > limit {
		return nil, errMCPMessageTooLarge
	}
	return data, err
}

func readMCPLine(r *bufio.Reader, limit int) ([]byte, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(line)+len(part) > limit {
			return nil, errMCPMessageTooLarge
		}
		line = append(line, part...)
		if err != bufio.ErrBufferFull {
			return line, err
		}
	}
}

func readMCPSSEEvent(scanner *bufio.Scanner) (string, []byte, error) {
	var data strings.Builder
	event, size := "", 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if size == 0 {
				continue
			}
			return event, []byte(data.String()), nil
		}
		size += len(line) + 1
		if size > mcpMessageLimit {
			return "", nil, errMCPMessageTooLarge
		}
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(line[6:])
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(line[5:], " "))
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return "", nil, errMCPMessageTooLarge
		}
		return "", nil, err
	}
	return "", nil, io.EOF
}
