package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerstacks"
	"aurago/internal/dockerwatch"
)

// ── Docker Manager live data: the seam to internal/dockerwatch ──────────────
// The live routes (docker_manager_stream.go, docker_manager_logs.go,
// docker_manager_terminal_api.go and the disk-usage route below) use the hub
// through the narrow interfaces here, so the handler tests can substitute a
// fake for the failure cases that a real Engine cannot be told to produce.

// dmDFTimeout bounds GET /hosts/{id}/df: /system/df is slow on hosts with many
// images and volumes.
const dmDFTimeout = 60 * time.Second

// dmLive is what the live routes use of *dockerwatch.Hub.
type dmLive interface {
	Attach() (dmLiveClient, error)
	Client(id string) (dmLiveClient, bool)
	FollowContainer(ctx context.Context, host, ref string, opt dockerwatch.FollowOptions, out func(dockerwatch.LogBatch) error, notice func(dockerwatch.Notice) error) error
	FollowProject(ctx context.Context, host string, sel dockerwatch.ProjectSelector, opt dockerwatch.FollowOptions, out func(dockerwatch.LogBatch) error, notice func(dockerwatch.Notice) error) error
	Acquire(host string) (release func(), err error)
	DiskUsage(ctx context.Context, host string, refresh bool) (usage dockerhost.DiskUsage, fetchedAt time.Time, cached bool, err error)
}

// dmLiveClient is one open /stream as the handlers see it (*dockerwatch.Client).
type dmLiveClient interface {
	ID() string
	Messages() <-chan dockerwatch.Message
	Set(sub dockerwatch.Subscription) (dockerwatch.Accepted, error)
	Close()
}

// dmWatchHub adapts the hub: Attach and Client return the interface, never a
// nil *dockerwatch.Client wrapped in a non-nil interface.
type dmWatchHub struct{ *dockerwatch.Hub }

func (h dmWatchHub) Attach() (dmLiveClient, error) {
	c, err := h.Hub.Attach()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (h dmWatchHub) Client(id string) (dmLiveClient, bool) {
	c, ok := h.Hub.Client(id)
	if !ok {
		return nil, false
	}
	return c, true
}

var (
	_ dmLive       = dmWatchHub{}
	_ dmLiveClient = (*dockerwatch.Client)(nil)
)

// liveService returns the live data hub, or answers 503. The hub exists as soon
// as the host registry does, so this only fires for a server that never opened
// either.
func (dm *dmRequest) liveService() (dmLive, bool) {
	if dm.live == nil {
		dmWriteError(dm.w, http.StatusServiceUnavailable, dmCodeUnavailable, "The Docker live data service is not available", nil)
		return nil, false
	}
	return dm.live, true
}

// dmLiveFailureOf classifies an error of a live call that touched a host: the
// stream budget (429), a host the registry forgot (404) and then everything
// engineFailureOf knows.
func dmLiveFailureOf(err error, notFoundCode string) dmFailure {
	switch {
	case errors.Is(err, dockerwatch.ErrTooManyStreams):
		return dmFailure{http.StatusTooManyRequests, dmCodeTooManyStreams, "Too many live connections to this Docker host are open; close a log or terminal panel and try again", nil}
	case errors.Is(err, dockerwatch.ErrTooManyClients):
		return dmFailure{http.StatusTooManyRequests, dmCodeTooManyStreams, "Too many live streams are open; close another Docker Manager window and try again", nil}
	case errors.Is(err, dockerwatch.ErrHubClosed):
		return dmFailure{http.StatusServiceUnavailable, dmCodeUnavailable, "The Docker live data service is shutting down", nil}
	case errors.Is(err, dockerhost.ErrHostNotFound):
		return dmFailure{http.StatusNotFound, dmCodeHostNotFound, "Docker host not found", nil}
	}
	return engineFailureOf(err, notFoundCode)
}

func (dm *dmRequest) liveFailure(err error, notFoundCode string) {
	dm.writeFailure(dmLiveFailureOf(err, notFoundCode))
}

// diskUsage serves GET /hosts/{id}/df[?refresh=1]: Docker's own disk use on the
// host (images, containers, volumes, build cache) from the hub's cache; refresh
// reads it again when the cache is older than a minute. docker.readonly allows it.
func (dm *dmRequest) diskUsage(hostID string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	live, ok := dm.liveService()
	if !ok {
		return
	}
	ctx, cancel := dm.context(dmDFTimeout)
	defer cancel()
	if _, ok := dm.host(ctx, hostID); !ok {
		return
	}
	usage, fetchedAt, cached, err := live.DiskUsage(ctx, hostID, dmTruthy(dm.r.URL.Query().Get("refresh")))
	if err != nil {
		dm.liveFailure(err, "")
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"df": usage, "fetched_at": fetchedAt, "cached": cached})
}

// ── Operation bridge ────────────────────────────────────────────────────────

// dmOperationObserver is *dockerstacks.Service as the bridge uses it.
type dmOperationObserver interface {
	Observe(fn func(dockerstacks.Operation)) (cancel func())
}

// dmPublisher is *dockerwatch.Hub as the bridge uses it.
type dmPublisher interface {
	Publish(host string, msg dockerwatch.Message)
}

var (
	_ dmOperationObserver = (*dockerstacks.Service)(nil)
	_ dmPublisher         = (*dockerwatch.Hub)(nil)
)

// dmOperationMessage is the data of the `operation` stream message.
type dmOperationMessage struct {
	Host      string                 `json:"host"`
	Operation dockerstacks.Operation `json:"operation"`
}

// bridgeDockerOperations forwards every state change of a stack operation (never
// its output) to the stream clients subscribed to the operation's host. The
// observer runs with the service's locks held, and Publish never blocks. The
// returned func stops the forwarding.
func bridgeDockerOperations(src dmOperationObserver, dst dmPublisher) (cancel func()) {
	return src.Observe(func(op dockerstacks.Operation) {
		op.Output = ""
		dst.Publish(op.HostID, dockerwatch.Message{
			Event: dockerwatch.MsgOperation,
			Data:  dmOperationMessage{Host: op.HostID, Operation: op},
		})
	})
}

// initDockerWatch creates the live data hub on the host registry and connects
// the stacks service's operation state changes to it. The hub starts nothing
// until a stream client subscribes.
func (s *Server) initDockerWatch() {
	hub := dockerwatch.New(s.DockerHosts, dockerwatch.Options{Logger: s.Logger})
	s.DockerWatch = hub
	if s.DockerStacks != nil {
		s.dockerOpsBridge = bridgeDockerOperations(s.DockerStacks, hub)
	}
}

// closeDockerWatch stops the operation bridge and closes the hub: every open
// stream ends and the watchers and samplers stop.
func (s *Server) closeDockerWatch() {
	if s.dockerOpsBridge != nil {
		s.dockerOpsBridge()
		s.dockerOpsBridge = nil
	}
	if s.DockerWatch != nil {
		s.DockerWatch.Close()
	}
}
