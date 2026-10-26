package server

import (
	"log/slog"

	"aurago/internal/security"
)

// Both the legacy writer and setup's staged writes use the same field extraction.
type configSecretWriter interface {
	WriteSecret(string, string) error
}

type configSecretMutations []vaultMutation

func (mutations *configSecretMutations) WriteSecret(key, value string) error {
	*mutations = append(*mutations, vaultMutation{key: key, value: value})
	return nil
}

func configSecretWriterMissing(writer configSecretWriter) bool {
	vault, isVault := writer.(*security.Vault)
	return writer == nil || isVault && vault == nil
}

// prepareConfigSecrets strips credentials without touching the Vault. The
// shared config transaction applies these writes only after saving the YAML.
func prepareConfigSecrets(patch map[string]interface{}, logger *slog.Logger) ([]vaultMutation, error) {
	var mutations configSecretMutations
	if err := extractRecursive(patch, "", &mutations, logger); err != nil {
		return nil, err
	}
	return mutations, nil
}
