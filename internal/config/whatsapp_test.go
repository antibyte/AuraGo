package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func loadWhatsAppYAML(t *testing.T, content string) *Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestWhatsAppLoadDefaultsAndNormalization(t *testing.T) {
	w := loadWhatsAppYAML(t, "{}\n").WhatsApp
	if w.Enabled || w.ReadOnly || w.Mode != WhatsAppModeDedicated || !w.SendReadReceipts ||
		w.SelfChatPrefix != DefaultWhatsAppSelfChatPrefix || w.OwnerNumbers != nil {
		t.Fatalf("defaults = %+v", w)
	}
	w = loadWhatsAppYAML(t, "whatsapp:\n  enabled: true\n  mode: SELF_CHAT\n  send_read_receipts: false\n  self_chat_prefix: \"\"\n"+
		"  owner_numbers: [\"+49 170 123-4567\", \"0049 (170) 1234567\", \"12345\", \"\", \"+43 660 1234567\"]\n").WhatsApp
	if !w.Enabled || w.Mode != WhatsAppModeSelfChat || w.SendReadReceipts || w.SelfChatPrefix != "" {
		t.Fatalf("explicit values lost: %+v", w)
	}
	if got := strings.Join(w.OwnerNumbers, ","); got != "+491701234567,+436601234567" {
		t.Fatalf("owner numbers = %q", got)
	}
	if mode := loadWhatsAppYAML(t, "whatsapp:\n  mode: bogus\n").WhatsApp.Mode; mode != WhatsAppModeDedicated {
		t.Fatalf("unknown mode loaded as %q, want dedicated", mode)
	}
}

func TestNormalizeE164(t *testing.T) {
	for raw, want := range map[string]string{
		"+491701234567":         "+491701234567",
		" +49 (170) 123-45.67 ": "+491701234567",
		"0043 660 1234567":      "+436601234567",
		"+1234567":              "+1234567",
		"+123456789012345":      "+123456789012345",
	} {
		if got, err := NormalizeE164(raw); err != nil || got != want {
			t.Errorf("NormalizeE164(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"", "01701234567", "+0491701234567", "+49170x1234567", "+123456", "+1234567890123456"} {
		if got, err := NormalizeE164(raw); err == nil {
			t.Errorf("NormalizeE164(%q) = %q, want an error", raw, got)
		}
	}
}

func TestValidateWhatsAppConfig(t *testing.T) {
	valid := WhatsAppConfig{Mode: "self_chat", OwnerNumbers: []string{"+49 170 1234567", ""}, SelfChatPrefix: DefaultWhatsAppSelfChatPrefix}
	if err := ValidateWhatsAppConfig(valid); err != nil {
		t.Fatalf("valid section rejected: %v", err)
	}
	var tooMany []string
	for i := 0; i < 11; i++ {
		tooMany = append(tooMany, fmt.Sprintf("+4917000000%02d", i))
	}
	// The cap counts distinct numbers: a blank row and a reformatted duplicate do not count.
	atCap := append(slices.Clone(tooMany[:10]), "", "0049 1700 000000")
	if err := ValidateWhatsAppConfig(WhatsAppConfig{OwnerNumbers: atCap}); err != nil {
		t.Fatalf("10 distinct numbers plus a blank row and a duplicate rejected: %v", err)
	}
	for name, c := range map[string]WhatsAppConfig{
		"mode":     {Mode: "group"},
		"number":   {OwnerNumbers: []string{"+491701234567", "01701234567"}},
		"too many": {OwnerNumbers: tooMany},
		"prefix":   {SelfChatPrefix: strings.Repeat("x", 17)},
	} {
		if ValidateWhatsAppConfig(c) == nil {
			t.Errorf("%s: invalid section accepted", name)
		}
	}
}

func TestWhatsAppStorePathsStayOutOfBackups(t *testing.T) {
	if WhatsAppDeviceDBPath("") != "" || WhatsAppProtectedPaths(&Config{}) != nil {
		t.Fatal("an empty data dir produced a store path")
	}
	cfg := &Config{}
	cfg.Directories.DataDir = filepath.Join("srv", "data")
	db := filepath.Join("srv", "data", "whatsapp", "device.db")
	if got := WhatsAppDeviceDBPath(cfg.Directories.DataDir); got != db {
		t.Fatalf("device db = %q, want %q", got, db)
	}
	if got, want := WhatsAppProtectedPaths(cfg), []string{db, db + "-wal", db + "-shm", db + "-journal"}; !slices.Equal(got, want) {
		t.Fatalf("protected paths = %v, want %v", got, want)
	}
	if slices.Contains(SQLiteDatabasePaths(cfg), db) {
		t.Fatal("the WhatsApp device store must never be backed up")
	}
}

func TestNormalizeWhatsAppConfigWarnsWhenOwnerCapTruncates(t *testing.T) {
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	var c WhatsAppConfig
	for i := 0; i < 12; i++ {
		c.OwnerNumbers = append(c.OwnerNumbers, fmt.Sprintf("+4917000000%02d", i))
	}
	c.OwnerNumbers = append(c.OwnerNumbers, c.OwnerNumbers[11]) // a repeated dropped number counts once
	NormalizeWhatsAppConfig(&c)
	if len(c.OwnerNumbers) != 10 {
		t.Fatalf("kept %d owner numbers, want 10", len(c.OwnerNumbers))
	}
	if out := logged.String(); !strings.Contains(out, "count=2") || strings.Contains(out, "4917000000") {
		t.Fatalf("truncation warning = %q, want the dropped count and no number", out)
	}
}
