package hass

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"aurago/internal/security"
)

// Connection states and failure reasons reported through WSHandler.OnStatus.
const (
	WSConnecting = "connecting"
	WSLive       = "live"
	WSDegraded   = "degraded"
	WSAuthFailed = "auth_failed"

	ReasonUnreachable = "unreachable"
	ReasonTimeout     = "timeout"
	ReasonProtocol    = "protocol"
	ReasonAuthFailed  = "auth_failed"

	kindStates    = "states"
	kindSubscribe = "subscribe"
	kindAreas     = "areas"
	kindDevices   = "devices"
	kindEntities  = "entities"
	kindPing      = "ping"
)

// WSStatus: Failures counts consecutive sessions that ended before auth_ok (not auth_invalid); the
// adapter switches to REST polling after 3.
type WSStatus struct {
	State, Reason string
	Failures      int
}

// WSHandler callbacks all run on the Run goroutine and must not block.
type WSHandler interface {
	OnStates([]HAState)
	OnStateChanged(StateChanged)
	OnRegistries(Registries)
	OnStatus(WSStatus)
}

// WSOptions tunes timing; zero values select the documented defaults.
type WSOptions struct {
	HandshakeTimeout time.Duration   // 10 s
	PingInterval     time.Duration   // 30 s
	PongTimeout      time.Duration   // 10 s
	AuthRetry        time.Duration   // 5 min
	RegistryDebounce time.Duration   // 2 s
	Backoff          []time.Duration // 1,2,4,8,16,30,60 s
	Jitter           float64         // 0 = 0.2; negative = none
	ReadLimit        int64           // 16 MiB
}

func (o WSOptions) withDefaults() WSOptions {
	def := func(d *time.Duration, v time.Duration) {
		if *d <= 0 {
			*d = v
		}
	}
	def(&o.HandshakeTimeout, 10*time.Second)
	def(&o.PingInterval, 30*time.Second)
	def(&o.PongTimeout, 10*time.Second)
	def(&o.AuthRetry, 5*time.Minute)
	def(&o.RegistryDebounce, 2*time.Second)
	if len(o.Backoff) == 0 {
		o.Backoff = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, time.Minute}
	}
	if o.Jitter == 0 {
		o.Jitter = 0.2
	}
	if o.ReadLimit <= 0 {
		o.ReadLimit = 16 << 20
	}
	return o
}

var (
	errAuthInvalid = errors.New("hass: access token rejected")
	errNoPong      = errors.New("hass: no pong")
	errProtocol    = errors.New("hass: unexpected message")
	errDial        = errors.New("hass: websocket handshake failed")
)

var registryCommand = map[string]string{
	kindAreas: "config/area_registry/list", kindDevices: "config/device_registry/list", kindEntities: "config/entity_registry/list",
}

// WSClient talks to the Home Assistant websocket API. The token only ever travels in the auth message.
type WSClient struct {
	url, token string
	h          WSHandler
	o          WSOptions
	dialer     *websocket.Dialer
	failures   int // touched only by the Run goroutine

	// Unexported timing knobs; tests shorten them after NewWSClient.
	registryTimeout time.Duration // a registry batch not complete after this long is abandoned
	stableAfter     time.Duration // an authenticated session must last this long to reset the backoff
}

const (
	defaultRegistryTimeout = 30 * time.Second
	defaultStableAfter     = time.Minute
)

