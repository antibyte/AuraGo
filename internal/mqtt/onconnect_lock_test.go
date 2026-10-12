package mqtt

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

func TestOnConnectReleasesCallbackLockBeforeBrokerWait(t *testing.T) {
	broker, _ := lifecycleBroker(t)
	controller := NewMQTTController(nil)
	t.Cleanup(func() { _ = controller.Stop(context.Background()) })

	observed := make(chan bool, 1)
	controller.reconcileHook = func(gen *mqttGeneration, _ *config.Config) {
		released := gen.callbackMu.TryLock()
		if released {
			gen.callbackMu.Unlock()
		}
		observed <- released
	}

	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Broker = broker
	controller.UpdateConfig(cfg)

	select {
	case released := <-observed:
		if !released {
			t.Fatal("callbackMu held while reconcile waits on the broker")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reconcile hook did not run")
	}
}

// TestOnConnectKeepsReconnectingAfterLossDuringBrokerWait closes the accepted
// broker connection inside reconcileHook and waits until ConnectionLost stores
// reconnecting. OnConnect must not replace that with connected while the
// socket is still down. The broker accepts one connection, so a later reconnect
// cannot finish a handshake during the assertion. The check also stays inside
// the 10s connect-retry interval from options.go.
func TestOnConnectKeepsReconnectingAfterLossDuringBrokerWait(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	var (
		connMu   sync.Mutex
		accepted net.Conn
	)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		connMu.Lock()
		accepted = conn
		connMu.Unlock()
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		packet, readErr := packets.ReadPacket(conn)
		if readErr != nil {
			return
		}
		if _, ok := packet.(*packets.ConnectPacket); !ok {
			return
		}
		ack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
		if ack.Write(conn) != nil {
			return
		}
		for {
			packet, readErr = packets.ReadPacket(conn)
			if readErr != nil {
				return
			}
			switch p := packet.(type) {
			case *packets.SubscribePacket:
				suback := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
				suback.MessageID, suback.ReturnCodes = p.MessageID, p.Qoss
				_ = suback.Write(conn)
			case *packets.PingreqPacket:
				_ = packets.NewControlPacket(packets.Pingresp).Write(conn)
			case *packets.DisconnectPacket:
				return
			}
		}
	}()

	controller := NewMQTTController(nil)
	t.Cleanup(func() { _ = controller.Stop(context.Background()) })

	hookDone := make(chan struct{})
	controller.reconcileHook = func(gen *mqttGeneration, _ *config.Config) {
		defer close(hookDone)
		connMu.Lock()
		conn := accepted
		connMu.Unlock()
		if conn == nil {
			t.Error("reconcile hook ran before the broker accepted a connection")
			return
		}
		if closeErr := conn.Close(); closeErr != nil {
			t.Errorf("close accepted connection: %v", closeErr)
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			controller.mu.RLock()
			state := controller.state
			controller.mu.RUnlock()
			open := gen.client != nil && gen.client.IsConnectionOpen()
			if state == "reconnecting" && !open {
				// Drain a ConnectionLost or Reconnecting handler that already
				// holds callbackMu so it cannot hide the later status write.
				gen.callbackMu.Lock()
				gen.callbackMu.Unlock()
				return
			}
			if time.Now().After(deadline) {
				t.Errorf("ConnectionLost did not store reconnecting (state=%s open=%v)", state, open)
				return
			}
			time.Sleep(time.Millisecond)
		}
	}

	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Broker = "tcp://" + listener.Addr().String()
	controller.UpdateConfig(cfg)

	select {
	case <-hookDone:
	case <-time.After(5 * time.Second):
		t.Fatal("reconcile hook did not return")
	}

	// The status write follows the hook on the same goroutine. Read Status
	// while IsConnectionOpen is still false; Status copies State through
	// unchanged in that case, and overwrites it when the socket is open.
	deadline := time.Now().Add(250 * time.Millisecond)
	var last MQTTStatus
	for {
		last = controller.Status()
		if last.Connected {
			t.Fatalf("connection reopened before the assertion: %+v", last)
		}
		if last.State == "connected" {
			t.Fatalf("OnConnect stored connected after a loss during broker wait: %+v", last)
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if last.State != "reconnecting" {
		t.Fatalf("stored state after connection loss: %+v", last)
	}
}
