package integrationstatus

import (
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

// SetWhatsAppConfigState publishes a new generation's first state and clears
// the previous connection's details. Counters survive generations.
func SetWhatsAppConfigState(state, mode string) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	s := &whatsAppRuntime.status
	s.Enabled = state != WhatsAppStateDisabled
	s.State = state
	s.Mode = mode
	s.ConnectedSince = ""
	s.LastError = ""
	s.TempBanExpiresAt = ""
	if state == WhatsAppStateDisabled || state == WhatsAppStateNotConfigured {
		s.Linked = false
		s.LinkedJIDMasked = ""
	}
}

// SetWhatsAppState moves the runtime to state. healthy clears the last error
// and an expired temporary ban.
func SetWhatsAppState(state string) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	s := &whatsAppRuntime.status
	s.State = state
	switch state {
	case WhatsAppStateHealthy:
		s.LastError = ""
		s.TempBanExpiresAt = ""
	case WhatsAppStateNotLinked, WhatsAppStateLinking:
		s.ConnectedSince = ""
	}
}

// SetWhatsAppLinked records whether the store holds a linked device and its
// masked number.
func SetWhatsAppLinked(linked bool, masked string) {
	whatsAppRuntime.Lock()
	defer whatsAppRuntime.Unlock()
	whatsAppRuntime.status.Linked = linked
	if !linked {
		masked = ""
	}
	whatsAppRuntime.status.LinkedJIDMasked = masked
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
	whatsAppRuntime.status.State = state
	whatsAppRuntime.status.LastError = code
}

// MarkWhatsAppTempBan records a temporary ban. A zero expiry means WhatsApp
// sent none.
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