// WebsocketURL derives ws(s)://<base>/api/websocket from the validated Home Assistant base URL.
func WebsocketURL(base string) (string, error) {
	if err := security.ValidateHTTPBaseURL(base); err != nil {
		return "", err
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/websocket"
	return u.String(), nil
}

// NewWSClient validates the base URL and prepares a client; call Run to connect.
func NewWSClient(baseURL, token string, h WSHandler, o WSOptions) (*WSClient, error) {
	u, err := WebsocketURL(baseURL)
	if err != nil {
		return nil, err
	}
	o = o.withDefaults()
	// The gorilla dialer never follows redirects: a 3xx fails the handshake, which keeps the
	// same-origin rule of the REST client. No Origin or Authorization header is sent.
	return &WSClient{url: u, token: token, h: h, o: o,
		dialer:          &websocket.Dialer{HandshakeTimeout: o.HandshakeTimeout, Proxy: http.ProxyFromEnvironment},
		registryTimeout: defaultRegistryTimeout, stableAfter: defaultStableAfter}, nil
}

func (c *WSClient) jitter(d time.Duration) time.Duration {
	if c.o.Jitter <= 0 {
		return d
	}
	return time.Duration(float64(d) * (1 + c.o.Jitter*(2*rand.Float64()-1)))
}

func classify(err error) string {
	var ne net.Error
	switch {
	case errors.Is(err, errNoPong), errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return ReasonTimeout
	case errors.Is(err, errDial):
		return ReasonUnreachable
	}
	return ReasonProtocol
}

// Run connects, resyncs and reconnects until ctx ends. auth_invalid retries only every AuthRetry.
func (c *WSClient) Run(ctx context.Context) error {
	if c.token != "" {
		security.RegisterSensitive(c.token)
	}
	attempt := 0
	for ctx.Err() == nil {
		c.h.OnStatus(WSStatus{State: WSConnecting, Failures: c.failures})
		started := time.Now()
		authed, err := c.session(ctx)
		if ctx.Err() != nil {
			return nil
		}
		var wait time.Duration
		if errors.Is(err, errAuthInvalid) {
			c.h.OnStatus(WSStatus{State: WSAuthFailed, Reason: ReasonAuthFailed, Failures: c.failures})
			wait, attempt = c.o.AuthRetry, 0
		} else {
			if !authed {
				c.failures++
			} else if time.Since(started) >= c.stableAfter {
				// Only a stable session resets the backoff; one that authenticated and then dropped
				// early still counts as a failed attempt for the delay (not for the REST fallback).
				attempt = 0
			}
			c.h.OnStatus(WSStatus{State: WSDegraded, Reason: classify(err), Failures: c.failures})
			wait = c.jitter(c.o.Backoff[min(attempt, len(c.o.Backoff)-1)])
			attempt++
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
		}
	}
	return nil
}

func (c *WSClient) session(ctx context.Context) (bool, error) {
	dctx, cancel := context.WithTimeout(ctx, c.o.HandshakeTimeout)
	conn, resp, err := c.dialer.DialContext(dctx, c.url, nil)
	cancel()
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return false, errors.Join(errDial, err)
	}
	defer conn.Close()
	conn.SetReadLimit(c.o.ReadLimit)
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	s := &wsSession{c: c, conn: conn, pending: map[int64]string{}, pendingBatch: map[int64]int64{}}
	if err := s.authenticate(); err != nil {
		return false, err
	}
	c.failures = 0
	return true, s.serve(ctx)
}

// wsSession owns one connection. wmu serializes every write (single writer); mu guards the
// request bookkeeping and the registry re-list state. The connection itself is read by the
// serve goroutine only; Close may be called from any goroutine.
type wsSession struct {
	c        *WSClient
	conn     *websocket.Conn
	wmu      sync.Mutex // single writer
	mu       sync.Mutex // pending, pendingBatch, nextID, and every registry batch field below
	nextID   int64
	pending  map[int64]string
	lastPong atomic.Int64

	// Registry batch state, all guarded by mu. At most one batch is in flight (batchActive).
	pendingBatch map[int64]int64 // request id -> batch number, registry requests only
	batch        int64           // number of the newest batch
	batchActive  bool
	reg          Registries
	regLeft      int
	regTimer     *time.Timer // abandons the active batch after registryTimeout
	dirty        bool        // a re-list was requested while a batch was in flight
	relist       *time.Timer // the single debounce timer, created once
	relistArmed  bool
	closed       bool
}

func (s *wsSession) write(v any) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return s.conn.WriteJSON(v)
}

func (s *wsSession) request(kind string, body map[string]any) error {
	return s.requestBatch(kind, body, 0)
}

