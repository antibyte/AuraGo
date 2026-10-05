package dockerutil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// MaxJSONMessageLine is the longest accepted line of a Docker JSON-message
// stream (image pull, build, push). Engine progress and build lines are far
// shorter; the limit only bounds memory against a broken or hostile endpoint.
const MaxJSONMessageLine = 1 << 20

// MaxErrorBody is the number of bytes read from a non-2xx Docker response.
const MaxErrorBody = 64 << 10

// JSONMessageError is the first error event of a JSON-message stream. The
// Engine reports failures that happen after the HTTP 200 status line this way.
type JSONMessageError struct {
	Message string
}

func (e *JSONMessageError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// DrainJSONMessages reads a Docker JSON-message stream (pull/build/push) to
// EOF or the first error event. The first error event wins (errorDetail.message before error) and is
// returned as *JSONMessageError. Lines that are not JSON are skipped. A read
// error, a stream that ends inside a message, or a line longer than
// MaxJSONMessageLine is returned as an error. A clean EOF without an error
// event returns nil. Memory stays bounded by one line however long the stream
// runs; callers bound the duration with their context or client timeout.
func DrainJSONMessages(r io.Reader) error {
	if r == nil {
		return errors.New("Docker message stream is missing")
	}
	// The Engine ends every message with a newline. A final fragment without
	// one that is not valid JSON means the stream was cut inside a message.
	unterminated := false
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), MaxJSONMessageLine)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		advance, token, err := bufio.ScanLines(data, atEOF)
		unterminated = atEOF && len(data) > 0 && advance == len(data) && data[len(data)-1] != '\n'
		return advance, token, err
	})
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event struct {
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			if unterminated {
				// A failed read hands the scanner its last fragment as if the
				// stream had ended; keep the cause (reset, deadline, cancel).
				if readErr := scanner.Err(); readErr != nil {
					return fmt.Errorf("read Docker message stream: %w", readErr)
				}
				return fmt.Errorf("Docker message stream ended inside a message: %w", io.ErrUnexpectedEOF)
			}
			continue
		}
		if msg := strings.TrimSpace(event.ErrorDetail.Message); msg != "" {
			return &JSONMessageError{Message: msg}
		}
		if msg := strings.TrimSpace(event.Error); msg != "" {
			return &JSONMessageError{Message: msg}
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return fmt.Errorf("Docker message stream line exceeds %d bytes: %w", MaxJSONMessageLine, err)
		}
		return fmt.Errorf("read Docker message stream: %w", err)
	}
	return nil
}

// ReadErrorBody reads at most MaxErrorBody bytes of a non-2xx body. A read
// error ends the read; whatever arrived is returned for the error message.
func ReadErrorBody(r io.Reader) []byte {
	if r == nil {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(r, MaxErrorBody))
	return data
}
