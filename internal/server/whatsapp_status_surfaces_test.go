package server

import (
	"context"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/selftest"
	"aurago/internal/whatsapp"
)

func TestDashboardIntegrationFlagsIncludeWhatsApp(t *testing.T) {
	cfg := &config.Config{}
	enabled, ok := dashboardIntegrationFlags(cfg)["whatsapp"]
	if !ok || enabled {
		t.Fatalf("whatsapp flag = %v (present %v), want present and false", enabled, ok)
	}
	cfg.WhatsApp.Enabled = true
	if !dashboardIntegrationFlags(cfg)["whatsapp"] {
		t.Fatal("whatsapp flag must follow whatsapp.enabled")
	}
}

// sendForbiddenWhatsAppController fails the test on any call that could reach
// the network or the owner.
type sendForbiddenWhatsAppController struct{ t *testing.T }

func (c sendForbiddenWhatsAppController) Link(context.Context, whatsapp.LinkRequest, bool) (whatsapp.LinkSnapshot, error) {
	c.t.Fatal("the WhatsApp self-test probe must not start a link")
	return whatsapp.LinkSnapshot{}, nil
}

func (c sendForbiddenWhatsAppController) LinkSnapshot() whatsapp.LinkSnapshot {
	return whatsapp.LinkSnapshot{}
}

func (c sendForbiddenWhatsAppController) Logout(context.Context) (whatsapp.LogoutResult, error) {
	c.t.Fatal("the WhatsApp self-test probe must not unlink")
	return whatsapp.LogoutResult{}, nil
}

func (c sendForbiddenWhatsAppController) SendTest(context.Context, time.Time) error {
	c.t.Fatal("the WhatsApp self-test probe must never send a message")
	return nil
}

// No t.Parallel: the test changes the process-wide WhatsApp status.
func TestSelfTestWhatsAppProbeReadsStatusOnly(t *testing.T) {
	previous := integrationstatus.WhatsAppStatus()
	t.Cleanup(func() { integrationstatus.SetWhatsAppConfigState(previous.State, previous.Mode) })

	cfg := &config.Config{}
	cfg.WhatsApp.Enabled = true
	inventory := selfTestIntegrations(cfg)
	found := false
	for _, capability := range inventory {
		found = found || capability.Name == "integration:whatsapp"
	}
	if !found {
		t.Fatalf("selfTestIntegrations(enabled) = %+v, want integration:whatsapp", inventory)
	}
	s := &Server{Cfg: cfg, whatsAppCtl: sendForbiddenWhatsAppController{t: t}}
	s.bindSelfTestProbes(inventory)
	for _, capability := range inventory {
		if capability.Name == "integration:whatsapp" && (!capability.Callable || capability.Status != "active") {
			t.Fatalf("integration:whatsapp = %+v, want callable and active", capability)
		}
	}

	for _, tc := range []struct {
		state       string
		wantVerdict string
		wantCode    string
	}{
		{integrationstatus.WhatsAppStateHealthy, "PASS", ""},
		{integrationstatus.WhatsAppStateDegraded, "FAIL", "connection_probe_failed"},
		{integrationstatus.WhatsAppStateNotLinked, "FAIL", "setup"},
		{integrationstatus.WhatsAppStateDisabled, "FAIL", "setup"},
	} {
		integrationstatus.SetWhatsAppConfigState(tc.state, config.WhatsAppModeDedicated)
		out := s.executeSelfTestProbe(context.Background(), selftest.Case{Tool: "integration:whatsapp"}, nil)
		if out.Verdict != tc.wantVerdict || out.Code != tc.wantCode {
			t.Errorf("state %s: verdict %q code %q, want %q %q", tc.state, out.Verdict, out.Code, tc.wantVerdict, tc.wantCode)
		}
	}
}
