package meshcore

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var defaultManager atomic.Pointer[Manager]

func DefaultManager() *Manager     { return defaultManager.Load() }
func SetDefaultManager(m *Manager) { defaultManager.Store(m) }

type Manager struct {
	lifecycle   sync.Mutex
	mu          sync.Mutex
	refreshSlot chan struct{}
	store       *store
	hooks       Hooks
	cfg         Config
	docker      bool
	suspended   bool
	status      Status
	conn        *companion
	root        context.Context
	cancel      context.CancelFunc
	runCancel   context.CancelFunc
	wg          sync.WaitGroup
	queue       chan Message
	runs        map[string][]time.Time
	sends       []time.Time
	open        func(context.Context, Config, bool) (frameLink, error)
	manualSlots chan struct{}
	writeSlot   chan struct{}
	editing     bool
	closed      bool
}

func NewManager(ctx context.Context, dir string, hooks Hooks) (*Manager, error) {
	s, err := openStore(dir)
	if err != nil {
		return nil, err
	}
	return &Manager{store: s, hooks: hooks, root: ctx, refreshSlot: make(chan struct{}, 1), manualSlots: make(chan struct{}, 16), writeSlot: make(chan struct{}, 1), status: Status{State: "disabled", Contacts: []Contact{}, Channels: []Channel{}}, runs: map[string][]time.Time{}, open: openLink}, nil
}
func (m *Manager) Configure(cfg Config, docker bool) error {
	cfg.TrustedNodes = slices.Clone(cfg.TrustedNodes)
	cfg.SendNodes = slices.Clone(cfg.SendNodes)
	cfg.Channels = slices.Clone(cfg.Channels)
	if err := cfg.Normalize(); err != nil {
		return err
	}
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.mu.Lock()
	if !m.suspended && reflect.DeepEqual(cfg, m.cfg) && docker == m.docker {
		m.mu.Unlock()
		return nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
	m.mu.Unlock()
	m.wg.Wait()
	// Queued input survives a reconfiguration but is never replayed automatically.
	if _, err := m.store.db.Exec("UPDATE meshcore_messages SET state='received' WHERE state='pending'"); err != nil {
		return err
	}
	if err := m.store.prune(cfg); err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = cfg
	m.suspended = false
	m.docker = docker
	m.status = Status{State: "disabled", Contacts: []Contact{}, Channels: []Channel{}}
	ctx, cancel := context.WithCancel(m.root)
	m.cancel = cancel
	queue := make(chan Message, 128)
	m.queue = queue
	m.mu.Unlock()
	if cfg.Enabled {
		m.wg.Add(2)
		go func() { defer m.wg.Done(); m.connectLoop(ctx, cfg, docker, queue) }()
		go func() {
			defer m.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case msg := <-queue:
					m.process(ctx, msg)
				}
			}
		}()
	}
	return nil
}
func (m *Manager) Close() error {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.mu.Lock()
	m.closed = true
	if m.cancel != nil {
		m.cancel()
	}
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
	m.mu.Unlock()
	m.wg.Wait()
	return m.store.db.Close()
}

// Suspend revokes current work before a changed configuration is published.
func (m *Manager) Suspend() {
	m.mu.Lock()
	m.suspended = true
	m.status.State = "suspended"
	if m.cancel != nil {
		m.cancel()
	}
	c := m.conn
	m.mu.Unlock()
	if c != nil {
		c.Close()
	}
}
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.status
	st.Contacts = append([]Contact{}, st.Contacts...)
	st.Channels = append([]Channel{}, st.Channels...)
	return st
}
func (m *Manager) Messages(limit, offset int) ([]Message, error) { return m.store.list(limit, offset) }
func (m *Manager) issue(code string, active bool) {
	if m.hooks.Issue != nil {
		m.hooks.Issue(code, active)
	}
}
func (m *Manager) scrub(s string) string {
	if m.hooks.Scrub != nil {
		return m.hooks.Scrub(s)
	}
	return s
}
func (m *Manager) save(msg Message) bool {
	if err := m.store.save(msg); err != nil {
		m.issue("persistence", true)
		return false
	}
	m.issue("persistence", false)
	m.chatChanged(msg, false)
	return true
}

