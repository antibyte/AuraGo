package tools

import (
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
