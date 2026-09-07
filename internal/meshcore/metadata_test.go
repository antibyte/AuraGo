package meshcore

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestMessageReceptionMetadata(t *testing.T) {
	st := Status{IdentityKey: deviceKey, Name: "Receiver", Contacts: []Contact{{Key: nodeKey, Name: "Operator", Type: 1}}, Channels: []Channel{{Index: 2, Name: "Public", Kind: "public", Binding: strings.Repeat("33", 32)}}}
	for _, kind := range []byte{7, 8, 16, 17} {
		for _, path := range []byte{0, 3, 0x42, 0x83, 0xff} {
			frame := directFrame(7, 0, "Operator: hello")
			frame[7] = path
			if kind == 8 || kind == 17 {
				frame = append([]byte{8, 2, path, 0}, frame[9:]...)
			}
			if kind >= 16 {
				frame = append([]byte{kind, 0xf7, 0, 0}, frame[1:]...) // -2.25 dB
			}
			msg, err := decodeMessage(frame, st)
			if err != nil {
				t.Fatal(err)
			}
			rx := msg.Reception
			if rx.FrameType != kind || rx.FrameBytes != len(frame) || rx.Path.Encoded != path || msg.Receiver.Name != "Receiver" {
				t.Fatalf("missing metadata: %+v", msg)
			}
			if kind >= 16 {
				if rx.SNR == nil || *rx.SNR != -2.25 {
					t.Fatalf("signed SNR: %+v", rx)
				}
			} else if rx.SNR != nil {
				t.Fatal("legacy SNR invented")
			}
			if path == 0xff {
				if rx.Path.Route != "direct" || rx.Path.Hops != nil {
					t.Fatal("direct routing claimed zero hops")
				}
			} else if rx.Path.Route != "flood" || rx.Path.Hops == nil || *rx.Path.Hops != int(path&63) || *rx.Path.HashBytes != int(path>>6)+1 {
				t.Fatalf("packed hop count: %+v", rx.Path)
			}
			if msg.Kind == "direct" && (msg.SenderContact == nil || msg.SenderContact.Key != nodeKey) {
				t.Fatal("missing contact")
			}
			if msg.Kind == "channel" && (msg.ChannelName != "Public" || msg.ChannelKind != "public" || msg.SenderLabel != "Operator" || msg.SenderContact != nil) {
				t.Fatal("channel metadata or trust changed")
			}
		}
	}
	for _, path := range []byte{0xc0, 0x7f, 0xbf} {
		frame := directFrame(7, 0, "hello")
		frame[7] = path
		if _, err := decodeMessage(frame, st); err == nil {
			t.Fatalf("invalid path accepted: %x", path)
		}
	}
	frame := directFrame(16, 2, "forwarded")
	frame[1] = 0
	msg, err := decodeMessage(frame, st)
	if err != nil || msg.Reception.ForwardedSenderPrefix != "01020304" || msg.Reception.SNR == nil || *msg.Reception.SNR != 0 {
		t.Fatalf("forward prefix or zero SNR: %+v %v", msg, err)
	}
	st.Contacts = append(st.Contacts, Contact{Key: nodeKey[:12] + strings.Repeat("44", 26)})
	msg, err = decodeMessage(directFrame(7, 0, "hello"), st)
	if err != nil || msg.SenderContact != nil || msg.PeerKey != "" {
		t.Fatal("ambiguous prefix attributed to a contact")
	}
}

type metadataRadio struct{ *testRadio }

func (r metadataRadio) ReadFrame() ([]byte, error) {
	b, err := r.testRadio.ReadFrame()
	if err != nil {
		return b, err
	}
	switch b[0] {
	case 13:
		copy(b[8:20], "2026-09-07")
		copy(b[20:60], "TestBoard")
		b[80], b[81] = 1, 2
	case 5:
		b[1], b[2], b[3] = 1, 22, 30
		binary.LittleEndian.PutUint32(b[36:40], 52500000)
		binary.LittleEndian.PutUint32(b[40:44], 13400000)
		binary.LittleEndian.PutUint32(b[48:52], 869525)
		binary.LittleEndian.PutUint32(b[52:56], 250000)
		b[56], b[57] = 11, 5
	case 3:
		b[34], b[35] = 3, 0x42
		copy(b[36:100], []byte{0xab, 0xcd, 0xef, 0x12})
		binary.LittleEndian.PutUint32(b[132:136], 1700000000)
		lat := int32(-33500000)
		binary.LittleEndian.PutUint32(b[136:140], uint32(lat))
		binary.LittleEndian.PutUint32(b[140:144], 151200000)
		binary.LittleEndian.PutUint32(b[144:148], 1700000001)
	}
	return b, nil
}

