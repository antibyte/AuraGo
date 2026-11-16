package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/remote"
)

type ambiguityTransport struct{ sent int }

func (*ambiguityTransport) IsConnected(string) bool { return true }
func (*ambiguityTransport) Disconnect(string)       {}
func (t *ambiguityTransport) SendCommand(context.Context, string, remote.CommandPayload, time.Duration) (remote.ResultPayload, error) {
	t.sent++
	return remote.ResultPayload{}, nil
}

func TestAmbiguousRemoteDeviceNeverSends(t *testing.T) {
	db, err := remote.InitDB(filepath.Join(t.TempDir(), "devices.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, err := remote.CreateDevice(db, remote.DeviceRecord{Name: "Host"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := remote.CreateDevice(db, remote.DeviceRecord{Name: "host"}); err != nil {
		t.Fatal(err)
	}
	hub := remote.NewRemoteHub(db, nil, nil)
	transport := &ambiguityTransport{}
	hub.RegisterCommandTransport("fixture", transport)
	call := ToolCall{DeviceName: "HOST", Command: "fixture"}
	if _, err := resolveRemoteDevice(hub, call); !errors.Is(err, remote.ErrAmbiguousDeviceName) {
		t.Fatalf("lost ambiguity: %v", err)
	}
	if out := remoteExecuteCommand(context.Background(), hub, call, nil); !strings.Contains(out, "device_id") {
		t.Fatalf("missing ID guidance: %s", out)
	}
	if transport.sent != 0 {
		t.Fatal("ambiguous name dispatched")
	}
	call.DeviceID = id
	if got, err := resolveRemoteDevice(hub, call); err != nil || got != id {
		t.Fatalf("explicit ID: %s %v", got, err)
	}
}

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