func (m *Manager) connectLoop(ctx context.Context, cfg Config, docker bool, queue chan Message) {
	backoff := time.Second
	for ctx.Err() == nil {
		m.mu.Lock()
		m.status.State = "connecting"
		m.mu.Unlock()
		link, err := m.open(ctx, cfg, docker)
		if err == nil {
			c := newCompanion(link)
			m.mu.Lock()
			m.conn = c
			m.mu.Unlock()
			err = m.receiveLoop(ctx, c, queue)
			c.Close()
			m.mu.Lock()
			if m.conn == c {
				m.conn = nil
				if m.runCancel != nil {
					m.runCancel()
				}
			}
			m.mu.Unlock()
		}
		if ctx.Err() != nil {
			return
		}
		m.mu.Lock()
		m.status.State = "disconnected"
		m.status.ErrorCode = "connection_failed"
		m.mu.Unlock()
		m.issue("connection", true)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}
func (m *Manager) refresh(ctx context.Context, c *companion) (Status, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	select {
	case m.refreshSlot <- struct{}{}:
		defer func() { <-m.refreshSlot }()
	case <-ctx.Done():
		return Status{}, ctx.Err()
	}
	st, err := c.snapshot(ctx, m.store.salt)
	if err != nil {
		return st, err
	}
	m.scrubStatus(&st)
	if err := m.store.syncConversations(st); err != nil {
		return st, err
	}
	var pending string
	if err := m.store.db.QueryRow("SELECT COALESCE((SELECT value FROM meshcore_meta WHERE key='mutation_pending'),'0')").Scan(&pending); err != nil {
		return st, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conn != c || m.suspended {
		return st, fmt.Errorf("connection changed")
	}
	if m.cfg.IdentityKey == "" || m.cfg.IdentityKey != st.IdentityKey {
		st.State = "binding_required"
	} else {
		for _, r := range m.cfg.Channels {
			if r.Binding != "" {
				if !channelMatches(st, r) {
					st.State = "binding_changed"
					break
				}
			}
		}
	}
	if m.editing {
		st.State = "updating"
	} else if pending == "1" {
		st.State = "binding_changed"
	}
	m.status = st
	if st.State != "connected" && m.runCancel != nil {
		m.runCancel()
	}
	return st, nil
}
func channelMatches(st Status, r ChannelRule) bool {
	for _, ch := range st.Channels {
		if ch.Index == r.Index && r.Binding != "" && ch.Binding == r.Binding {
			return true
		}
	}
	return false
}
func (m *Manager) Test(ctx context.Context) (Status, error) {
	m.mu.Lock()
	c := m.conn
	enabled := m.cfg.Enabled
	m.mu.Unlock()
	if !enabled || c == nil {
		return m.Status(), fmt.Errorf("meshcore_not_connected")
	}
	return m.refresh(ctx, c)
}
func (m *Manager) receiveLoop(ctx context.Context, c *companion, queue chan Message) error {
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		if err := m.receiveBatch(ctx, c, queue); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
			return fmt.Errorf("disconnected")
		case <-c.wake:
		case <-tick.C:
		}
	}
}

