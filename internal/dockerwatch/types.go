// Package dockerwatch is the Docker Manager's live data: one Engine events
// watcher per host (with reconnect, resync and a list-poll fallback), a stats
// sampler with rings for the containers the UI shows and for the host gauges,
// the disk-usage cache, the per-host stream budget and the hub that fans all of
// it out to the open /api/docker-manager/stream clients.
package dockerwatch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// Hosts is satisfied by *dockerhost.Registry.
type Hosts interface {
	Host(ctx context.Context, id string) (dockerhost.Host, error)
	Client(ctx context.Context, id string) (*dockerutil.Client, error)
}

// Limits.
const (
	MaxStreamsPerHost  = 16  // concurrent streaming Engine connections per host
	MaxClients         = 32  // open stream clients per process
	ClientBuffer       = 512 // messages a client may fall behind before it is dropped
	RingSize           = 60  // stats points kept per container
	MaxStatsIDs        = 50  // stats ids per host per client
	MaxHostSample      = 100 // containers sampled for the host gauges
	MaxSubscribedHosts = 20  // hosts per subscription list
)

// MaxProjectFollowsPerHost is how many project and stack log follows a host
// serves at once (each also holds a stream slot per followed container). A
// follow starts with a history read of every container of the project, which
// is the expensive part; one more is refused with ErrTooManyProjectFollows
// before that read begins.
const MaxProjectFollowsPerHost = 4

// Host states of a HostStatus.
const (
	StateConnecting  = "connecting"
	StateLive        = "live"
	StatePaused      = "paused"      // events fail repeatedly; the host is polled instead
	StateUnreachable = "unreachable" // the host does not answer; Reason says why
)

// ReasonHostNotFound is the Reason of an unreachable host that the registry
// no longer knows (it was deleted while a client was subscribed).
const ReasonHostNotFound = "host_not_found"

// Names of the SSE messages the hub produces (Message.Event).
const (
	MsgHostStatus   = "host_status"
	MsgEvent        = "event"
	MsgResync       = "resync"
	MsgStats        = "stats"
	MsgStatsHistory = "stats_history"
	MsgStatsGone    = "stats_gone"
	MsgHostStats    = "host_stats"
	MsgOperation    = "operation" // produced by the server's dockerstacks bridge through Publish
	MsgDropped      = "dropped"
)

// Errors, matchable with errors.Is. Errors of Set also match
// ErrInvalidSubscription.
var (
	ErrTooManyClients      = errors.New("too many open streams")
	ErrTooManyStreams      = errors.New("too many streaming connections to the host")
	ErrTooManyIDs          = errors.New("too many container ids in the subscription")
	ErrTooManyHosts        = errors.New("too many hosts in the subscription")
	ErrInvalidSubscription = errors.New("invalid subscription")
	ErrClientClosed        = errors.New("the stream is closed")
	ErrHubClosed           = errors.New("the live data hub is closed")
)

// ErrTooManyProjectFollows is an ErrTooManyStreams (errors.Is): the host
// already has MaxProjectFollowsPerHost project or stack log follows.
var ErrTooManyProjectFollows = fmt.Errorf("%w: too many project log follows", ErrTooManyStreams)

// Options configure a Hub. Zero values take the defaults.
type Options struct {
	Now            func() time.Time // default time.Now
	StatsInterval  time.Duration    // container stats, default 3 s
	HostInterval   time.Duration    // host gauges, default 10 s
	PollInterval   time.Duration    // list poll of a paused host, default 10 s
	RetryPaused    time.Duration    // events retry while paused, default 60 s
	IdleStop       time.Duration    // watcher stop delay after the last user, default 30 s
	ConnectTimeout time.Duration    // wait for the Engine's answer to /events, default 20 s
	Backoff        []time.Duration  // reconnect delays, default 1, 2, 4, 8, 16, 30 s
	Logger         *slog.Logger
}

