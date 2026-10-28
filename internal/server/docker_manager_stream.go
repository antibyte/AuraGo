package server

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sort"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerwatch"
)

// ── Docker Manager live stream ──────────────────────────────────────────────
// GET /stream is one server-sent event stream per open Docker Manager window; it
// carries what the hub produces (host_status, event, resync, stats,
// stats_history, stats_gone, host_stats, operation, dropped). What a window
// receives is chosen with PUT /stream/subscriptions, which names the stream by
// the id of its hello message. Both routes read only: docker.readonly allows them.

// dmStreamLimits are the limits the hello message tells the client about.
type dmStreamLimits struct {
	StatsIDs int `json:"stats_ids"`
	Hosts    int `json:"hosts"`
}

// dmStreamHello is the first message of a stream.
type dmStreamHello struct {
	StreamID    string         `json:"stream_id"`
	HeartbeatMS int64          `json:"heartbeat_ms"`
	Limits      dmStreamLimits `json:"limits"`
}

// liveStream serves GET /stream. The first frame is `hello`; then every message
// of the hub's client in order; a comment line every dmSSEHeartbeat while quiet.
// The stream ends when the request ends, when a write fails or times out, or when
// the hub closes the client (a client that fell 512 messages behind is sent
// `dropped` first). The hub client is always closed.
func (dm *dmRequest) liveStream() {
	if !dm.allow(http.MethodGet) {
		return
	}
	live, ok := dm.liveService()
	if !ok {
		return
	}
	flusher, ok := dm.w.(http.Flusher)
	if !ok {
		dmWriteError(dm.w, http.StatusInternalServerError, dmCodeInternal, "streaming is not supported", nil)
		return
	}
	client, err := live.Attach()
	if err != nil {
		if errors.Is(err, dockerwatch.ErrTooManyClients) || errors.Is(err, dockerwatch.ErrHubClosed) {
			dm.liveFailure(err, "")
			return
		}
		dm.internalFailure("Docker live stream could not be opened", err)
		return
	}
	defer client.Close()

	header := dm.w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("X-Accel-Buffering", "no")
	dm.w.WriteHeader(http.StatusOK)
	stream := &dmEventStream{w: dm.w, flusher: flusher, rc: http.NewResponseController(dm.w)}

	hello := dmStreamHello{
		StreamID:    client.ID(),
		HeartbeatMS: dmSSEHeartbeat.Milliseconds(),
		Limits:      dmStreamLimits{StatsIDs: dockerwatch.MaxStatsIDs, Hosts: dockerwatch.MaxSubscribedHosts},
	}
	if stream.send("hello", hello) != nil {
		return
	}
	beat := time.NewTicker(dmSSEHeartbeat)
	defer beat.Stop()
	ctx := dm.r.Context()
	messages := client.Messages()
	for {
		select {
		case <-ctx.Done():
			return
		case <-beat.C:
			if stream.comment("heartbeat") != nil {
				return
			}
		case msg, open := <-messages:
			if !open {
				return
			}
			if stream.send(msg.Event, msg.Data) != nil {
				return
			}
		}
	}
}

// dmSubscriptionRequest is the body of PUT /stream/subscriptions. It replaces
// the whole subscription of the stream.
type dmSubscriptionRequest struct {
	StreamID  string              `json:"stream_id"`
	Hosts     []string            `json:"hosts"`
	Stats     map[string][]string `json:"stats"`
	HostStats []string            `json:"host_stats"`
}

// Reasons of a rejected host.
const (
	dmRejectHostNotFound    = dmCodeHostNotFound // no host with this id (a malformed id included)
	dmRejectHostUnavailable = "host_unavailable" // the registry could not look the host up
)

// dmRejectedHost names a host of a subscription that was left out and why.
type dmRejectedHost struct {
	Host   string `json:"host"`
	Reason string `json:"reason"`
}

