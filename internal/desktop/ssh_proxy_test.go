package desktop

import (
	"strings"
	"testing"

	"aurago/internal/inventory"
)

func TestResolveSSHAccessRejectsProtocolNone(t *testing.T) {
	device := inventory.DeviceRecord{
		Name:          "Registry Only",
		Type:          "printer",
		Protocol:      inventory.ProtocolNone,
		IPAddress:     "192.168.1.90",
		Port:          22,
		Username:      "root",
		VaultSecretID: "secret-1",
	}

	_, _, _, _, err := resolveSSHAccess(device, nil, nil)
	if err == nil {
		t.Fatal("resolveSSHAccess succeeded for protocol none; expected error")
	}
	if !strings.Contains(err.Error(), `protocol "none"`) {
		t.Fatalf("error = %q, want protocol none guidance", err.Error())
	}
}
