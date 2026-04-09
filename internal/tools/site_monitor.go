package tools

import (
	"aurago/internal/dbutil"
	"aurago/internal/security"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// siteMonitorDB holds the lazily-initialized database connection.
// siteMonitorMu protects access to siteMonitorDB to allow safe concurrent queries
// and clean shutdown via CloseSiteMonitorDB.
var (
	siteMonitorDB   *sql.DB
	siteMonitorOnce sync.Once
	siteMonitorErr  error
	siteMonitorMu   sync.RWMutex
)

type siteMonitorResult struct {
	Status   string      `json:"status"`
	Message  string      `json:"message,omitempty"`
	Monitors interface{} `json:"monitors,omitempty"`
	Changes  interface{} `json:"changes,omitempty"`
	Changed  *bool       `json:"changed,omitempty"`
}

type monitorEntry struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Selector  string `json:"selector,omitempty"`
	Interval  string `json:"interval,omitempty"`
	LastCheck string `json:"last_check,omitempty"`
	LastHash  string `json:"last_hash,omitempty"`
	CreatedAt string `json:"created_at"`
}

type changeEntry struct {
	ID        int    `json:"id"`
	MonitorID string `json:"monitor_id"`
	URL       string `json:"url"`
	ChangedAt string `json:"changed_at"`
	OldHash   string `json:"old_hash"`
	NewHash   string `json:"new_hash"`
	Preview   string `json:"preview,omitempty"`
}

func siteMonitorJSON(r siteMonitorResult) string {
	b, _ := json.Marshal(r)
	return string(b)
}

func initSiteMonitorDB(dbPath string) (*sql.DB, error) {
	siteMonitorOnce.Do(func() {
		siteMonitorMu.Lock()
		defer siteMonitorMu.Unlock()
		siteMonitorDB, siteMonitorErr = dbutil.Open(dbPath)
		if siteMonitorErr != nil {
			return
		}
		schema := `
		CREATE TABLE IF NOT EXISTS site_monitors (
			id TEXT PRIMARY KEY,
			url TEXT NOT NULL,
			selector TEXT DEFAULT '',
			interval TEXT DEFAULT '',
			last_check DATETIME,
			last_hash TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS site_changes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			monitor_id TEXT NOT NULL,
			changed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			old_hash TEXT,
			new_hash TEXT,
			preview TEXT DEFAULT '',
			FOREIGN KEY (monitor_id) REFERENCES site_monitors(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_changes_monitor ON site_changes(monitor_id);
		`
		if _, err := siteMonitorDB.Exec(schema); err != nil {
			siteMonitorDB.Close()
			siteMonitorDB = nil
			siteMonitorErr = fmt.Errorf("schema creation: %w", err)
		}
	})
	return siteMonitorDB, siteMonitorErr
}

// ExecuteSiteMonitor dispatches site monitoring operations.
func ExecuteSiteMonitor(dbPath, operation, monitorID, url, selector, interval string, limit int) string {
	db, err := initSiteMonitorDB(dbPath)
	if err != nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: fmt.Sprintf("database init failed: %v", err)})
	}

	siteMonitorMu.RLock()
	defer siteMonitorMu.RUnlock()

	switch strings.ToLower(operation) {
	case "add_monitor":
		return siteMonitorAdd(db, url, selector, interval)
	case "remove_monitor":
		return siteMonitorRemove(db, monitorID)
	case "list_monitors":
		return siteMonitorList(db)
	case "check_now":
		return siteMonitorCheck(db, monitorID, url)
	case "check_all":
		return siteMonitorCheckAll(db)
	case "get_history":
		return siteMonitorHistory(db, monitorID, limit)
	default:
		return siteMonitorJSON(siteMonitorResult{
			Status:  "error",
			Message: fmt.Sprintf("unknown operation: %s (valid: add_monitor, remove_monitor, list_monitors, check_now, check_all, get_history)", operation),
		})
	}
}

