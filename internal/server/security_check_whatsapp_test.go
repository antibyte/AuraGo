package server

import (
	"strings"
	"testing"

	"aurago/internal/config"
)

const (
	whatsAppSelfChatHintID  = "whatsapp_self_chat_personal_account"
	whatsAppNoOwnerHintID   = "whatsapp_dedicated_no_owner"
	whatsAppSecurityOwnerNr = "+4917012345678"
)

func whatsAppSecurityConfig(enabled bool, mode string, owners []string) *config.Config {
	cfg := &config.Config{}
	cfg.WhatsApp.Enabled = enabled
	cfg.WhatsApp.Mode = mode
	cfg.WhatsApp.OwnerNumbers = owners
	return cfg
}

func TestCheckSecurityWarnsAboutWhatsAppSelfChatOnAPersonalAccount(t *testing.T) {
	t.Parallel()

	hint := findSecurityHint(CheckSecurity(whatsAppSecurityConfig(true, config.WhatsAppModeSelfChat, nil)), whatsAppSelfChatHintID)
	if hint == nil {
		t.Fatal("expected the self-chat personal account hint")
	}
	if hint.Severity != SevWarning {
		t.Fatalf("severity = %q, want %q", hint.Severity, SevWarning)
	}
	if hint.AutoFixable || hint.FixPatch != nil {
		t.Fatalf("the hint must not be auto-fixable: %#v", hint)
	}
	for _, want := range []string{"device.db", "dedicated"} {
		if !strings.Contains(hint.Description, want) {
			t.Fatalf("description misses %q: %s", want, hint.Description)
		}
	}

	eggMode := whatsAppSecurityConfig(true, config.WhatsAppModeSelfChat, nil)
	eggMode.EggMode.Enabled = true
	for name, cfg := range map[string]*config.Config{
		"disabled":   whatsAppSecurityConfig(false, config.WhatsAppModeSelfChat, nil),
		"dedicated":  whatsAppSecurityConfig(true, config.WhatsAppModeDedicated, []string{whatsAppSecurityOwnerNr}),
		"egg mode":   eggMode,
		"empty mode": whatsAppSecurityConfig(true, "", []string{whatsAppSecurityOwnerNr}),
	} {
		if hasSecurityHint(CheckSecurity(cfg), whatsAppSelfChatHintID) {
			t.Fatalf("%s: no self-chat hint expected", name)
		}
	}
}

func TestCheckSecurityWarnsWhenDedicatedWhatsAppHasNoOwner(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{config.WhatsAppModeDedicated, ""} {
		for name, owners := range map[string][]string{
			"nil":     nil,
			"empty":   {},
			"invalid": {"0170 123"},
		} {
			hint := findSecurityHint(CheckSecurity(whatsAppSecurityConfig(true, mode, owners)), whatsAppNoOwnerHintID)
			if hint == nil {
				t.Fatalf("mode %q, owners %s: expected the no-owner hint", mode, name)
			}
			if hint.Severity != SevWarning || hint.AutoFixable || hint.FixPatch != nil {
				t.Fatalf("mode %q, owners %s: want a warning that is not auto-fixable, got %#v", mode, name, hint)
			}
		}
	}

	eggMode := whatsAppSecurityConfig(true, config.WhatsAppModeDedicated, nil)
	eggMode.EggMode.Enabled = true
	for name, cfg := range map[string]*config.Config{
		"valid owner":          whatsAppSecurityConfig(true, config.WhatsAppModeDedicated, []string{whatsAppSecurityOwnerNr}),
		"valid owner, no mode": whatsAppSecurityConfig(true, "", []string{whatsAppSecurityOwnerNr}),
		"self chat":            whatsAppSecurityConfig(true, config.WhatsAppModeSelfChat, nil),
		"disabled":             whatsAppSecurityConfig(false, config.WhatsAppModeDedicated, nil),
		"egg mode":             eggMode,
	} {
		hints := CheckSecurity(cfg)
		if hasSecurityHint(hints, whatsAppNoOwnerHintID) {
			t.Fatalf("%s: no no-owner hint expected", name)
		}
		for _, hint := range hints {
			if strings.Contains(hint.Title+hint.Description, "4917012345678") {
				t.Fatalf("%s: hint %s carries the owner number", name, hint.ID)
			}
		}
	}

	for _, hint := range CheckSecurity(whatsAppSecurityConfig(true, config.WhatsAppModeDedicated, []string{"0170 123"})) {
		if strings.Contains(hint.Title+hint.Description, "0170 123") {
			t.Fatalf("hint %s carries the typed owner number", hint.ID)
		}
	}
}
