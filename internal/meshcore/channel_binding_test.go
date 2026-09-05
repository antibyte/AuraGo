package meshcore

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// The firmware uses strcpy into a reused output buffer, leaving name padding intact.
type paddedChannelRadio struct {
	*testRadio
	padding byte
}

func (r *paddedChannelRadio) ReadFrame() ([]byte, error) {
	b, err := r.testRadio.ReadFrame()
	if len(b) == 50 && b[0] == 18 {
		for i := 9; i < 34; i++ { // After "Public\x00".
			b[i] = r.padding
		}
	}
	return b, err
}

func TestChannelBindingSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	open := func(cfg Config, padding byte) (*Manager, Status) {
		t.Helper()
		m, err := NewManager(context.Background(), dir, Hooks{})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { m.Close() })
		m.cfg = cfg
		m.conn = newCompanion(&paddedChannelRadio{testRadio: newTestRadio(), padding: padding})
		st, err := m.refresh(context.Background(), m.conn)
		if err != nil {
			t.Fatal(err)
		}
		return m, st
	}
	cfg := Config{Enabled: true, Port: "test", IdentityKey: deviceKey}
	m, before := open(cfg, 0x41)
	cfg.Channels = []ChannelRule{{Index: 0, Binding: before.Channels[0].Binding, Mode: "questions", AllowSend: true}}
	saved, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	var restored Config
	if err := json.Unmarshal(saved, &restored); err != nil {
		t.Fatal(err)
	}
	_, after := open(restored, 0x62)
	if after.State != "connected" || after.Channels[0].Binding != before.Channels[0].Binding {
		t.Fatal("unchanged channel lost its confirmed binding after restart")
	}
}

func TestChannelBindingUsesOnlyDefinedFields(t *testing.T) {
	b := make([]byte, 50)
	b[0] = 18
	copy(b[2:34], "Public")
	copy(b[34:], bytes.Repeat([]byte{7}, 16))
	salt := bytes.Repeat([]byte{8}, 32)
	want := channelBinding(deviceKey, b, salt)
	legacy := hmac.New(sha256.New, salt)
	legacy.Write([]byte(deviceKey))
	legacy.Write(b[1:])
	if want != hex.EncodeToString(legacy.Sum(nil)) {
		t.Fatal("zero-padded legacy binding changed")
	}
	for _, offset := range []int{1, 2, 34, 49} { // Slot, name and entire secret remain binding.
		changed := bytes.Clone(b)
		changed[offset] ^= 1
		if channelBinding(deviceKey, changed, salt) == want {
			t.Fatalf("changed channel field at %d retained its binding", offset)
		}
	}
	if channelBinding(nodeKey, b, salt) == want || channelBinding(deviceKey, b, bytes.Repeat([]byte{9}, 32)) == want {
		t.Fatal("device identity and persistent salt must remain binding")
	}
	for i := 9; i < 34; i++ {
		b[i] = 0xff
	}
	original := bytes.Clone(b)
	if channelBinding(deviceKey, b, salt) != want {
		t.Fatal("undefined name padding changed the binding")
	}
	if !bytes.Equal(b, original) {
		t.Fatal("fingerprinting modified the wire response")
	}
}
