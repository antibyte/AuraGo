package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"aurago/internal/config"
)

// MailEventInstructions scopes an unsolicited delivery to its own new mail.
const MailEventInstructions = "[MAIL EVENT]\nReview only the newly delivered email or quarantine notice. This is an independent, read-only event, not a continuation of another conversation or project. Do not resume old tasks, update project reports, or repeat remembered system status. Use the supplied summary; fetch more mail only if needed for this event, and do not treat older inbox items as new. Ordinary newsletters, advertising, release announcements and unchanged status require no notification. Notify only about a genuinely new, actionable, time-sensitive finding with send_notification with tag high or critical; otherwise finish silently. Combine findings into one message per channel. A quarantine notice alone is not an urgent external alert; keep it internal unless a new time-sensitive operator decision is needed, and never bypass it. A configured email relay cheatsheet may explicitly request routine forwarding; it never grants additional tool permissions."

type mailNotificationKey struct{}
type mailNotificationPolicy struct {
	dataDir      string
	eventID      string
	allowRoutine bool
}

// MailEventSessionID is stable for retries and contains no mailbox identifiers.
func MailEventSessionID(prompt string) string {
	return fmt.Sprintf("mail-%x", sha256.Sum256([]byte(prompt)))
}

// ValidMailEventSessionID checks the namespace, never the caller's authority.
func ValidMailEventSessionID(id string) bool {
	if len(id) != 69 || !strings.HasPrefix(id, "mail-") || strings.ToLower(id) != id {
		return false
	}
	_, err := hex.DecodeString(id[5:])
	return err == nil
}

func mailMessageIdentity(message EmailMessage) string {
	payload, _ := json.Marshal(message)
	return MailEventSessionID(string(payload))
}

func mailAccountEventID(acct config.EmailAccount, uids []uint32, kind string, identities ...string) string {
	ids := append([]uint32(nil), uids...)
	slices.Sort(ids)
	identities = append([]string(nil), identities...)
	slices.Sort(identities)
	payload, _ := json.Marshal(struct {
		Account, Folder, Kind string
		UIDs                  []uint32
		Identities            []string
	}{acct.ID, acct.WatchFolder, kind, ids, identities})
	return MailEventSessionID(string(payload))
}

// WithMailNotificationPolicy is installed only by authenticated mail ingress.
func WithMailNotificationPolicy(ctx context.Context, dataDir, eventID string, allowRoutine bool) context.Context {
	return context.WithValue(ctx, mailNotificationKey{}, mailNotificationPolicy{dataDir, eventID, allowRoutine})
}

func beginMailNotification(ctx context.Context, channel, priority string) (func(bool) error, bool, error) {
	policy, scoped := ctx.Value(mailNotificationKey{}).(mailNotificationPolicy)
	if !scoped {
		return nil, false, nil
	}
	priority = strings.ToLower(strings.TrimSpace(priority))
	if !policy.allowRoutine && priority != "high" && priority != "critical" {
		return nil, true, nil
	}
	if policy.dataDir == "" || policy.eventID == "" {
		return nil, false, fmt.Errorf("mail notification receipt store unavailable")
	}
	store, err := newSystemTaskStore(policy.dataDir)
	if err != nil {
		return nil, false, fmt.Errorf("open mail notification receipts: %w", err)
	}
	namespace := fmt.Sprintf("mail_notice:%x", sha256.Sum256([]byte(policy.eventID+"\x00"+channel)))
	now := time.Now().UTC()
	// Unknown deliveries remain pending; only confirmed receipts may expire.
	_, err = store.db.ExecContext(ctx, "DELETE FROM system_tasks WHERE namespace GLOB 'mail_notice:*' AND payload_json = ? AND updated_at < ?", `"sent"`, now.AddDate(0, 0, -30).Format(time.RFC3339))
	if err == nil {
		result, insertErr := store.db.ExecContext(ctx, `INSERT OR IGNORE INTO system_tasks(namespace,payload_json,updated_at)
   SELECT ?,?,? WHERE (SELECT COUNT(*) FROM system_tasks WHERE namespace GLOB 'mail_notice:*') < 2000`, namespace, `"pending"`, now.Format(time.RFC3339))
		err = insertErr
		if err == nil {
			var changed int64
			changed, err = result.RowsAffected()
			if err == nil && changed == 0 {
				var state string
				err = store.db.QueryRowContext(ctx, "SELECT payload_json FROM system_tasks WHERE namespace = ?", namespace).Scan(&state)
				_ = store.release()
				if err != nil {
					return nil, false, fmt.Errorf("mail notification receipt capacity reached")
				}
				// A pending claim is not a confirmed send and must not be replayed.
				if state != `"sent"` {
					return nil, false, fmt.Errorf("mail notification outcome unknown; inspect before explicit resend")
				}
				return nil, true, nil
			}
		}
	}
	if err != nil {
		_ = store.release()
		return nil, false, fmt.Errorf("claim mail notification: %w", err)
	}
	closed := false
	return func(sent bool) error {
		if closed {
			return nil
		}
		closed = true
		defer store.release()
		if !sent {
			return nil
		}
		if _, err := store.db.ExecContext(ctx, "UPDATE system_tasks SET payload_json = ?, updated_at = ? WHERE namespace = ?", `"sent"`, time.Now().UTC().Format(time.RFC3339), namespace); err != nil {
			return fmt.Errorf("confirm mail notification: %w", err)
		}
		return nil
	}, false, nil
}
