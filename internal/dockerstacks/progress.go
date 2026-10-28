package dockerstacks

import (
	"bytes"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"

	"aurago/internal/dockerutil"
)

// maxLineBytes bounds one buffered line; a longer one is handed on in pieces.
const maxLineBytes = 64 << 10

// lineWriter splits what is written to it into lines and hands every complete
// line (without its line ending, empty lines skipped) to fn. Flush hands on a
// last line that had no line ending.
type lineWriter struct {
	buf []byte
	fn  func(line string)
}

func newLineWriter(fn func(line string)) *lineWriter { return &lineWriter{fn: fn} }

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.emit(string(w.buf[:i]))
		w.buf = w.buf[i+1:]
	}
	if len(w.buf) > maxLineBytes {
		w.emit(string(w.buf))
		w.buf = nil
	}
	return len(p), nil
}

// Flush hands on the unterminated rest.
func (w *lineWriter) Flush() {
	if len(w.buf) > 0 {
		w.emit(string(w.buf))
		w.buf = nil
	}
}

func (w *lineWriter) emit(line string) {
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) != "" {
		w.fn(line)
	}
}

// parseProgressLine reads one line of `docker compose --progress json`: a JSON
// object with the fields id, parent_id (also parent), status, text, current,
// total and percent. Fields are mapped by name, unknown ones are ignored, and a
// number may arrive as a JSON number or a numeric string. A line that is not a
// JSON object, or an object with none of id, status and text, is not progress.
func parseProgressLine(line string) (Progress, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "{") {
		return Progress{}, false
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return Progress{}, false
	}
	p := Progress{
		ID:      jsonStringField(raw, "id"),
		Parent:  jsonStringField(raw, "parent_id", "parent", "parentId"),
		Status:  jsonStringField(raw, "status"),
		Text:    jsonStringField(raw, "text"),
		Current: int64(jsonNumberField(raw, "current")),
		Total:   int64(jsonNumberField(raw, "total")),
		Percent: jsonNumberField(raw, "percent"),
	}
	if p.ID == "" && p.Status == "" && p.Text == "" {
		return Progress{}, false
	}
	if p.Percent == 0 && p.Total > 0 && p.Current > 0 {
		p.Percent = math.Round(float64(p.Current)*1000/float64(p.Total)) / 10
	}
	return p, true
}

func jsonStringField(raw map[string]any, names ...string) string {
	for _, name := range names {
		if v, ok := raw[name].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func jsonNumberField(raw map[string]any, name string) float64 {
	switch v := raw[name].(type) {
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	}
	return 0
}

// progressText renders a progress item as a readable output line: "<id>
// <status> <text>" with the empty parts left out.
func progressText(p Progress) string {
	parts := make([]string, 0, 3)
	for _, part := range []string{p.ID, p.Status, p.Text} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " ")
}

// isByteTick reports an item that carries a byte count of a download or an
// extraction. It updates the progress bar but is not worth a line of output;
// the items before and after it ("Pulling fs layer", "Pull complete") are.
func isByteTick(p Progress) bool { return p.Total > 0 }

var (
	reANSI       = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")
	reErrorWord  = regexp.MustCompile(`(?i)(\berror\b|\bfailed\b|\bdenied\b|\binvalid\b|\bcannot\b|\bunable\b|\brefused\b|not found|no such|required variable|\byaml:)`)
	reLineNumber = regexp.MustCompile(`(?i)\bline (\d+)\b`)
)

// stripANSI removes terminal escape sequences.
func stripANSI(s string) string { return reANSI.ReplaceAllString(s, "") }

// errorScan watches the lines of an operation for the one that explains a
// failure. Plain lines win over progress items, a plain line that names an
// error wins over the last plain line, and the last such line wins over earlier
// ones, because the compose CLI prints its final error last.
type errorScan struct {
	lastPlain   string
	lastErrWord string
	progress    string
}

// observe takes one line of output; p is its parsed progress item, if it is one.
func (e *errorScan) observe(line string, p *Progress) {
	line = strings.TrimSpace(stripANSI(line))
	if line == "" {
		return
	}
	if p != nil {
		if strings.EqualFold(p.Status, "error") && e.progress == "" {
			e.progress = progressErrorText(*p)
		}
		return
	}
	e.lastPlain = line
	if reErrorWord.MatchString(line) {
		e.lastErrWord = line
	}
}

// progressErrorText is the message of a compose error item: its text when it
// says more than "Error", else the readable line.
func progressErrorText(p Progress) string {
	if p.Text != "" && !strings.EqualFold(p.Text, "error") {
		if p.ID != "" {
			return p.ID + ": " + p.Text
		}
		return p.Text
	}
	return progressText(p)
}

// message is the one-line explanation of the failure, "" when nothing was seen.
func (e *errorScan) message() string {
	for _, candidate := range []string{e.lastErrWord, e.progress, e.lastPlain} {
		if candidate != "" {
			return dockerutil.SanitizeOneLine(candidate, 300)
		}
	}
	return ""
}

// errorLineOf reads the compose file line a compose error names ("yaml: line
// 12: ...", "line 12:"), 0 when it names none.
func errorLineOf(message string) int {
	m := reLineNumber.FindStringSubmatch(message)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 || n > 1_000_000 {
		return 0
	}
	return n
}
