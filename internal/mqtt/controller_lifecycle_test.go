package mqtt

import (
	"context"
	"net"
	"testing"
	"time"

	"aurago/internal/config"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

// lifecycleBroker accepts a real MQTT handshake and rejects only the explicitly
// rotated credential. No external broker or configured credentials are used.
func lifecycleBroker(t *testing.T) (string, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	connections := make(chan string, 20)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
				packet, err := packets.ReadPacket(conn)
				if err != nil {
					return
				}
				connect, ok := packet.(*packets.ConnectPacket)
				if !ok {
					return
				}
				connections <- string(connect.Password)
				ack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
				if string(connect.Password) == "rejected-fixture" {
					ack.ReturnCode = 4
				}
				if ack.Write(conn) != nil || ack.ReturnCode != 0 {
					return
				}
				for {
					packet, err := packets.ReadPacket(conn)
					if err != nil {
						return
					}
					switch p := packet.(type) {
					case *packets.SubscribePacket:
						a := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
						a.MessageID, a.ReturnCodes = p.MessageID, p.Qoss
						_ = a.Write(conn)
					case *packets.UnsubscribePacket:
						a := packets.NewControlPacket(packets.Unsuback).(*packets.UnsubackPacket)
						a.MessageID = p.MessageID
						_ = a.Write(conn)
					case *packets.PingreqPacket:
						_ = packets.NewControlPacket(packets.Pingresp).Write(conn)
					case *packets.DisconnectPacket:
						return
					}
				}
			}()
		}
	}()
	return "tcp://" + listener.Addr().String(), connections
}

func waitLifecycleStatus(t *testing.T, c *MQTTController, predicate func(MQTTStatus) bool) MQTTStatus {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		s := c.Status()
		if predicate(s) {
			return s
		}
		if time.Now().After(deadline) {
			t.Fatalf("MQTT lifecycle did not converge: %+v", s)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestMQTTControllerHotReloadCredentialsAndLogicalSettings(t *testing.T) {
	broker, connections := lifecycleBroker(t)
	c := NewMQTTController(nil)
	t.Cleanup(func() { _ = c.Stop(context.Background()) })
	cfg := &config.Config{}
	cfg.MQTT.Enabled, cfg.MQTT.Broker = true, broker
	cfg.MQTT.Username, cfg.MQTT.Password = "fixture", "initial-fixture"
	c.UpdateConfig(cfg)
	first := waitLifecycleStatus(t, c, func(s MQTTStatus) bool { return s.Connected })
	select {
	case password := <-connections:
		if password != "initial-fixture" {
			t.Fatal("initial credential was not used")
		}
	case <-time.After(time.Second):
		t.Fatal("missing initial connection")
	}
	c.mu.RLock()
	oldGeneration := c.current
	c.mu.RUnlock()

	logical := cfg.Clone()
	logical.MQTT.Topics, logical.MQTT.ReadOnly = []string{"fixture/#"}, true
	c.UpdateConfig(logical)
	waitLifecycleStatus(t, c, func(s MQTTStatus) bool { return s.ActiveRevision > first.ActiveRevision })
	if c.Status().Generation != first.Generation {
		t.Fatal("logical update replaced connection")
	}
	select {
	case <-connections:
		t.Fatal("logical update opened another connection")
	default:
	}

	rotated := logical.Clone()
	rotated.MQTT.Password = "rejected-fixture"
	c.UpdateConfig(rotated)
	select {
	case password := <-connections:
		if password != "rejected-fixture" {
			t.Fatal("rotation reused obsolete credential")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("credential rotation did not connect")
	}
	waitLifecycleStatus(t, c, func(s MQTTStatus) bool { return s.Generation > first.Generation && !s.Connected })
	if c.generationCurrent(oldGeneration) {
		t.Fatal("retired callback generation is still active")
	}
	if c.currentConfig().MQTT.Password != "rejected-fixture" {
		t.Fatal("failed credential rotation fell back to old password")
	}

	disabled := rotated.Clone()
	disabled.MQTT.Enabled = false
	c.UpdateConfig(disabled)
	waitLifecycleStatus(t, c, func(s MQTTStatus) bool {
		return s.State == "disabled" && !s.Connected && s.ActiveRevision == s.DesiredRevision
	})
	if err := c.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	finalRevision := c.Status().DesiredRevision
	c.UpdateConfig(cfg)
	if c.Status().DesiredRevision != finalRevision || c.Status().Connected {
		t.Fatal("stopped controller accepted new work")
	}
}
