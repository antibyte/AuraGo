package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionRejectsReplayTamperingAndWrongIdentity(t *testing.T) {
	key := validKey(t)
	msg, _ := NewMessage(MsgTask, "egg", "nest", key, TaskPayload{TaskID: "once"})
	master := testSession(t, "egg", "nest", "master")
	if err := master.Prepare(msg, key); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"id", "type", "identity", "direction", "challenge", "sequence", "expired", "future", "legacy"} {
		t.Run(name, func(t *testing.T) {
			copy := *msg
			switch name {
			case "id":
				copy.ID += "changed"
			case "type":
				copy.Type, copy.EggID = "tas", "kegg"
			case "identity":
				copy.NestID = "other"
			case "direction":
				copy.Sender = "egg"
			case "challenge":
				copy.Session = strings.Repeat("b", 64)
			case "sequence":
				copy.Sequence = 0
			case "expired":
				copy.Timestamp = time.Now().Add(-3 * time.Minute).Format(time.RFC3339)
			case "future":
				copy.Timestamp = time.Now().Add(time.Minute).Format(time.RFC3339)
			case "legacy":
				copy.Protocol = 0
			}
			// Even authenticated but stale or misbound traffic must be rejected.
			if name != "id" && name != "type" && name != "legacy" {
				_ = SignMessage(&copy, key)
			}
			if err := testSession(t, "egg", "nest", "egg").Accept(copy, key, ""); err == nil {
				t.Fatal("invalid message accepted")
			}
		})
	}
	egg := testSession(t, "egg", "nest", "egg")
	if err := egg.Accept(*msg, key, ""); err != nil {
		t.Fatal(err)
	}
	if err := egg.Accept(*msg, key, ""); err == nil {
		t.Fatal("replay accepted")
	}
	otherID, _ := NewChallenge()
	other, _ := NewSession(otherID, "egg", "nest", "egg")
	if err := other.Accept(*msg, key, ""); err == nil {
		t.Fatal("previous connection message accepted")
	}
}

func TestReplacedConnectionCleanupKeepsNewGeneration(t *testing.T) {
	hub := NewEggHub(testLogger())
	s1, _, cleanup1 := wsPair(t)
	defer cleanup1()
	s2, _, cleanup2 := wsPair(t)
	defer cleanup2()
	old := &EggConnection{Conn: s1, EggID: "egg", NestID: "nest", SharedKey: validKey(t)}
	newConn := &EggConnection{Conn: s2, EggID: "egg", NestID: "nest", SharedKey: validKey(t)}
	if err := registerTestConnection(t, hub, "nest", old); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); hub.HandleMessages(old) }()
	var disconnects atomic.Int32
	hub.OnDisconnect = func(string, string) { disconnects.Add(1) }
	if err := registerTestConnection(t, hub, "nest", newConn); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("old reader did not exit")
	}
	hub.unregister("nest", old, false, time.Nanosecond, func(string, string) { t.Error("stale callback for replaced connection") })
	if hub.GetConnection("nest") != newConn || disconnects.Load() != 0 {
		t.Fatal("replacement was unregistered")
	}
}

func TestHubRejectsAckFromDifferentConnection(t *testing.T) {
	hub := NewEggHub(testLogger())
	a, b := &EggConnection{}, &EggConnection{}
	ch := make(chan AckPayload, 1)
	hub.pendingAcks["ref"] = pendingAck{conn: a, ch: ch}
	hub.resolveAck(b, AckPayload{RefID: "ref", Success: true})
	select {
	case <-ch:
		t.Fatal("other connection resolved ack")
	default:
	}
	hub.resolveAck(a, AckPayload{RefID: "ref", Success: true})
	select {
	case <-ch:
	default:
		t.Fatal("own ack was not resolved")
	}
}

func TestHeartbeatAndRekeyRemainOrderedUnderConcurrentTraffic(t *testing.T) {
	hub := NewEggHub(testLogger())
	s, c, cleanup := wsPair(t)
	defer cleanup()
	key := validKey(t)
	server := &EggConnection{Conn: s, EggID: "egg", NestID: "nest", SharedKey: key}
	if err := registerTestConnection(t, hub, "nest", server); err != nil {
		t.Fatal(err)
	}
	client := NewEggClient("", "egg", "nest", key, "fixture", testLogger())
	client.conn, client.session = c, testSession(t, "egg", "nest", "egg")
	client.OnRekey = func(string, int) error { return nil }
	errCh := make(chan error, 1)
	client.OnTask = func(task TaskPayload) {
		if err := client.SendResult(ResultPayload{TaskID: task.TaskID}); err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
	}
	var results atomic.Int32
	hub.OnResult = func(string, ResultPayload) { results.Add(1) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub.StartHeartbeatMonitor(ctx, time.Millisecond, time.Hour, nil)
	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); hub.HandleMessages(server) }()
	go func() { defer readers.Done(); client.readLoop() }()
	defer func() { client.Stop(); readers.Wait() }()
	var writers sync.WaitGroup
	writers.Add(1)
	go func() {
		defer writers.Done()
		for i := 0; i < 30; i++ {
			if err := client.send(MsgHeartbeat, HeartbeatPayload{Status: "busy"}); err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
		}
	}()
	for i := 0; i < 10; i++ {
		if _, err := hub.SendRekey(context.Background(), "nest", validKey(t)); err != nil {
			t.Fatal(err)
		}
		if err := hub.SendTask("nest", TaskPayload{TaskID: fmt.Sprint(i)}); err != nil {
			t.Fatal(err)
		}
		// Wait for each key to arrive before rotating again; one prior key may be in flight.
		deadline := time.Now().Add(2 * time.Second)
		for results.Load() < int32(i+1) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if results.Load() != int32(i+1) {
			t.Fatal("message/key ordering lost task result")
		}
	}
	writers.Wait()
	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}
	if client.SharedKeySnapshot() != server.SharedKey {
		t.Fatal("key states diverged")
	}
}

func TestSignedEnvelopeJSONRoundTrip(t *testing.T) {
	key := validKey(t)
	msg, _ := NewMessage(MsgTask, "egg", "nest", key, json.RawMessage(`{ "text": "<&>", "nested": [1, 2] }`))
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyMessage(decoded, key); err != nil || !ok {
		t.Fatal("JSON transport invalidated signature", err)
	}
}
