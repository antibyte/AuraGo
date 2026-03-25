package credentials

import (
	"path/filepath"
	"testing"

	"aurago/internal/inventory"
)

func TestCredentialsCRUD(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()

	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	id, err := Create(db, Record{
		Name:               "Prod SSH",
		Type:               "ssh",
		Host:               "192.168.1.10",
		Username:           "root",
		Description:        "Main hypervisor",
		PasswordVaultID:    "vault-password",
		CertificateVaultID: "vault-cert",
		CertificateMode:    "upload",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := GetByID(db, id)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if !got.HasPassword || !got.HasCertificate {
		t.Fatalf("expected secret presence flags to be true, got %+v", got)
	}
	if got.CertificateMode != "upload" {
		t.Fatalf("CertificateMode = %q, want upload", got.CertificateMode)
	}

	got.Name = "Prod SSH Updated"
	got.Host = "10.0.0.42"
	got.CertificateMode = "text"
	if err := Update(db, got); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	items, err := List(db)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(items))
	}
	if items[0].Name != "Prod SSH Updated" || items[0].Host != "10.0.0.42" {
		t.Fatalf("updated record mismatch: %+v", items[0])
	}

	if err := Delete(db, id); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := GetByID(db, id); err == nil {
		t.Fatalf("GetByID() after delete = nil error, want error")
	}
}

func TestCredentialLoginType(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	// Login type should not require host
	id, err := Create(db, Record{
		Name:            "Web Login",
		Type:            "login",
		Username:        "admin",
		Description:     "Web panel login",
		PasswordVaultID: "vault-pw-login",
	})
	if err != nil {
		t.Fatalf("Create(login) error = %v", err)
	}

	got, err := GetByID(db, id)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Type != "login" {
		t.Fatalf("Type = %q, want login", got.Type)
	}
	if got.Host != "" {
		t.Fatalf("Host = %q, want empty for login type", got.Host)
	}
	if !got.HasPassword {
		t.Fatal("expected HasPassword = true")
	}
	if got.HasToken {
		t.Fatal("expected HasToken = false")
	}
}

func TestCredentialTokenType(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	id, err := Create(db, Record{
		Name:         "API Token",
		Type:         "token",
		Username:     "svc-account",
		Description:  "Service API token",
		TokenVaultID: "vault-token-123",
	})
	if err != nil {
		t.Fatalf("Create(token) error = %v", err)
	}

	got, err := GetByID(db, id)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Type != "token" {
		t.Fatalf("Type = %q, want token", got.Type)
	}
	if !got.HasToken {
		t.Fatal("expected HasToken = true")
	}
	if got.HasPassword {
		t.Fatal("expected HasPassword = false")
	}
}

func TestCredentialAllowPython(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	// Create with AllowPython = true
	id, err := Create(db, Record{
		Name:            "Python Cred",
		Type:            "login",
		Username:        "pyuser",
		PasswordVaultID: "vault-pw-py",
		AllowPython:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := GetByID(db, id)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if !got.AllowPython {
		t.Fatal("expected AllowPython = true")
	}

	// Update to false
	got.AllowPython = false
	if err := Update(db, got); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	got2, _ := GetByID(db, id)
	if got2.AllowPython {
		t.Fatal("expected AllowPython = false after update")
	}
}

func TestListPythonAccessible(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	// Create two credentials: one with AllowPython, one without
	_, err = Create(db, Record{
		Name:        "Allowed",
		Type:        "login",
		Username:    "user1",
		AllowPython: true,
	})
	if err != nil {
		t.Fatalf("Create(allowed) error = %v", err)
	}
	_, err = Create(db, Record{
		Name:     "Not Allowed",
		Type:     "login",
		Username: "user2",
	})
	if err != nil {
		t.Fatalf("Create(not-allowed) error = %v", err)
	}

	items, err := ListPythonAccessible(db)
	if err != nil {
		t.Fatalf("ListPythonAccessible() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListPythonAccessible() returned %d items, want 1", len(items))
	}
	if items[0].Name != "Allowed" {
		t.Fatalf("ListPythonAccessible()[0].Name = %q, want Allowed", items[0].Name)
	}
}

func TestCredentialSSHRequiresHost(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	// SSH without host should fail
	_, err = Create(db, Record{
		Name:     "SSH No Host",
		Type:     "ssh",
		Username: "root",
	})
	if err == nil {
		t.Fatal("Create(ssh without host) should have failed")
	}

	// Login without host should succeed
	_, err = Create(db, Record{
		Name:     "Login No Host",
		Type:     "login",
		Username: "admin",
	})
	if err != nil {
		t.Fatalf("Create(login without host) should succeed, got error = %v", err)
	}
}

func TestValidCredentialTypes(t *testing.T) {
	expected := []string{"ssh", "login", "token"}
	for _, typ := range expected {
		if !ValidCredentialTypes[typ] {
			t.Errorf("ValidCredentialTypes[%q] = false, want true", typ)
		}
	}
	if ValidCredentialTypes["unknown"] {
		t.Error("ValidCredentialTypes[unknown] = true, want false")
	}
}
