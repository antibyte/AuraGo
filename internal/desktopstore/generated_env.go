package desktopstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"aurago/internal/dbutil"
	"aurago/internal/security"
)

func splitGeneratedEnv(env []string, entry CatalogEntry) ([]string, map[string]string) {
	var values map[string]string
	remaining := env
	for _, secret := range entry.GeneratedSecrets {
		if secret.Env == "" {
			continue
		}
		value, present := envValue(remaining, secret.Env)
		if !present {
			continue
		}
		if values == nil {
			values = make(map[string]string)
		}
		if value != "" {
			values[storeSecretVaultKey(entry.ID, strings.ToLower(strings.TrimSpace(secret.Key)))] = value
		}
		remaining = slices.DeleteFunc(slices.Clone(remaining), func(item string) bool {
			return strings.HasPrefix(item, secret.Env+"=")
		})
	}
	return remaining, values
}

func (s *Service) addGeneratedRuntimeEnv(app InstalledApp, spec *ContainerSpec) error {
	if IsRelayApp(app.AppID) {
		return nil
	}
	for _, secret := range s.catalogByID[app.AppID].GeneratedSecrets {
		if secret.Env == "" {
			continue
		}
		if s.cfg.Secrets == nil {
			return fmt.Errorf("Store Vault is unavailable")
		}
		key := storeSecretVaultKey(app.AppID, strings.ToLower(strings.TrimSpace(secret.Key)))
		value, err := s.cfg.Secrets.ReadSecret(key)
		if errors.Is(err, security.ErrSecretNotFound) {
			// A newly introduced catalog secret may be absent from the old runtime.
			if !hasStoreSecretRef(app.SecretRefs, key) {
				continue
			}
		}
		if err != nil {
			return fmt.Errorf("read Store runtime secret %s: %w", secret.Key, err)
		}
		if value == "" {
			return fmt.Errorf("missing Store Vault secret %s; restore the matching Vault", secret.Key)
		}
		security.RegisterSensitive(value)
		spec.Env = appendEnvValue(spec.Env, secret.Env, value)
	}
	return nil
}

func hasStoreSecretRef(refs []SecretRef, key string) bool {
	for _, ref := range refs {
		if ref.VaultKey == key {
			return true
		}
	}
	return false
}

// SanitizeBackup makes a private, staged Store snapshot safe to archive even if
// the lazy Store service has never opened the live database on this version.
func SanitizeBackup(ctx context.Context, snapshot string, secrets SecretStore) error {
	db, err := dbutil.Open(snapshot)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=DELETE"); err != nil {
		return err
	}
	return migrateGeneratedEnv(ctx, db, secrets, DefaultCatalog())
}

func migrateGeneratedEnv(ctx context.Context, db *sql.DB, secrets SecretStore, catalog []CatalogEntry) error {
	var hasEnv bool
	if err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pragma_table_info('desktop_store_apps') WHERE name='env_json')").Scan(&hasEnv); err != nil || !hasEnv {
		return err
	}
	entries := make(map[string]CatalogEntry)
	for _, entry := range catalog {
		if len(entry.GeneratedSecrets) != 0 {
			entries[entry.ID] = entry
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT app_id, env_json FROM desktop_store_apps")
	if err != nil {
		return err
	}
	defer rows.Close()
	type envUpdate struct {
		id     string
		env    []string
		values map[string]string
	}
	var updates []envUpdate
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		entry, known := entries[id]
		if !known {
			continue
		}
		var env []string
		if err := json.Unmarshal([]byte(raw), &env); err != nil {
			return fmt.Errorf("decode stored environment for %s: %w", id, err)
		}
		if clean, values := splitGeneratedEnv(env, entry); values != nil {
			updates = append(updates, envUpdate{id, clean, values})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA secure_delete=ON"); err != nil {
		return err
	}
	for _, update := range updates {
		for key, value := range update.values {
			if secrets == nil {
				return fmt.Errorf("Store Vault is unavailable")
			}
			existing, err := secrets.ReadSecret(key)
			if err != nil && !errors.Is(err, security.ErrSecretNotFound) {
				return fmt.Errorf("read legacy Store Vault key %s: %w", key, err)
			}
			if existing == "" {
				var writeErr error
				if vault, ok := secrets.(*security.Vault); ok {
					if errors.Is(err, security.ErrSecretNotFound) {
						writeErr = vault.WriteSecretIfAbsent(key, value)
					} else {
						writeErr = vault.CompareAndSwapSecret(key, "", value)
					}
				} else {
					writeErr = secrets.WriteSecret(key, value)
				}
				if writeErr != nil && !errors.Is(writeErr, security.ErrSecretExists) && !errors.Is(writeErr, security.ErrSecretChanged) {
					return fmt.Errorf("preserve legacy Store Vault key %s: %w", key, writeErr)
				}
			}
			security.RegisterSensitive(value)
		}
		clean, err := json.Marshal(update.env)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE desktop_store_apps SET env_json=? WHERE app_id=?", string(clean), update.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