// requestBatch is request that also remembers the registry batch (0 = none) the answer belongs to.
func (s *wsSession) requestBatch(kind string, body map[string]any, batch int64) error {
	// The id is allocated and the frame written under one lock so ids reach the wire in order.
	s.wmu.Lock()
	defer s.wmu.Unlock()
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	s.pending[id] = kind
	if batch != 0 {
		s.pendingBatch[id] = batch
	}
	s.mu.Unlock()
	body["id"] = id
	_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return s.conn.WriteJSON(body)
}

func (s *wsSession) take(id int64) string {
	k, _ := s.takeBatch(id)
	return k
}

func (s *wsSession) takeBatch(id int64) (string, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, b := s.pending[id], s.pendingBatch[id]
	delete(s.pending, id)
	delete(s.pendingBatch, id)
	return k, b
}

func (s *wsSession) authenticate() error {
	_ = s.conn.SetReadDeadline(time.Now().Add(s.c.o.HandshakeTimeout))
	var m wsMsg
	if err := s.conn.ReadJSON(&m); err != nil {
		return err
	}
	if m.Type != "auth_required" {
		return errProtocol
	}
	if err := s.write(map[string]any{"type": "auth", "access_token": s.c.token}); err != nil {
		return err
	}
	m = wsMsg{}
	if err := s.conn.ReadJSON(&m); err != nil {
		return err
	}
	switch m.Type {
	case "auth_ok":
		return s.conn.SetReadDeadline(time.Time{})
	case "auth_invalid":
		return errAuthInvalid
	}
	return errProtocol
}

// listRegistries starts a registry batch unless one is already in flight; in that case it only
// marks the session dirty so a new batch follows the current one.
func (s *wsSession) listRegistries() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	if s.batchActive {
		s.dirty = true
		s.mu.Unlock()
		return nil
	}
	s.batch++
	batch := s.batch
	s.batchActive, s.dirty = true, false
	s.reg, s.regLeft = Registries{}, 3
	s.regTimer = time.AfterFunc(s.c.registryTimeout, func() { s.abandonBatch(batch) })
	s.mu.Unlock()
	for _, k := range []string{kindAreas, kindDevices, kindEntities} {
		if err := s.requestBatch(k, map[string]any{"type": registryCommand[k]}, batch); err != nil {
			return err
		}
	}
	return nil
}

// abandonBatch drops a batch that did not complete in time and schedules one new batch.
func (s *wsSession) abandonBatch(batch int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !s.batchActive || s.batch != batch {
		return
	}
	s.batchActive, s.dirty = false, false
	for id, b := range s.pendingBatch {
		if b == batch {
			delete(s.pendingBatch, id)
			delete(s.pending, id)
		}
	}
	s.armRelistLocked()
}

func (s *wsSession) serve(ctx context.Context) error {
	defer s.stopRelist()
	if err := s.request(kindStates, map[string]any{"type": "get_states"}); err != nil {
		return err
	}
	if err := s.request(kindSubscribe, map[string]any{"type": "subscribe_events", "event_type": "state_changed"}); err != nil {
		return err
	}
	if err := s.listRegistries(); err != nil {
		return err
	}
	for _, ev := range []string{"area_registry_updated", "device_registry_updated", "entity_registry_updated"} {
		if err := s.request(kindSubscribe, map[string]any{"type": "subscribe_events", "event_type": ev}); err != nil {
			return err
		}
	}
	pctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pingErr := make(chan error, 1)
	go s.pinger(pctx, pingErr)
	for {
		var m wsMsg
		if err := s.conn.ReadJSON(&m); err != nil {
			select {
			case perr := <-pingErr:
				return perr
			default:
				return err
			}
		}
		if err := s.handle(m); err != nil {
			return err
		}
	}
}

