package launchpad

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	metadataURL     = "https://raw.githubusercontent.com/homarr-labs/dashboard-icons/main/metadata.json"
	iconCDNBaseSVG  = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg"
	iconCDNBasePNG  = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png"
	iconCDNBaseWEBP = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/webp"
	cacheMaxAge     = 24 * time.Hour
	httpTimeout     = 30 * time.Second
)

// IconSearchResult represents a single icon match from the Homarr database.
type IconSearchResult struct {
	Name    string `json:"name"`
	URLSVG  string `json:"url_svg,omitempty"`
	URLPNG  string `json:"url_png,omitempty"`
	URLWEBP string `json:"url_webp,omitempty"`
}

// metadataEntryV2 mirrors the new metadata.json structure where keys are icon names.
type metadataEntryV2 struct {
	Base    string              `json:"base"`
	Aliases []string            `json:"aliases"`
	Colors  map[string]string   `json:"colors,omitempty"`
}

// SearchIcons searches the cached Homarr icon database.
func SearchIcons(db *sql.DB, query string) ([]IconSearchResult, error) {
	if err := ensureIconCache(db); err != nil {
		return nil, fmt.Errorf("icon cache refresh failed: %w", err)
	}

	q := strings.ToLower(strings.TrimSpace(query))
	var rows *sql.Rows
	var err error
	if q == "" {
		rows, err = db.Query(`SELECT name, has_svg, has_png, has_webp FROM launchpad_icon_cache ORDER BY name ASC LIMIT 200`)
	} else {
		rows, err = db.Query(
			`SELECT name, has_svg, has_png, has_webp FROM launchpad_icon_cache WHERE lower(name) LIKE ? ORDER BY name ASC LIMIT 200`,
			"%"+q+"%",
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to search icon cache: %w", err)
	}
	defer rows.Close()

	var results []IconSearchResult
	for rows.Next() {
		var name string
		var hasSVG, hasPNG, hasWEBP int
		if err := rows.Scan(&name, &hasSVG, &hasPNG, &hasWEBP); err != nil {
			return nil, err
		}
		r := IconSearchResult{Name: name}
		if hasSVG == 1 {
			r.URLSVG = fmt.Sprintf("%s/%s.svg", iconCDNBaseSVG, name)
		}
		if hasPNG == 1 {
			r.URLPNG = fmt.Sprintf("%s/%s.png", iconCDNBasePNG, name)
		}
		if hasWEBP == 1 {
			r.URLWEBP = fmt.Sprintf("%s/%s.webp", iconCDNBaseWEBP, name)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ensureIconCache downloads metadata.json if the cache is empty or older than 24h.
func ensureIconCache(db *sql.DB) error {
	var count int
	var latestCache sql.NullString
	err := db.QueryRow(`SELECT count(*), max(cached_at) FROM launchpad_icon_cache`).Scan(&count, &latestCache)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	needsRefresh := count == 0
	if latestCache.Valid && !needsRefresh {
		t, err := time.Parse(time.RFC3339, latestCache.String)
		if err == nil && time.Since(t) < cacheMaxAge {
			return nil // cache is fresh
		}
		needsRefresh = true
	}

	if !needsRefresh {
		return nil
	}

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(metadataURL)
	if err != nil {
		return fmt.Errorf("failed to fetch metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metadata fetch returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20)) // 20 MB limit
	if err != nil {
		return fmt.Errorf("failed to read metadata body: %w", err)
	}

	// New format: top-level object where keys are icon names
	var root map[string]metadataEntryV2
	if err := json.Unmarshal(body, &root); err != nil {
		return fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM launchpad_icon_cache`); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO launchpad_icon_cache (name, has_svg, has_png, has_webp, has_light, has_dark, cached_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for name, entry := range root {
		if name == "" {
			continue
		}
		// Most icons in the repo have all three formats generated;
		// base tells us the source format but CDN usually serves svg/png/webp.
		// We mark all formats as available – the CDN will 404 if one is missing.
		hasSVG, hasPNG, hasWEBP := 0, 0, 0
		switch strings.ToLower(entry.Base) {
		case "svg":
			hasSVG = 1
			// fallthrough: assume png/webp also exist on CDN
			hasPNG = 1
			hasWEBP = 1
		case "png":
			hasPNG = 1
			hasSVG = 1
			hasWEBP = 1
		default:
			// Unknown base – mark all and let CDN handle it
			hasSVG = 1
			hasPNG = 1
			hasWEBP = 1
		}
		hasLight, hasDark := 0, 0
		if entry.Colors != nil {
			if _, ok := entry.Colors["light"]; ok {
				hasLight = 1
			}
			if _, ok := entry.Colors["dark"]; ok {
				hasDark = 1
			}
		}
		if _, err := stmt.Exec(name, hasSVG, hasPNG, hasWEBP, hasLight, hasDark, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}
