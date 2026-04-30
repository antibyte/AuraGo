package server

import (
	"encoding/json"
	"fmt"
	"os"

	"aurago/internal/config"
	"aurago/internal/security"
)

func exportTokenStore(vault *security.Vault, tokenFilePath, password string) ([]byte, int, error) {
	if vault == nil || tokenFilePath == "" {
		return nil, 0, nil
	}
	data, err := os.ReadFile(tokenFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("token store read: %w", err)
	}
	if len(data) == 0 {
		return nil, 0, nil
	}
	plain, err := vault.DecryptBytes(data)
	if err != nil {
		return nil, 0, fmt.Errorf("token store decrypt: %w", err)
	}
	var tokens []security.Token
	if err := json.Unmarshal(plain, &tokens); err != nil {
		return nil, 0, fmt.Errorf("token store parse: %w", err)
	}
	if len(tokens) == 0 {
		return nil, 0, nil
	}
	blob, err := encryptBackupPasswordBlob(tokenStoreMagic, plain, password)
	if err != nil {
		return nil, 0, err
	}
	return blob, len(tokens), nil
}

func importTokenStore(vault *security.Vault, encData []byte, password, tokenFilePath string) (int, error) {
	if vault == nil || tokenFilePath == "" {
		return 0, nil
	}
	plain, err := decryptBackupPasswordBlob(tokenStoreMagic, encData, password)
	if err != nil {
		return 0, err
	}
	var tokens []security.Token
	if err := json.Unmarshal(plain, &tokens); err != nil {
		return 0, fmt.Errorf("token store parse: %w", err)
	}
	ciphertext, err := vault.EncryptBytes(plain)
	if err != nil {
		return 0, fmt.Errorf("token store encrypt: %w", err)
	}
	if err := config.WriteFileAtomic(tokenFilePath, ciphertext, 0o600); err != nil {
		return 0, fmt.Errorf("token store write: %w", err)
	}
	return len(tokens), nil
}