// liveSubscriptions serves PUT /stream/subscriptions. A host the registry does
// not know (or cannot look up) is left out of the subscription and listed in
// `rejected` with a reason; it never fails the request. The rest is stored as a
// whole, and the hub sends the current `host_status` of every host that is new
// to this stream. An unknown stream id answers 404 stream_not_found, a
// subscription over a limit or with a malformed container id 400 bad_request
// with the hub's message (it names the limit).
func (dm *dmRequest) liveSubscriptions() {
	if !dm.allow(http.MethodPut) {
		return
	}
	live, ok := dm.liveService()
	if !ok {
		return
	}
	var req dmSubscriptionRequest
	if !dm.decode(&req) {
		return
	}
	client, found := live.Client(req.StreamID)
	if !found {
		dmWriteError(dm.w, http.StatusNotFound, dmCodeStreamNotFound, "There is no open stream with this id", nil)
		return
	}
	sub := dockerwatch.Subscription{Hosts: req.Hosts, Stats: req.Stats, HostStats: req.HostStats}
	rejected := []dmRejectedHost{}
	if dmFewHosts(sub) {
		ctx, cancel := dm.context(dmRegistryTimeout)
		sub, rejected = dm.dropUnusableHosts(ctx, sub)
		cancel()
	}
	accepted, err := client.Set(sub)
	switch {
	case err == nil:
		dmWriteOK(dm.w, http.StatusOK, dmFields{"accepted": accepted, "rejected": rejected})
	case errors.Is(err, dockerwatch.ErrInvalidSubscription):
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeBadRequest, err.Error(), nil)
	case errors.Is(err, dockerwatch.ErrClientClosed):
		dmWriteError(dm.w, http.StatusNotFound, dmCodeStreamNotFound, "There is no open stream with this id", nil)
	default:
		dm.liveFailure(err, "")
	}
}

// dmFewHosts reports whether a subscription can still be within the hub's host
// limits: more than three lists' worth of distinct hosts cannot, and the hub then
// refuses it as a whole, so the registry is not asked about each of them.
func dmFewHosts(sub dockerwatch.Subscription) bool {
	return len(dmSubscribedHosts(sub)) <= 3*dockerwatch.MaxSubscribedHosts
}

// dmSubscribedHosts lists the distinct host ids a subscription names, sorted.
func dmSubscribedHosts(sub dockerwatch.Subscription) []string {
	hosts := slices.Concat(sub.Hosts, sub.HostStats)
	for host := range sub.Stats {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return slices.Compact(hosts)
}

// dropUnusableHosts removes the hosts the registry does not know, or cannot look
// up, from every list of the subscription and returns them, sorted by id, with
// the reason.
func (dm *dmRequest) dropUnusableHosts(ctx context.Context, sub dockerwatch.Subscription) (dockerwatch.Subscription, []dmRejectedHost) {
	dropped := map[string]bool{}
	rejected := []dmRejectedHost{}
	for _, host := range dmSubscribedHosts(sub) {
		_, err := dm.registry().Host(ctx, host)
		if err == nil {
			continue
		}
		reason := dmRejectHostNotFound
		if !errors.Is(err, dockerhost.ErrHostNotFound) {
			dm.s.logDockerManager("Docker stream subscription could not look up a host", err)
			reason = dmRejectHostUnavailable
		}
		dropped[host] = true
		rejected = append(rejected, dmRejectedHost{Host: host, Reason: reason})
	}
	if len(dropped) == 0 {
		return sub, rejected
	}
	kept := dockerwatch.Subscription{
		Hosts:     slices.DeleteFunc(slices.Clone(sub.Hosts), func(host string) bool { return dropped[host] }),
		HostStats: slices.DeleteFunc(slices.Clone(sub.HostStats), func(host string) bool { return dropped[host] }),
		Stats:     map[string][]string{},
	}
	for host, ids := range sub.Stats {
		if !dropped[host] {
			kept.Stats[host] = ids
		}
	}
	return kept, rejected
}
