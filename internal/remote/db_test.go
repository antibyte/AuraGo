package remote

import (
	"path/filepath"
	"testing"
	"time"
)

func tempDB(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "remote_test.db")
}

// ── InitDB ──────────────────────────────────────────────────────────────────

func TestInitDB(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	for _, table := range []string{"remote_devices", "remote_enrollments", "remote_audit_log"} {
		var count int
		err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		if err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %s not found", table)
		}
	}
}

// ── Device CRUD ─────────────────────────────────────────────────────────────

func TestDeviceCRUD(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create
	device := DeviceRecord{
		Name:          "server-01",
		Hostname:      "ubuntu-node",
		OS:            "linux",
		Arch:          "amd64",
		IPAddress:     "192.168.1.50",
		Status:        "approved",
		ReadOnly:      false,
		AllowedPaths:  []string{"/home", "/var/log"},
		SharedKeyHash: "abc123hash",
		Version:       "1.0.0",
		Tags:          []string{"production"},
	}
	id, err := CreateDevice(db, device)
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if id == "" {
		t.Fatal("CreateDevice returned empty ID")
	}

	// Read by ID
	got, err := GetDevice(db, id)
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if got.Name != "server-01" {
		t.Errorf("Name = %q; want %q", got.Name, "server-01")
	}
	if got.OS != "linux" {
		t.Errorf("OS = %q; want %q", got.OS, "linux")
	}
	if len(got.AllowedPaths) != 2 {
		t.Errorf("AllowedPaths length = %d; want 2", len(got.AllowedPaths))
	}

	// Read by name
	got2, err := GetDeviceByName(db, "server-01")
	if err != nil {
		t.Fatalf("GetDeviceByName: %v", err)
	}
	if got2.ID != id {
		t.Errorf("GetDeviceByName ID = %q; want %q", got2.ID, id)
	}

	// List
	devices, err := ListDevices(db)
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("ListDevices count = %d; want 1", len(devices))
	}

	// Update
	got.Name = "server-01-renamed"
	got.ReadOnly = true
	if err := UpdateDevice(db, got); err != nil {
		t.Fatalf("UpdateDevice: %v", err)
	}
	updated, _ := GetDevice(db, id)
	if updated.Name != "server-01-renamed" {
		t.Errorf("after update Name = %q; want %q", updated.Name, "server-01-renamed")
	}
	if !updated.ReadOnly {
		t.Error("after update ReadOnly should be true")
	}

	// Update status
	if err := UpdateDeviceStatus(db, id, "connected"); err != nil {
		t.Fatalf("UpdateDeviceStatus: %v", err)
	}
	statusCheck, _ := GetDevice(db, id)
	if statusCheck.Status != "connected" {
		t.Errorf("Status = %q; want %q", statusCheck.Status, "connected")
	}

	// Delete
	if err := DeleteDevice(db, id); err != nil {
		t.Fatalf("DeleteDevice: %v", err)
	}
	_, err = GetDevice(db, id)
	if err == nil {
		t.Error("GetDevice succeeded after delete; expected error")
	}
}

func TestGetDeviceNotFound(t *testing.T) {
	db, _ := InitDB(tempDB(t))
	defer db.Close()
	_, err := GetDevice(db, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent device")
	}
}

// ── Enrollment ──────────────────────────────────────────────────────────────

func TestEnrollmentCRUD(t *testing.T) {
	db, _ := InitDB(tempDB(t))
	defer db.Close()

	e := EnrollmentRecord{
		TokenHash:  "hash-of-token-abc",
		DeviceName: "new-server",
		ExpiresAt:  time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
	}
	id, err := CreateEnrollment(db, e)
	if err != nil {
		t.Fatalf("CreateEnrollment: %v", err)
	}
	if id == "" {
		t.Fatal("CreateEnrollment returned empty ID")
	}

	got, err := GetEnrollmentByTokenHash(db, "hash-of-token-abc")
	if err != nil {
		t.Fatalf("GetEnrollmentByTokenHash: %v", err)
	}
	if got.DeviceName != "new-server" {
		t.Errorf("DeviceName = %q; want %q", got.DeviceName, "new-server")
	}
	if got.Used {
		t.Error("enrollment should not be marked as used")
	}

	// Mark used
	if err := MarkEnrollmentUsed(db, id, "device-xyz"); err != nil {
		t.Fatalf("MarkEnrollmentUsed: %v", err)
	}
	marked, _ := GetEnrollmentByTokenHash(db, "hash-of-token-abc")
	if !marked.Used {
		t.Error("enrollment should be marked as used")
	}
}

func TestCleanExpiredEnrollments(t *testing.T) {
	db, _ := InitDB(tempDB(t))
	defer db.Close()

	// Create an expired enrollment
	e := EnrollmentRecord{
		TokenHash:  "expired-hash",
		DeviceName: "old-server",
		ExpiresAt:  time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
	}
	CreateEnrollment(db, e)

	// Create a valid enrollment
	e2 := EnrollmentRecord{
		TokenHash:  "valid-hash",
		DeviceName: "new-server",
		ExpiresAt:  time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
	}
	CreateEnrollment(db, e2)

	if err := CleanExpiredEnrollments(db); err != nil {
		t.Fatalf("CleanExpiredEnrollments: %v", err)
	}

	// expired should be gone
	_, err := GetEnrollmentByTokenHash(db, "expired-hash")
	if err == nil {
		t.Error("expired enrollment should have been cleaned")
	}

	// valid should remain
	_, err = GetEnrollmentByTokenHash(db, "valid-hash")
	if err != nil {
		t.Errorf("valid enrollment should still exist: %v", err)
	}
}

// ── Audit Log ───────────────────────────────────────────────────────────────

func TestAuditLog(t *testing.T) {
	db, _ := InitDB(tempDB(t))
	defer db.Close()

	if err := LogAudit(db, "dev-1", "shell_exec", "ls -la", "ok", 42); err != nil {
		t.Fatalf("LogAudit: %v", err)
	}
	if err := LogAudit(db, "dev-1", "file_read", "/etc/hosts", "ok", 5); err != nil {
		t.Fatalf("LogAudit: %v", err)
	}
	if err := LogAudit(db, "dev-2", "sysinfo", "", "ok", 10); err != nil {
		t.Fatalf("LogAudit: %v", err)
	}

	// List all for dev-1
	entries, err := ListAuditLog(db, "dev-1", 100)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("audit entries for dev-1 = %d; want 2", len(entries))
	}

	// List with limit
	limited, err := ListAuditLog(db, "dev-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 {
		t.Errorf("limited entries = %d; want 1", len(limited))
	}

	// List all devices (empty device_id)
	all, err := ListAuditLog(db, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("all entries = %d; want 3", len(all))
	}
}
