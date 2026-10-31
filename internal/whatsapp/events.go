package whatsapp

import (
	"fmt"
	"time"

	"aurago/internal/integrationstatus"

	"go.mau.fi/whatsmeow/types/events"
)

// eventEffect is what one whatsmeow event means for the runtime.
type eventEffect struct {
	State        string        // status state to publish; "" keeps the current one
	ErrorCode    string        // fixed code for status.last_error
	Connected    bool          // a login completed (events.Connected)
	Synced       bool          // the offline sync of this connection finished
	Healthy      bool          // messages flow; resets the connect backoff
	Terminal     bool          // stop retrying until the user acts or RetryAfter passes
	Reconnect    bool          // whatsmeow will not reconnect by itself
	RetryAfter   time.Duration // temporary ban expiry; 0 waits for user action
	SendPresence bool          // publish presence "unavailable"
	Message      bool          // an inbound message (stage 1 only counts it)
}

// classifyEvent maps a whatsmeow event to status and reconnect policy. synced
// reports whether the current connection already finished its offline sync.
// StreamReplaced, ClientOutdated and TemporaryBan are hard errors (the
// mautrix-whatsapp pattern). The connection is healthy only after
// OfflineSyncCompleted. Transient drops are left to whatsmeow's own
// auto-reconnect. ConnectFailure disables that auto-reconnect, so the Bot
// reconnects itself with backoff.
func classifyEvent(evt any, synced bool) eventEffect {
	switch e := evt.(type) {
	case *events.Connected:
		return eventEffect{State: integrationstatus.WhatsAppStateConnecting, Connected: true}
	case *events.OfflineSyncCompleted:
		return eventEffect{State: integrationstatus.WhatsAppStateHealthy, Synced: true, Healthy: true, SendPresence: true}
	case *events.KeepAliveRestored:
		if synced {
			return eventEffect{State: integrationstatus.WhatsAppStateHealthy, Healthy: true}
		}
	case *events.Disconnected:
		return eventEffect{State: integrationstatus.WhatsAppStateDegraded, ErrorCode: "disconnected"}
	case *events.KeepAliveTimeout:
		return eventEffect{State: integrationstatus.WhatsAppStateDegraded, ErrorCode: "keepalive_timeout"}
	case *events.StreamError:
		return eventEffect{State: integrationstatus.WhatsAppStateDegraded, ErrorCode: "stream_error"}
	case *events.LoggedOut:
		return eventEffect{State: integrationstatus.WhatsAppStateLoggedOut, ErrorCode: "logged_out", Terminal: true}
	case *events.StreamReplaced:
		return eventEffect{State: integrationstatus.WhatsAppStateConflict, ErrorCode: "stream_replaced", Terminal: true}
	case *events.ClientOutdated:
		return eventEffect{State: integrationstatus.WhatsAppStateOutdated, ErrorCode: "client_outdated", Terminal: true}
	case *events.TemporaryBan:
		return eventEffect{
			State:      integrationstatus.WhatsAppStateTempBanned,
			ErrorCode:  fmt.Sprintf("temp_ban_%d", int(e.Code)),
			Terminal:   true,
			RetryAfter: e.Expire,
		}
	case *events.ConnectFailure:
		return eventEffect{
			State:     integrationstatus.WhatsAppStateDegraded,
			ErrorCode: "connect_failure_" + e.Reason.NumberString(),
			Reconnect: true,
		}
	case *events.PushNameSetting:
		if synced {
			return eventEffect{SendPresence: true}
		}
	case *events.Message:
		return eventEffect{Message: true}
	}
	return eventEffect{}
}
