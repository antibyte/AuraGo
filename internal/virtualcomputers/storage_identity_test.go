package virtualcomputers

import (
	"testing"

	"aurago/internal/config"
)

func TestNormalizeStorageMode(t *testing.T) {
	if got := NormalizeStorageMode("", ""); got != StorageModeManagedGarage {
		t.Fatalf("empty = %q", got)
	}
	if got := NormalizeStorageMode("", "minio:9000"); got != StorageModeExternalS3 {
		t.Fatalf("legacy endpoint = %q", got)
	}
	if got := NormalizeStorageMode("MANAGED_GARAGE", ""); got != StorageModeManagedGarage {
		t.Fatalf("managed = %q", got)
	}
}

func TestStorageIdentityHashChangesWithHost(t *testing.T) {
	a := StorageIdentity{
		Mode: StorageModeManagedGarage, Endpoint: config.ManagedGarageEndpoint,
		Bucket: config.ManagedGarageBucket, Region: config.ManagedGarageRegion,
		ControlPlaneMode: "ssh_host", ControlPlaneHost: "a.local", InstallDir: "/opt/boring-computers",
	}
	b := a
	b.ControlPlaneHost = "b.local"
	if a.Hash() == b.Hash() {
		t.Fatalf("host change must change identity hash")
	}
	if a.Hash() == "" || len(a.Hash()) != 64 {
		t.Fatalf("hash length = %d", len(a.Hash()))
	}
}

func TestGenerateGarageSecrets(t *testing.T) {
	ak, sk, rpc, err := GenerateGarageSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if len(ak) != 32 || len(sk) != 64 || len(rpc) != 64 {
		t.Fatalf("unexpected lengths ak=%d sk=%d rpc=%d", len(ak), len(sk), len(rpc))
	}
}

func TestManagedGarageDataDir(t *testing.T) {
	if got := ManagedGarageDataDir("/opt/boring-computers/"); got != "/opt/boring-computers/data/sidecars/garage" {
		t.Fatalf("got %q", got)
	}
}
