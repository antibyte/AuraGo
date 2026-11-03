package agent

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
)

// The tests below share the process-wide WhatsApp status, so none of them runs in parallel.
func TestWhatsAppIntegrationCheckReadsPassiveStatus(t *testing.T) {
	t.Cleanup(func() { integrationstatus.SetWhatsAppConfigState(integrationstatus.WhatsAppStateDisabled, "") })

	// Every row starts from a fresh generation, which clears the previous error code. The linked
	// device (fixture JID) lets each row prove that no masked JID reaches the result.
	fresh := func() {
		integrationstatus.SetWhatsAppConfigState(integrationstatus.WhatsAppStateConnecting, "dedicated")
		integrationstatus.SetWhatsAppLinked(true, "4917012345678@s.whatsapp.net")
	}
	tests := []struct {
		name       string
		setup      func()
		wantStatus string
		wantCode   string
	}{
		{"disabled", func() { integrationstatus.SetWhatsAppConfigState(integrationstatus.WhatsAppStateDisabled, "") }, "skipped", "disabled"},
		{"not configured", func() {
			integrationstatus.SetWhatsAppConfigState(integrationstatus.WhatsAppStateNotConfigured, "dedicated")
		}, "skipped", "not_configured"},
		{"healthy", func() { fresh(); integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateHealthy) }, "passed", ""},
		{"degraded with a code", func() {
			fresh()
			integrationstatus.MarkWhatsAppError(integrationstatus.WhatsAppStateDegraded, "keepalive_timeout")
		}, "failed", "keepalive_timeout"},
		{"not linked", func() { fresh(); integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateNotLinked) }, "failed", "not_linked"},
		{"connecting", fresh, "failed", "connecting"},
		{"logged out", func() {
			fresh()
			integrationstatus.MarkWhatsAppError(integrationstatus.WhatsAppStateLoggedOut, "logged_out")
		}, "failed", "logged_out"},
	}
	wantKeys := []string{"delivery_failures", "dropped_unresolved_lid", "linked", "state"}
	now := time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			result := runWhatsAppIntegrationCheck(now)
			if result.ID != "whatsapp" || result.Status != tt.wantStatus || result.Code != tt.wantCode {
				t.Fatalf("got id=%q status=%q code=%q, want whatsapp/%s/%s", result.ID, result.Status, result.Code, tt.wantStatus, tt.wantCode)
			}
			if result.CheckedAt != "2026-11-02T03:00:00Z" {
				t.Fatalf("CheckedAt = %q", result.CheckedAt)
			}
			var keys []string
			for key := range result.Data {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			if !reflect.DeepEqual(keys, wantKeys) {
				t.Fatalf("data keys = %v, want %v", keys, wantKeys)
			}
			status := integrationstatus.WhatsAppStatus()
			if result.Data["state"] != status.State || result.Data["linked"] != status.Linked ||
				result.Data["delivery_failures"] != status.DeliveryFailures ||
				result.Data["dropped_unresolved_lid"] != status.DroppedUnresolvedLID {
				t.Fatalf("data %v does not mirror the status %+v", result.Data, status)
			}
			if rendered := fmt.Sprintf("%+v", result); strings.Contains(rendered, "5678") || strings.Contains(rendered, "s.whatsapp.net") {
				t.Fatalf("the result carries a number or JID: %s", rendered)
			}
		})
	}
}

func TestMaintenanceIntegrationChecksIncludeWhatsAppAfterTelegram(t *testing.T) {
	t.Cleanup(func() { integrationstatus.SetWhatsAppConfigState(integrationstatus.WhatsAppStateDisabled, "") })
	integrationstatus.SetWhatsAppConfigState(integrationstatus.WhatsAppStateDisabled, "")

	var ids []string
	for _, check := range runMaintenanceIntegrationChecks(&config.Config{}, nil, time.Now()) {
		ids = append(ids, check.ID)
	}
	want := []string{"sandbox", "virtual_computers", "telegram", "whatsapp", "model_limits"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("check IDs = %v, want %v", ids, want)
	}
}
