package virtualcomputers

import (
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestFromAuraConfigMapsStorageAndLedgerPath(t *testing.T) {
	cfg := &config.Config{}
	cfg.SQLite.VirtualComputersPath = "data/vc.db"
	cfg.VirtualComputers.Storage.Endpoint = "minio.home:9000"
	cfg.VirtualComputers.Storage.Bucket = "vc-data"
	cfg.VirtualComputers.Storage.Region = "home-1"
	cfg.VirtualComputers.Storage.UseSSL = true

	got := FromAuraConfig(cfg)
	if got.LedgerPath != "data/vc.db" || got.Storage.Endpoint != "minio.home:9000" || got.Storage.Bucket != "vc-data" || got.Storage.Region != "home-1" || !got.Storage.UseSSL {
		t.Fatalf("tool config = %+v", got)
	}
}

func TestFromAuraConfigRegistersStorageCredentialsAsSensitive(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualComputers.S3AccessKeyID = "storage-access-sensitive"
	cfg.VirtualComputers.S3SecretKey = "storage-secret-sensitive"
	_ = FromAuraConfig(cfg)
	redacted := security.Scrub("storage-access-sensitive storage-secret-sensitive")
	if strings.Contains(redacted, "storage-access-sensitive") || strings.Contains(redacted, "storage-secret-sensitive") {
		t.Fatalf("storage credentials were not registered as sensitive: %s", redacted)
	}
}
