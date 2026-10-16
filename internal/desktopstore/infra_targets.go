package desktopstore

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// InfraTarget is the minimum persisted Software Store association needed by
// the Infra app. It deliberately contains no runtime configuration or secrets.
type InfraTarget struct {
	AppID       string
	ContainerID string
}

// ReadInfraTargets reads existing installed-app associations without opening
// the Store service, running migrations, recovering operations, or reconciling
// Desktop state. A missing database/table is an empty target list.
func ReadInfraTargets(ctx context.Context, dbPath string) ([]InfraTarget, error) {
	if strings.TrimSpace(dbPath) == "" {
		return []InfraTarget{}, nil
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []InfraTarget{}, nil
		}
		return nil, fmt.Errorf("inspect Desktop Store target database: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("Desktop Store target database is not a regular file")
	}

	abs, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("resolve Desktop Store target database: %w", err)
	}
	uriPath := filepath.ToSlash(abs)
	if runtime.GOOS == "windows" {
		uriPath = "/" + uriPath
	}
	dsn := (&url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open Desktop Store target database read-only: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	var tableCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='desktop_store_apps'`).Scan(&tableCount); err != nil {
		return nil, fmt.Errorf("inspect Desktop Store target schema: %w", err)
	}
	if tableCount == 0 {
		return []InfraTarget{}, nil
	}
	rows, err := db.QueryContext(ctx, `SELECT app_id, container_id FROM desktop_store_apps
		WHERE status IN (?, ?) AND trim(container_id) <> '' ORDER BY app_id`, AppStatusRunning, AppStatusStopped)
	if err != nil {
		return nil, fmt.Errorf("read Desktop Store targets: %w", err)
	}
	defer rows.Close()
	targets := make([]InfraTarget, 0)
	for rows.Next() {
		var target InfraTarget
		if err := rows.Scan(&target.AppID, &target.ContainerID); err != nil {
			return nil, fmt.Errorf("read Desktop Store target row: %w", err)
		}
		target.AppID = strings.TrimSpace(target.AppID)
		target.ContainerID = strings.TrimSpace(target.ContainerID)
		if target.AppID == "" || target.ContainerID == "" {
			continue
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read Desktop Store targets: %w", err)
	}
	return targets, nil
}
