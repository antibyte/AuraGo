package dockerwatch

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
)

// Client is one open /stream: a buffered channel of messages and the
// subscription that decides what is sent to it.
type Client struct {
	hub *Hub
	id  string
	ch  chan Message

	mu     sync.Mutex // closed and every send on ch
	closed bool

	sub Subscription // guarded by hub.mu
}

// Attach opens a stream client. It fails with ErrTooManyClients when
// MaxClients are open.
func (h *Hub) Attach() (*Client, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, fmt.Errorf("generate stream id: %w", err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrHubClosed
	}
	if len(h.clients) >= MaxClients {
		return nil, ErrTooManyClients
	}
	c := &Client{hub: h, id: hex.EncodeToString(raw[:]), ch: make(chan Message, ClientBuffer)}
	h.clients[c.id] = c
	return c, nil
}

// Client returns the open client with the given stream id.
func (h *Hub) Client(id string) (*Client, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.clients[id]
	if !ok || c.isClosed() {
		return nil, false
	}
	return c, true
}

// ID is the stream id: 32 lowercase hex characters.
func (c *Client) ID() string { return c.id }

// Messages is the client's stream. It is closed when the client is closed or
// dropped; a dropped client reads a final `dropped` message first.
func (c *Client) Messages() <-chan Message { return c.ch }

// Close ends the client and releases what only it needed. It is idempotent.
func (c *Client) Close() {
	c.shut()
	h := c.hub
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c.id]; !ok {
		return
	}
	delete(h.clients, c.id)
	for _, host := range subscribedHosts(c.sub) {
		h.reconcileLocked(host)
	}
}

// shut closes the message channel once.
func (c *Client) shut() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.ch)
	}
}

func (c *Client) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// push queues a message without blocking. A client whose buffer is full is
// sent `dropped` in place of the oldest unread message and closed; the UI
// reconnects and subscribes again.
func (c *Client) push(msg Message) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	select {
	case c.ch <- msg:
		c.mu.Unlock()
		return
	default:
	}
	select {
	case <-c.ch:
	default:
	}
	c.ch <- Message{Event: MsgDropped, Data: Dropped{Reason: "slow_client"}} // a slot was just freed and only push sends
	c.closed = true
	close(c.ch)
	c.mu.Unlock()
	go c.Close() // takes Hub.mu, which the caller holds
}

// Set replaces the client's subscription. The whole subscription is checked
// first; on an error nothing changes. Errors match ErrInvalidSubscription and,
// for the limits, ErrTooManyIDs or ErrTooManyHosts.
func (c *Client) Set(sub Subscription) (Accepted, error) {
	next, err := normalizeSubscription(sub)
	if err != nil {
		return Accepted{}, err
	}
	h := c.hub
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return Accepted{}, ErrHubClosed
	}
	if c.isClosed() {
		return Accepted{}, ErrClientClosed
	}
	prev := c.sub
	c.sub = next
	sampling := map[string]bool{} // hosts whose sampling loop already ran before this call
	for host := range next.Stats {
		if rt := h.rts[host]; rt != nil && rt.st.loop {
			sampling[host] = true
		}
	}
	touched := slices.Concat(subscribedHosts(prev), subscribedHosts(next))
	slices.Sort(touched)
	for _, host := range slices.Compact(touched) {
		h.reconcileLocked(host)
	}
	for _, host := range next.Hosts {
		if slices.Contains(prev.Hosts, host) {
			continue
		}
		if rt := h.rts[host]; rt != nil && rt.status.State != "" {
			c.push(Message{Event: MsgHostStatus, Data: rt.status})
		}
	}
	for host, ids := range next.Stats {
		rt := h.rts[host]
		if rt == nil {
			continue
		}
		added := false
		for _, id := range ids {
			if slices.Contains(prev.Stats[host], id) {
				continue
			}
			h.subscribeStatsLocked(rt, c, id)
			added = true
		}
		if added && sampling[host] {
			rt.st.wake() // a new loop samples at once anyway; a running one would wait for its next tick
		}
	}
	accepted := Accepted{Hosts: len(next.Hosts), HostStats: len(next.HostStats)}
	for _, ids := range next.Stats {
		accepted.Stats += len(ids)
	}
	return accepted, nil
}

// subscribedHosts lists every host a subscription mentions.
func subscribedHosts(sub Subscription) []string {
	hosts := append([]string(nil), sub.Hosts...)
	hosts = append(hosts, sub.HostStats...)
	for host := range sub.Stats {
		hosts = append(hosts, host)
	}
	slices.Sort(hosts)
	return slices.Compact(hosts)
}

var (
	reHostID      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
	reContainerID = regexp.MustCompile(`^[0-9a-f]{12,64}$`)
)

// subscriptionError matches ErrInvalidSubscription and its own kind.
type subscriptionError struct {
	kind error
	msg  string
}

func (e *subscriptionError) Error() string { return e.msg }
func (e *subscriptionError) Is(target error) bool {
	return target == e.kind || target == ErrInvalidSubscription
}

func invalidSubscription(format string, args ...any) error {
	return &subscriptionError{kind: ErrInvalidSubscription, msg: "invalid subscription: " + fmt.Sprintf(format, args...)}
}

// normalizeHosts validates a list of host ids, drops duplicates and sorts it.
func normalizeHosts(list []string, field string) ([]string, error) {
	seen := map[string]struct{}{}
	for _, host := range list {
		if !reHostID.MatchString(host) {
			return nil, invalidSubscription("%s: %q is not a host id", field, host)
		}
		seen[host] = struct{}{}
	}
	if len(seen) > MaxSubscribedHosts {
		return nil, &subscriptionError{kind: ErrTooManyHosts, msg: fmt.Sprintf("%s: at most %d hosts per subscription", field, MaxSubscribedHosts)}
	}
	out := make([]string, 0, len(seen))
	for host := range seen {
		out = append(out, host)
	}
	sort.Strings(out)
	return out, nil
}

// normalizeSubscription checks the limits and reduces container ids to their
// first 12 characters, lowercase.
func normalizeSubscription(sub Subscription) (Subscription, error) {
	hosts, err := normalizeHosts(sub.Hosts, "hosts")
	if err != nil {
		return Subscription{}, err
	}
	gauges, err := normalizeHosts(sub.HostStats, "host_stats")
	if err != nil {
		return Subscription{}, err
	}
	if len(sub.Stats) > MaxSubscribedHosts {
		return Subscription{}, &subscriptionError{kind: ErrTooManyHosts, msg: fmt.Sprintf("stats: at most %d hosts per subscription", MaxSubscribedHosts)}
	}
	stats := map[string][]string{}
	for host, list := range sub.Stats {
		if !reHostID.MatchString(host) {
			return Subscription{}, invalidSubscription("stats: %q is not a host id", host)
		}
		seen := map[string]struct{}{}
		for _, id := range list {
			id = strings.ToLower(id)
			if !reContainerID.MatchString(id) {
				return Subscription{}, invalidSubscription("stats: %q is not a container id", id)
			}
			seen[id[:12]] = struct{}{}
		}
		if len(seen) > MaxStatsIDs {
			return Subscription{}, &subscriptionError{kind: ErrTooManyIDs, msg: fmt.Sprintf("stats: at most %d container ids per host", MaxStatsIDs)}
		}
		if len(seen) == 0 {
			continue
		}
		ids := make([]string, 0, len(seen))
		for id := range seen {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		stats[host] = ids
	}
	return Subscription{Hosts: hosts, Stats: stats, HostStats: gauges}, nil
}
