package meshcore

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

var deviceKey = strings.Repeat("11", 32)
var nodeKey = strings.Repeat("22", 32)

// Byte fixtures follow the pinned MyMesh.cpp layouts, including 64-byte paths.
type testRadio struct {
	mu            sync.Mutex
	in            chan []byte
	done          chan struct{}
	once          sync.Once
	contacts      []Contact
	key           string
	channelSecret byte
	sent          [][]byte
	ack           bool
	nextMessages  [][]byte
}

func newTestRadio() *testRadio {
	return &testRadio{in: make(chan []byte, 64), done: make(chan struct{}), key: deviceKey, contacts: []Contact{{Key: nodeKey, Name: "Operator", Type: 1}}, channelSecret: 7, ack: true}
}
func (r *testRadio) ReadFrame() ([]byte, error) {
	select {
	case b := <-r.in:
		return b, nil
	case <-r.done:
		return nil, io.EOF
	}
}
func (r *testRadio) Close() error { r.once.Do(func() { close(r.done) }); return nil }
func (r *testRadio) WriteFrame(cmd []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	put := func(b []byte) { r.in <- b }
	switch cmd[0] {
	case 22:
		b := make([]byte, 82)
		b[0] = 13
		b[1] = 10
		b[2] = 10
		b[3] = 1
		copy(b[4:8], []byte("PIN!"))
		copy(b[60:80], "test-firmware")
		put(b)
	case 1:
		b := make([]byte, 64)
		b[0] = 5
		key, _ := hex.DecodeString(r.key)
		copy(b[4:36], key)
		copy(b[58:], "AuraGo")
		put(b)
	case 4:
		put([]byte{2, byte(len(r.contacts)), 0, 0, 0})
		for _, c := range r.contacts {
			b := make([]byte, 148)
			b[0] = 3
			k, _ := hex.DecodeString(c.Key)
			copy(b[1:33], k)
			b[33] = c.Type
			copy(b[100:132], c.Name)
			put(b)
		}
		put([]byte{4, 0, 0, 0, 0})
	case 31:
		b := make([]byte, 50)
		b[0] = 18
		b[1] = cmd[1]
		copy(b[2:34], "Public")
		for i := 34; i < 50; i++ {
			b[i] = r.channelSecret
		}
		put(b)
	case 10:
		if len(r.nextMessages) > 0 {
			put(r.nextMessages[0])
			r.nextMessages = r.nextMessages[1:]
		} else {
			put([]byte{10})
		}
	case 2:
		r.sent = append(r.sent, bytes.Clone(cmd))
		b := make([]byte, 10)
		b[0] = 6
		binary.LittleEndian.PutUint32(b[2:], uint32(len(r.sent)))
		binary.LittleEndian.PutUint32(b[6:], 1)
		if r.ack {
			put([]byte{0x82, byte(len(r.sent)), 0, 0, 0, 1, 0, 0, 0})
		}
		put(b)
	case 3:
		r.sent = append(r.sent, bytes.Clone(cmd))
		put([]byte{0})
	default:
		return fmt.Errorf("unexpected command: %d", cmd[0])
	}
	return nil
}
func testManager(t *testing.T, hooks Hooks) (*Manager, *testRadio, Config) {
	t.Helper()
	m, err := NewManager(context.Background(), t.TempDir(), hooks)
	if err != nil {
		t.Fatal(err)
	}
	r := newTestRadio()
	c := newCompanion(r)
	m.conn = c
	cfg := Config{Enabled: true, Port: "test", IdentityKey: deviceKey, TrustedNodes: []string{nodeKey}, DirectReplies: true}
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	m.cfg = cfg
	st, err := m.refresh(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Channels = []ChannelRule{{Index: 0, Binding: st.Channels[0].Binding, Mode: "prefix", Prefix: "!aura"}}
	m.cfg = cfg
	t.Cleanup(func() { m.Close() })
	return m, r, cfg
}
func directFrame(kind byte, typ byte, text string) []byte {
	b := make([]byte, 13)
	b[0] = 7
	k, _ := hex.DecodeString(nodeKey)
	copy(b[1:7], k[:6])
	b[7] = 0xff
	b[8] = typ
	binary.LittleEndian.PutUint32(b[9:], uint32(time.Now().Unix()))
	if typ == 2 {
		b = append(b, 1, 2, 3, 4)
	}
	b = append(b, text...)
	if kind == 16 {
		b = append([]byte{16, 40, 0, 0}, b[1:]...)
	}
	return b
}

type choppedIO struct{ *bytes.Buffer }

func (c choppedIO) Read(b []byte) (int, error)  { return c.Buffer.Read(b[:min(1, len(b))]) }
func (c choppedIO) Write(b []byte) (int, error) { return c.Buffer.Write(b[:min(2, len(b))]) }
func (c choppedIO) Close() error                { return nil }

func TestSerialFramingAndUTF8(t *testing.T) {
	buf := bytes.NewBuffer([]byte{'x', '>', 2, 0, 10, 11, '>', 1, 0, 12})
	s := &serialLink{choppedIO{buf}}
	for _, want := range [][]byte{{10, 11}, {12}} {
		got, err := s.ReadFrame()
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%v %v", got, err)
		}
	}
	for _, size := range []uint16{0, 177, 65535} {
		buf.Reset()
		buf.Write([]byte{'>', byte(size), byte(size >> 8)})
		if _, err := s.ReadFrame(); err == nil {
			t.Fatal("invalid size accepted")
		}
	}
	buf.Reset()
	if err := s.WriteFrame([]byte{1, 2, 3}); err != nil || !bytes.Equal(buf.Bytes(), []byte{'<', 3, 0, 1, 2, 3}) {
		t.Fatalf("partial write: %x %v", buf.Bytes(), err)
	}
	text := strings.Repeat("Grüße 🌍 ", 25)
	parts, err := splitText(text, 133)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, p := range parts {
		if len(p) > 133 || !utf8.ValidString(p) {
			t.Fatal(p)
		}
		joined += p[6:]
	}
	if joined != strings.TrimSpace(text) {
		t.Fatal("text truncated")
	}
	if _, err := splitText(strings.Repeat("a", 400), 133); err == nil {
		t.Fatal("four packets accepted")
	}
	for n := 1; n < 390; n++ {
		_, _ = splitText(strings.Repeat("é", n), 133)
	}
}
func TestCodecSnapshotAndAsyncEvents(t *testing.T) {
	m, r, _ := testManager(t, Hooks{})
	st := m.Status()
	if st.IdentityKey != deviceKey || st.Contacts[0].Key != nodeKey || st.Channels[0].Binding == "" {
		t.Fatal(st)
	}
	b, _ := json.Marshal(st)
	if bytes.Contains(b, []byte("PIN!")) || bytes.Contains(b, bytes.Repeat([]byte{7}, 16)) {
		t.Fatal("secret leak")
	}
	r.in <- []byte{0x83}
	if _, err := m.conn.request(context.Background(), []byte{10}, 10); err != nil {
		t.Fatal(err)
	}
	legacy, err := decodeMessage(directFrame(7, 0, "Hallo 🌍"), st)
	if err != nil {
		t.Fatal(err)
	}
	v3, err := decodeMessage(directFrame(16, 0, "Hallo 🌍"), st)
	if err != nil || legacy.ID != v3.ID {
		t.Fatalf("path/SNR duplicate not detected: %v", err)
	}
	for _, bad := range [][]byte{{16, 1}, directFrame(7, 0, "hidden\x00action"), directFrame(7, 0, "\xff"), make([]byte, 176)} {
		if _, err := decodeMessage(bad, st); err == nil {
			t.Fatalf("bad frame accepted: %x", bad)
		}
	}
	r.mu.Lock()
	r.channelSecret++
	r.mu.Unlock()
	changed, err := m.refresh(context.Background(), m.conn)
	if err != nil || changed.State != "binding_changed" {
		t.Fatalf("channel rebinding: %+v %v", changed, err)
	}
	r.mu.Lock()
	r.key = strings.Repeat("33", 32)
	r.mu.Unlock()
	changed, err = m.refresh(context.Background(), m.conn)
	if err != nil || changed.State != "binding_required" {
		t.Fatalf("device identity: %+v %v", changed, err)
	}
}
func TestAdmissionIsBoundToPlainDirectIdentity(t *testing.T) {
	m, _, cfg := testManager(t, Hooks{})
	st := m.Status()
	msg, _ := decodeMessage(directFrame(7, 0, "/reset"), st)
	if mode, got := admit(msg, cfg, st, time.Now()); mode != "trusted" || got.Sender != nodeKey {
		t.Fatalf("%s %+v", mode, got)
	}
	for _, change := range []func(*Message, *Status){
		func(m *Message, _ *Status) { m.TextType = 2 }, func(m *Message, _ *Status) { m.TextType = 1 }, func(m *Message, _ *Status) { m.Sender = "abababababab" },
		func(m *Message, _ *Status) { m.Timestamp -= 601 }, func(m *Message, _ *Status) { m.Timestamp += 121 }, func(m *Message, _ *Status) { m.IdentityKey = "changed" },
		func(_ *Message, s *Status) {
			s.Contacts = append(s.Contacts, Contact{Key: nodeKey[:12] + strings.Repeat("33", 26), Name: "Operator", Type: 1})
		},
		func(_ *Message, s *Status) { s.Contacts = []Contact{{Key: nodeKey, Name: "Operator", Type: 3}} },
	} {
		copyMsg, copySt := msg, st
		change(&copyMsg, &copySt)
		if mode, _ := admit(copyMsg, cfg, copySt, time.Now()); mode != "" {
			t.Fatal("untrusted direct admitted")
		}
	}
	msg = Message{IdentityKey: deviceKey, Kind: "channel", Channel: 0, Binding: cfg.Channels[0].Binding, Text: "Operator: !aura What is LoRa?"}
	if mode, got := admit(msg, cfg, st, time.Now()); mode != "prefix" || got.Text != "What is LoRa?" {
		t.Fatalf("%s %+v", mode, got)
	}
	for _, text := range []string{"Operator: !aurabot delete files", "AuraGo: !aura hello", "Operator: [AuraGo] answer", "Operator: [1/3] answer"} {
		msg.Text = text
		if mode, _ := admit(msg, cfg, st, time.Now()); mode != "" {
			t.Fatal("echo/invalid prefix admitted")
		}
	}
	msg.Text = "Operator: !aura delete files"
	if mode, _ := admit(msg, cfg, st, time.Now()); mode == "trusted" {
		t.Fatal("channel sender gained trust")
	}
}
func TestPersistentClaimQuarantineAndBoundReplies(t *testing.T) {
	scans, runs := 0, 0
	m, r, _ := testManager(t, Hooks{Scan: func(context.Context, Message) Review { scans++; return Review{Decision: "safe"} }, Run: func(_ context.Context, msg Message, mode string) (string, error) {
		runs++
		if mode != "trusted" || msg.Sender != nodeKey {
			t.Fatal("wrong run")
		}
		return "answer", nil
	}})
	msg, _ := decodeMessage(directFrame(7, 0, "hello"), m.Status())
	if ok, err := m.store.insert(msg); !ok || err != nil {
		t.Fatal(err)
	}
	m.process(context.Background(), msg)
	m.process(context.Background(), msg)
	if scans != 1 || runs != 1 {
		t.Fatalf("duplicate execution %d/%d", scans, runs)
	}
	got, err := m.store.get(msg.ID)
	if err != nil || got.SendState != "delivered" || got.State != "completed" {
		t.Fatalf("%+v %v", got, err)
	}
	r.mu.Lock()
	sent := bytes.Clone(r.sent[0])
	r.mu.Unlock()
	if !bytes.Equal(sent[7:13], bytes.Repeat([]byte{0x22}, 6)) {
		t.Fatal("reply destination changed")
	}
	if state, err := m.Send(context.Background(), "direct", nodeKey, "proactive", -1); err == nil || state != "not_sent" {
		t.Fatal("proactive sending allowed")
	}
	if err := m.Recheck(msg.ID); err == nil {
		t.Fatal("completed command retried")
	}
	m.hooks.Scan = func(context.Context, Message) Review { return Review{Decision: "suspicious", Reason: "bad_verdict"} }
	msg, _ = decodeMessage(directFrame(7, 0, "another"), m.Status())
	m.store.insert(msg)
	m.process(context.Background(), msg)
	got, _ = m.store.get(msg.ID)
	if got.State != "quarantine" || runs != 1 {
		t.Fatal("unsafe scan dispatched")
	}
	notice, ids, err := m.PendingNotice()
	if err != nil || len(ids) != 1 || strings.Contains(notice, "another") || strings.Contains(notice, "bad_verdict") {
		t.Fatalf("unsafe notice: %q %v", notice, err)
	}
	if err := m.MarkNotified(ids); err != nil {
		t.Fatal(err)
	}
	notice, _, _ = m.PendingNotice()
	if notice != "" {
		t.Fatal("notice repeated")
	}
}
func TestStoreCrashRecoveryAndRetention(t *testing.T) {
	dir := t.TempDir()
	s, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i, state := range []string{"pending", "processing", "sending", "completed"} {
		s.insert(Message{ID: fmt.Sprint(i), State: state, ReceivedAt: time.Now().Unix(), Text: "persisted"})
	}
	salt := bytes.Clone(s.salt)
	s.db.Close()
	s, err = openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	if !bytes.Equal(salt, s.salt) {
		t.Fatal("binding changed on restart")
	}
	for i, want := range []string{"received", "outcome_unknown", "outcome_unknown", "completed"} {
		msg, err := s.get(fmt.Sprint(i))
		if err != nil || msg.State != want {
			t.Fatalf("%d %s %v", i, msg.State, err)
		}
		if ok, _ := s.claim(msg.ID); ok {
			t.Fatal("replayed after crash")
		}
	}
	if ok, _ := s.insert(Message{ID: "0", State: "pending"}); ok {
		t.Fatal("duplicate persisted again")
	}
	if err := s.prune(Config{RetentionDays: 7, MaxMessages: 2}); err != nil {
		t.Fatal(err)
	}
	msgs, _ := s.list(100, 0)
	if len(msgs) != 2 {
		t.Fatal(len(msgs))
	}
}

