package server

import (
	"context"
	"net"
	"testing"
	"time"

	"aurago/internal/mqtt"
	"aurago/internal/tools"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

func mqttMissionBroker(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
				for {
					p, err := packets.ReadPacket(conn)
					if err != nil {
						return
					}
					switch packet := p.(type) {
					case *packets.ConnectPacket:
						_ = packets.NewControlPacket(packets.Connack).Write(conn)
					case *packets.SubscribePacket:
						ack := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
						ack.MessageID, ack.ReturnCodes = packet.MessageID, packet.Qoss
						_ = ack.Write(conn)
					case *packets.UnsubscribePacket:
						ack := packets.NewControlPacket(packets.Unsuback).(*packets.UnsubackPacket)
						ack.MessageID = packet.MessageID
						_ = ack.Write(conn)
					case *packets.PingreqPacket:
						_ = packets.NewControlPacket(packets.Pingresp).Write(conn)
					case *packets.DisconnectPacket:
						return
					}
				}
			}()
		}
	}()
	return "tcp://" + l.Addr().String()
}

func TestMQTTMissionOwnershipFollowsEnableUpdateAndDelete(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{MissionsEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	s := newMQTTConfigTestServer(t)
	cfg := s.ConfigSnapshot().Clone()
	cfg.MQTT.Broker = mqttMissionBroker(t)
	cfg.MQTT.Topics = []string{"mission/home/#"}
	c := mqtt.NewMQTTController(s.Logger)
	mqtt.SetDefaultController(c)
	s.MQTTController = c
	t.Cleanup(func() { _ = c.Stop(context.Background()) })
	s.replaceConfigSnapshot(cfg)
	manager := tools.NewMissionManagerV2(t.TempDir(), nil)
	manager.SetMQTTManager(&missionMQTTAdapter{logger: s.Logger})
	mission := &tools.MissionV2{
		ID: "mqtt-owner-roundtrip", Name: "MQTT owner roundtrip", Prompt: "fixture",
		ExecutionType: tools.ExecutionTriggered, TriggerType: tools.TriggerMQTTMessage,
		TriggerConfig: &tools.TriggerConfig{MQTTTopic: "mission/home/#"}, Enabled: true,
	}
	if err := manager.Create(mission); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Delete(mission.ID) })
	// Desired mission ownership is registered before the integration is enabled.
	on := cfg.Clone()
	on.MQTT.Enabled = true
	s.replaceConfigSnapshot(on)
	waitOwner := func(filter, kind string, expected bool) {
		t.Helper()
		deadline := time.Now().Add(4 * time.Second)
		for {
			status := c.Status()
			found := false
			for _, subscription := range status.Subscriptions {
				if subscription.Filter != filter {
					continue
				}
				for _, owner := range subscription.Owners {
					if owner.Kind == kind {
						found = true
					}
				}
			}
			if status.Connected && found == expected {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("owner %s on %s expected %v: %+v", kind, filter, expected, status)
			}
			time.Sleep(time.Millisecond)
		}
	}
	waitOwner("mission/home/#", "mission", true)
	waitOwner("mission/home/#", "config", true)
	updated, _ := manager.Get(mission.ID)
	updated.TriggerConfig.MQTTTopic = "mission/garage/#"
	if err := manager.Update(mission.ID, updated); err != nil {
		t.Fatal(err)
	}
	waitOwner("mission/garage/#", "mission", true)
	waitOwner("mission/home/#", "mission", false)
	waitOwner("mission/home/#", "config", true)
	updated, _ = manager.Get(mission.ID)
	updated.Enabled = false
	if err := manager.Update(mission.ID, updated); err != nil {
		t.Fatal(err)
	}
	waitOwner("mission/garage/#", "mission", false)
	updated.Enabled = true
	if err := manager.Update(mission.ID, updated); err != nil {
		t.Fatal(err)
	}
	waitOwner("mission/garage/#", "mission", true)
	if err := manager.Delete(mission.ID); err != nil {
		t.Fatal(err)
	}
	waitOwner("mission/garage/#", "mission", false)
	waitOwner("mission/home/#", "config", true)
}
