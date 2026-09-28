package meshcore

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type deviceRadio struct {
	*testRadio
	settingsMu sync.Mutex
	values     DeviceValues
	clock      uint32
	reject     byte
	ignore     byte
	version    byte
	commands   []byte
	diagnostic bool
	tag        uint32
}

func newDeviceRadio() *deviceRadio {
	b, f, h := byte(2), false, byte(0)
	return &deviceRadio{testRadio: newTestRadio(), version: 10, tag: 81, diagnostic: true, clock: uint32(time.Now().Unix()), values: DeviceValues{Name: "AuraGo", Latitude: 51, Longitude: 7, FrequencyKHz: 869525, BandwidthHz: 250000, SpreadingFactor: 11, CodingRate: 5, TxPower: 22, AutoAddMask: &b, AutoAddMaxHops: &h, Repeat: &f, PathHashMode: &h}}
}
func (r *deviceRadio) ReadFrame() ([]byte, error) {
	b, err := r.testRadio.ReadFrame()
	if err != nil {
		return b, err
	}
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	v := r.values
	if b[0] == 5 {
		b = append(b[:58:58], v.Name...)
		b[2] = byte(v.TxPower)
		b[3] = 30
		binary.LittleEndian.PutUint32(b[36:], uint32(int32(v.Latitude*1e6)))
		binary.LittleEndian.PutUint32(b[40:], uint32(int32(v.Longitude*1e6)))
		b[44] = v.MultiACKs
		b[45] = v.AdvertLocationPolicy
		b[46] = v.TelemetryBase | v.TelemetryLocation<<2 | v.TelemetryEnvironment<<4
		b[47] = v.ManualAddContacts
		binary.LittleEndian.PutUint32(b[48:], v.FrequencyKHz)
		binary.LittleEndian.PutUint32(b[52:], v.BandwidthHz)
		b[56] = v.SpreadingFactor
		b[57] = v.CodingRate
	}
	if b[0] == 13 {
		b[1] = r.version
		if r.version < 9 {
			b = b[:80]
		} else {
			if *v.Repeat {
				b[80] = 1
			}
			b[81] = *v.PathHashMode
		}
	}
	return b, nil
}
func (r *deviceRadio) WriteFrame(cmd []byte) error {
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	r.commands = append(r.commands, cmd[0])
	if cmd[0] == r.reject {
		r.in <- []byte{1, 1}
		return nil
	}
	if cmd[0] == r.ignore {
		r.in <- []byte{0}
		return nil
	}
	put := func(b []byte) { r.in <- b }
	v := &r.values
	switch cmd[0] {
	case 5:
		b := make([]byte, 5)
		b[0] = 9
		binary.LittleEndian.PutUint32(b[1:], r.clock)
		put(b)
	case 6:
		r.clock = binary.LittleEndian.Uint32(cmd[1:])
		put([]byte{0})
	case 8:
		v.Name = string(cmd[1:])
		put([]byte{0})
	case 14:
		v.Latitude = float64(int32(binary.LittleEndian.Uint32(cmd[1:]))) / 1e6
		v.Longitude = float64(int32(binary.LittleEndian.Uint32(cmd[5:]))) / 1e6
		put([]byte{0})
	case 11:
		v.FrequencyKHz = binary.LittleEndian.Uint32(cmd[1:])
		v.BandwidthHz = binary.LittleEndian.Uint32(cmd[5:])
		v.SpreadingFactor = cmd[9]
		v.CodingRate = cmd[10]
		if len(cmd) > 11 {
			f := cmd[11] != 0
			v.Repeat = &f
		}
		put([]byte{0})
	case 12:
		v.TxPower = int8(cmd[1])
		put([]byte{0})
	case 38:
		v.ManualAddContacts = cmd[1]
		v.TelemetryBase = cmd[2] & 3
		v.TelemetryLocation = cmd[2] >> 2 & 3
		v.TelemetryEnvironment = cmd[2] >> 4 & 3
		v.AdvertLocationPolicy = cmd[3]
		if len(cmd) > 4 {
			v.MultiACKs = cmd[4]
		}
		put([]byte{0})
	case 58:
		mask := cmd[1]
		v.AutoAddMask = &mask
		if len(cmd) > 2 {
			n := cmd[2]
			v.AutoAddMaxHops = &n
		}
		put([]byte{0})
	case 59:
		put([]byte{25, *v.AutoAddMask, *v.AutoAddMaxHops})
	case 60:
		b := make([]byte, 9)
		b[0] = 26
		binary.LittleEndian.PutUint32(b[1:], 869000)
		binary.LittleEndian.PutUint32(b[5:], 870000)
		put(b)
	case 61:
		n := cmd[2]
		v.PathHashMode = &n
		put([]byte{0})
	case 20:
		put([]byte{12, 0x74, 0x0e, 1, 0, 0, 0, 8, 0, 0, 0})
	case 56:
		lengths := []int{11, 14, 30}
		b := make([]byte, lengths[cmd[1]])
		b[0] = 24
		b[1] = cmd[1]
		if cmd[1] == 1 {
			b[4] = 190
			b[5] = 232
		}
		put(b)
	case 50, 52:
		b := make([]byte, 10)
		b[0] = 6
		b[1] = 1
		binary.LittleEndian.PutUint32(b[2:], r.tag)
		binary.LittleEndian.PutUint32(b[6:], 60000)
		if r.diagnostic {
			p := []byte{0x8c, 0, 0, 0, 0, 0, 1, 116, 1, 144, 1, 103, 0xff, 0xce}
			binary.LittleEndian.PutUint32(p[2:], r.tag)
			if cmd[0] == 52 {
				key, _ := hex.DecodeString(nodeKey)
				p = append([]byte{0x8d, 0}, key[:6]...)
				p = append(p, 0x42, 1, 2, 3, 4, 0x81, 5, 6, 7)
			}
			put(p) // An asynchronous reply may beat RESP_SENT.
		}
		put(b)
		r.tag++
	default:
		return r.testRadio.WriteFrame(cmd)
	}
	return nil
}
func deviceManager(t *testing.T) (*Manager, *deviceRadio) {
	m, _, cfg := testManager(t, Hooks{})
	m.conn.Close()
	r := newDeviceRadio()
	m.conn = newCompanion(r)
	cfg.AllowDeviceSettings = true
	cfg.AllowRemoteDiagnostics = true
	m.cfg = cfg
	return m, r
}
func TestDeviceSettingsReadbackConflictAndRecovery(t *testing.T) {
	m, r := deviceManager(t)
	ctx := context.Background()
	d, err := m.Device(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.Local.Groups["radio"].Values["last_snr_db"] != -6 {
		t.Fatal(d.Local)
	}
	before := len(r.commands)
	if _, err = m.Device(ctx); err != nil {
		t.Fatal(err)
	}
	for _, c := range r.commands[before:] {
		if c == 20 || c == 56 {
			t.Fatal("local polling bypassed 30s cache")
		}
	}
	values := d.Values
	values.Name = "New name"
	values.Latitude = 52.123456
	next, err := m.SaveDeviceSettings(ctx, DeviceSettingsRequest{deviceKey, d.Revision, "identity", values})
	if err != nil || next.Values.Name != "New name" || next.Values.Latitude != values.Latitude {
		t.Fatalf("save: %+v %v", next, err)
	}
	for _, c := range r.commands {
		if c == 7 {
			t.Fatal("position save advertised")
		}
	}
	if _, err = m.SaveDeviceSettings(ctx, DeviceSettingsRequest{deviceKey, d.Revision, "identity", values}); err == nil || err.Error() != "settings_conflict" {
		t.Fatalf("stale edit: %v", err)
	}
	if _, err = m.SaveDeviceSettings(ctx, DeviceSettingsRequest{nodeKey, next.Revision, "identity", values}); err == nil {
		t.Fatal("wrong identity")
	}
	r.settingsMu.Lock()
	r.reject = 14
	r.settingsMu.Unlock()
	values = next.Values
	values.Name = "Partial"
	values.Latitude = 53
	partial, err := m.SaveDeviceSettings(ctx, DeviceSettingsRequest{deviceKey, next.Revision, "identity", values})
	if err == nil || partial.Values.Name != "Partial" || partial.Values.Latitude == 53 || partial.Status.State != "settings_uncertain" {
		t.Fatalf("partial: %+v %v", partial, err)
	}
	channels := cloneRules(m.cfg.Channels)
	r.settingsMu.Lock()
	r.reject = 0
	r.settingsMu.Unlock()
	actual, err := m.Device(ctx)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := m.SaveDeviceSettings(ctx, DeviceSettingsRequest{deviceKey, actual.Revision, "reconcile", DeviceValues{}})
	if err != nil || resolved.Status.State != "connected" || !reflect.DeepEqual(channels, m.cfg.Channels) {
		t.Fatalf("reconcile: %v %v", resolved.Status.State, err)
	}
	m.mu.Lock()
	m.cfg.AllowDeviceSettings = false
	m.mu.Unlock()
	if _, err = m.SaveDeviceSettings(ctx, DeviceSettingsRequest{deviceKey, resolved.Revision, "clock", DeviceValues{}}); err == nil || err.Error() != "permission_denied" {
		t.Fatal(err)
	}
}
func cloneRules(r []ChannelRule) []ChannelRule { return append([]ChannelRule(nil), r...) }

func TestDeviceSettingsMismatchUnsupportedAndValidation(t *testing.T) {
	m, r := deviceManager(t)
	ctx := context.Background()
	d, err := m.Device(ctx)
	if err != nil {
		t.Fatal(err)
	}
	v := d.Values
	v.FrequencyKHz = 100
	before := len(r.commands)
	if _, _, err = deviceCommands(d, "radio", v); err == nil || len(r.commands) != before {
		t.Fatal("invalid frequency accepted")
	}
	v = d.Values
	v.Name = "Ignored"
	r.ignore = 8
	next, err := m.SaveDeviceSettings(ctx, DeviceSettingsRequest{deviceKey, d.Revision, "identity", v})
	if err == nil || next.Status.State != "settings_uncertain" {
		t.Fatal("unverified write accepted")
	}
	r.ignore = 0
	r.reject = 59
	d, err = m.Device(ctx)
	if err != nil || d.Features["auto_add"] != "unsupported" {
		t.Fatalf("optional failure: %v %v", d.Features, err)
	}
	if _, err = m.conn.request(ctx, []byte{4}, 4); err != nil {
		t.Fatalf("unsupported broke messaging: %v", err)
	}
}

func waitDiagnostic(t *testing.T, m *Manager, id string) Diagnostic {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		d, err := m.Diagnostic(id)
		if err != nil {
			t.Fatal(err)
		}
		if d.State != "pending" {
			return d
		}
		time.Sleep(time.Millisecond * 5)
	}
	t.Fatal("diagnostic did not complete")
	return Diagnostic{}
}
func TestDiagnosticsCorrelationGatesPathsAndTimeout(t *testing.T) {
	m, r := deviceManager(t)
	ctx := context.Background()
	for _, kind := range []string{"telemetry", "path"} {
		d, err := m.StartDiagnostic(ctx, DiagnosticRequest{deviceKey, nodeKey, kind})
		if err != nil {
			t.Fatal(err)
		}
		result := waitDiagnostic(t, m, d.ID)
		if result.State != "completed" {
			t.Fatalf("result: %+v", result)
		}
		if kind == "telemetry" && (result.Telemetry[0].Values[0] != 4 || result.Telemetry[1].Values[0] != -5) {
			t.Fatal(result.Telemetry)
		}
		if kind == "path" && (*result.Outgoing.HashBytes != 2 || *result.Incoming.HashBytes != 3) {
			t.Fatal(result)
		}
	}
	select {
	case <-m.conn.done:
	default:
		t.Fatal("successful path query reused an untagged session")
	}
	m, r = deviceManager(t)
	r.settingsMu.Lock()
	r.diagnostic = false
	r.settingsMu.Unlock()
	d, err := m.StartDiagnostic(ctx, DiagnosticRequest{deviceKey, nodeKey, "telemetry"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.StartDiagnostic(ctx, DiagnosticRequest{deviceKey, nodeKey, "path"}); err == nil || err.Error() != "busy" {
		t.Fatal("concurrent diagnosis accepted")
	}
	r.in <- []byte{0x8c, 0, 0, 0, 0, 0, 1, 116, 1, 144}
	time.Sleep(20 * time.Millisecond)
	if result, _ := m.Diagnostic(d.ID); result.State != "pending" {
		t.Fatal("foreign tag accepted")
	}
	m.conn.Close()
	waitDiagnostic(t, m, d.ID)
	m.mu.Lock()
	m.cfg.AllowRemoteDiagnostics = false
	m.mu.Unlock()
	if _, err = m.StartDiagnostic(ctx, DiagnosticRequest{deviceKey, nodeKey, "telemetry"}); err == nil || err.Error() != "permission_denied" {
		t.Fatal(err)
	}
	// A path timeout closes the session; old untagged pushes cannot reach a retry.
	m2, r2 := deviceManager(t)
	r2.diagnostic = false
	short, cancel := context.WithTimeout(ctx, 60*time.Millisecond)
	defer cancel()
	m2.root = short
	d, err = m2.StartDiagnostic(ctx, DiagnosticRequest{deviceKey, nodeKey, "path"})
	if err != nil {
		t.Fatal(err)
	}
	if result := waitDiagnostic(t, m2, d.ID); result.State != "timeout" {
		t.Fatal(result)
	}
	select {
	case <-m2.conn.done:
	default:
		t.Fatal("path timeout reused connection")
	}
}

func TestDiagnosticMalformedFramesAndRetention(t *testing.T) {
	for _, b := range [][]byte{{1, 103, 1}, {1, 255, 0}, {1, 104, 255}, {1, 136, 127, 255, 255, 0, 0, 0, 0, 0, 0}} {
		if _, err := decodeTelemetry(b); err == nil {
			t.Fatalf("accepted %x", b)
		}
	}
	for _, b := range [][]byte{{255, 0}, {0xc1, 0, 0, 0, 0, 0}, {0x42, 1, 2, 0}, {0, 0, 1}} {
		if _, _, err := decodeDiscovery(b); err == nil {
			t.Fatalf("accepted path %x", b)
		}
	}
	if v, err := decodeLocal([]byte{12, 0, 0}, "storage"); err != nil || len(v) != 1 {
		t.Fatal("legacy battery", v, err)
	}
	m, _ := deviceManager(t)
	m.mu.Lock()
	m.diagnostics = map[string]Diagnostic{}
	for i := 0; i < 128; i++ {
		id := strings.Repeat("x", i+1)
		m.diagnostics[id] = Diagnostic{StartedAt: time.Now().Unix()}
	}
	m.pruneDiagnostics()
	count := len(m.diagnostics)
	m.diagnostics["expired"] = Diagnostic{StartedAt: time.Now().Unix() - 601}
	m.mu.Unlock()
	if count != 127 {
		t.Fatal(count)
	}
	if _, err := m.Diagnostic("expired"); err == nil {
		t.Fatal("expired result visible")
	}
}

func TestChatDetailsSurviveInboxAndMigration(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(context.Background(), dir, Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	snr := -6.25
	msg := Message{ID: "history-data", IdentityKey: deviceKey, Kind: "direct", PeerKey: nodeKey, Sender: nodeKey, Direction: "incoming", ReceivedAt: now - 8*86400, Timestamp: now - 8*86400 - 3, Review: "safe", Text: "safe", State: "completed", Reply: "answer", SendState: "delivered", Reception: &ReceptionInfo{SNR: &snr}, SenderLabel: "safe label"}
	if _, err = m.store.insert(msg); err != nil {
		t.Fatal(err)
	}
	msg.ID = "protected-data"
	msg.Review = "suspicious"
	msg.Text = "hidden-body"
	msg.SenderLabel = "hidden-label"
	msg.Reply = ""
	if _, err = m.store.insert(msg); err != nil {
		t.Fatal(err)
	}
	conv := messageConversation(msg).ID
	rows, err := m.ChatMessages(conv, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(rows)
	if strings.Contains(string(b), "hidden-") {
		t.Fatal("protected projection leak")
	}
	for _, row := range rows {
		if row.Direction == "outgoing" && row.Details != nil {
			t.Fatal("reply inherited reception")
		}
	}
	// Cleared conversations and deliberately mismatched bindings must not backfill.
	cleared := msg
	cleared.ID = "cleared-data"
	cleared.PeerKey = strings.Repeat("33", 32)
	cleared.Sender = cleared.PeerKey
	if _, err = m.store.insert(cleared); err != nil {
		t.Fatal(err)
	}
	clearedID := messageConversation(cleared).ID
	if err = m.UpdateConversation(clearedID, 0, nil, nil, true); err != nil {
		t.Fatal(err)
	}
	wrong := msg
	wrong.ID = "wrong-binding"
	wrong.Kind = "channel"
	wrong.Binding = "original-binding"
	if _, err = m.store.insert(wrong); err != nil {
		t.Fatal(err)
	}
	if _, err = m.store.db.Exec("UPDATE meshcore_messages SET binding='changed-binding' WHERE id=?", wrong.ID); err != nil {
		t.Fatal(err)
	}
	// Simulate the schema-2 layout and projection, then exercise the real startup migration.
	if _, err = m.store.db.Exec("ALTER TABLE meshcore_send_parts DROP COLUMN route; ALTER TABLE meshcore_send_parts DROP COLUMN ack_millis; UPDATE meshcore_meta SET value='2' WHERE key='version'; UPDATE meshcore_chat SET data=json_remove(data,'$.details')"); err != nil {
		t.Fatal(err)
	}
	m.Close()
	m, err = NewManager(context.Background(), dir, Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	backups, _ := filepath.Glob(filepath.Join(dir, "meshcore-v2-*.backup.db"))
	if len(backups) != 1 {
		t.Fatal("missing migration backup")
	}
	if hidden, err := m.ChatMessages(clearedID, 0, ""); err != nil || len(hidden) != 0 {
		t.Fatal("cleared history resurrected", hidden, err)
	}
	wrongRows, err := m.ChatMessages(messageConversation(wrong).ID, 0, "")
	if err != nil || len(wrongRows) != 1 || wrongRows[0].Details != nil {
		t.Fatal("changed binding backfilled", wrongRows, err)
	}
	cfg := Config{}
	cfg.Normalize()
	if err = m.store.prune(cfg); err != nil {
		t.Fatal(err)
	}
	rows, err = m.ChatMessages(conv, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.ID == "history-data" {
			found = true
		}
		if row.ID == "history-data" && (row.Details == nil || *row.Details.Reception.SNR != snr) {
			t.Fatal("metadata lost", row)
		}
	}
	if !found {
		t.Fatal("history vanished after inbox pruning")
	}
}

func TestDeviceRadioContactsClockAndReconnect(t *testing.T) {
	m, r := deviceManager(t)
	ctx := context.Background()
	d, err := m.Device(ctx)
	if err != nil {
		t.Fatal(err)
	}
	v := d.Values
	v.FrequencyKHz = 869500
	v.BandwidthHz = 125000
	v.SpreadingFactor = 9
	v.CodingRate = 6
	v.TxPower = 10
	v.MultiACKs = 2
	repeat := true
	mode := byte(2)
	v.Repeat = &repeat
	v.PathHashMode = &mode
	d, err = m.SaveDeviceSettings(ctx, DeviceSettingsRequest{deviceKey, d.Revision, "radio", v})
	if err != nil || !reflect.DeepEqual(d.Values, v) {
		t.Fatal("radio readback", d.Values, err)
	}
	v = d.Values
	v.ManualAddContacts = 1
	v.TelemetryBase = 2
	v.TelemetryLocation = 1
	v.TelemetryEnvironment = 0
	mask, hops := byte(6), byte(3)
	v.AutoAddMask = &mask
	v.AutoAddMaxHops = &hops
	d, err = m.SaveDeviceSettings(ctx, DeviceSettingsRequest{deviceKey, d.Revision, "contacts", v})
	if err != nil || !reflect.DeepEqual(d.Values, v) {
		t.Fatal("contact readback", d.Values, err)
	}
	d, err = m.SaveDeviceSettings(ctx, DeviceSettingsRequest{deviceKey, d.Revision, "clock", DeviceValues{}})
	if err != nil || d.Clock == nil || time.Now().Unix()-int64(*d.Clock) > 2 {
		t.Fatal("clock readback", d.Clock, err)
	}
	previous := d.Revision
	m.conn.Close()
	r2 := newDeviceRadio()
	r.settingsMu.Lock()
	r2.values = r.values
	r.settingsMu.Unlock()
	m.conn = newCompanion(r2)
	if _, err = m.SaveDeviceSettings(ctx, DeviceSettingsRequest{deviceKey, previous, "radio", v}); err == nil || err.Error() != "settings_conflict" {
		t.Fatal("reconnected revision accepted", err)
	}
	r2.version = 4
	d, err = m.Device(ctx)
	if err != nil || d.Features["other"] != "unsupported" || d.Features["path_hash"] != "unsupported" || d.Local.Groups["core"].State != "unsupported" {
		t.Fatal("legacy device", d.Features, err)
	}
}

func TestMessengerMigrationBacksUpExecutionOnlyDatabase(t *testing.T) {
	dir := t.TempDir()
	s, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`UPDATE meshcore_meta SET value='1' WHERE key='version';
DROP TABLE meshcore_send_parts; DROP TABLE meshcore_send_keys;
DROP TABLE meshcore_chat; DROP TABLE meshcore_conversations;
INSERT INTO meshcore_executions(id,expires) VALUES('execution-only',2000000000)`)
	s.db.Close()
	if err != nil {
		t.Fatal(err)
	}
	s, err = openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	backups, err := filepath.Glob(filepath.Join(dir, "meshcore-v1-*.backup.db"))
	if err != nil || len(backups) != 1 {
		t.Fatal("missing execution-only migration backup", err)
	}
	var count int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM meshcore_executions WHERE id='execution-only'").Scan(&count); err != nil || count != 1 {
		t.Fatal("lost execution reservation", err)
	}
}