func (s *wsSession) pinger(ctx context.Context, errc chan<- error) {
	t := time.NewTicker(s.c.o.PingInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		sent := time.Now()
		if err := s.request(kindPing, map[string]any{"type": "ping"}); err != nil {
			return
		}
		pt := time.NewTimer(s.c.o.PongTimeout)
		select {
		case <-ctx.Done():
			pt.Stop()
			return
		case <-pt.C:
		}
		if s.lastPong.Load() < sent.UnixNano() {
			errc <- errNoPong
			s.conn.Close()
			return
		}
	}
}

func (s *wsSession) handle(m wsMsg) error {
	switch m.Type {
	case "pong":
		s.lastPong.Store(time.Now().UnixNano())
		s.take(m.ID)
	case "event":
		var ev struct {
			EventType string          `json:"event_type"`
			Data      json.RawMessage `json:"data"`
		}
		if json.Unmarshal(m.Event, &ev) != nil {
			return nil
		}
		switch ev.EventType {
		case "state_changed":
			var sc StateChanged
			if json.Unmarshal(ev.Data, &sc) == nil && sc.EntityID != "" {
				s.c.h.OnStateChanged(sc)
			}
		case "area_registry_updated", "device_registry_updated", "entity_registry_updated":
			s.scheduleRelist()
		}
	case "result":
		kind, batch := s.takeBatch(m.ID)
		ok := m.Success != nil && *m.Success
		switch kind {
		case kindStates:
			if !ok {
				return errProtocol
			}
			var st []HAState
			if err := json.Unmarshal(m.Result, &st); err != nil {
				return fmt.Errorf("hass: decode states: %w", err)
			}
			s.c.h.OnStates(st)
			s.c.h.OnStatus(WSStatus{State: WSLive})
		case kindAreas, kindDevices, kindEntities:
			s.registryResult(batch, kind, ok, m.Result)
		}
	}
	return nil
}

// registryResult attributes an answer to its batch; answers of abandoned or older batches are ignored.
func (s *wsSession) registryResult(batch int64, kind string, ok bool, raw json.RawMessage) {
	s.mu.Lock()
	if s.closed || !s.batchActive || batch != s.batch || s.regLeft <= 0 {
		s.mu.Unlock()
		return
	}
	if ok {
		switch kind {
		case kindAreas:
			_ = json.Unmarshal(raw, &s.reg.Areas)
		case kindDevices:
			_ = json.Unmarshal(raw, &s.reg.Devices)
		case kindEntities:
			_ = json.Unmarshal(raw, &s.reg.Entities)
		}
	}
	s.regLeft--
	done, reg := s.regLeft == 0, s.reg
	if done {
		s.batchActive = false
		if s.regTimer != nil {
			s.regTimer.Stop()
		}
		if s.dirty {
			s.dirty = false
			s.armRelistLocked()
		}
	}
	s.mu.Unlock()
	if done {
		s.c.h.OnRegistries(reg)
	}
}

// scheduleRelist debounces a re-list request; while a batch is in flight it only marks it dirty.
func (s *wsSession) scheduleRelist() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.batchActive {
		s.dirty = true
		return
	}
	s.armRelistLocked()
}

// armRelistLocked (mu held) arms the single debounce timer, created once per session.
func (s *wsSession) armRelistLocked() {
	if s.closed {
		return
	}
	s.relistArmed = true
	if s.relist == nil {
		s.relist = time.AfterFunc(s.c.o.RegistryDebounce, s.relistFire)
		return
	}
	s.relist.Reset(s.c.o.RegistryDebounce)
}

func (s *wsSession) relistFire() {
	s.mu.Lock()
	armed, closed := s.relistArmed, s.closed
	s.relistArmed = false
	s.mu.Unlock()
	if armed && !closed {
		_ = s.listRegistries()
	}
}

func (s *wsSession) stopRelist() {
	s.mu.Lock()
	s.closed = true
	s.relistArmed = false
	if s.relist != nil {
		s.relist.Stop()
	}
	if s.regTimer != nil {
		s.regTimer.Stop()
	}
	clear(s.pending)
	clear(s.pendingBatch)
	s.mu.Unlock()
}
