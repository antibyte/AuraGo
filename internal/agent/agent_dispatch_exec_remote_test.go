package agent

import (
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/remote"
	"aurago/internal/security"
)

func TestResolveDeviceSSHAccessUsesCredentialReference(t *testing.T) {
	t.Parallel()

	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("init inventory db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := credentials.EnsureSchema(db); err != nil {
		t.Fatalf("ensure credentials schema: %v", err)
	}

	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	if err := vault.WriteSecret("cred-secret", "supersecret"); err != nil {
		t.Fatalf("write vault secret: %v", err)
	}

	credentialID, err := credentials.Create(db, credentials.Record{
		Name:            "Test SSH",
		Type:            "ssh",
		Host:            "10.0.0.5",
		Username:        "root",
		PasswordVaultID: "cred-secret",
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	deviceID, err := inventory.CreateDevice(db, "legacy-device", "server", "192.168.1.10", 2222, "legacy", "legacy-secret", credentialID, "desc", nil, "")
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	device, err := inventory.GetDeviceByID(db, deviceID)
	if err != nil {
		t.Fatalf("get device: %v", err)
	}

	access, err := resolveDeviceSSHAccess(device, db, vault)
	if err != nil {
		t.Fatalf("resolve access: %v", err)
	}

	if access.Host != "10.0.0.5" {
		t.Fatalf("expected host from credential, got %q", access.Host)
	}
	if access.Username != "root" {
		t.Fatalf("expected username from credential, got %q", access.Username)
	}
	if access.Port != 2222 {
		t.Fatalf("expected port from device, got %d", access.Port)
	}
	if string(access.Secret) != "supersecret" {
		t.Fatalf("expected credential vault secret, got %q", string(access.Secret))
	}
}

func TestRemoteRevokeDeviceFailsWhenStatusPersistenceFails(t *testing.T) {
	t.Parallel()

	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("init remote db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close remote db: %v", err)
	}

	hub := remote.NewRemoteHub(db, nil, nil)
	out := remoteRevokeDevice(hub, ToolCall{DeviceID: "device-1"}, nil)
	if !strings.Contains(out, `"status":"error"`) {
		t.Fatalf("expected error output, got %s", out)
	}
	if !strings.Contains(out, "failed to persist revoked status") {
		t.Fatalf("expected persistence error, got %s", out)
	}
}
