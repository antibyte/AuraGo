package config

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"unicode/utf8"
)

// WhatsApp channel modes.
const (
	WhatsAppModeDedicated = "dedicated"
	WhatsAppModeSelfChat  = "self_chat"
)

// The linked-device keys live in <data_dir>/whatsapp/device.db. The store is
// never backed up (internal/audit sqliteBackupExclusions) and agent file tools
// never read it (internal/tools protectedSystemFilesFromConfig).
const (
	WhatsAppDirName          = "whatsapp"
	WhatsAppDeviceDBFilename = "device.db"
)

// DefaultWhatsAppSelfChatPrefix marks AuraGo's messages in the self chat.
const DefaultWhatsAppSelfChatPrefix = "🤖 "

const (
	maxWhatsAppOwnerNumbers = 10
	maxWhatsAppPrefixRunes  = 16
	minE164Digits           = 7
	maxE164Digits           = 15
)

// WhatsAppConfig controls the WhatsApp channel. No field is secret: the linked
// device in the store is the credential.
type WhatsAppConfig struct {
	Enabled          bool     `yaml:"enabled" json:"enabled"`
	Mode             string   `yaml:"mode" json:"mode"`
	OwnerNumbers     []string `yaml:"owner_numbers" json:"owner_numbers"`
	ReadOnly         bool     `yaml:"readonly" json:"readonly"`
	SendReadReceipts bool     `yaml:"send_read_receipts" json:"send_read_receipts"`
	SelfChatPrefix   string   `yaml:"self_chat_prefix" json:"self_chat_prefix"`
}

var errInvalidE164 = errors.New("not an international phone number")

// Equal reports whether two loaded sections are identical. Normalization
// turns empty owner lists into nil, so loaded sections compare reliably.
func (c WhatsAppConfig) Equal(other WhatsAppConfig) bool {
	return reflect.DeepEqual(c, other)
}

// ApplyWhatsAppDefaults runs before YAML is decoded: absent keys keep the
// defaults, explicit values (including false and "") win.
func ApplyWhatsAppDefaults(c *WhatsAppConfig) {
	if c == nil {
		return
	}
	c.Mode = WhatsAppModeDedicated
	c.SendReadReceipts = true
	c.SelfChatPrefix = DefaultWhatsAppSelfChatPrefix
}

// NormalizeWhatsAppConfig canonicalizes a loaded section without failing
// startup. An unknown mode becomes dedicated. Owner numbers become unique
// E.164 values, and invalid ones are dropped with a warning that carries no
// number. The config save path rejects them first.
func NormalizeWhatsAppConfig(c *WhatsAppConfig) {
	if c == nil {
		return
	}
	c.Mode = strings.ToLower(strings.TrimSpace(c.Mode))
	if c.Mode != WhatsAppModeSelfChat {
		c.Mode = WhatsAppModeDedicated
	}
	var owners []string
	seen := make(map[string]bool, len(c.OwnerNumbers))
	dropped := 0
	for _, raw := range c.OwnerNumbers {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		number, err := NormalizeE164(raw)
		if err != nil {
			dropped++
			continue
		}
		if seen[number] || len(owners) == maxWhatsAppOwnerNumbers {
			continue
		}
		seen[number] = true
		owners = append(owners, number)
	}
	if dropped > 0 {
		slog.Warn("[Config] Ignored invalid whatsapp.owner_numbers entries", "count", dropped)
	}
	c.OwnerNumbers = owners
	if utf8.RuneCountInString(c.SelfChatPrefix) > maxWhatsAppPrefixRunes {
		c.SelfChatPrefix = string([]rune(c.SelfChatPrefix)[:maxWhatsAppPrefixRunes])
	}
}

// ValidateWhatsAppConfig rejects values the save path must not persist.
func ValidateWhatsAppConfig(c WhatsAppConfig) error {
	switch strings.ToLower(strings.TrimSpace(c.Mode)) {
	case "", WhatsAppModeDedicated, WhatsAppModeSelfChat:
	default:
		return errors.New("whatsapp.mode must be dedicated or self_chat")
	}
	if len(c.OwnerNumbers) > maxWhatsAppOwnerNumbers {
		return fmt.Errorf("whatsapp.owner_numbers allows at most %d numbers", maxWhatsAppOwnerNumbers)
	}
	for i, raw := range c.OwnerNumbers {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if _, err := NormalizeE164(raw); err != nil {
			return fmt.Errorf("whatsapp.owner_numbers[%d] must be an international number such as +491701234567", i)
		}
	}
	if utf8.RuneCountInString(c.SelfChatPrefix) > maxWhatsAppPrefixRunes {
		return fmt.Errorf("whatsapp.self_chat_prefix allows at most %d characters", maxWhatsAppPrefixRunes)
	}
	return nil
}

// NormalizeE164 turns a typed phone number into +<digits>. Spaces, dashes,
// dots and parentheses are ignored, and a leading 00 counts as +. The result
// has 7 to 15 digits, and the first digit is not 0.
func NormalizeE164(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "00") {
		s = "+" + s[2:]
	}
	if !strings.HasPrefix(s, "+") {
		return "", errInvalidE164
	}
	var b strings.Builder
	b.WriteByte('+')
	digits := 0
	for _, r := range s[1:] {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			digits++
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')':
		default:
			return "", errInvalidE164
		}
	}
	out := b.String()
	if digits < minE164Digits || digits > maxE164Digits || out[1] == '0' {
		return "", errInvalidE164
	}
	return out, nil
}

// WhatsAppStoreDir is <dataDir>/whatsapp, or "" without a data directory.
func WhatsAppStoreDir(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		return ""
	}
	return filepath.Join(dataDir, WhatsAppDirName)
}

// WhatsAppDeviceDBPath is the linked-device database, or "" without a data
// directory.
func WhatsAppDeviceDBPath(dataDir string) string {
	dir := WhatsAppStoreDir(dataDir)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, WhatsAppDeviceDBFilename)
}

// WhatsAppProtectedPaths lists the device database and its SQLite sidecars,
// which agent file tools must refuse like the vault.
func WhatsAppProtectedPaths(cfg *Config) []string {
	if cfg == nil {
		return nil
	}
	db := WhatsAppDeviceDBPath(cfg.Directories.DataDir)
	if db == "" {
		return nil
	}
	return []string{db, db + "-wal", db + "-shm", db + "-journal"}
}