func TestReceptionSnapshotPersistenceAndSecrets(t *testing.T) {
	c := newCompanion(metadataRadio{newTestRadio()})
	defer c.Close()
	st, err := c.snapshot(context.Background(), bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if st.Device.Manufacturer != "TestBoard" || st.Device.ContactCapacity != 20 || !*st.Device.RepeatEnabled || *st.Device.PathHashMode != 2 || st.SnapshotAt == 0 {
		t.Fatalf("device metadata: %+v", st)
	}
	if st.Radio.FrequencyKHz != 869525 || st.Radio.BandwidthHz != 250000 || st.Radio.SpreadingFactor != 11 || st.Radio.Position.Latitude != 52.5 {
		t.Fatalf("radio metadata: %+v", st.Radio)
	}
	contact := st.Contacts[0]
	if contact.Flags != 3 || contact.OutPath.Hashes != "abcdef12" || *contact.OutPath.Hops != 2 || contact.Position.Latitude != -33.5 || contact.LastAdvert != 1700000000 || contact.LastModified != 1700000001 {
		t.Fatalf("contact metadata: %+v", contact)
	}
	msg, err := decodeMessage(directFrame(16, 0, "hello"), st)
	if err != nil {
		t.Fatal(err)
	}
	s, err := openStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	if _, err = s.insert(msg); err != nil {
		t.Fatal(err)
	}
	got, err := s.get(msg.ID)
	if err != nil || !reflect.DeepEqual(got, msg) {
		t.Fatalf("metadata lost after persistence: %+v %v", got, err)
	}
	b, _ := json.Marshal(got)
	if bytes.Contains(b, []byte("PIN!")) || bytes.Contains(b, []byte("07070707070707070707070707070707")) {
		t.Fatal("device PIN or channel secret leaked")
	}
	var legacy Message
	if err := json.Unmarshal([]byte(`{"kind":"direct","text":"old"}`), &legacy); err != nil || legacy.Reception != nil || legacy.Receiver != nil {
		t.Fatal("old records must retain unknown reception")
	}
	if p, err := decodePath(0xff, make([]byte, 64)); err != nil || p.Route != "unknown" || p.Hops != nil {
		t.Fatal("unknown outgoing path invented")
	}
	if decodePosition(make([]byte, 8)) != nil {
		t.Fatal("unset GPS treated as a location")
	}
}

func TestReceptionMetadataSurvivesAgentAdmission(t *testing.T) {
	var runs []Message
	m, r, _ := testManager(t, Hooks{
		Scan: func(context.Context, Message) Review { return Review{Decision: "safe"} },
		Run: func(_ context.Context, msg Message, mode string) (string, error) {
			if (msg.Kind == "direct" && mode != "trusted") || (msg.Kind == "channel" && mode != "prefix") {
				t.Fatalf("unexpected admission: %s/%s", msg.Kind, mode)
			}
			runs = append(runs, msg)
			return "", nil
		},
	})
	direct := directFrame(16, 0, "hello")
	channel := append([]byte{17, 0xf7, 0, 0, 0, 0x42, 0}, direct[12:16]...)
	channel = append(channel, "Operator: !aura hello"...)
	r.mu.Lock()
	r.nextMessages = [][]byte{direct, channel}
	r.mu.Unlock()
	queue := make(chan Message, 2)
	if err := m.receiveBatch(context.Background(), m.conn, queue); err != nil {
		t.Fatal(err)
	}
	if len(queue) != 2 {
		t.Fatalf("messages not drained: %d", len(queue))
	}
	for len(queue) > 0 {
		msg := <-queue
		m.process(context.Background(), msg)
		stored, err := m.store.get(msg.ID)
		if err != nil || !reflect.DeepEqual(stored.Reception, msg.Reception) || stored.Receiver == nil {
			t.Fatalf("processing lost metadata: %+v %v", stored, err)
		}
	}
	if len(runs) != 2 || runs[0].SenderContact == nil || runs[0].Sender != nodeKey || runs[0].Reception.SNR == nil {
		t.Fatalf("direct context did not reach runner: %+v", runs)
	}
	if runs[1].Text != "hello" || runs[1].SenderLabel != "Operator" || runs[1].ChannelName != "Public" || *runs[1].Reception.Path.Hops != 2 || *runs[1].Reception.SNR != -2.25 {
		t.Fatalf("channel admission lost context: %+v", runs[1])
	}
}