func siteMonitorAdd(db *sql.DB, url, selector, interval string) string {
	if db == nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: "site monitor database not initialized"})
	}
	if url == "" {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: "url is required"})
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: "url must start with http:// or https://"})
	}

	id := fmt.Sprintf("mon_%s", shortHash(url+selector))

	// Check for existing monitor with same URL+selector
	var existing string
	err := db.QueryRow("SELECT id FROM site_monitors WHERE id = ?", id).Scan(&existing)
	if err == nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: fmt.Sprintf("monitor already exists with ID %s", id)})
	}

	_, err = db.Exec(
		"INSERT INTO site_monitors (id, url, selector, interval, created_at) VALUES (?, ?, ?, ?, ?)",
		id, url, selector, interval, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: fmt.Sprintf("insert failed: %v", err)})
	}

	// Do initial fetch to establish baseline hash
	content, fetchErr := fetchSiteContent(url)
	if fetchErr == nil {
		hash := contentHash(content)
		db.Exec("UPDATE site_monitors SET last_check = ?, last_hash = ? WHERE id = ?",
			time.Now().UTC().Format(time.RFC3339), hash, id)
	}

	msg := fmt.Sprintf("monitor added: %s (ID: %s)", url, id)
	if interval != "" {
		msg += fmt.Sprintf(". To schedule automatic checks, create a cron job with: check_now for monitor_id=%s", id)
	}

	return siteMonitorJSON(siteMonitorResult{Status: "success", Message: msg})
}

func siteMonitorRemove(db *sql.DB, monitorID string) string {
	if db == nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: "site monitor database not initialized"})
	}
	if monitorID == "" {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: "monitor_id is required"})
	}

	res, err := db.Exec("DELETE FROM site_monitors WHERE id = ?", monitorID)
	if err != nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: fmt.Sprintf("delete failed: %v", err)})
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: fmt.Sprintf("monitor not found: %s", monitorID)})
	}

	// Clean up change history
	db.Exec("DELETE FROM site_changes WHERE monitor_id = ?", monitorID)

	return siteMonitorJSON(siteMonitorResult{Status: "success", Message: fmt.Sprintf("monitor %s removed", monitorID)})
}

func siteMonitorList(db *sql.DB) string {
	if db == nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: "site monitor database not initialized"})
	}
	rows, err := db.Query("SELECT id, url, selector, interval, last_check, last_hash, created_at FROM site_monitors ORDER BY created_at DESC")
	if err != nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: fmt.Sprintf("query failed: %v", err)})
	}
	defer rows.Close()

	var monitors []monitorEntry
	for rows.Next() {
		var m monitorEntry
		var lastCheck, lastHash sql.NullString
		if err := rows.Scan(&m.ID, &m.URL, &m.Selector, &m.Interval, &lastCheck, &lastHash, &m.CreatedAt); err != nil {
			continue
		}
		if lastCheck.Valid {
			m.LastCheck = lastCheck.String
		}
		if lastHash.Valid {
			m.LastHash = lastHash.String
		}
		monitors = append(monitors, m)
	}

	if len(monitors) == 0 {
		return siteMonitorJSON(siteMonitorResult{Status: "success", Message: "no monitors configured"})
	}

	return siteMonitorJSON(siteMonitorResult{Status: "success", Monitors: monitors})
}

func siteMonitorCheck(db *sql.DB, monitorID, url string) string {
	if db == nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: "site monitor database not initialized"})
	}
	// Allow checking by ID or direct URL
	var m monitorEntry
	if monitorID != "" {
		err := db.QueryRow("SELECT id, url, selector, last_hash FROM site_monitors WHERE id = ?", monitorID).
			Scan(&m.ID, &m.URL, &m.Selector, &m.LastHash)
		if err != nil {
			return siteMonitorJSON(siteMonitorResult{Status: "error", Message: fmt.Sprintf("monitor not found: %s", monitorID)})
		}
	} else if url != "" {
		m.URL = url
	} else {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: "monitor_id or url is required"})
	}

	content, err := fetchSiteContent(m.URL)
	if err != nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: fmt.Sprintf("fetch failed: %v", err)})
	}

	newHash := contentHash(content)
	now := time.Now().UTC().Format(time.RFC3339)
	changed := m.LastHash != "" && m.LastHash != newHash

	if m.ID != "" {
		// Update the monitor
		db.Exec("UPDATE site_monitors SET last_check = ?, last_hash = ? WHERE id = ?", now, newHash, m.ID)

		if changed {
			preview := content
			if len(preview) > 500 {
				preview = preview[:500] + "..."
			}
			db.Exec("INSERT INTO site_changes (monitor_id, changed_at, old_hash, new_hash, preview) VALUES (?, ?, ?, ?, ?)",
				m.ID, now, m.LastHash, newHash, preview)
		}
	}

	result := siteMonitorResult{
		Status:  "success",
		Changed: &changed,
	}

	if changed {
		result.Message = fmt.Sprintf("CHANGE DETECTED at %s (hash: %s → %s)", m.URL, m.LastHash[:8], newHash[:8])
	} else if m.LastHash == "" {
		result.Message = fmt.Sprintf("baseline captured for %s (hash: %s)", m.URL, newHash[:8])
	} else {
		result.Message = fmt.Sprintf("no change at %s (hash: %s)", m.URL, newHash[:8])
	}

	return siteMonitorJSON(result)
}

