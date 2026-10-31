package integrationstatus

import (
	"testing"
	"time"
)

func TestWhatsAppStatusTransitions(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	SetWhatsAppConfigState(WhatsAppStateDisabled, "")
	if s := WhatsAppStatus(); s.Enabled || s.State != WhatsAppStateDisabled {
		t.Fatalf("disabled status = %+v", s)
	}
	SetWhatsAppConfigState(WhatsAppStateConnecting, "dedicated")
	SetWhatsAppLinked(true, "***5678")
	MarkWhatsAppConnected(now)
	MarkWhatsAppTempBan("temp_ban_101", now.Add(time.Hour))
	s := WhatsAppStatus()
	if !s.Enabled || s.State != WhatsAppStateTempBanned || s.LastError != "temp_ban_101" ||
		s.TempBanExpiresAt != "2026-10-31T13:00:00Z" || s.ConnectedSince != "2026-10-31T12:00:00Z" ||
		!s.Linked || s.LinkedJIDMasked != "***5678" || s.Mode != "dedicated" {
		t.Fatalf("temp ban status = %+v", s)
	}
	received, failures := s.MessagesReceived, s.DeliveryFailures
	CountWhatsAppMessage(now)
	MarkWhatsAppDeliveryFailure()
	MarkWhatsAppOutbound(now)
	SetWhatsAppState(WhatsAppStateHealthy)
	s = WhatsAppStatus()
	if s.State != WhatsAppStateHealthy || s.LastError != "" || s.TempBanExpiresAt != "" ||
		s.MessagesReceived != received+1 || s.DeliveryFailures != failures+1 ||
		s.LastInboundAt == "" || s.LastOutboundAt == "" {
		t.Fatalf("healthy status = %+v", s)
	}
	SetWhatsAppConfigState(WhatsAppStateNotConfigured, "dedicated")
	if s := WhatsAppStatus(); !s.Enabled || s.Linked || s.LinkedJIDMasked != "" || s.ConnectedSince != "" {
		t.Fatalf("not_configured status kept connection details: %+v", s)
	}
}
