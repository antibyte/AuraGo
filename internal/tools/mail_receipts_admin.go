package tools

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrMailReceiptConflict = errors.New("mail receipt changed or delivery attempt is active")
	ErrMailReceiptNotFound = errors.New("mail receipt not found")
	ErrMailReceiptRequest  = errors.New("invalid mail receipt request")
)

// MailReceipt is the complete metadata-only administrator response.
type MailReceipt struct {
	ID          string     `json:"id"`
	Channel     string     `json:"channel"`
	Status      string     `json:"status"`
	AttemptedAt time.Time  `json:"attempted_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Revision    string     `json:"revision"`
}

type MailReceiptChannelUsage struct {
	Channel string `json:"channel"`
	Count   int    `json:"count"`
	Limit   int    `json:"limit"`
}

type MailReceiptPage struct {
	Receipts   []MailReceipt             `json:"receipts"`
	Channels   []MailReceiptChannelUsage `json:"channels"`
	NextCursor string                    `json:"next_cursor,omitempty"`
}

type mailReceiptCursor struct {
	At string `json:"at"`
	ID string `json:"id"`
}

func validMailReceiptID(id string) bool {
	if len(id) != 64 || strings.ToLower(id) != id {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func mailReceiptRevision(namespace, payload, updated string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(namespace+"\x00"+payload+"\x00"+updated)))
}

// ListMailReceipts never returns stored payloads, event identities or delivery content.
func ListMailReceipts(ctx context.Context, dataDir, channel, cursor string, limit int) (MailReceiptPage, error) {
	page := MailReceiptPage{Receipts: []MailReceipt{}, Channels: []MailReceiptChannelUsage{}}
	if channel != "" && channel != "legacy" && !validMailReceiptChannel(channel) || limit < 1 || limit > 50 {
		return page, ErrMailReceiptRequest
	}
	var after mailReceiptCursor
	if cursor != "" {
		if len(cursor) > 512 {
			return page, ErrMailReceiptRequest
		}
		data, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || json.Unmarshal(data, &after) != nil || !validMailReceiptID(after.ID) {
			return page, ErrMailReceiptRequest
		}
		if _, err := time.Parse(time.RFC3339Nano, after.At); err != nil {
			return page, ErrMailReceiptRequest
		}
	}
	store, err := openMailReceiptStore(dataDir)
	if err != nil {
		return page, err
	}
	defer store.release()
	mailReceipts.Lock()
	defer mailReceipts.Unlock()
	if err := expireMailReceipts(ctx, store.db, time.Now().UTC()); err != nil {
		return page, err
	}
	counts := map[string]int{}
	rows, err := store.db.QueryContext(ctx, `SELECT `+mailReceiptChannelSQL+`,COUNT(*) FROM system_tasks WHERE namespace GLOB 'mail_notice:*' GROUP BY 1`)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		var c string
		var n int
		if err := rows.Scan(&c, &n); err != nil {
			_ = rows.Close()
			return page, err
		}
		counts[c] = n
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return page, err
	}
	for _, c := range mailReceiptChannels {
		page.Channels = append(page.Channels, MailReceiptChannelUsage{c, counts[c], mailReceiptLimit})
	}
	page.Channels = append(page.Channels, MailReceiptChannelUsage{"legacy", counts["legacy"], 0})
	query := `SELECT namespace,payload_json,updated_at FROM system_tasks WHERE namespace GLOB 'mail_notice:*'`
	args := []any{}
	if channel != "" {
		query += ` AND (` + mailReceiptChannelSQL + `) = ?`
		args = append(args, channel)
	}
	if cursor != "" {
		query += ` AND (julianday(updated_at) < julianday(?) OR (julianday(updated_at) = julianday(?) AND namespace > ?))`
		args = append(args, after.At, after.At, mailReceiptPrefix+after.ID)
	}
	query += ` ORDER BY julianday(updated_at) DESC,namespace ASC LIMIT ?`
	args = append(args, limit+1)
	rows, err = store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var last mailReceiptCursor
	for rows.Next() {
		var namespace, payload, updated string
		if err := rows.Scan(&namespace, &payload, &updated); err != nil {
			return page, err
		}
		if len(page.Receipts) == limit {
			data, _ := json.Marshal(last)
			page.NextCursor = base64.RawURLEncoding.EncodeToString(data)
			break
		}
		id := strings.TrimPrefix(namespace, mailReceiptPrefix)
		if !validMailReceiptID(id) {
			continue
		}
		record := decodeMailReceipt(payload, updated)
		status := record.Status
		if status == "active" {
			owner := mailReceipts.active[mailReceiptKey(store, namespace)]
			if owner == nil || owner.token != record.Token {
				status = "unknown" // An interrupted process cannot prove non-delivery.
			}
		}
		receipt := MailReceipt{ID: id, Channel: record.Channel, Status: status, AttemptedAt: record.AttemptedAt, Revision: mailReceiptRevision(namespace, payload, updated)}
		if status == "sent" && !record.ExpiresAt.IsZero() {
			receipt.ExpiresAt = &record.ExpiresAt
		}
		page.Receipts = append(page.Receipts, receipt)
		last = mailReceiptCursor{updated, id}
	}
	return page, rows.Err()
}

// ResolveMailReceipt changes only accounting. It never invokes a sender.
func ResolveMailReceipt(ctx context.Context, dataDir, id, revision, resolution string) error {
	if !validMailReceiptID(id) || !validMailReceiptID(revision) || (resolution != "sent" && resolution != "not_sent") {
		return ErrMailReceiptRequest
	}
	store, err := openMailReceiptStore(dataDir)
	if err != nil {
		return err
	}
	defer store.release()
	mailReceipts.Lock()
	defer mailReceipts.Unlock()
	namespace := mailReceiptPrefix + id
	var payload, updated string
	if err := store.db.QueryRowContext(ctx, "SELECT payload_json,updated_at FROM system_tasks WHERE namespace = ?", namespace).Scan(&payload, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrMailReceiptNotFound
		}
		return err
	}
	if mailReceipts.active[mailReceiptKey(store, namespace)] != nil || revision != mailReceiptRevision(namespace, payload, updated) {
		return ErrMailReceiptConflict
	}
	var result sql.Result
	if resolution == "not_sent" {
		result, err = store.db.ExecContext(ctx, "DELETE FROM system_tasks WHERE namespace = ? AND payload_json = ? AND updated_at = ?", namespace, payload, updated)
	} else {
		record := decodeMailReceipt(payload, updated)
		record.Status, record.Token, record.ExpiresAt = "sent", newMailReceiptToken(), time.Now().UTC().Add(mailReceiptLifetime)
		data, _ := json.Marshal(record)
		result, err = store.db.ExecContext(ctx, "UPDATE system_tasks SET payload_json = ?,updated_at = ? WHERE namespace = ? AND payload_json = ? AND updated_at = ?", string(data), time.Now().UTC().Format(time.RFC3339Nano), namespace, payload, updated)
	}
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return ErrMailReceiptConflict
	}
	return err
}
