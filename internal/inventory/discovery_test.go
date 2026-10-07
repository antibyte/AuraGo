package inventory

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDiscoveryCannotReplaceAuthenticatedDevice(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	want := DeviceRecord{ID: "trusted", Name: "Printer", Type: "server", Protocol: ProtocolSSH, IPAddress: "192.0.2.1", Port: 22, Username: "admin", VaultSecretID: "fixture-vault", CredentialID: "fixture-credential", Description: "previous observation", Tags: []string{"trusted"}, MACAddress: "00:11:22:33:44:55"}
	if err := AddDevice(db, want); err != nil {
		t.Fatal(err)
	}
	spoof := DeviceRecord{Name: "printer", Type: "attacker", Protocol: ProtocolVNC, IPAddress: "198.51.100.2", Port: 5900, Username: "evil", VaultSecretID: "evil", CredentialID: "evil", MACAddress: "aa:bb:cc:dd:ee:ff", Description: "new observation"}
	created, updated, err := UpsertDeviceByName(db, spoof, true)
	if err != nil || created || !updated {
		t.Fatalf("upsert: %v %v %v", created, updated, err)
	}
	got, err := GetDeviceByID(db, want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got.Description, "Unverified discovery:") {
		t.Fatal("observation not identified")
	}
	got.Description = want.Description
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discovery replaced trusted record: %+v", got)
	}
	spoof.Name = "new-device"
	if _, _, err := UpsertDeviceByName(db, spoof, true); err != nil {
		t.Fatal(err)
	}
	got, err = GetDeviceByIDOrName(db, "new-device")
	if err != nil {
		t.Fatal(err)
	}
	if got.Protocol != ProtocolNone || got.Username != "" || got.CredentialID != "" || got.VaultSecretID != "" {
		t.Fatal("discovery created authenticated device")
	}
}