// Keep contact/channel snapshots and queue draining atomic with device edits.
func (m *Manager) receiveBatch(ctx context.Context, c *companion, queue chan Message) error {
	select {
	case m.writeSlot <- struct{}{}:
		defer func() { <-m.writeSlot }()
	case <-ctx.Done():
		return ctx.Err()
	}
	st, err := m.refresh(ctx, c)
	if err != nil {
		return err
	}
	m.issue("connection", false)
	var uncertain string
	if err := m.store.db.QueryRow("SELECT COALESCE((SELECT value FROM meshcore_meta WHERE key='input_binding_uncertain'),'0')").Scan(&uncertain); err != nil {
		return err
	}
	for i := 0; i < 128; i++ {
		frames, err := c.request(ctx, []byte{10}, 7, 8, 10, 16, 17, 27)
		if err != nil {
			return err
		}
		b := frames[0]
		if b[0] == 10 {
			if _, err := m.store.db.Exec("DELETE FROM meshcore_meta WHERE key='input_binding_uncertain'"); err != nil {
				return err
			}
			break
		}
		if b[0] == 27 {
			continue
		} // binary datagrams never enter agent text
		msg, err := decodeMessage(b, st)
		if err != nil {
			m.issue("invalid_frame", true)
			continue
		}
		m.issue("invalid_frame", false)
		msg.Text = m.scrub(msg.Text)
		if uncertain == "1" {
			msg.BindingUncertain = true
			msg.Binding = ""
			msg.PeerKey = ""
		}
		added, err := m.store.insert(msg)
		if err != nil {
			return fmt.Errorf("persist incoming message: %w", err)
		}
		m.issue("persistence", false)
		if !added {
			continue
		}
		m.chatChanged(msg, true)
		select {
		case queue <- msg:
		default:
			msg.State = "received"
			msg.Reason = "queue_full"
			m.save(msg)
			m.notify(msg)
		}
	}
	m.mu.Lock()
	cfg := m.cfg
	m.mu.Unlock()
	if err := m.store.prune(cfg); err != nil {
		m.issue("persistence", true)
	}
	return nil
}
func (m *Manager) notify(msg Message) {
	if m.hooks.Notify != nil {
		m.issue("notification", m.hooks.Notify(msg) != nil)
	}
}
func uniqueContact(st Status, key string) (Contact, bool) {
	if len(key) != 12 && !ValidKey(key) {
		return Contact{}, false
	}
	// Firmware routes direct sends by six bytes even when the caller has a full key.
	if len(key) == 64 {
		key = key[:12]
	}
	var found Contact
	n := 0
	for _, c := range st.Contacts {
		if strings.HasPrefix(c.Key, key) {
			found = c
			n++
		}
	}
	return found, n == 1
}
func admit(msg Message, cfg Config, st Status, now time.Time) (string, Message) {
	if !cfg.Enabled || st.State != "connected" || msg.IdentityKey != cfg.IdentityKey || msg.TextType != 0 || msg.BindingUncertain {
		return "", msg
	}
	if msg.Kind == "direct" {
		if isReplyText(msg.Text) {
			return "", msg
		}
		contact, ok := uniqueContact(st, msg.Sender)
		if !ok || (msg.PeerKey != contact.Key && msg.Sender != contact.Key) || contact.Type != 1 || contact.Key == st.IdentityKey || !slices.Contains(cfg.TrustedNodes, contact.Key) || !Fresh(msg, cfg, now) {
			return "", msg
		}
		msg.Sender = contact.Key
		return "trusted", msg
	}
	for _, r := range cfg.Channels {
		if r.Index != msg.Channel || r.Binding != msg.Binding || !channelMatches(st, r) {
			continue
		}
		body := msg.Text
		if strings.HasPrefix(body, st.Name+": ") {
			return "", msg
		}
		if _, text, ok := strings.Cut(body, ": "); ok {
			body = text
		}
		body = strings.TrimSpace(body)
		if isReplyText(body) {
			return "", msg
		}
		if r.Mode == "prefix" {
			if !strings.HasPrefix(body, r.Prefix) || (len(body) > len(r.Prefix) && !strings.ContainsAny(body[len(r.Prefix):len(r.Prefix)+1], " \t\r\n")) {
				return "", msg
			}
			body = strings.TrimSpace(strings.TrimPrefix(body, r.Prefix))
			if body == "" {
				return "", msg
			}
		}
		if r.Mode == "receive" {
			return "", msg
		}
		msg.Text = body
		return r.Mode, msg
	}
	return "", msg
}
func isReplyText(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "[AuraGo]") || strings.HasPrefix(text, "[1/") || strings.HasPrefix(text, "[2/") || strings.HasPrefix(text, "[3/")
}
func (m *Manager) allowRun(peer string, cfg Config, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, events := range m.runs {
		events = slices.DeleteFunc(events, func(t time.Time) bool { return now.Sub(t) >= time.Minute })
		if len(events) == 0 {
			delete(m.runs, key)
		} else {
			m.runs[key] = events
		}
	}
	if len(m.runs[""]) >= cfg.RunsPerMinute || len(m.runs[peer]) >= cfg.PeerRunsPerMinute {
		return false
	}
	m.runs[""] = append(m.runs[""], now)
	m.runs[peer] = append(m.runs[peer], now)
	return true
}
func (m *Manager) process(ctx context.Context, msg Message) {
	if ctx.Err() != nil {
		return
	}
	claimed, err := m.store.claim(msg.ID)
	if err != nil {
		m.issue("persistence", true)
		return
	}
	if !claimed {
		return
	}
	msg.State = "processing"
	scanCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	review := Review{Decision: "suspicious", Reason: "scanner_unavailable"}
	if m.hooks.Scan != nil {
		review = m.hooks.Scan(scanCtx, msg)
	}
	cancel()
	msg.Review = review.Decision
	msg.Reason = m.scrub(review.Reason)
	if review.Decision != "safe" || ctx.Err() != nil {
		msg.State = "quarantine"
		if m.save(msg) {
			m.notify(msg)
		}
		return
	}
	m.mu.Lock()
	c := m.conn
	cfg := m.cfg
	m.mu.Unlock()
	var st Status
	if c != nil {
		st, err = m.refresh(ctx, c)
	}
	if c == nil || err != nil {
		msg.State = "received"
		msg.Reason = "connection_unavailable"
		if m.save(msg) {
			m.notify(msg)
		}
		return
	}
	mode, admitted := admit(msg, cfg, st, time.Now())
	peer := admitted.Sender
	if msg.Kind == "channel" {
		peer = fmt.Sprintf("channel:%d", msg.Channel)
	}
	if mode == "" || !m.allowRun(peer, cfg, time.Now()) {
		msg.State = "received"
		if mode != "" {
			msg.Reason = "rate_limited"
		}
		if m.save(msg) {
			m.notify(msg)
		}
		return
	}
	if m.hooks.Run == nil {
		msg.State = "received"
		msg.Reason = "runner_unavailable"
		m.save(msg)
		return
	}
	reserved, reserveErr := m.store.reserveExecution(msg.ID)
	if reserveErr != nil || !reserved {
		msg.State = "received"
		msg.Reason = "execution_not_reserved"
		m.save(msg)
		return
	}
	runCtx, stop := context.WithTimeout(ctx, 5*time.Minute)
	m.mu.Lock()
	m.runCancel = stop
	if m.conn != c || !reflect.DeepEqual(cfg, m.cfg) || m.status.State != "connected" {
		stop()
	}
	m.mu.Unlock()
	if runCtx.Err() != nil {
		m.mu.Lock()
		m.runCancel = nil
		m.mu.Unlock()
		msg.State = "received"
		msg.Reason = "permission_changed"
		m.save(msg)
		return
	}
	answer, err := m.hooks.Run(runCtx, admitted, mode)
	interrupted := runCtx.Err() != nil
	stop()
	m.mu.Lock()
	m.runCancel = nil
	m.mu.Unlock()
	if err != nil || interrupted || ctx.Err() != nil {
		msg.State = "outcome_unknown"
		msg.Reason = "agent_interrupted"
		m.save(msg)
		return
	}
	msg.State = "completed"
	msg.Reply = m.scrub(strings.TrimSpace(answer))
	if msg.Reply == "NO_REPLY" {
		msg.Reply = ""
		msg.State = "received"
		msg.Reason = "not_a_question"
		if m.save(msg) {
			m.notify(msg)
		}
		return
	}
	if !m.save(msg) {
		return
	}
	if msg.Reply != "" && (mode != "trusted" || cfg.DirectReplies) {
		msg.Reply = "[AuraGo] " + msg.Reply
		msg.State = "sending"
		if !m.save(msg) {
			return
		}
		state, sendErr := m.send(ctx, admitted, msg.Reply, true, c)
		msg.SendState = state
		msg.State = "completed"
		if state == "outcome_unknown" {
			msg.State = state
		}
		if sendErr != nil {
			msg.Reason = "reply_not_confirmed"
		}
		m.save(msg)
	}
}

