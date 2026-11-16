package desktopstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
)

var ErrDeepTutorRequiresLogin = errors.New("Store model relays require AuraGo login; uninstall DeepTutor and Open WebUI before disabling login")

func RequireDeepTutorLogin(enabled bool, passwordHash string) error {
	if !enabled || passwordHash == "" {
		return ErrDeepTutorRequiresLogin
	}
	return nil
}

// DeepTutorNeedsLogin reads only existing metadata, including admitted installs.
// It never initializes the Store or starts Docker/recovery/reconciliation.
func DeepTutorNeedsLogin(ctx context.Context, dbPath string) (bool, error) {
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		return false, err
	}
	path := filepath.ToSlash(abs)
	if runtime.GOOS == "windows" {
		path = "/" + path
	}
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}).String())
	if err != nil {
		return false, err
	}
	defer db.Close()
	for _, table := range []string{"desktop_store_apps", "desktop_store_operations"} {
		var exists int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&exists); err != nil {
			return false, fmt.Errorf("read DeepTutor installation metadata: %w", err)
		}
		if exists == 0 {
			continue
		}
		query := `SELECT COUNT(*) FROM desktop_store_apps WHERE app_id = ? OR (app_id = ? AND secret_refs_json LIKE '%llm_relay_token%')`
		if table == "desktop_store_operations" {
			query = `SELECT COUNT(*) FROM desktop_store_operations WHERE (app_id = ? OR (app_id = ? AND type IN ('install', 'update'))) AND status IN ('pending', 'running')`
		}
		var count int
		if err := db.QueryRowContext(ctx, query, DeepTutorAppID, OpenWebUIAppID).Scan(&count); err != nil {
			return false, fmt.Errorf("read DeepTutor installation metadata: %w", err)
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}
