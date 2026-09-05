package meshcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func waitManual(t *testing.T, m *Manager, id string) Message {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := m.store.get(id)
		if err == nil && msg.State != "sending" {
			return msg
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("manual send did not finish")
	return Message{}
}

func TestMessengerManualSendIsIdempotentAndIndependent(t *testing.T) {
	m, r, cfg := testManager(t, Hooks{})
	if cfg.ProactiveSend {
		t.Fatal("fixture must prohibit agent sending")
	}
	conversation := conversationID(deviceKey, "direct", nodeKey)
	id, err := m.SendManual(context.Background(), "request-1234567890", conversation, "Grüße 👋")
	if err != nil {
		t.Fatal(err)
	}
	if again, err := m.SendManual(context.Background(), "request-1234567890", conversation, "Grüße 👋"); err != nil || again != id {
		t.Fatalf("duplicate: %s %v", again, err)
	}
	if _, err := m.SendManual(context.Background(), "request-1234567890", conversation, "different"); err == nil {
		t.Fatal("changed idempotency payload accepted")
	}
	msg := waitManual(t, m, id)
	if msg.SendState != "delivered" {
		t.Fatalf("state: %+v", msg)
	}
	r.mu.Lock()
	count := len(r.sent)
	r.mu.Unlock()
	if count != 1 {
		t.Fatalf("sent %d times", count)
	}
	if state, err := m.Send(context.Background(), "direct", nodeKey, "agent", -1); err == nil || state != "not_sent" {
		t.Fatalf("agent bypassed permission: %s %v", state, err)
	}
	rows, err := m.ChatMessages(conversation, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.ID == id {
			found = true
			if row.Origin != "manual" || row.SendState != "delivered" || len(row.Parts) != 1 {
				t.Fatalf("history: %+v", row)
			}
		}
	}
	if !found {
		t.Fatal("missing manual history")
	}
}

func TestMessengerProtectedHistoryAndReadState(t *testing.T) {
	m, _, cfg := testManager(t, Hooks{})
	msg := Message{ID: strings.Repeat("a", 64), Kind: "direct", Sender: nodeKey[:12], PeerKey: nodeKey, IdentityKey: deviceKey, Channel: -1, Direction: "incoming", Text: "<script>private</script>", ReceivedAt: time.Now().Unix(), State: "quarantine", Review: "suspicious"}
	if _, err := m.store.insert(msg); err != nil {
		t.Fatal(err)
	}
	id := conversationID(deviceKey, "direct", nodeKey)
	rows, err := m.ChatMessages(id, 0, "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("history %v %v", rows, err)
	}
	if !rows[0].Protected || rows[0].Text != "" {
		t.Fatal("unsafe text leaked into history")
	}
	text, err := m.RevealMessage(msg.ID)
	if err != nil || text != msg.Text {
		t.Fatal("explicit reveal failed")
	}
	if err = m.UpdateConversation(id, rows[0].Seq, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	var notified int
	m.store.db.QueryRow("SELECT agent_notified FROM meshcore_messages WHERE id=?", msg.ID).Scan(&notified)
	if notified != 0 {
		t.Fatal("reading acknowledged agent notice")
	}
	msg.Review = "safe"
	msg.State = "received"
	if err = m.store.save(msg); err != nil {
		t.Fatal(err)
	}
	if _, err = m.store.reserveExecution(msg.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.UpdateConversation(id, 0, nil, nil, true); err != nil {
		t.Fatal(err)
	}
	if err = m.store.save(msg); err != nil {
		t.Fatal(err)
	}
	rows, err = m.ChatMessages(id, 0, "")
	if err != nil || len(rows) != 0 {
		t.Fatal("cleared history resurrected")
	}
	if ok, err := m.store.reserveExecution(msg.ID); err != nil || ok {
		t.Fatal("history clear lost execution reservation")
	}
	// Security inbox expiry does not erase approved messenger history.
	msg.ID = strings.Repeat("b", 64)
	msg.ReceivedAt = time.Now().Add(-8 * 24 * time.Hour).Unix()
	if _, err = m.store.insert(msg); err != nil {
		t.Fatal(err)
	}
	if err = m.store.prune(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err = m.store.get(msg.ID); err == nil {
		t.Fatal("security retention ignored")
	}
	rows, err = m.ChatMessages(id, 0, "")
	if err != nil || len(rows) != 1 || rows[0].Text != msg.Text {
		t.Fatalf("history retention: %v %v", rows, err)
	}
}

func TestMessengerMigrationKeepsUnknownPrefixesAndCrashState(t *testing.T) {
	dir := t.TempDir()
	s, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	msg := Message{ID: "legacy", IdentityKey: deviceKey, Kind: "direct", Sender: nodeKey[:12], Direction: "incoming", Text: "hello", Review: "safe", State: "sending", Reply: "answer", ReceivedAt: time.Now().Unix()}
	b, _ := json.Marshal(msg)
	if _, err = s.db.Exec("INSERT INTO meshcore_messages(id,received,state,data,binding) VALUES(?,?,?,?,?)", msg.ID, msg.ReceivedAt, msg.State, b, ""); err != nil {
		t.Fatal(err)
	}
	s.db.Exec("DROP TABLE meshcore_chat; DROP TABLE meshcore_conversations; DROP TABLE meshcore_send_parts; DROP TABLE meshcore_send_keys; UPDATE meshcore_meta SET value='1' WHERE key='version'")
	s.db.Close()
	s, err = openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	backups, _ := filepath.Glob(filepath.Join(dir, "*.backup.db"))
	if len(backups) != 1 {
		t.Fatal("migration has no backup")
	}
	if info, err := os.Stat(backups[0]); err != nil || info.Size() == 0 {
		t.Fatal("empty backup")
	}
	m := &Manager{store: s}
	rows, err := m.ChatMessages(conversationID(deviceKey, "unknown", nodeKey[:12]), 0, "")
	if err != nil || len(rows) != 2 {
		t.Fatalf("migration %v %v", rows, err)
	}
	for _, row := range rows {
		if row.ID == "legacy:reply" && row.SendState != "outcome_unknown" {
			t.Fatalf("crash state: %+v", row)
		}
	}
}

type editingRadio struct {
	*testRadio
	channels map[byte][]byte
	reject   bool
}

func (r *editingRadio) WriteFrame(cmd []byte) error {
	switch cmd[0] {
	case 22:
		b := make([]byte, 82)
		b[0] = 13
		b[1] = 10
		b[3] = 2
		r.in <- b
		return nil
	case 31:
		b := make([]byte, 50)
		b[0] = 18
		b[1] = cmd[1]
		if old := r.channels[cmd[1]]; old != nil {
			copy(b[2:], old[2:])
		}
		r.in <- b
		return nil
	case 32:
		if r.reject {
			r.in <- []byte{1, 3}
			return nil
		}
		r.channels[cmd[1]] = append([]byte{}, cmd...)
		r.in <- []byte{0}
		return nil
	case 9:
		if r.reject {
			r.in <- []byte{1, 3}
			return nil
		}
		r.mu.Lock()
		r.contacts = append(r.contacts, Contact{Key: hex.EncodeToString(cmd[1:33]), Name: wireText(cmd[100:132]), Type: cmd[33]})
		r.mu.Unlock()
		r.in <- []byte{0}
		return nil
	case 15:
		r.mu.Lock()
		key := hex.EncodeToString(cmd[1:])
		contacts := []Contact{}
		for _, c := range r.contacts {
			if c.Key != key {
				contacts = append(contacts, c)
			}
		}
		r.contacts = contacts
		r.mu.Unlock()
		r.in <- []byte{0}
		return nil
	case 7:
		r.in <- []byte{0}
		return nil
	}
	return r.testRadio.WriteFrame(cmd)
}

func TestMessengerChannelAndContactAdministration(t *testing.T) {
	m, _, cfg := testManager(t, Hooks{})
	m.conn.Close()
	r := &editingRadio{testRadio: newTestRadio(), channels: map[byte][]byte{}}
	m.conn = newCompanion(r)
	cfg.Channels = nil
	m.cfg = cfg
	if _, err := m.refresh(context.Background(), m.conn); err != nil {
		t.Fatal(err)
	}
	publish := func(c Config) error { cfg = c; return nil }
	req := EditRequest{Action: "channel_add", Identity: deviceKey, Name: "Team", Kind: "private", Secret: strings.Repeat("ab", 16)}
	if err := m.Edit(context.Background(), req, publish); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Channels) != 1 || cfg.Channels[0].Mode != "receive" || cfg.Channels[0].AllowSend {
		t.Fatal("channel inherited agent rights")
	}
	st := m.Status()
	id := conversationID(deviceKey, "channel", st.Channels[0].Binding)
	invite, err := m.Invitation(context.Background(), deviceKey, id)
	if err != nil || !strings.Contains(invite, "secret=") {
		t.Fatalf("invite %v", err)
	}
	public, _ := json.Marshal(st)
	if strings.Contains(string(public), req.Secret) {
		t.Fatal("channel secret in status")
	}
	parsed := EditRequest{Action: "channel_add", Invitation: invite}
	if err = parseInvitation(&parsed); err != nil || parsed.Name != "Team" || parsed.Secret != req.Secret {
		t.Fatal("invitation not interoperable")
	}
	if err = m.Edit(context.Background(), EditRequest{Action: "channel_remove", Identity: deviceKey, Conversation: id}, publish); err != nil {
		t.Fatal(err)
	}
	if len(m.Status().Channels) != 0 || len(cfg.Channels) != 0 {
		t.Fatal("channel removal not reconciled")
	}
	if err = m.Edit(context.Background(), EditRequest{Action: "contact_remove", Identity: deviceKey, Conversation: conversationID(deviceKey, "direct", nodeKey)}, publish); err != nil {
		t.Fatal(err)
	}
	if len(cfg.TrustedNodes) != 0 {
		t.Fatal("removed contact retains trust")
	}
	r.reject = true
	if err = m.Edit(context.Background(), EditRequest{Action: "contact_add", Identity: deviceKey, Key: nodeKey, Name: "Node", Type: 1}, publish); err == nil {
		t.Fatal("rejected edit succeeded")
	}
	if m.Status().State != "binding_changed" {
		t.Fatal("failed edit was not locked")
	}
	if err = m.Edit(context.Background(), EditRequest{Action: "confirm_mapping", Identity: deviceKey}, publish); err != nil {
		t.Fatal(err)
	}
	r.reject = false
	if err = m.Edit(context.Background(), EditRequest{Action: "contact_add", Identity: deviceKey, Key: nodeKey, Name: "Node", Type: 1}, publish); err != nil {
		t.Fatal(err)
	}
	if len(cfg.TrustedNodes) != 0 || len(cfg.SendNodes) != 0 {
		t.Fatal("reimport restored trust")
	}
	link, err := m.Invitation(context.Background(), deviceKey, conversationID(deviceKey, "direct", nodeKey))
	if err != nil {
		t.Fatal(err)
	}
	contact := EditRequest{Action: "contact_add", Invitation: link}
	if err = parseInvitation(&contact); err != nil || contact.Key != nodeKey || contact.Type != 1 {
		t.Fatal("contact export/import")
	}
	for _, kind := range []string{"public", "hashtag"} {
		name := "Public"
		if kind == "hashtag" {
			name = "#local"
		}
		if err = m.Edit(context.Background(), EditRequest{Action: "channel_add", Identity: deviceKey, Name: name, Kind: kind}, publish); err != nil {
			t.Fatal(err)
		}
	}
	if err = m.Edit(context.Background(), req, publish); err == nil || err.Error() != "channels_full" {
		t.Fatal("occupied slot overwritten")
	}
	if _, err = m.Invitation(context.Background(), deviceKey, id); err == nil {
		t.Fatal("stale channel invitation exported")
	}
	for _, flood := range []bool{false, true} {
		if err = m.Edit(context.Background(), EditRequest{Action: "advert", Identity: deviceKey, Flood: flood}, publish); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMessengerQueuedInputAcrossDeviceEditStaysUnbound(t *testing.T) {
	m, r, cfg := testManager(t, Hooks{})
	if _, err := m.store.db.Exec("INSERT INTO meshcore_meta(key,value) VALUES('input_binding_uncertain','1')"); err != nil {
		t.Fatal(err)
	}
	r.nextMessages = [][]byte{directFrame(7, 0, "queued before channel/contact edit")}
	queue := make(chan Message, 128)
	if err := m.receiveBatch(context.Background(), m.conn, queue); err != nil {
		t.Fatal(err)
	}
	msg := <-queue
	if !msg.BindingUncertain || msg.PeerKey != "" || messageConversation(msg).Kind != "unknown" {
		t.Fatal("ambiguous backlog assigned to current contact")
	}
	if mode, _ := admit(msg, cfg, m.Status(), time.Now()); mode != "" {
		t.Fatal("ambiguous backlog authorized")
	}
	r.nextMessages = [][]byte{directFrame(7, 0, "fresh after drain")}
	if err := m.receiveBatch(context.Background(), m.conn, queue); err != nil {
		t.Fatal(err)
	}
	msg = <-queue
	if msg.BindingUncertain || msg.PeerKey != nodeKey {
		t.Fatal("fresh input failed to bind after empty queue")
	}
	msg.PeerKey = nodeKey[:12] + strings.Repeat("ff", 26)
	if mode, _ := admit(msg, cfg, m.Status(), time.Now()); mode != "" {
		t.Fatal("historical prefix retargeted to another full key")
	}
	msg.PeerKey = ""
	if mode, _ := admit(msg, cfg, m.Status(), time.Now()); mode != "" {
		t.Fatal("historical unresolved prefix acquired trust")
	}
}

func TestMessengerInvitationsRejectUnsupportedAndAmbiguousInput(t *testing.T) {
	for _, link := range []string{"https://example.com", "meshcore://channel/add?name=Test", "meshcore://channel/add?name=Test&secret=" + strings.Repeat("ab", 16) + "&region_scope=test", "meshcore://contact/add?name=A&public_key=" + nodeKey + "&type=1&type=2"} {
		r := EditRequest{Action: "channel_add", Invitation: link}
		if strings.Contains(link, "contact") {
			r.Action = "contact_add"
		}
		if parseInvitation(&r) == nil {
			t.Fatalf("accepted %q", link)
		}
	}
}

func TestMessengerHistoryCursorAndClearedRetention(t *testing.T) {
	m, _, cfg := testManager(t, Hooks{})
	conv := conversationID(deviceKey, "direct", nodeKey)
	for i := 0; i < 110; i++ {
		msg := Message{ID: fmt.Sprintf("history-%03d", i), Direction: "incoming", Kind: "direct", Sender: nodeKey[:12], PeerKey: nodeKey, IdentityKey: deviceKey, Text: fmt.Sprintf("row %d", i), Review: "safe", State: "received", ReceivedAt: time.Now().Add(-48*time.Hour).Unix() + int64(i)}
		if _, err := m.store.insert(msg); err != nil {
			t.Fatal(err)
		}
	}
	first, err := m.ChatMessages(conv, 0, "")
	if err != nil || len(first) != 50 {
		t.Fatalf("first page: %d %v", len(first), err)
	}
	second, err := m.ChatMessages(conv, first[len(first)-1].Seq, "")
	if err != nil || len(second) != 50 || second[0].Seq >= first[len(first)-1].Seq {
		t.Fatal("unstable cursor")
	}
	query, err := m.ChatMessages(conv, 0, "row 109")
	if err != nil || len(query) != 1 {
		t.Fatal("history search")
	}
	if err = m.UpdateConversation(conv, 0, nil, nil, true); err != nil {
		t.Fatal(err)
	}
	cfg.HistoryDays = 1
	cfg.HistoryMessages = 1
	if err = m.store.pruneChat(cfg); err != nil {
		t.Fatal(err)
	}
	msg, err := m.store.get("history-109")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.store.save(msg); err != nil {
		t.Fatal(err)
	}
	rows, err := m.ChatMessages(conv, 0, "")
	if err != nil || len(rows) != 0 {
		t.Fatal("retention/recheck restored cleared history")
	}
	var text string
	if err = m.store.db.QueryRow("SELECT text FROM meshcore_chat WHERE id=?", msg.ID).Scan(&text); err != nil || text != "" {
		t.Fatal("cleared text retained in chat")
	}
	msg.ID = "fresh-history"
	msg.ReceivedAt = time.Now().Unix()
	if _, err = m.store.insert(msg); err != nil {
		t.Fatal(err)
	}
	if err = m.store.pruneChat(cfg); err != nil {
		t.Fatal(err)
	}
	rows, err = m.ChatMessages(conv, 0, "")
	if err != nil || len(rows) != 1 || rows[0].ID != msg.ID {
		t.Fatal("new history hidden by clear watermark")
	}
}

func TestMessengerCrashReservationDoesNotRetransmit(t *testing.T) {
	dir := t.TempDir()
	s, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	conv := conversationID(deviceKey, "direct", nodeKey)
	requestID := "crash-request-123456"
	id := "manual-" + requestID
	msg := Message{ID: id, Direction: "outgoing", Origin: "manual", Kind: "direct", Sender: nodeKey, PeerKey: nodeKey, IdentityKey: deviceKey, Text: "hello", State: "sending", SendState: "sending", ReceivedAt: time.Now().Unix()}
	if _, err = s.insert(msg); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte(conv + "\x00hello"))
	if _, err = s.db.Exec("INSERT INTO meshcore_send_keys(id,fingerprint,created) VALUES(?,?,?)", id, hex.EncodeToString(h[:]), msg.ReceivedAt); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO meshcore_send_parts(message,number,identity,session,created,state) VALUES(?,1,?,'crashed',?,'sending')", id, deviceKey, msg.ReceivedAt); err != nil {
		t.Fatal(err)
	}
	s.db.Close()
	m, err := NewManager(context.Background(), dir, Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	// No connection exists: an idempotent retry must only return its original ID.
	if got, err := m.SendManual(context.Background(), requestID, conv, "hello"); err != nil || got != id {
		t.Fatalf("restart retry: %s %v", got, err)
	}
	rows, err := m.ChatMessages(conv, 0, "")
	if err != nil || len(rows) != 1 || rows[0].SendState != "outcome_unknown" || rows[0].Parts[0].State != "outcome_unknown" {
		t.Fatalf("crash state: %+v %v", rows, err)
	}
}

func TestMessengerLateAcknowledgementUpdatesHistory(t *testing.T) {
	m, r, _ := testManager(t, Hooks{})
	r.ack = false
	conv := conversationID(deviceKey, "direct", nodeKey)
	id, err := m.SendManual(context.Background(), "late-123456789012", conv, "Late ACK")
	if err != nil {
		t.Fatal(err)
	}
	if msg := waitManual(t, m, id); msg.SendState != "device_accepted" {
		t.Fatal(msg.SendState)
	}
	r.in <- []byte{0x82, 1, 0, 0, 0, 1, 0, 0, 0}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		rows, err := m.ChatMessages(conv, 0, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 1 && rows[0].SendState == "delivered" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("late ACK not reflected")
}
