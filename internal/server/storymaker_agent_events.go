package server

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/storymaker/render"

	"github.com/google/uuid"
)

// The event types of the agent stream.
const (
	agentEventPhase   = "phase"
	agentEventTool    = "tool"
	agentEventModel   = "model"
	agentEventUsage   = "usage"
	agentEventCheck   = "check"
	agentEventOutcome = "outcome"
)

const (
	// agentEventMaxLines caps one job file. The newest lines stay, so the outcome is never lost.
	agentEventMaxLines = 500
	// agentEventMaxFiles is the number of job files that a start keeps.
	agentEventMaxFiles = 200
)

// agentEventLine is one stored event: its number and its JSON, without the line end.
type agentEventLine struct {
	Seq  int
	Data []byte
}

// agentEventJob is what the log remembers about the file of one job.
type agentEventJob struct {
	seq   int
	lines int
}

type agentEventSub struct {
	project string
	ch      chan struct{}
}

// agentEventLog stores the progress events of agent jobs as <dir>/<job id>.jsonl. Callers pass closed
// values only: no tool arguments and no model text ever reach it. A failed write loses that event and
// never touches the job.
type agentEventLog struct {
	dir string
	now func() time.Time

	mu      sync.Mutex
	jobs    map[string]*agentEventJob
	subs    map[int]*agentEventSub
	nextSub int
}

// newAgentEventLog opens the log and removes all but the newest agentEventMaxFiles job files.
func newAgentEventLog(dir string) *agentEventLog {
	l := &agentEventLog{
		dir:  dir,
		now:  func() time.Time { return time.Now().UTC() },
		jobs: map[string]*agentEventJob{},
		subs: map[int]*agentEventSub{},
	}
	l.prune()
	return l
}

func (l *agentEventLog) prune() {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return
	}
	type storedFile struct {
		name string
		at   time.Time
	}
	files := make([]storedFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, storedFile{name: entry.Name(), at: info.ModTime()})
	}
	if len(files) <= agentEventMaxFiles {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		if !files[i].at.Equal(files[j].at) {
			return files[i].at.After(files[j].at)
		}
		return files[i].name > files[j].name
	})
	for _, file := range files[agentEventMaxFiles:] {
		_ = os.Remove(filepath.Join(l.dir, file.name))
	}
}

// path returns the file of a job. Only a canonical job id names a file.
func (l *agentEventLog) path(jobID string) (string, bool) {
	id, err := uuid.Parse(jobID)
	if err != nil || id.String() != jobID {
		return "", false
	}
	return filepath.Join(l.dir, jobID+".jsonl"), true
}

// emit stores one event as {job_id, seq, at, type, ...fields} and wakes the streams of the project.
func (l *agentEventLog) emit(projectID, jobID, eventType string, fields map[string]any) {
	path, ok := l.path(jobID)
	if !ok {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	state := l.jobs[jobID]
	if state == nil {
		state = &agentEventJob{}
		if stored := l.readLocked(path); len(stored) > 0 {
			state.seq, state.lines = stored[len(stored)-1].Seq, len(stored)
		}
		l.jobs[jobID] = state
	}
	state.seq++
	event := make(map[string]any, len(fields)+4)
	for key, value := range fields {
		event[key] = value
	}
	event["job_id"] = jobID
	event["seq"] = state.seq
	event["at"] = l.now().Format(time.RFC3339Nano)
	event["type"] = eventType
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	if err := l.writeLocked(path, state, data); err != nil {
		return
	}
	for _, sub := range l.subs {
		if sub.project != projectID {
			continue
		}
		select {
		case sub.ch <- struct{}{}:
		default:
		}
	}
}

// writeLocked appends one line. A full file is rewritten with its newest lines.
func (l *agentEventLog) writeLocked(path string, state *agentEventJob, data []byte) error {
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return err
	}
	if state.lines >= agentEventMaxLines {
		stored := l.readLocked(path)
		keep := stored[max(0, len(stored)-agentEventMaxLines+1):]
		var content bytes.Buffer
		for _, line := range keep {
			content.Write(line.Data)
			content.WriteByte('\n')
		}
		content.Write(data)
		content.WriteByte('\n')
		if err := fileutil.WriteFileContext(context.Background(), path, content.Bytes(), 0o600); err != nil {
			return err
		}
		state.lines = len(keep) + 1
		return nil
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(data, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	state.lines++
	return nil
}

// readLocked returns the stored lines in order. A line that a crash cut short is skipped.
func (l *agentEventLog) readLocked(path string) []agentEventLine {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []agentEventLine
	for _, raw := range bytes.Split(data, []byte{'\n'}) {
		if len(raw) == 0 {
			continue
		}
		var head struct {
			Seq int `json:"seq"`
		}
		if json.Unmarshal(raw, &head) != nil || head.Seq <= 0 {
			continue
		}
		out = append(out, agentEventLine{Seq: head.Seq, Data: raw})
	}
	return out
}

// after returns the stored events of a job with a number above seq, oldest first. The file cap bounds
// the answer to agentEventMaxLines.
func (l *agentEventLog) after(jobID string, seq int) []agentEventLine {
	path, ok := l.path(jobID)
	if !ok {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	stored := l.readLocked(path)
	for i, line := range stored {
		if line.Seq > seq {
			return stored[i:]
		}
	}
	return nil
}

// subscribe returns a channel that receives a signal after each event of the project and a function
// that ends the subscription. Signals coalesce; call after for the events themselves.
func (l *agentEventLog) subscribe(projectID string) (<-chan struct{}, func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := l.nextSub
	l.nextSub++
	sub := &agentEventSub{project: projectID, ch: make(chan struct{}, 1)}
	l.subs[id] = sub
	return sub.ch, func() {
		l.mu.Lock()
		delete(l.subs, id)
		l.mu.Unlock()
	}
}

// agentStreamJob is the job whose events a stream shows: the unfinished agent job of the project, or
// else its latest one. The queue coalesces per project, so at most one job is unfinished.
func (rt *storymakerRuntime) agentStreamJob(projectID string) (render.Job, bool) {
	jobs := rt.agent.List(projectID)
	if len(jobs) == 0 {
		return render.Job{}, false
	}
	for _, job := range jobs {
		if !job.Terminal() {
			return job, true
		}
	}
	return jobs[0], true
}
