package invasion

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"aurago/internal/config"
	"aurago/internal/invasion/bridge"
	"aurago/internal/security"
)

const EggSecurityPolicyVersion = 1
const EggBootstrapFile = "egg-bootstrap.json"
const eggBootstrapReceiptKey = "egg_shared_bootstrap_receipt"

// EggBootstrap is private, temporary input consumed before an Egg can run tools.
// ExportKey decrypts only VaultData; the existing local Vault key never changes.
type EggBootstrap struct {
	Version   int               `json:"version"`
	Secrets   map[string]string `json:"secrets"`
	ExportKey string            `json:"export_key,omitempty"`
	VaultData []byte            `json:"vault_data,omitempty"`
}

func ValidateEggPolicy(cfg *config.Config) error {
	if cfg == nil || !cfg.EggMode.Enabled {
		return nil
	}
	if cfg.EggMode.SecurityPolicyVersion != EggSecurityPolicyVersion {
		return fmt.Errorf("%s: regenerate the Egg configuration with the current master", bridge.UpgradeRequired)
	}
	if cfg.EggMode.TLSSkipVerify {
		return fmt.Errorf("egg_mode.tls_skip_verify is forbidden; configure a certificate pin or trusted TLS")
	}
	if pin := cfg.EggMode.TLSPinSHA256; pin != "" && !bridge.ValidTLSPin(pin) {
		return fmt.Errorf("invalid egg_mode.tls_pin_sha256")
	}
	if cfg.Agent.AllowUnsandboxedShell && !cfg.Agent.AllowShell {
		return fmt.Errorf("unsandboxed shell requires an explicit shell tool grant")
	}
	return nil
}

// ApplyEggBootstrap merges secrets into the local Vault and retires the input.
// A Vault receipt makes a leftover input harmless after a crash or failed removal.
func ApplyEggBootstrap(dataDir string, vault *security.Vault) error {
	root, err := os.OpenRoot(dataDir)
	if err != nil {
		return fmt.Errorf("open egg bootstrap directory: %w", err)
	}
	defer root.Close()
	info, err := root.Lstat(EggBootstrapFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat egg bootstrap: %w", err)
	}
	if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return fmt.Errorf("egg bootstrap must be a private regular file (0600)")
	}
	file, err := root.Open(EggBootstrapFile)
	if err != nil {
		return fmt.Errorf("open egg bootstrap: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return fmt.Errorf("read egg bootstrap: %w", errors.Join(readErr, closeErr))
	}
	defer clear(data)
	if len(data) > 10<<20 {
		return fmt.Errorf("egg bootstrap exceeds 10 MiB")
	}
	var bootstrap EggBootstrap
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bootstrap); err != nil {
		return fmt.Errorf("invalid egg bootstrap JSON")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("egg bootstrap contains trailing JSON")
	}
	if bootstrap.Version != EggSecurityPolicyVersion || vault == nil {
		return fmt.Errorf("unsupported egg bootstrap version or unavailable Vault")
	}
	digest := sha256.Sum256(data)
	receipt := hex.EncodeToString(digest[:])
	previous, err := vault.ReadSecret(eggBootstrapReceiptKey)
	if err != nil && !errors.Is(err, security.ErrSecretNotFound) {
		return fmt.Errorf("read egg bootstrap receipt: %w", err)
	}
	if previous != receipt {
		secrets := make(map[string]string)
		if len(bootstrap.VaultData) > 0 {
			security.RegisterSensitive(bootstrap.ExportKey)
			exportVault, err := security.NewVault(bootstrap.ExportKey, filepath.Join(dataDir, ".egg-export-unused"))
			if err != nil {
				return fmt.Errorf("invalid egg export key")
			}
			plain, err := exportVault.DecryptBytes(bootstrap.VaultData)
			if err != nil {
				return fmt.Errorf("decrypt egg Vault export: %w", err)
			}
			err = json.Unmarshal(plain, &secrets)
			clear(plain)
			if err != nil || secrets == nil {
				return fmt.Errorf("invalid egg Vault export")
			}
		}
		for key, value := range bootstrap.Secrets {
			if key == "" || key == eggBootstrapReceiptKey {
				return fmt.Errorf("invalid egg bootstrap secret name")
			}
			secrets[key] = value
		}
		for _, value := range secrets {
			security.RegisterSensitive(value)
		}
		if key, present := secrets["egg_shared_key"]; present {
			decoded, err := hex.DecodeString(key)
			if err != nil || len(decoded) != 32 {
				return fmt.Errorf("invalid bootstrap shared key")
			}
		}
		secrets[eggBootstrapReceiptKey] = receipt
		if err := vault.WriteSecrets(secrets, nil); err != nil {
			return fmt.Errorf("persist egg bootstrap: %w", err)
		}
	}
	if err := root.Remove(EggBootstrapFile); err != nil {
		return fmt.Errorf("remove applied egg bootstrap: %w", err)
	}
	return nil
}
