package desktopstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aurago/internal/launchpad"
)

// SQLiteLaunchpadAdapter writes managed store links into the existing Launchpad
// table. Store links intentionally use aurago-store:// URLs so the desktop UI
// can open the matching generated desktop app instead of a new browser tab.
type SQLiteLaunchpadAdapter struct {
	DB      *sql.DB
	DataDir string
}

func (a SQLiteLaunchpadAdapter) UpsertStoreLink(ctx context.Context, link LaunchpadLink) (string, error) {
	if a.DB == nil {
		return "", nil
	}
	link.ID = strings.TrimSpace(link.ID)
	if link.ID == "" {
		return "", fmt.Errorf("managed launchpad link id is required")
	}
	if !strings.HasPrefix(link.URL, "aurago-store://") {
		return "", fmt.Errorf("managed store link URL must use aurago-store://")
	}
	iconPath := link.IconPath
	if strings.HasPrefix(iconPath, "http://") || strings.HasPrefix(iconPath, "https://") {
		if result, err := launchpad.DownloadIcon(a.DataDir, iconPath, link.ID); err == nil && result.LocalPath != "" {
			iconPath = result.LocalPath
		}
	}
	tagsJSON, _ := json.Marshal(link.Tags)
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := a.DB.ExecContext(ctx, `INSERT INTO launchpad_links (id, title, url, description, icon_path, category, tags, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			url = excluded.url,
			description = excluded.description,
			icon_path = excluded.icon_path,
			category = excluded.category,
			tags = excluded.tags,
			sort_order = excluded.sort_order,
			updated_at = excluded.updated_at`,
		link.ID, link.Title, link.URL, link.Description, iconPath, link.Category, string(tagsJSON), link.SortOrder, now, now)
	if err != nil {
		return "", fmt.Errorf("upsert managed launchpad link: %w", err)
	}
	return link.ID, nil
}

func (a SQLiteLaunchpadAdapter) DeleteStoreLink(ctx context.Context, id string) error {
	if a.DB == nil || strings.TrimSpace(id) == "" {
		return nil
	}
	_, err := launchpad.Delete(a.DB, id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no rows") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil
		}
		return err
	}
	return nil
}