func (o Options) withDefaults() Options {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.StatsInterval <= 0 {
		o.StatsInterval = 3 * time.Second
	}
	if o.HostInterval <= 0 {
		o.HostInterval = 10 * time.Second
	}
	if o.PollInterval <= 0 {
		o.PollInterval = 10 * time.Second
	}
	if o.RetryPaused <= 0 {
		o.RetryPaused = 60 * time.Second
	}
	if o.IdleStop <= 0 {
		o.IdleStop = 30 * time.Second
	}
	if o.ConnectTimeout <= 0 {
		o.ConnectTimeout = 20 * time.Second
	}
	if len(o.Backoff) == 0 {
		o.Backoff = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second}
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return o
}

// Subscription is what one stream client wants to receive. Set replaces it as a whole.
type Subscription struct {
	Hosts     []string            `json:"hosts"`      // events, host_status, resync, operation
	Stats     map[string][]string `json:"stats"`      // host -> container ids (full or 12-char)
	HostStats []string            `json:"host_stats"` // hosts for the rail gauges
}

// Accepted counts what a Set stored.
type Accepted struct {
	Hosts     int `json:"hosts"`
	Stats     int `json:"stats"`
	HostStats int `json:"host_stats"`
}

// Message is one item of a client's stream: the SSE event name and a value
// that the server encodes as the data line.
type Message struct {
	Event string
	Data  any
}

// Event is one Engine event after normalisation; it is also what WatchHost
// delivers (a resync arrives as Type "resync" with Action "reconnect" or "poll").
type Event struct {
	Host     string `json:"host"`
	Type     string `json:"type"`
	Action   string `json:"action"`
	ID       string `json:"id"`
	ShortID  string `json:"short_id"`
	Name     string `json:"name"`
	Project  string `json:"project"`
	Service  string `json:"service"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Health   string `json:"health,omitempty"`
	T        int64  `json:"t"` // unix ms
}

// HostStatus is the `host_status` message.
type HostStatus struct {
	Host   string `json:"host"`
	State  string `json:"state"`
	Reason string `json:"reason"`
	T      int64  `json:"t"`
}

// Resync is the `resync` message: the client should reload the host's lists.
type Resync struct {
	Host   string `json:"host"`
	Reason string `json:"reason"` // reconnect | poll
}

// StatsPoint is one sample of a container. CPU is -1 when unknown.
type StatsPoint struct {
	T        int64   `json:"t"` // unix ms
	CPU      float64 `json:"cpu"`
	Mem      uint64  `json:"mem"`
	MemLimit uint64  `json:"mem_limit"`
	NetRx    uint64  `json:"net_rx"`
	NetTx    uint64  `json:"net_tx"`
	BlkR     uint64  `json:"blk_r"`
	BlkW     uint64  `json:"blk_w"`
	Pids     uint64  `json:"pids"`
}

// StatsMsg is the `stats` message.
type StatsMsg struct {
	Host  string     `json:"host"`
	ID    string     `json:"id"` // 12 chars
	Point StatsPoint `json:"point"`
}

// StatsHistoryMsg is the `stats_history` message, oldest point first.
type StatsHistoryMsg struct {
	Host   string       `json:"host"`
	ID     string       `json:"id"`
	Points []StatsPoint `json:"points"`
}

// StatsGoneMsg is the `stats_gone` message: the container no longer exists or is not running.
type StatsGoneMsg struct {
	Host string `json:"host"`
	ID   string `json:"id"`
}

// HostStats is the `host_stats` message for the rail gauges. CPU is the share
// of the whole host in percent (-1 when no container's CPU is known yet).
type HostStats struct {
	Host     string  `json:"host"`
	T        int64   `json:"t"`
	CPU      float64 `json:"cpu"`
	NCPU     int     `json:"ncpu"`
	MemUsed  uint64  `json:"mem_used"`
	MemTotal uint64  `json:"mem_total"`
	Running  int     `json:"running"`
	Sampled  int     `json:"sampled"`
	Partial  bool    `json:"partial"`
}

// Dropped is the `dropped` message sent to a client that fell too far behind.
type Dropped struct {
	Reason string `json:"reason"` // slow_client
}
