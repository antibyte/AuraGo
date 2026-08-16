package virtualcomputers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"aurago/internal/config"
)

// NormalizeStorageMode returns managed_garage or external_s3.
// Legacy empty mode with a non-empty endpoint becomes external_s3.
func NormalizeStorageMode(mode, endpoint string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	endpoint = strings.TrimSpace(endpoint)
	switch mode {
	case StorageModeManagedGarage, StorageModeExternalS3:
		return mode
	case "":
		if endpoint != "" {
			return StorageModeExternalS3
		}
		return StorageModeManagedGarage
	default:
		if endpoint != "" {
			return StorageModeExternalS3
		}
		return StorageModeManagedGarage
	}
}

// EffectiveStorageFromConfig returns runtime storage settings for boringd and tests.
func EffectiveStorageFromConfig(vc config.VirtualComputersConfig) StorageConfig {
	endpoint, bucket, region, useSSL, _, _ := config.EffectiveVirtualComputersStorage(vc)
	mode := NormalizeStorageMode(vc.Storage.Mode, vc.Storage.Endpoint)
	return StorageConfig{
		Mode:     mode,
		Endpoint: endpoint,
		Bucket:   bucket,
		Region:   region,
		UseSSL:   useSSL,
	}
}

// EffectiveCredentials returns access/secret keys for the active storage mode.
func EffectiveCredentials(vc config.VirtualComputersConfig) (accessKey, secretKey string) {
	_, _, _, _, accessKey, secretKey = config.EffectiveVirtualComputersStorage(vc)
	return accessKey, secretKey
}

// StorageIdentity describes the object-store binding for volumes.
// Any field change is a storage switch (including control-plane host for managed garage).
type StorageIdentity struct {
	Mode             string
	Endpoint         string
	Bucket           string
	Region           string
	UseSSL           bool
	ControlPlaneMode string
	ControlPlaneHost string
	InstallDir       string
}

// StorageIdentityFromConfig builds identity from AuraGo virtual computers config.
func StorageIdentityFromConfig(vc config.VirtualComputersConfig) StorageIdentity {
	eff := EffectiveStorageFromConfig(vc)
	id := StorageIdentity{
		Mode:     eff.Mode,
		Endpoint: strings.TrimSpace(eff.Endpoint),
		Bucket:   strings.TrimSpace(eff.Bucket),
		Region:   strings.TrimSpace(eff.Region),
		UseSSL:   eff.UseSSL,
	}
	if eff.Mode == StorageModeManagedGarage {
		id.ControlPlaneMode = strings.ToLower(strings.TrimSpace(vc.ControlPlane.Mode))
		id.ControlPlaneHost = strings.ToLower(strings.TrimSpace(vc.ControlPlane.Host))
		id.InstallDir = strings.TrimSpace(vc.ControlPlane.InstallDir)
	}
	return id
}

// Hash returns a stable hex SHA-256 of the identity.
func (id StorageIdentity) Hash() string {
	ssl := "0"
	if id.UseSSL {
		ssl = "1"
	}
	canonical := strings.Join([]string{
		NormalizeStorageMode(id.Mode, id.Endpoint),
		strings.TrimSpace(id.Endpoint),
		strings.TrimSpace(id.Bucket),
		strings.TrimSpace(id.Region),
		ssl,
		strings.ToLower(strings.TrimSpace(id.ControlPlaneMode)),
		strings.ToLower(strings.TrimSpace(id.ControlPlaneHost)),
		strings.TrimSpace(id.InstallDir),
	}, "\n")
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

// String is a non-secret debug form.
func (id StorageIdentity) String() string {
	return fmt.Sprintf("mode=%s endpoint=%s bucket=%s region=%s ssl=%v cp=%s@%s dir=%s",
		id.Mode, id.Endpoint, id.Bucket, id.Region, id.UseSSL,
		id.ControlPlaneMode, id.ControlPlaneHost, id.InstallDir)
}

// ManagedGarageDataDir returns the host path for Garage sidecar data under install_dir.
func ManagedGarageDataDir(installDir string) string {
	installDir = strings.TrimRight(strings.TrimSpace(installDir), "/")
	if installDir == "" {
		installDir = "/opt/boring-computers"
	}
	return installDir + "/data/sidecars/garage"
}
