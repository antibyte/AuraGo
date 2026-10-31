package integrationstatus

import (
	"strings"
	"sync"
	"time"
)

// WhatsApp runtime states. disabled and not_configured come from the
// configuration. The others come from the device store, link sessions and
// whatsmeow events.
const (
	WhatsAppStateDisabled      = "disabled"
	WhatsAppStateNotConfigured = "not_configured"
	WhatsAppStateNotLinked     = "not_linked"
	WhatsAppStateLinking       = "linking"
	WhatsAppStateConnecting    = "connecting"
	WhatsAppStateHealthy       = "healthy"
	WhatsAppStateDegraded      = "degraded"
	WhatsAppStateLoggedOut     = "logged_out"
	WhatsAppStateConflict      = "conflict"
	WhatsAppStateTempBanned    = "temp_banned"
	WhatsAppStateOutdated      = "outdated"
)

// WhatsAppRuntimeStatus is a sanitized passive snapshot. Numbers keep only
// their last four digits. It never holds message content, keys or QR codes.
type WhatsAppRuntimeStatus struct {
	Enabled              bool   `json:"enabled"`
	State                string `json:"state"`
	Mode                 string `json:"mode,omitempty"`
	Linked               bool   `json:"linked"`
	LinkedJIDMasked      string `json:"linked_jid_masked,omitempty"`
	ConnectedSince       string `json:"connected_since,omitempty"`
	LastInboundAt        string `json:"last_inbound_at,omitempty"`
	LastOutboundAt       string `json:"last_outbound_at,omitempty"`
	MessagesReceived     int    `json:"messages_received"`
	DroppedNonOwner      int    `json:"dropped_non_owner"`
	DroppedUnresolvedLID int    `json:"dropped_unresolved_lid"`
	DeliveryFailures     int    `json:"delivery_failures"`
	LastError            string `json:"last_error,omitempty"`
	TempBanExpiresAt     string `json:"temp_ban_expires_at,omitempty"`
	LastTestAt           string `json:"last_test_at,omitempty"`
}

var whatsAppRuntime = struct {
	sync.RWMutex
	status WhatsAppRuntimeStatus
}{status: WhatsAppRuntimeStatus{State: WhatsAppStateDisabled}}

func whatsAppStamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// whatsAppMask keeps only the last four digits of a number, an already masked
// number or a JID such as 491701234567:12@s.whatsapp.net. It returns "" when
// fewer than four digits remain.
func whatsAppMask(raw string) string {
	user := raw
	if at := strings.IndexByte(user, '@'); at >= 0 {
		user = user[:at]
		if i := strings.IndexAny(user, ":."); i >= 0 {
			user = user[:i]
		}
	}
	var digits []byte
	for i := 0; i < len(user); i++ {
		if c := user[i]; c >= '0' && c <= '9' {
			digits = append(digits, c)
		}
	}
	if len(digits) < 4 {
		return ""
	}
	return "***" + string(digits[len(digits)-4:])
}

// enterWhatsAppState sets state and drops the details that only belong to
// other states: a ban expiry outside temp_banned, a login time outside healthy,
// and the linked device once the account is not linked or logged out. The
// caller holds the lock.
func enterWhatsAppState(s *WhatsAppRuntimeStatus, state string) {
	s.State = state
	if state != WhatsAppStateTempBanned {
		s.TempBanExpiresAt = ""
	}
	if state != WhatsAppStateHealthy {
		s.ConnectedSince = ""
	}
	if state == WhatsAppStateNotLinked || state == WhatsAppStateLoggedOut {
		s.Linked = false
		s.LinkedJIDMasked = ""
	}
}

// SetWhatsAppConfigState publishes a new generation's first state and clears
// the previous connection's details. Counters survive generations.
func SetWhatsAppConfigState(state, mode string) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	s := &whatsAppRuntime.status
	enterWhatsAppState(s, state)
	s.Enabled = state != WhatsAppStateDisabled
	s.Mode = mode
	s.ConnectedSince = ""
	s.LastError = ""
	if state == WhatsAppStateDisabled || state == WhatsAppStateNotConfigured {
		s.Linked = false
		s.LinkedJIDMasked = ""
	}
}

// SetWhatsAppState moves the runtime to state. healthy clears the last error
// and keeps the login time, every other state drops it. not_linked and
// logged_out also drop the linked device.
func SetWhatsAppState(state string) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	s := &whatsAppRuntime.status
	enterWhatsAppState(s, state)
	if state == WhatsAppStateHealthy {
		s.LastError = ""
	}
}

// SetWhatsAppLinked records whether the store holds a linked device. It keeps
// only the last four digits of masked, so callers may pass a full JID.
func SetWhatsAppLinked(linked bool, masked string) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.Linked = linked
	if !linked {
		masked = ""
	}
	whatsAppRuntime.status.LinkedJIDMasked = whatsAppMask(masked)
}

// MarkWhatsAppConnected records the latest completed login.
func MarkWhatsAppConnected(now time.Time) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.ConnectedSince = whatsAppStamp(now)
}

// MarkWhatsAppError sets state together with a fixed error code.
func MarkWhatsAppError(state, code string) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	enterWhatsAppState(&whatsAppRuntime.status, state)
	whatsAppRuntime.status.LastError = code
}

// MarkWhatsAppTempBan records a temporary ban and keeps the last login time.
// A zero expiry means WhatsApp sent none.
func MarkWhatsAppTempBan(code string, expires time.Time) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	s := &whatsAppRuntime.status
	s.State = WhatsAppStateTempBanned
	s.LastError = code
	s.TempBanExpiresAt = ""
	if !expires.IsZero() {
		s.TempBanExpiresAt = whatsAppStamp(expires)
	}
}

// CountWhatsAppMessage counts an inbound message event before any filtering.
func CountWhatsAppMessage(now time.Time) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.MessagesReceived++
	whatsAppRuntime.status.LastInboundAt = whatsAppStamp(now)
}

// MarkWhatsAppOutbound records a delivered message.
func MarkWhatsAppOutbound(now time.Time) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.LastOutboundAt = whatsAppStamp(now)
}

// MarkWhatsAppDeliveryFailure counts a message that could not be delivered.
func MarkWhatsAppDeliveryFailure() {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.DeliveryFailures++
}

// MarkWhatsAppTest records a delivered manual test message.
func MarkWhatsAppTest(now time.Time) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.LastTestAt = whatsAppStamp(now)
}

// WhatsAppStatus returns a copy of the current snapshot.
func WhatsAppStatus() WhatsAppRuntimeStatus {
	whatsAppRuntime.RLock()
	defer whatsAppRuntime.RUnlock()
	return whatsAppRuntime.status
}