// Recheck only re-admits quarantined/received input, never an already attempted action.
func (m *Manager) Recheck(id string) error {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.mu.Lock()
	enabled := m.cfg.Enabled
	q := m.queue
	m.mu.Unlock()
	if !enabled || q == nil {
		return fmt.Errorf("meshcore_disabled")
	}
	msg, err := m.store.get(id)
	if err != nil {
		return err
	}
	if msg.State != "quarantine" && msg.State != "received" {
		return fmt.Errorf("message_cannot_be_retried")
	}
	msg.State = "pending"
	if err = m.store.save(msg); err != nil {
		return err
	}
	select {
	case q <- msg:
		return nil
	default:
		msg.State = "received"
		m.store.save(msg)
		return fmt.Errorf("meshcore_queue_full")
	}
}
func (m *Manager) Send(ctx context.Context, kind, target, text string, index int) (string, error) {
	m.mu.Lock()
	c := m.conn
	cfg := m.cfg
	m.mu.Unlock()
	if c == nil || !cfg.Enabled {
		return "not_sent", fmt.Errorf("meshcore_not_connected")
	}
	st, err := m.refresh(ctx, c)
	if err != nil {
		return "not_sent", err
	}
	msg := Message{Direction: "outgoing", IdentityKey: st.IdentityKey, Kind: kind, Sender: strings.ToLower(target), Channel: index, ReceivedAt: time.Now().Unix(), State: "sending", Text: m.scrub(text)}
	for _, ch := range st.Channels {
		if ch.Index == index {
			msg.Binding = ch.Binding
		}
	}
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "not_sent", err
	}
	msg.ID = hex.EncodeToString(b)
	if _, err = m.store.insert(msg); err != nil {
		return "not_sent", err
	}
	state, err := m.send(ctx, msg, msg.Text, false, c)
	msg.SendState = state
	msg.State = "completed"
	if state == "outcome_unknown" {
		msg.State = state
	}
	m.save(msg)
	if err := m.store.prune(cfg); err != nil {
		m.issue("persistence", true)
	}
	return state, err
}
func (m *Manager) send(ctx context.Context, msg Message, text string, reply bool, c *companion) (string, error) {
	mode := sendAgent
	if reply {
		mode = sendReply
	}
	return m.sendMessage(ctx, msg, text, mode, c)
}

