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
