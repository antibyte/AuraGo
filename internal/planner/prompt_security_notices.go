package planner

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// OperationalIssueSourcePromptSecurity is reserved for runtime-owned prompt
// checks. Their new revisions are due at the next direct contact, even after
// the ordinary daily notice opportunity has already been used.
const OperationalIssueSourcePromptSecurity = "prompt_security"

func ListPendingPromptSecurityNotices(db *sql.DB, now time.Time, limit int) ([]OperationalIssueNotice, error) {
	return listPendingOperationalIssueNotices(db, now, limit, OperationalIssueSourcePromptSecurity)
}

// ClaimPromptSecurityNoticeBatch leases a revision batch across concurrent
// contacts and restarts. It does not acknowledge delivery; an interrupted batch
// becomes eligible again after five minutes unless it was actually delivered.
func ClaimPromptSecurityNoticeBatch(db *sql.DB, notices []OperationalIssueNotice, now time.Time) (bool, error) {
	if db == nil || len(notices) == 0 {
		return false, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	refs := make([]string, 0, len(notices))
	for _, notice := range notices {
		refs = append(refs, fmt.Sprintf("%s:%d", notice.Fingerprint, notice.Revision))
	}
	sort.Strings(refs)
	digest := sha256.Sum256([]byte(strings.Join(refs, "\x00")))
	key := fmt.Sprintf("prompt_security_notice_lease:%x", digest)
	if _, err := db.Exec(`DELETE FROM planner_meta WHERE key GLOB 'prompt_security_notice_lease:*' AND CAST(value AS INTEGER) <= ?`, now.Unix()); err != nil {
		return false, fmt.Errorf("expire prompt security notice leases: %w", err)
	}
	result, err := db.Exec(`INSERT INTO planner_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
		WHERE CAST(planner_meta.value AS INTEGER) <= ?`, key, fmt.Sprint(now.Add(5*time.Minute).Unix()), now.Unix())
	if err != nil {
		return false, fmt.Errorf("claim prompt security notice: %w", err)
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
