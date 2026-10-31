package whatsapp

import (
	"testing"
	"time"

	"aurago/internal/integrationstatus"

	"go.mau.fi/whatsmeow/types/events"
)

func TestClassifyEventMapsStatesAndPolicies(t *testing.T) {
	const (
		connecting = integrationstatus.WhatsAppStateConnecting
		healthy    = integrationstatus.WhatsAppStateHealthy
		degraded   = integrationstatus.WhatsAppStateDegraded
	)
	for name, tc := range map[string]struct {
		evt    any
		synced bool
		want   eventEffect
	}{
		"connected":                      {&events.Connected{}, true, eventEffect{State: connecting, Connected: true}},
		"offline sync":                   {&events.OfflineSyncCompleted{Count: 3}, false, eventEffect{State: healthy, Synced: true, Healthy: true, SendPresence: true}},
		"keepalive restored before sync": {&events.KeepAliveRestored{}, false, eventEffect{}},
		"keepalive restored after sync":  {&events.KeepAliveRestored{}, true, eventEffect{State: healthy, Healthy: true}},
		"disconnected":                   {&events.Disconnected{}, true, eventEffect{State: degraded, ErrorCode: "disconnected"}},
		"keepalive timeout":              {&events.KeepAliveTimeout{ErrorCount: 2}, true, eventEffect{State: degraded, ErrorCode: "keepalive_timeout"}},
		"stream error":                   {&events.StreamError{Code: "999"}, true, eventEffect{State: degraded, ErrorCode: "stream_error"}},
		"logged out": {&events.LoggedOut{Reason: events.ConnectFailureLoggedOut}, true,
			eventEffect{State: integrationstatus.WhatsAppStateLoggedOut, ErrorCode: "logged_out", Terminal: true}},
		"stream replaced": {&events.StreamReplaced{}, true,
			eventEffect{State: integrationstatus.WhatsAppStateConflict, ErrorCode: "stream_replaced", Terminal: true}},
		"client outdated": {&events.ClientOutdated{}, false,
			eventEffect{State: integrationstatus.WhatsAppStateOutdated, ErrorCode: "client_outdated", Terminal: true}},
		"temporary ban": {&events.TemporaryBan{Code: events.TempBanSentToTooManyPeople, Expire: 2 * time.Hour}, true,
			eventEffect{State: integrationstatus.WhatsAppStateTempBanned, ErrorCode: "temp_ban_101", Terminal: true, RetryAfter: 2 * time.Hour}},
		"unknown connect failure": {&events.ConnectFailure{Reason: events.ConnectFailureReason(418)}, false,
			eventEffect{State: degraded, ErrorCode: "connect_failure_418", Reconnect: true}},
		"push name before sync": {&events.PushNameSetting{}, false, eventEffect{}},
		"push name after sync":  {&events.PushNameSetting{}, true, eventEffect{SendPresence: true}},
		"message":               {&events.Message{}, true, eventEffect{Message: true}},
		"history sync ignored":  {&events.HistorySync{}, true, eventEffect{}},
	} {
		if got := classifyEvent(tc.evt, tc.synced); got != tc.want {
			t.Errorf("%s: got %+v, want %+v", name, got, tc.want)
		}
	}
}
