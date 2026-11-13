package tools

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptrace"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const mailReceiptLimit = 2000
const mailReceiptLifetime = 30 * 24 * time.Hour
const mailReceiptPrefix = "mail_notice:"

var mailReceiptChannels = []string{"ntfy", "pushover", "telegram", "whatsapp", "discord", "push", "telnyx", "cyd"}

// Receipts deliberately contain no event identity, recipient or notification text.
type mailReceiptRecord struct {
	Version     int       `json:"v"`
	Channel     string    `json:"channel"`
	Status      string    `json:"status"`
	AttemptedAt time.Time `json:"attempted_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	Token       string    `json:"token"`
}

type mailReceiptAttempt struct {
	token    string
	possible atomic.Bool
	accepted atomic.Bool
}

type mailReceiptAttemptKey struct{}

var mailReceipts = struct {
	sync.Mutex
	active map[string]*mailReceiptAttempt
}{active: make(map[string]*mailReceiptAttempt)}

// IsMailNotificationContext tests authority installed by authenticated ingress,
// rather than an attacker-controlled session name or message prefix.
func IsMailNotificationContext(ctx context.Context) bool {
	_, ok := ctx.Value(mailNotificationKey{}).(mailNotificationPolicy)
	return ok
}

func validMailReceiptChannel(channel string) bool {
	for _, c := range mailReceiptChannels {
		if c == channel {
			return true
		}
	}
	return false
}

func mailReceiptNamespace(policy mailNotificationPolicy, channel string) string {
	return fmt.Sprintf("%s%x", mailReceiptPrefix, sha256.Sum256([]byte(policy.eventID+"\x00"+channel)))
}

func openMailReceiptStore(dataDir string) (*systemTaskStore, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, errors.New("mail notification receipt store unavailable")
	}
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	return newSystemTaskStore(abs)
}

func mailReceiptKey(store *systemTaskStore, namespace string) string {
	path := store.path
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path + "\x00" + namespace
}

func newMailReceiptToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand failure is fatal; never reuse a reservation identity.
	}
	return hex.EncodeToString(b[:])
}

func decodeMailReceipt(payload, updated string) mailReceiptRecord {
	var record mailReceiptRecord
	if json.Unmarshal([]byte(payload), &record) == nil && record.Version == 1 &&
		(validMailReceiptChannel(record.Channel) || record.Channel == "legacy") &&
		(record.Status == "active" || record.Status == "unknown" || record.Status == "sent") {
		return record
	}
	at, _ := time.Parse(time.RFC3339Nano, updated)
	record = mailReceiptRecord{Version: 1, Channel: "legacy", Status: "unknown", AttemptedAt: at}
	if payload == `"sent"` {
		record.Status, record.ExpiresAt = "sent", at.Add(mailReceiptLifetime)
	}
	return record
}

// CASE guards json_extract for old string states and damaged legacy records.
const mailReceiptChannelSQL = `CASE WHEN json_valid(payload_json) THEN
 CASE WHEN json_extract(payload_json,'$.v') = 1 AND
 json_extract(payload_json,'$.channel') IN ('ntfy','pushover','telegram','whatsapp','discord','push','telnyx','cyd')
 THEN json_extract(payload_json,'$.channel') ELSE 'legacy' END ELSE 'legacy' END`

func expireMailReceipts(ctx context.Context, db *sql.DB, now time.Time) error {
	_, err := db.ExecContext(ctx, `DELETE FROM system_tasks WHERE namespace GLOB 'mail_notice:*' AND
 ((payload_json = '"sent"' AND julianday(updated_at) <= julianday(?)) OR
 CASE WHEN json_valid(payload_json) THEN json_extract(payload_json,'$.v') = 1 AND
 json_extract(payload_json,'$.status') = 'sent' AND julianday(json_extract(payload_json,'$.expires_at')) <= julianday(?) ELSE 0 END)`,
		now.Add(-mailReceiptLifetime).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	return err
}

// checkMailNotification runs before opening any attachment. The final atomic
// reservation still runs after local validation, so this check grants no send.
func checkMailNotification(ctx context.Context, channel, priority string) (bool, string, error) {
	policy, scoped := ctx.Value(mailNotificationKey{}).(mailNotificationPolicy)
	if !scoped {
		return false, "", nil
	}
	priority = strings.ToLower(strings.TrimSpace(priority))
	if !policy.allowRoutine && priority != "high" && priority != "critical" {
		return true, "mail event requires a new urgent notice or explicit relay instructions", nil
	}
	if policy.eventID == "" || !validMailReceiptChannel(channel) {
		return false, "", errors.New("mail notification receipt policy unavailable")
	}
	store, err := openMailReceiptStore(policy.dataDir)
	if err != nil {
		return false, "", err
	}
	defer store.release()
	mailReceipts.Lock()
	defer mailReceipts.Unlock()
	if err := expireMailReceipts(ctx, store.db, time.Now().UTC()); err != nil {
		return false, "", err
	}
	var payload, updated string
	err = store.db.QueryRowContext(ctx, "SELECT payload_json,updated_at FROM system_tasks WHERE namespace = ?", mailReceiptNamespace(policy, channel)).Scan(&payload, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	if decodeMailReceipt(payload, updated).Status == "sent" {
		return true, "mail notification already delivered on this channel", nil
	}
	return false, "", errors.New("mail notification outcome unknown or attempt active; review the receipt before another attempt")
}

func beginMailNotification(ctx context.Context, channel, priority string) (func(bool) error, bool, error) {
	skipped, _, err := checkMailNotification(ctx, channel, priority)
	if err != nil || skipped {
		return nil, skipped, err
	}
	policy, scoped := ctx.Value(mailNotificationKey{}).(mailNotificationPolicy)
	if !scoped {
		return nil, false, nil
	}
	store, err := openMailReceiptStore(policy.dataDir)
	if err != nil {
		return nil, false, err
	}
	namespace, now := mailReceiptNamespace(policy, channel), time.Now().UTC()
	record := mailReceiptRecord{Version: 1, Channel: channel, Status: "active", AttemptedAt: now, Token: newMailReceiptToken()}
	payload, _ := json.Marshal(record)
	attempt := &mailReceiptAttempt{token: record.Token}
	key := mailReceiptKey(store, namespace)
	mailReceipts.Lock()
	// A single SQLite statement makes the channel quota atomic across connections.
	result, err := store.db.ExecContext(ctx, `INSERT OR IGNORE INTO system_tasks(namespace,payload_json,updated_at)
 SELECT ?,?,? WHERE (SELECT COUNT(*) FROM system_tasks WHERE namespace GLOB 'mail_notice:*' AND `+mailReceiptChannelSQL+` = ?) < ?`,
		namespace, string(payload), now.Format(time.RFC3339Nano), channel, mailReceiptLimit)
	var changed int64
	if err == nil {
		changed, err = result.RowsAffected()
	}
	if err == nil && changed == 1 {
		mailReceipts.active[key] = attempt
	}
	mailReceipts.Unlock()
	if err != nil || changed != 1 {
		_ = store.release()
		if err != nil {
			return nil, false, fmt.Errorf("claim mail notification: %w", err)
		}
		// Distinguish a competing claim from an independently full channel.
		skipped, _, err := checkMailNotification(ctx, channel, priority)
		if err != nil || skipped {
			return nil, skipped, err
		}
		return nil, false, errors.New("mail notification receipt capacity reached for this channel")
	}
	var once sync.Once
	var finishErr error
	return func(sent bool) error {
		once.Do(func() {
			defer store.release()
			finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			mailReceipts.Lock()
			defer mailReceipts.Unlock()
			defer delete(mailReceipts.active, key)
			if !sent && !attempt.possible.Load() {
				_, finishErr = store.db.ExecContext(finishCtx, "DELETE FROM system_tasks WHERE namespace = ? AND payload_json = ?", namespace, string(payload))
				return
			}
			record.Status, record.Token = "unknown", newMailReceiptToken()
			if sent {
				record.Status, record.ExpiresAt = "sent", time.Now().UTC().Add(mailReceiptLifetime)
			}
			updated, _ := json.Marshal(record)
			_, finishErr = store.db.ExecContext(finishCtx, "UPDATE system_tasks SET payload_json = ?,updated_at = ? WHERE namespace = ? AND payload_json = ?", string(updated), time.Now().UTC().Format(time.RFC3339Nano), namespace, string(payload))
		})
		return finishErr
	}, false, nil
}

func observeMailNotification(ctx context.Context, channel string) context.Context {
	policy, scoped := ctx.Value(mailNotificationKey{}).(mailNotificationPolicy)
	if !scoped {
		return ctx
	}
	store, err := openMailReceiptStore(policy.dataDir)
	if err != nil {
		return ctx // The reservation already holds its store; no new send is granted here.
	}
	mailReceipts.Lock()
	attempt := mailReceipts.active[mailReceiptKey(store, mailReceiptNamespace(policy, channel))]
	mailReceipts.Unlock()
	_ = store.release()
	if attempt == nil {
		return ctx
	}
	ctx = context.WithValue(ctx, mailReceiptAttemptKey{}, attempt)
	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{GotConn: func(httptrace.GotConnInfo) { attempt.possible.Store(true) }})
}

func markMailNotificationPossible(ctx context.Context) {
	if attempt, ok := ctx.Value(mailReceiptAttemptKey{}).(*mailReceiptAttempt); ok {
		attempt.possible.Store(true)
	}
}

func markMailNotificationAccepted(ctx context.Context) {
	if attempt, ok := ctx.Value(mailReceiptAttemptKey{}).(*mailReceiptAttempt); ok {
		attempt.accepted.Store(true)
		attempt.possible.Store(true)
	}
}

func markMailNotificationRejected(ctx context.Context) {
	if attempt, ok := ctx.Value(mailReceiptAttemptKey{}).(*mailReceiptAttempt); ok && !attempt.accepted.Load() {
		attempt.possible.Store(false)
	}
}
