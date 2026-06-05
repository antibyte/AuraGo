package agent

import (
	"testing"

	"aurago/internal/remote"
)

func TestResolveRemoteDeviceFromParams(t *testing.T) {
	hub := remote.NewRemoteHub(nil, nil, nil)

	tc := ToolCall{
		Params: map[string]interface{}{
			"device_id": "92bbdd07-848d-4731-8198-740fe59a0271",
		},
	}
	id, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		t.Fatalf("resolveRemoteDevice() error = %v", err)
	}
	if id != "92bbdd07-848d-4731-8198-740fe59a0271" {
		t.Fatalf("device id = %q", id)
	}
}

func TestResolveRemoteDeviceRequiresIdentifier(t *testing.T) {
	hub := remote.NewRemoteHub(nil, nil, nil)

	_, err := resolveRemoteDevice(hub, ToolCall{})
	if err == nil {
		t.Fatal("expected error for missing device")
	}
	if err.Error() != "device_id or device_name is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}
