package chibisensei

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

const (
	logLinePrefix   = LogPrefix + " "
	maxLogLineBytes = 4 << 10  // a whole "@@chibi {...}" line
	maxLogBodyBytes = 16 << 20 // only the tail beyond this is read
	maxLogEvents    = 20000
	maxLogMessage   = 2000 // runes kept of a message
	maxMetricStep   = 10_000_000
	// ExitSeq is the seq the parser gives the notebook wrapper's exit line, which carries none.
	ExitSeq int64 = 1<<53 - 1
)

// TrainerStages are the stage names of the training contract in run order.
// data runs before install so a bad dataset fails before the pip installs; converter probes the
// GGUF converter (and writes the base GGUF) right after the base download, long before training.
var TrainerStages = []string{"boot", "env_check", "data", "install", "load_base", "converter", "base_eval", "train",
	"tuned_eval", "save", "gguf", "manifest"}

var (
	logEventTypes = map[string]bool{"stage": true, "metric": true, "eval": true, "warn": true, "error": true,
		"done": true, "exit": true, "info": true}
	codePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

func isTrainerStage(s string) bool { return containsString(TrainerStages, s) }

// LogEvent is one structured trainer line.
type LogEvent struct {
	Seq int64           `json:"seq"`
	TS  string          `json:"ts"`
	T   string          `json:"t"`   // stage | metric | eval | warn | error | done | exit | info
	Raw json.RawMessage `json:"raw"` // the whole object, compacted
}

type kaggleLogEntry struct {
	StreamName string `json:"stream_name"`
	Data       string `json:"data"`
}

// ParseKernelLog accepts a JSON array of {stream_name, time, data} entries (also cut at the front by a tail
// read) or plain text, extracts the "@@chibi " lines, drops malformed and over-long ones, dedupes by seq
// (first wins) and returns them in seq order. The work is linear in the body size.
func ParseKernelLog(body []byte) []LogEvent {
	if len(body) > maxLogBodyBytes {
		body = body[len(body)-maxLogBodyBytes:]
	}
	entries, plainToo := decodeLogEntries(body)
	var texts []string
	if len(entries) > 0 {
		texts = joinLogStreams(entries)
	}
	if plainToo {
		texts = append(texts, string(body))
	}
	seen := map[int64]bool{}
	out := []LogEvent{}
	for _, text := range texts {
		eachLogLine(text, func(line string) bool {
			if len(out) >= maxLogEvents {
				return false
			}
			ev, ok := parseLogLine(line)
			if !ok || seen[ev.Seq] {
				return true
			}
			seen[ev.Seq] = true
			out = append(out, ev)
			return true
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

// eachLogLine calls fn with every line of text without allocating per line; a line longer than a log line
// (plus room for a prefix before the marker) is skipped. fn returns false to stop.
func eachLogLine(text string, fn func(string) bool) {
	for len(text) > 0 {
		line := text
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			line, text = text[:i], text[i+1:]
		} else {
			text = ""
		}
		if len(line) > maxLogLineBytes+maxLogLinePrefix {
			continue
		}
		if !fn(line) {
			return
		}
	}
}

// joinLogStreams joins the data of each stream in first-seen order.
func joinLogStreams(entries []kaggleLogEntry) []string {
	var order []string
	joined := map[string]*strings.Builder{}
	for _, e := range entries {
		b := joined[e.StreamName]
		if b == nil {
			b = &strings.Builder{}
			joined[e.StreamName] = b
			order = append(order, e.StreamName)
		}
		b.WriteString(e.Data)
	}
	out := make([]string, 0, len(order))
	for _, name := range order {
		out = append(out, joined[name].String())
	}
	return out
}

// decodeLogEntries extracts the Kaggle log entries of a body: a complete JSON array is decoded whole; a body
// that starts like an array or like the middle of one is scanned for entries; any other body (plain text) is
// scanned only when it mentions a stream key. plainToo reports that the body's own lines must be parsed as
// well: it is false only when the body is clearly a JSON log, so no "@@chibi" line of a plain log is lost.
func decodeLogEntries(body []byte) (entries []kaggleLogEntry, plainToo bool) {
	t := bytes.TrimSpace(body)
	if len(t) == 0 {
		return nil, true
	}
	arrayish := t[0] == '[' || t[0] == '{' || t[0] == ',' || t[0] == '}' || t[0] == ']' || t[0] == '"'
	if t[0] == '[' {
		var all []kaggleLogEntry
		if json.Unmarshal(t, &all) == nil && len(all) > 0 {
			return all, false
		}
	}
	entries = scanLogEntries(body)
	return entries, len(entries) == 0 || !arrayish
}

const (
	maxLogLinePrefix = 256     // room before the marker on a line
	maxLogEntryBytes = 1 << 20 // decode window of one embedded entry
	maxScanEntries   = 200000
)

var streamKey = []byte(`"stream_name"`)

// scanLogEntries decodes the entries of a JSON log whose head is missing, one object at a time. Each decode
// is bounded to the span that ends at the next stream key (or a fixed window), so the total work is linear.
func scanLogEntries(body []byte) []kaggleLogEntry {
	var out []kaggleLogEntry
	off := 0
	for off < len(body) && len(out) < maxScanEntries {
		i := bytes.Index(body[off:], streamKey)
		if i < 0 {
			break
		}
		at := off + i
		after := at + len(streamKey)
		end := len(body)
		if n := bytes.Index(body[after:], streamKey); n >= 0 {
			end = after + n
		}
		if open := bytes.LastIndexByte(body[off:at], '{'); open >= 0 {
			start := off + open
			if end-start > maxLogEntryBytes {
				end = start + maxLogEntryBytes
			}
			var e kaggleLogEntry
			if err := json.NewDecoder(bytes.NewReader(body[start:end])).Decode(&e); err == nil {
				out = append(out, e)
			}
		}
		off = after
	}
	return out
}

func parseLogLine(line string) (LogEvent, bool) {
	line = strings.TrimRight(line, "\r")
	i := strings.Index(line, logLinePrefix)
	if i < 0 {
		return LogEvent{}, false
	}
	payload := strings.TrimSpace(line[i+len(logLinePrefix):])
	if len(logLinePrefix)+len(payload) > maxLogLineBytes || !strings.HasPrefix(payload, "{") {
		return LogEvent{}, false
	}
	var head struct {
		V   int    `json:"v"`
		Seq *int64 `json:"seq"`
		TS  string `json:"ts"`
		T   string `json:"t"`
	}
	if err := json.Unmarshal([]byte(payload), &head); err != nil || head.V != 1 || !logEventTypes[head.T] {
		return LogEvent{}, false
	}
	seq := ExitSeq
	switch {
	case head.Seq != nil && *head.Seq >= 0 && *head.Seq < ExitSeq:
		seq = *head.Seq
	case head.Seq == nil && head.T == "exit":
	default:
		return LogEvent{}, false
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(payload)); err != nil {
		return LogEvent{}, false
	}
	return LogEvent{Seq: seq, TS: cutRunes(head.TS, 40), T: head.T, Raw: compact.Bytes()}, true
}

// Metric is one training or validation point.
type Metric struct {
	Step     int     `json:"step"`
	MaxSteps int     `json:"max_steps,omitempty"`
	Kind     string  `json:"kind"` // train | eval
	Epoch    float64 `json:"epoch"`
	Loss     float64 `json:"loss"`
	LR       float64 `json:"lr"`
	GradNorm float64 `json:"grad_norm"`
	TokS     float64 `json:"tok_s"`
	ElapsedS float64 `json:"elapsed_s,omitempty"`
	EtaS     float64 `json:"eta_s,omitempty"`
	VRAMGB   float64 `json:"vram_gb,omitempty"`
	TS       string  `json:"ts"`
}

type metricWire struct {
	T        string   `json:"t"`
	TS       string   `json:"ts"`
	Step     *int     `json:"step"`
	MaxSteps int      `json:"max_steps"`
	Kind     string   `json:"kind"`
	Epoch    float64  `json:"epoch"`
	Loss     *float64 `json:"loss"`
	EvalLoss *float64 `json:"eval_loss"`
	LR       float64  `json:"lr"`
	GradNorm float64  `json:"grad_norm"`
	TokS     float64  `json:"tok_s"`
	ElapsedS float64  `json:"elapsed_s"`
	EtaS     float64  `json:"eta_s"`
	VRAMGB   float64  `json:"vram_gb"`
}

// parseMetric decodes a metric object (a log payload or a chibi-metrics.jsonl line).
func parseMetric(raw []byte) (Metric, bool) {
	var w metricWire
	if json.Unmarshal(raw, &w) != nil || (w.T != "" && w.T != "metric") || w.Step == nil ||
		*w.Step < 0 || *w.Step > maxMetricStep || w.MaxSteps < 0 || w.MaxSteps > maxMetricStep {
		return Metric{}, false
	}
	kind := w.Kind
	switch kind {
	case "train", "eval":
	case "":
		kind = "train"
		if w.Loss == nil && w.EvalLoss != nil {
			kind = "eval"
		}
	default:
		return Metric{}, false
	}
	loss := w.Loss
	if loss == nil && kind == "eval" {
		loss = w.EvalLoss
	}
	if loss == nil {
		return Metric{}, false
	}
	return Metric{Step: *w.Step, MaxSteps: w.MaxSteps, Kind: kind, Epoch: w.Epoch, Loss: *loss, LR: w.LR,
		GradNorm: w.GradNorm, TokS: w.TokS, ElapsedS: w.ElapsedS, EtaS: w.EtaS, VRAMGB: w.VRAMGB,
		TS: cutRunes(w.TS, 40)}, true
}

// Metric decodes a metric event.
func (e LogEvent) Metric() (Metric, bool) {
	if e.T != "metric" {
		return Metric{}, false
	}
	m, ok := parseMetric(e.Raw)
	if ok && m.TS == "" {
		m.TS = e.TS
	}
	return m, ok
}

type StageEvent struct{ Stage, State string } // state: start | done | failed

// Stage decodes a stage event; the stage must be one of TrainerStages.
func (e LogEvent) Stage() (StageEvent, bool) {
	if e.T != "stage" {
		return StageEvent{}, false
	}
	var w struct {
		Stage string `json:"stage"`
		State string `json:"state"`
	}
	if json.Unmarshal(e.Raw, &w) != nil || !isTrainerStage(w.Stage) ||
		(w.State != "start" && w.State != "done" && w.State != "failed") {
		return StageEvent{}, false
	}
	return StageEvent{Stage: w.Stage, State: w.State}, true
}

type ErrorEvent struct{ Stage, Code, Message string }

// Error decodes an error event. The message is untrusted: callers scrub it before storing.
func (e LogEvent) Error() (ErrorEvent, bool) {
	if e.T != "error" {
		return ErrorEvent{}, false
	}
	var w struct {
		Stage   string `json:"stage"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(e.Raw, &w) != nil || !codePattern.MatchString(w.Code) || (w.Stage != "" && !isTrainerStage(w.Stage)) {
		return ErrorEvent{}, false
	}
	return ErrorEvent{Stage: w.Stage, Code: w.Code, Message: cutRunes(w.Message, maxLogMessage)}, true
}

// ExitCode decodes the wrapper's exit event.
func (e LogEvent) ExitCode() (int, bool) {
	if e.T != "exit" {
		return 0, false
	}
	var w struct {
		Code *int `json:"code"`
	}
	if json.Unmarshal(e.Raw, &w) != nil || w.Code == nil {
		return 0, false
	}
	return *w.Code, true
}