func siteMonitorCheckAll(db *sql.DB) string {
	if db == nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: "site monitor database not initialized"})
	}
	rows, err := db.Query("SELECT id, url, selector, last_hash FROM site_monitors")
	if err != nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: fmt.Sprintf("query failed: %v", err)})
	}
	defer rows.Close()

	type checkResult struct {
		ID      string `json:"id"`
		URL     string `json:"url"`
		Changed bool   `json:"changed"`
		Message string `json:"message"`
	}

	var results []checkResult
	changedCount := 0

	for rows.Next() {
		var m monitorEntry
		var lastHash sql.NullString
		if err := rows.Scan(&m.ID, &m.URL, &m.Selector, &lastHash); err != nil {
			continue
		}
		if lastHash.Valid {
			m.LastHash = lastHash.String
		}

		content, fetchErr := fetchSiteContent(m.URL)
		if fetchErr != nil {
			results = append(results, checkResult{ID: m.ID, URL: m.URL, Message: fmt.Sprintf("fetch error: %v", fetchErr)})
			continue
		}

		newHash := contentHash(content)
		now := time.Now().UTC().Format(time.RFC3339)
		changed := m.LastHash != "" && m.LastHash != newHash

		db.Exec("UPDATE site_monitors SET last_check = ?, last_hash = ? WHERE id = ?", now, newHash, m.ID)

		if changed {
			changedCount++
			preview := content
			if len(preview) > 500 {
				preview = preview[:500] + "..."
			}
			db.Exec("INSERT INTO site_changes (monitor_id, changed_at, old_hash, new_hash, preview) VALUES (?, ?, ?, ?, ?)",
				m.ID, now, m.LastHash, newHash, preview)
		}

		results = append(results, checkResult{
			ID:      m.ID,
			URL:     m.URL,
			Changed: changed,
			Message: fmt.Sprintf("hash: %s", newHash[:8]),
		})
	}

	msg := fmt.Sprintf("checked %d monitors, %d changed", len(results), changedCount)
	return siteMonitorJSON(siteMonitorResult{Status: "success", Message: msg, Changes: results})
}

func siteMonitorHistory(db *sql.DB, monitorID string, limit int) string {
	if db == nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: "site monitor database not initialized"})
	}
	if monitorID == "" {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: "monitor_id is required"})
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := db.Query(
		"SELECT c.id, c.monitor_id, m.url, c.changed_at, c.old_hash, c.new_hash, c.preview FROM site_changes c JOIN site_monitors m ON c.monitor_id = m.id WHERE c.monitor_id = ? ORDER BY c.changed_at DESC LIMIT ?",
		monitorID, limit,
	)
	if err != nil {
		return siteMonitorJSON(siteMonitorResult{Status: "error", Message: fmt.Sprintf("query failed: %v", err)})
	}
	defer rows.Close()

	var changes []changeEntry
	for rows.Next() {
		var c changeEntry
		var preview sql.NullString
		if err := rows.Scan(&c.ID, &c.MonitorID, &c.URL, &c.ChangedAt, &c.OldHash, &c.NewHash, &preview); err != nil {
			continue
		}
		if preview.Valid {
			c.Preview = preview.String
		}
		changes = append(changes, c)
	}

	if len(changes) == 0 {
		return siteMonitorJSON(siteMonitorResult{Status: "success", Message: fmt.Sprintf("no changes recorded for monitor %s", monitorID)})
	}

	return siteMonitorJSON(siteMonitorResult{Status: "success", Changes: changes})
}

// --- helpers ---

func fetchSiteContent(url string) (string, error) {
	// Use the SSRF-protected client that pins the resolved IP to prevent DNS-rebinding
	// TOCTOU attacks between the validation check and the actual HTTP dial.
	client, err := security.NewSSRFProtectedHTTPClientForURL(url, 15*time.Second)
	if err != nil {
		return "", fmt.Errorf("URL not allowed: %w", err)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AuraGo Site Monitor)")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Limit read to 1MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	// Clean HTML (reuse the same regex patterns from scraper.go)
	text := string(body)
	text = reScript.ReplaceAllString(text, "")
	text = reStyle.ReplaceAllString(text, "")
	text = reTag.ReplaceAllString(text, " ")
	text = reSpace.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	return text, nil
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

// CloseSiteMonitorDB closes the site monitor database connection.
func CloseSiteMonitorDB() error {
	siteMonitorMu.Lock()
	defer siteMonitorMu.Unlock()
	if siteMonitorDB != nil {
		err := siteMonitorDB.Close()
		siteMonitorDB = nil
		return err
	}
	return nil
}
