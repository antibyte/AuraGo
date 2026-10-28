package dockerwatch

import (
	"path"
	"time"

	"aurago/internal/dockerhost"
)

// Log follow limits.
const (
	// MaxLogTail is the largest history a follower reads per container.
	MaxLogTail = 5000
	// MaxLogHistory is the most history lines a project follower sends before
	// it goes live: the newest ones of the merged per-container tails.
	MaxLogHistory = 5000

	logFlushInterval   = 100 * time.Millisecond // a batch leaves at the latest this long after its first line
	logFlushLines      = 200                    // ... or as soon as it has this many lines
	logHistoryBytes    = 4 << 20                // text bytes of history kept per container while merging
	logHistoryParallel = 4                      // history reads in flight at once
	logHistoryTimeout  = 30 * time.Second       // one history read of a project follower is given up after this long
	logSettle          = time.Second            // pause before a project re-checks its containers after a stream ended
)

// Codes of Notice.Code.
const (
	NoticeNotFollowed      = "not_followed"
	NoticeAttached         = "attached"
	NoticeDetached         = "detached"
	NoticeTruncatedHistory = "truncated_history"
)

// FollowOptions select the history a follower reads before it goes live.
// Tail is per container: 0 means no history, at most MaxLogTail. A Since without
// a Tail asks for every line since then (up to MaxLogTail). The server applies
// the defaults of the routes (200 for a container, 100 for a project).
type FollowOptions struct {
	Tail  int
	Since time.Time
	// Opened, when set, is called once when the stream is established: for a
	// container when the Engine has accepted the log request, for a project when
	// the project has been listed. It comes before the first out or notice call.
	// An error that comes earlier (unknown container, no free stream slot,
	// unreachable host) is returned without a call, so the route can still answer
	// it as a plain response and switch to an event stream only from here on.
	Opened func()
}

// tail is the Engine tail the options ask for.
func (o FollowOptions) tail() int {
	switch {
	case o.Tail > MaxLogTail:
		return MaxLogTail
	case o.Tail <= 0 && !o.Since.IsZero():
		return MaxLogTail
	case o.Tail < 0:
		return 0
	}
	return o.Tail
}

// LogEntry is one line of a followed log (SSE `lines` message).
type LogEntry struct {
	T     time.Time `json:"t"`
	S     string    `json:"s"` // out | err
	Text  string    `json:"text"`
	Trunc bool      `json:"trunc"`
	Svc   string    `json:"svc,omitempty"` // compose service; project streams only
	Ctr   string    `json:"ctr,omitempty"` // 12-character container id; project streams only
}

// LogBatch is one SSE `lines` message.
type LogBatch struct {
	Lines []LogEntry `json:"lines"`
}

// Notice is one SSE `notice` message: something about which containers of a
// project are followed.
type Notice struct {
	Code      string   `json:"code"` // not_followed | attached | detached | truncated_history
	Message   string   `json:"message"`
	Services  []string `json:"services,omitempty"`
	Container string   `json:"container,omitempty"`
}

// ProjectSelector chooses the containers of a compose project.
type ProjectSelector struct {
	Project    string // com.docker.compose.project
	WorkingDir string // when set: only containers whose working dir equals it (a managed stack) ...
	Exclude    bool   // ... or, with Exclude, only those whose working dir differs (an external project sharing a stack's name)
}

// Matches reports whether the container belongs to the selection. Working
// directories are compared as cleaned paths; a container without the label never
// equals one.
func (s ProjectSelector) Matches(c dockerhost.ContainerSummary) bool {
	if s.Project == "" || c.ComposeProject != s.Project {
		return false
	}
	if s.WorkingDir == "" {
		return true
	}
	same := c.ComposeWorkingDir != "" && path.Clean(c.ComposeWorkingDir) == path.Clean(s.WorkingDir)
	return same != s.Exclude
}

func logEntryOf(l dockerhost.LogLine) LogEntry {
	return LogEntry{T: l.Time, S: l.Stream, Text: l.Text, Trunc: l.Truncated}
}
