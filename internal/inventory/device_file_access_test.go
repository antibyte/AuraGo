package inventory

import (
	"path/filepath"
	"testing"
)

func TestDeviceFileAccessMissingRowIsOff(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "inv.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fa, err := GetDeviceFileAccess(db, "nope")
	if err != nil {
		t.Fatal(err)
	}
	if fa.Mode != FileAccessOff || fa.SFTPPort != 22 || fa.SMBShare != "C$" || fa.CredentialID != "" {
		t.Fatalf("unexpected default %+v", fa)
	}
}

func TestDeviceFileAccessSetGetUpsertListDelete(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "inv.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := SetDeviceFileAccess(db, "d1", FileAccess{Mode: "smb", SFTPPort: 2222, SMBShare: " Data ", CredentialID: " c1 "}); err != nil {
		t.Fatal(err)
	}
	fa, _ := GetDeviceFileAccess(db, "d1")
	if fa.Mode != "smb" || fa.SFTPPort != 2222 || fa.SMBShare != "Data" || fa.CredentialID != "c1" {
		t.Fatalf("round trip %+v", fa)
	}
	if err := SetDeviceFileAccess(db, "d1", FileAccess{Mode: "auto"}); err != nil {
		t.Fatal(err)
	}
	fa, _ = GetDeviceFileAccess(db, "d1")
	if fa.Mode != "auto" || fa.SFTPPort != 22 || fa.SMBShare != "C$" {
		t.Fatalf("upsert %+v", fa)
	}
	all, err := ListDeviceFileAccess(db)
	if err != nil || len(all) != 1 || all["d1"].Mode != "auto" {
		t.Fatalf("list %v %v", all, err)
	}
	if err := DeleteDeviceFileAccess(db, "d1"); err != nil {
		t.Fatal(err)
	}
	fa, _ = GetDeviceFileAccess(db, "d1")
	if fa.Mode != FileAccessOff {
		t.Fatalf("delete %+v", fa)
	}
}

func TestNormalizeFileAccessRejectsBadInput(t *testing.T) {
	bad := []FileAccess{
		{Mode: "weird"},
		{Mode: "sftp", SFTPPort: -1},
		{Mode: "sftp", SFTPPort: 70000},
		{Mode: "smb", SMBShare: "a/b"},
		{Mode: "smb", SMBShare: `a\b`},
		{Mode: "smb", SMBShare: "ipc$"},
		{Mode: "smb", SMBShare: "a\r\nb"},
	}
	for _, fa := range bad {
		if _, err := NormalizeFileAccess(fa); err == nil {
			t.Errorf("expected error for %+v", fa)
		}
	}
	if _, err := NormalizeFileAccess(FileAccess{Mode: "auto", SMBShare: string(make([]byte, 81))}); err == nil {
		t.Error("expected error for over-long share")
	}
	fa, err := NormalizeFileAccess(FileAccess{Mode: " SFTP ", SMBShare: "  "})
	if err != nil || fa.Mode != "sftp" || fa.SMBShare != "C$" || fa.SFTPPort != 22 {
		t.Fatalf("normalize %+v %v", fa, err)
	}
}

func TestDeleteDeviceCascadesFileAccess(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "inv.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := AddDevice(db, DeviceRecord{ID: "dev1", Name: "x", Type: "server", Protocol: ProtocolRDP, IPAddress: "10.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	if err := SetDeviceFileAccess(db, "dev1", FileAccess{Mode: "auto"}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteDevice(db, "dev1"); err != nil {
		t.Fatal(err)
	}
	all, _ := ListDeviceFileAccess(db)
	if len(all) != 0 {
		t.Fatalf("row survived delete: %v", all)
	}
}
