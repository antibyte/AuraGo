package tools

import (
	"log/slog"
	"strings"
	"testing"
)

func TestMQTTPublishRespectsRuntimePermissions(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{MQTTEnabled: true, MQTTReadOnly: true})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	err := MQTTPublish("home/test", "payload", 0, false, nil)
	if err == nil || !strings.Contains(err.Error(), "mqtt publish is disabled") {
		t.Fatalf("MQTTPublish error = %v, want readonly denial", err)
	}
}

func TestMQTTMutatingSubscriptionsRespectReadOnlyRuntimePermissions(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{MQTTEnabled: true, MQTTReadOnly: true})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	if err := MQTTSubscribe("home/test", 0, nil); err == nil || !strings.Contains(err.Error(), "mqtt mutation is disabled") {
		t.Fatalf("MQTTSubscribe error = %v, want readonly denial", err)
	}
	if err := MQTTUnsubscribe("home/test", nil); err == nil || !strings.Contains(err.Error(), "mqtt mutation is disabled") {
		t.Fatalf("MQTTUnsubscribe error = %v, want readonly denial", err)
	}
}

func TestMQTTReadOperationsRequireRuntimePermission(t *testing.T) {
	ClearRuntimePermissionsForTest()
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	if err := MQTTSubscribe("home/test", 0, nil); err == nil || !strings.Contains(err.Error(), "mqtt is disabled") {
		t.Fatalf("MQTTSubscribe error = %v, want permission denial", err)
	}
	if _, err := MQTTGetMessages("home/test", 10, nil); err == nil || !strings.Contains(err.Error(), "mqtt is disabled") {
		t.Fatalf("MQTTGetMessages error = %v, want permission denial", err)
	}
}

func TestMQTTUnsubscribeWithDetailsReportsRemainingOwners(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{MQTTEnabled: true})
	RegisterMQTTUnsubscribeDetail(func(topic string, _ *slog.Logger) (MQTTUnsubscribeResult, error) {
		return MQTTUnsubscribeResult{
			Topic:   topic,
			Removed: false,
			RemainingOwners: []MQTTUnsubscribeOwner{{
				Kind: "config",
				Key:  topic,
				QoS:  1,
			}},
		}, nil
	})
	t.Cleanup(func() {
		RegisterMQTTUnsubscribeDetail(nil)
		RegisterMQTTBridge(nil, nil, nil, nil)
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	result, err := MQTTUnsubscribeWithDetails("home/shared", nil)
	if err != nil {
		t.Fatalf("MQTTUnsubscribeWithDetails: %v", err)
	}
	if result.Removed || len(result.RemainingOwners) != 1 || result.RemainingOwners[0].Kind != "config" {
		t.Fatalf("unsubscribe detail = %+v, want remaining config owner", result)
	}
}

func TestMQTTUnsubscribeWithDetailsFallsBackToLegacyBridge(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{MQTTEnabled: true})
	RegisterMQTTUnsubscribeDetail(nil)
	RegisterMQTTBridge(nil, nil, func(topic string, _ *slog.Logger) error {
		if topic != "home/legacy" {
			t.Fatalf("legacy unsubscribe topic = %q", topic)
		}
		return nil
	}, nil)
	t.Cleanup(func() {
		RegisterMQTTBridge(nil, nil, nil, nil)
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	result, err := MQTTUnsubscribeWithDetails("home/legacy", nil)
	if err != nil {
		t.Fatalf("MQTTUnsubscribeWithDetails legacy fallback: %v", err)
	}
	if !result.Removed || result.Topic != "home/legacy" {
		t.Fatalf("legacy fallback detail = %+v, want removed topic", result)
	}
}
