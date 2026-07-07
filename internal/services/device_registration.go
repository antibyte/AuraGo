package services

import (
	"database/sql"
	"fmt"
	"strings"

	"aurago/internal/inventory"
	"aurago/internal/security"

	"aurago/internal/uid"
)

// RegisterDevice handles the dual-ingestion logic for enrolling a new device.
func RegisterDevice(db *sql.DB, v *security.Vault, name string, deviceType string, ipAddress string, port int, username string, password string, keyPath string, description string, tags []string, macAddress string) (string, error) {
	if strings.TrimSpace(keyPath) != "" {
		return "", fmt.Errorf("private_key_path is no longer supported; store SSH keys in the credentials registry/vault and link devices by credential_id")
	}

	protocol := protocolForRegisteredDevice(deviceType, username, password)
	if port <= 0 && protocol == inventory.ProtocolSSH {
		port = 22
	}

	var secretValue string
	if password != "" {
		secretValue = password
	}

	var vaultSecretID string
	// Store in Vault only if auth details provided
	if secretValue != "" && v != nil {
		vaultSecretID = "dev-" + uid.New()
		if err := v.WriteSecret(vaultSecretID, secretValue); err != nil {
			return "", fmt.Errorf("failed to store secret in vault: %w", err)
		}
	}

	// Store in Inventory DB
	id, err := inventory.CreateDevice(db, name, deviceType, protocol, ipAddress, port, username, vaultSecretID, "", description, tags, macAddress)
	if err != nil {
		return "", fmt.Errorf("failed to create device record: %w", err)
	}

	return id, nil
}

func protocolForRegisteredDevice(deviceType, username, password string) string {
	if strings.TrimSpace(username) != "" || password != "" {
		return inventory.ProtocolSSH
	}
	switch strings.ToLower(strings.TrimSpace(deviceType)) {
	case "server", "vm", "container", "docker", "nas":
		return inventory.ProtocolSSH
	default:
		return inventory.ProtocolNone
	}
}

// ParseTags converts a comma-separated string into a slice of strings.
func ParseTags(tagsStr string) []string {
	if tagsStr == "" {
		return []string{}
	}
	parts := strings.Split(tagsStr, ",")
	var tags []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}