func (m *Manager) sendMessage(ctx context.Context, msg Message, text string, mode sendOrigin, c *companion) (string, error) {
	reply := mode == sendReply
	m.mu.Lock()
	cfg := m.cfg
	st := m.status
	m.mu.Unlock()
	allowed := func(cfg Config, st Status) bool {
		if !cfg.Enabled || st.State != "connected" || msg.IdentityKey != cfg.IdentityKey {
			return false
		}
		if msg.Kind == "direct" {
			contact, ok := uniqueContact(st, msg.Sender)
			if !ok || !ValidKey(msg.Sender) || contact.Key != msg.Sender {
				return false
			}
			if reply {
				return cfg.DirectReplies && slices.Contains(cfg.TrustedNodes, msg.Sender)
			}
			if mode == sendHuman {
				return contact.Type == 1 && contact.Key != st.IdentityKey
			}
			return cfg.ProactiveSend && slices.Contains(cfg.SendNodes, msg.Sender)
		}
		if msg.Kind != "channel" {
			return false
		}
		if mode == sendHuman {
			return channelMatches(st, ChannelRule{Index: msg.Channel, Binding: msg.Binding})
		}
		for _, r := range cfg.Channels {
			if r.Index == msg.Channel && r.Binding == msg.Binding && channelMatches(st, r) {
				if reply {
					return r.Mode == "prefix" || r.Mode == "questions"
				}
				return cfg.ProactiveSend && r.AllowSend
			}
		}
		return false
	}
	if !allowed(cfg, st) {
		return "not_sent", fmt.Errorf("meshcore_send_denied")
	}
	limit := 133
	if msg.Kind == "channel" {
		limit = min(limit, 160-max(len(st.Name), st.nameBytes)-2)
	}
	parts, err := splitText(text, limit)
	if err != nil {
		return "not_sent", err
	}
	chatID := msg.ID
	if reply {
		chatID += ":reply"
	}
	if err := m.prepareParts(chatID, msg.IdentityKey, c, parts); err != nil {
		return "not_sent", err
	}
	c.ackMu.Lock()
	c.onACK = func(tag uint32) { m.lateACK(c, tag) }
	c.ackMu.Unlock()
	state := "device_accepted"
	for i, part := range parts {
		if ctx.Err() != nil {
			if i == 0 {
				return "not_sent", ctx.Err()
			}
			return "outcome_unknown", ctx.Err()
		}
		if _, err := m.refresh(ctx, c); err != nil {
			if i > 0 {
				return "outcome_unknown", err
			}
			return "not_sent", err
		}
		m.mu.Lock()
		m.sends = slices.DeleteFunc(m.sends, func(t time.Time) bool { return time.Since(t) >= time.Minute })
		valid := m.conn == c && allowed(m.cfg, m.status) && len(m.sends) < 6
		if valid {
			m.sends = append(m.sends, time.Now())
		}
		m.mu.Unlock()
		if !valid {
			if i > 0 {
				return "outcome_unknown", fmt.Errorf("partial_send_interrupted")
			}
			return "not_sent", fmt.Errorf("meshcore_send_denied_or_rate_limited")
		}
		var cmd []byte
		expected := byte(0)
		if msg.Kind == "direct" {
			key, _ := hex.DecodeString(msg.Sender)
			cmd = make([]byte, 13)
			cmd[0] = 2
			copy(cmd[7:13], key[:6])
			expected = 6
		} else {
			cmd = make([]byte, 7)
			cmd[0] = 3
			cmd[2] = byte(msg.Channel)
		}
		binary.LittleEndian.PutUint32(cmd[3:7], uint32(time.Now().Unix()+int64(i)))
		cmd = append(cmd, part...)
		if err := m.recordPart(chatID, i+1, "sending", 0, c); err != nil {
			return "outcome_unknown", err
		}
		frames, err := func() ([][]byte, error) {
			select {
			case m.writeSlot <- struct{}{}:
				defer func() { <-m.writeSlot }()
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			m.mu.Lock()
			valid := m.conn == c && !m.editing && allowed(m.cfg, m.status)
			m.mu.Unlock()
			if !valid {
				return nil, fmt.Errorf("permission_changed")
			}
			return c.request(ctx, cmd, expected)
		}()
		if err != nil {
			_ = m.recordPart(chatID, i+1, "outcome_unknown", 0, c)
			return "outcome_unknown", err
		}
		if expected == 0 {
			if err := m.recordPart(chatID, i+1, "device_accepted", 0, c); err != nil {
				return "outcome_unknown", err
			}
		}
		if expected == 6 {
			if len(frames[0]) != 10 {
				return "outcome_unknown", fmt.Errorf("invalid send response")
			}
			tag := binary.LittleEndian.Uint32(frames[0][2:6])
			if err := m.recordPart(chatID, i+1, "device_accepted", tag, c); err != nil {
				return "outcome_unknown", err
			}
			timeout := time.Duration(binary.LittleEndian.Uint32(frames[0][6:10])) * time.Millisecond
			timeout = min(max(timeout, time.Second), 60*time.Second)
			confirmed := c.waitACK(ctx, tag, timeout)
			if !confirmed {
				state = "device_accepted"
			} else if i == 0 || state == "delivered" {
				state = "delivered"
			}
			if confirmed {
				if err := m.recordPart(chatID, i+1, "delivered", tag, c); err != nil {
					return "outcome_unknown", err
				}
			}
		}
	}
	return state, nil
}
func (c *companion) waitACK(ctx context.Context, tag uint32, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		c.ackMu.Lock()
		_, ok := c.acks[tag]
		if ok {
			delete(c.acks, tag)
		}
		c.ackMu.Unlock()
		if ok {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-c.done:
			return false
		case <-timer.C:
			return false
		case <-tick.C:
		}
	}
}
