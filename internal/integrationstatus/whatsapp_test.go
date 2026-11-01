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
		s.TempBanExpiresAt != "2026-10-31T13:00:00Z" || s.ConnectedSince != "" ||
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

func TestWhatsAppStatusDropsStaleFields(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	// not_linked and logged_out drop the linked device, whichever way they are entered.
	for name, enter := range map[string]func(){
		"state not_linked":  func() { SetWhatsAppState(WhatsAppStateNotLinked) },
		"config not_linked": func() { SetWhatsAppConfigState(WhatsAppStateNotLinked, "dedicated") },
		"error logged_out":  func() { MarkWhatsAppError(WhatsAppStateLoggedOut, "logged_out") },
		"state logged_out":  func() { SetWhatsAppState(WhatsAppStateLoggedOut) },
	} {
		SetWhatsAppConfigState(WhatsAppStateConnecting, "dedicated")
		SetWhatsAppLinked(true, "***5678")
		enter()
		if s := WhatsAppStatus(); s.Linked || s.LinkedJIDMasked != "" {
			t.Errorf("%s kept the linked device: %+v", name, s)
		}
	}

	// A ban expiry only lives in temp_banned; the login time only in healthy.
	SetWhatsAppConfigState(WhatsAppStateConnecting, "dedicated")
	MarkWhatsAppConnected(now)
	SetWhatsAppState(WhatsAppStateHealthy)
	SetWhatsAppState(WhatsAppStateHealthy)
	if s := WhatsAppStatus(); s.ConnectedSince != "2026-10-31T12:00:00Z" {
		t.Fatalf("healthy to healthy lost the login time: %+v", s)
	}
	MarkWhatsAppTempBan("temp_ban_101", now.Add(time.Hour))
	SetWhatsAppState(WhatsAppStateDegraded)
	if s := WhatsAppStatus(); s.TempBanExpiresAt != "" || s.ConnectedSince != "" {
		t.Fatalf("degraded kept stale ban expiry or login time: %+v", s)
	}

	// SetWhatsAppLinked masks whatever it receives.
	for raw, want := range map[string]string{
		"***5678":                        "***5678",
		"491701234567@s.whatsapp.net":    "***4567",
		"491701234567:12@s.whatsapp.net": "***4567",
		"+49 170 1234567":                "***4567",
		"123":                            "",
		"":                               "",
	} {
		SetWhatsAppLinked(true, raw)
		if got := WhatsAppStatus().LinkedJIDMasked; got != want {
			t.Errorf("SetWhatsAppLinked(%q) stored %q, want %q", raw, got, want)
		}
	}
}

func TestWhatsAppDropCountersAreSeparatedAndCopied(t *testing.T) {
	before := WhatsAppStatus()
	CountWhatsAppDrop(WhatsAppDropNonOwner)
	CountWhatsAppDrop(WhatsAppDropUnresolvedLID)
	CountWhatsAppDrop(WhatsAppDropUnresolvedLID)
	CountWhatsAppDrop("group")
	CountWhatsAppDrop("group")
	CountWhatsAppDrop("group")
	CountWhatsAppDrop("") // no reason, no count
	s := WhatsAppStatus()
	if s.DroppedNonOwner != before.DroppedNonOwner+1 || s.DroppedUnresolvedLID != before.DroppedUnresolvedLID+2 {
		t.Fatalf("own-field counters = %d/%d, before %d/%d", s.DroppedNonOwner, s.DroppedUnresolvedLID, before.DroppedNonOwner, before.DroppedUnresolvedLID)
	}
	if got, want := s.DroppedFiltered["group"], before.DroppedFiltered["group"]+3; got != want {
		t.Fatalf("DroppedFiltered[group] = %d, want %d", got, want)
	}
	for _, own := range []string{WhatsAppDropNonOwner, WhatsAppDropUnresolvedLID, ""} {
		if _, ok := s.DroppedFiltered[own]; ok {
			t.Fatalf("reason %q leaked into DroppedFiltered: %v", own, s.DroppedFiltered)
		}
	}

	// The snapshot is a copy: changing it changes neither the next snapshot nor the counter.
	s.DroppedFiltered["group"] = 9999
	s.DroppedFiltered["injected"] = 1
	next := WhatsAppStatus()
	if next.DroppedFiltered["group"] != before.DroppedFiltered["group"]+3 {
		t.Fatalf("a mutated snapshot changed the counter: %v", next.DroppedFiltered)
	}
	if _, ok := next.DroppedFiltered["injected"]; ok {
		t.Fatal("a key added to a snapshot reached the next one")
	}

	// A new generation does not reset the counters.
	SetWhatsAppConfigState(WhatsAppStateConnecting, "dedicated")
	if after := WhatsAppStatus(); after.DroppedFiltered["group"] != next.DroppedFiltered["group"] || after.DroppedNonOwner != next.DroppedNonOwner {
		t.Fatalf("SetWhatsAppConfigState reset the drop counters: %+v", after)
	}
}
