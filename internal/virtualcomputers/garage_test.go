package virtualcomputers

import (
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestGarageEnsureScriptFailClosedKeyImport(t *testing.T) {
	gm := GarageManager{
		InstallDir:  "/opt/boring-computers",
		AccessKeyID: "aabbccddeeff00112233445566778899",
		SecretKey:   strings.Repeat("ab", 32),
		RPCSecret:   strings.Repeat("cd", 32),
		Fingerprint: "fp-test",
	}
	script := gm.ensureScript()
	for _, want := range []string{
		config.ManagedGarageImage,
		config.ManagedGarageContainerName,
		"aurago.managed=boring-garage",
		"127.0.0.1:3900:3900",
		"--single-node",
		"key import",
		"bucket create",
		"bucket allow",
		"garage_key_present",
		"metadata_fsync = true",
		"db_engine = \"sqlite\"",
		"replication_factor = 1",
		"block_ram_buffer_max = \"64MiB\"",
		"rpc_secret_file",
		"exit 15",
		"Vault access key",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("ensure script missing %q", want)
		}
	}
	if strings.Contains(script, `/garage -c /etc/garage/garage.toml key new`) {
		t.Fatal("ensure script must never create unmanaged keys via key new")
	}
	if strings.Contains(script, "server --single-node --default-bucket") || strings.Contains(script, "-e GARAGE_DEFAULT_") {
		t.Fatal("bootstrap must not expose Vault credentials through Garage default-bucket environment variables")
	}
	if !strings.Contains(script, "NEED_BOOTSTRAP=1") {
		t.Fatal("ensure script must force bootstrap when Vault key is missing")
	}
}

func TestGarageEnsureSnippetSkipsWhenNotManaged(t *testing.T) {
	snippet := garageEnsureSnippet(SetupInstallOptions{
		AllowVolumes: true, StorageMode: StorageModeExternalS3, ProjectGarage: true,
	})
	if !strings.Contains(snippet, "managed Garage not requested") {
		t.Fatalf("unexpected snippet: %s", snippet)
	}
	if strings.Contains(snippet, "docker pull") {
		t.Fatal("external mode must not pull garage image")
	}
}

func TestInstallScriptClearsManagedS3ByModeNotEndpoint(t *testing.T) {
	manager := SetupManager{InstallOptions: SetupInstallOptions{
		AllowVolumes: true, ProjectGarage: true, StorageMode: StorageModeManagedGarage,
		S3Endpoint: config.ManagedGarageEndpoint, S3Bucket: config.ManagedGarageBucket,
		S3AccessKeyID: "ak", S3SecretKey: "sk", GarageRPCSecret: "rpc",
	}}
	script := manager.installScript()
	if !strings.Contains(script, `STORAGE_MODE_VALUE='managed_garage'`) {
		t.Fatal("install script missing STORAGE_MODE_VALUE")
	}
	if !strings.Contains(script, `[ "${STORAGE_MODE_VALUE}" = "managed_garage" ] && [ "${GARAGE_OK:-0}" != "1" ]`) {
		t.Fatal("install script must clear S3 only for managed_garage mode")
	}
	if strings.Contains(script, `[ "${BORING_S3_ENDPOINT_VALUE}" = "127.0.0.1:3900" ]`) {
		t.Fatal("install script must not use endpoint heuristic for S3 clear")
	}
}

func TestStorageIdentityHashStable(t *testing.T) {
	a := StorageIdentity{Mode: StorageModeManagedGarage, Endpoint: config.ManagedGarageEndpoint,
		Bucket: config.ManagedGarageBucket, Region: config.ManagedGarageRegion,
		ControlPlaneMode: "local_host", InstallDir: "/opt/boring-computers"}
	b := a
	if a.Hash() != b.Hash() {
		t.Fatal("identical identity hashes must match")
	}
}
