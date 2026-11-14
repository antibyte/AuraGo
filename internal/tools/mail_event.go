package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

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
