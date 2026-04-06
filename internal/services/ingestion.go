package services

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"aurago/internal/inventory"
	"aurago/internal/security"

	"aurago/internal/uid"
)

// RegisterDevice handles the dual-ingestion logic for enrolling a new device.
func RegisterDevice(db *sql.DB, v *security.Vault, name string, deviceType string, ipAddress string, port int, username string, password string, keyPath string, description string, tags []string, macAddress string) (string, error) {
	if port <= 0 {
		port = 22
	}

	var secretValue string
	if keyPath != "" {
		data, err := os.ReadFile(keyPath)
		if err != nil {
			return "", fmt.Errorf("failed to read private key at %s: %w", keyPath, err)
		}
		secretValue = string(data)
	} else if password != "" {
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
	id, err := inventory.CreateDevice(db, name, deviceType, ipAddress, port, username, vaultSecretID, "", description, tags, macAddress)
	if err != nil {
		return "", fmt.Errorf("failed to create device record: %w", err)
	}

	return id, nil
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