func TestProactiveDestinationsAndDeliveryStates(t *testing.T) {
	m, radio, _ := testManager(t, Hooks{})
	m.cfg.ProactiveSend = true
	m.cfg.SendNodes = []string{nodeKey}
	m.cfg.Channels[0].AllowSend = true
	if state, err := m.Send(context.Background(), "direct", nodeKey, "direct", -1); err != nil || state != "delivered" {
		t.Fatalf("direct: %s %v", state, err)
	}
	if state, err := m.Send(context.Background(), "channel", "", "channel", 0); err != nil || state != "device_accepted" {
		t.Fatalf("channel: %s %v", state, err)
	}
	radio.mu.Lock()
	radio.ack = false
	radio.mu.Unlock()
	if state, err := m.Send(context.Background(), "direct", nodeKey, "unconfirmed", -1); err != nil || state != "device_accepted" {
		t.Fatalf("unconfirmed: %s %v", state, err)
	}
	if state, err := m.Send(context.Background(), "channel", "", "forbidden", 1); err == nil || state != "not_sent" {
		t.Fatal("unlisted channel allowed")
	}
	radio.mu.Lock()
	radio.contacts = append(radio.contacts, Contact{Key: nodeKey[:12] + strings.Repeat("33", 26), Type: 1})
	radio.mu.Unlock()
	if state, err := m.Send(context.Background(), "direct", nodeKey, "ambiguous", -1); err == nil || state != "not_sent" {
		t.Fatal("colliding prefix allowed")
	}
}
func TestRevocationCancelsActiveRun(t *testing.T) {
	started := make(chan struct{})
	m, _, cfg := testManager(t, Hooks{Scan: func(context.Context, Message) Review { return Review{Decision: "safe"} }, Run: func(ctx context.Context, _ Message, _ string) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}})
	msg, _ := decodeMessage(directFrame(7, 0, "do work"), m.Status())
	m.store.insert(msg)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	done := make(chan struct{})
	go func() { m.process(ctx, msg); close(done) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("not started")
	}
	m.Suspend()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("revocation did not cancel")
	}
	got, _ := m.store.get(msg.ID)
	if got.State != "outcome_unknown" {
		t.Fatal(got.State)
	}
	if m.allowRun("node", cfg, time.Now()) != true || m.allowRun("node", cfg, time.Now()) != true || m.allowRun("node", cfg, time.Now()) != false {
		t.Fatal("peer rate limit")
	}
}
func TestConfigDefaultsAndValidation(t *testing.T) {
	cfg := Config{}
	if err := cfg.Normalize(); err != nil || cfg.Enabled || cfg.ProactiveSend || cfg.MaxMessages != 1000 || cfg.PeerRunsPerMinute != 2 || cfg.MaxCommandAgeSeconds != 600 {
		t.Fatalf("%+v %v", cfg, err)
	}
	for _, cfg := range []Config{{Enabled: true}, {Transport: "tcp"}, {Enabled: true, Transport: "ble", Address: "bad"}, {TrustedNodes: []string{nodeKey[:12]}}, {TrustedNodes: []string{nodeKey, nodeKey}}, {IdentityKey: deviceKey, Channels: []ChannelRule{{Mode: "prefix", Index: 0}}}, {MaxMessages: -1}} {
		if err := cfg.Normalize(); err == nil {
			t.Fatalf("invalid config accepted: %+v", cfg)
		}
	}
}

func TestBLEBoundsAndExecutionTombstones(t *testing.T) {
	for _, tt := range []struct {
		mtu uint16
		n   int
		ok  bool
	}{{23, 20, false}, {175, 148, false}, {176, 148, true}, {176, 173, false}, {176, 174, false}, {247, 176, true}, {247, 177, false}, {176, 0, false}} {
		if got := validBLENotification(make([]byte, tt.n), tt.mtu); got != tt.ok {
			t.Fatalf("MTU %d frame %d: %v", tt.mtu, tt.n, got)
		}
	}
	m, _, _ := testManager(t, Hooks{})
	msg, _ := decodeMessage(directFrame(7, 0, "once"), m.Status())
	m.store.insert(msg)
	if ok, err := m.store.reserveExecution(msg.ID); err != nil || !ok {
		t.Fatal(err)
	}
	if _, err := m.store.db.Exec("DELETE FROM meshcore_messages WHERE id=?", msg.ID); err != nil {
		t.Fatal(err)
	}
	m.store.insert(msg)
	if ok, err := m.store.reserveExecution(msg.ID); err != nil || ok {
		t.Fatal("inbox eviction replayed command", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.refreshSlot <- struct{}{}
	if _, err := m.Test(ctx); err == nil {
		t.Fatal("test ignored cancellation")
	}
	<-m.refreshSlot
}

func TestReconnectDrainsAndDeduplicates(t *testing.T) {
	var scans, opens atomic.Int32
	m, err := NewManager(context.Background(), t.TempDir(), Hooks{Scan: func(context.Context, Message) Review { scans.Add(1); return Review{Decision: "safe"} }})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	frame := directFrame(7, 0, "untrusted notification")
	first, second := newTestRadio(), newTestRadio()
	first.nextMessages = [][]byte{frame}
	second.nextMessages = [][]byte{frame}
	m.open = func(context.Context, Config, bool) (frameLink, error) {
		switch opens.Add(1) {
		case 1:
			return nil, fmt.Errorf("offline")
		case 2:
			return first, nil
		default:
			return second, nil
		}
	}
	cfg := Config{Enabled: true, Port: "fixture", IdentityKey: deviceKey}
	if err := m.Configure(cfg, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for scans.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if scans.Load() != 1 {
		t.Fatal("reconnect did not drain")
	}
	first.Close()
	for (opens.Load() < 3 || m.Status().State != "connected") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if opens.Load() < 3 || scans.Load() != 1 {
		t.Fatalf("reconnect/duplicate: opens=%d scans=%d", opens.Load(), scans.Load())
	}
	msgs, err := m.Messages(100, 0)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("duplicate inbox: %d %v", len(msgs), err)
	}
}
