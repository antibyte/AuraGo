package audit

import (
	"strings"
	"testing"
)

// fixtureWhatsAppNumber is the phone number the tests use. It must never reach a document.
const fixtureWhatsAppNumber = "4917012345678"

// The operator guide carries the WhatsApp design rules. Dropping one of them would let an
// operator link AuraGo without learning about the ban risk or the phone rules.
func TestWhatsAppSetupGuideCoversOperatorRules(t *testing.T) {
	t.Parallel()
	const name = "documentation/whatsapp_setup.md"
	doc := readRepoFile(t, name)
	requireAll(t, name, doc, []string{
		"# WhatsApp Setup Guide",
		"## Before you start",
		"## Choose a mode",
		"## Step 1: Configure AuraGo",
		"## Step 2: Link the account",
		"## Step 3: Send a test message",
		"## Phone rules",
		"## No bulk sending",
		"## Read-only mode",
		"## What reaches WhatsApp",
		"## Status and troubleshooting",
		"## Unlink, relink and backups",
		"## Security and privacy",
		"## License",
		// The design rules.
		"at least every 14 days", "up to 4 linked devices", "dedicated number", "never bulk-send",
		"20 messages per minute",
		// The configuration and the operator surfaces.
		"whatsapp:", "mode: dedicated", "self_chat", "owner_numbers", "device.db",
		"allow_unsafe_host_execution", "/api/whatsapp/status", "integration:whatsapp",
		// The 11 states, as table rows of the troubleshooting table.
		"| `disabled` |", "| `not_configured` |", "| `not_linked` |", "| `linking` |", "| `connecting` |",
		"| `healthy` |", "| `degraded` |", "| `logged_out` |", "| `conflict` |", "| `temp_banned` |", "| `outdated` |",
		"`dropped_non_owner`", "`dropped_unresolved_lid`", "`dropped_filtered`", "`delivery_failures`",
		// The morning briefing line, as maintenance_briefing.go renders it.
		"### The morning briefing line",
		"- whatsapp: skipped (disabled)", "- whatsapp: skipped (not_configured)", "- whatsapp: passed", "- whatsapp: failed (<code>)",
		// The security hints in internal/server/security_check.go.
		"`whatsapp_self_chat_personal_account`", "`whatsapp_dedicated_no_owner`",
		// The license position.
		"GPL-3.0", "THIRD_PARTY_NOTICES.md",
	})
}

func TestWhatsAppManualSectionsAndReadme(t *testing.T) {
	t.Parallel()
	en08 := readRepoFile(t, "documentation/manuals/en/08-integrations.md")
	de08 := readRepoFile(t, "documentation/manuals/de/08-integrations.md")
	requireAll(t, "manuals/en/08-integrations.md", en08, []string{
		"[WhatsApp](#whatsapp-setup)", "### WhatsApp Setup", "#### Web UI Setup", "whatsapp:",
		"(../../whatsapp_setup.md)", "14 days", "4 linked devices", "bulk-send",
	})
	requireAll(t, "manuals/de/08-integrations.md", de08, []string{
		"[WhatsApp](#whatsapp-einrichten)", "### WhatsApp einrichten", "#### Einrichtung in der Web-UI", "whatsapp:",
		"(../../whatsapp_setup.md)", "14 Tage", "4 verknüpfte Geräte", "Massenversand",
	})
	// The sections sit after Telegram and before the testing chapter.
	for _, tt := range []struct{ name, doc, telegram, section, next string }{
		{"en", en08, "### Telegram Bot Setup", "### WhatsApp Setup", "\n## Testing Integrations\n"},
		{"de", de08, "### Telegram Bot Setup", "### WhatsApp einrichten", "\n## Integrationen testen\n"},
	} {
		telegram, section, next := strings.Index(tt.doc, tt.telegram), strings.Index(tt.doc, tt.section), strings.Index(tt.doc, tt.next)
		if telegram < 0 || section < 0 || next < 0 || !(telegram < section && section < next) {
			t.Errorf("%s 08-integrations: want %q < %q < %q, got %d, %d, %d", tt.name, tt.telegram, tt.section, tt.next, telegram, section, next)
		}
	}

	en07 := readRepoFile(t, "documentation/manuals/en/07-configuration.md")
	de07 := readRepoFile(t, "documentation/manuals/de/07-configuration.md")
	requireAll(t, "manuals/en/07-configuration.md", en07, []string{
		"| `whatsapp` | WhatsApp channel (linked device, owner numbers) |",
		"whatsapp:         # WhatsApp channel (linked device)",
		"| `whatsapp` | WhatsApp channel | `whatsapp:`<br>`  enabled: false`<br>`  mode: dedicated`",
	})
	requireAll(t, "manuals/de/07-configuration.md", de07, []string{
		"whatsapp:         # WhatsApp-Kanal (verknüpftes Gerät)",
		"| `whatsapp` | WhatsApp-Kanal | `whatsapp:`<br>`  enabled: false`<br>`  mode: dedicated`",
	})

	en14 := readRepoFile(t, "documentation/manuals/en/14-security.md")
	de14 := readRepoFile(t, "documentation/manuals/de/14-security.md")
	requireAll(t, "manuals/en/14-security.md", en14, []string{"`data/whatsapp/device.db`", "`data/vault.bin`"})
	requireAll(t, "manuals/de/14-security.md", de14, []string{"`data/whatsapp/device.db`", "`data/vault.bin`"})

	readme := readRepoFile(t, "README.md")
	requireAll(t, "README.md", readme, []string{
		"- **Give it a voice.** Telegram, WhatsApp, Discord, Rocket.Chat, email, realtime speech and SIP telephony.",
		"[WhatsApp setup](documentation/whatsapp_setup.md)",
	})

	// No document may carry the fixture number.
	for name, doc := range map[string]string{
		"documentation/whatsapp_setup.md":              readRepoFile(t, "documentation/whatsapp_setup.md"),
		"documentation/manuals/en/08-integrations.md":  en08,
		"documentation/manuals/de/08-integrations.md":  de08,
		"documentation/manuals/en/07-configuration.md": en07,
		"documentation/manuals/de/07-configuration.md": de07,
		"documentation/manuals/en/14-security.md":      en14,
		"documentation/manuals/de/14-security.md":      de14,
		"README.md": readme,
	} {
		if strings.Contains(doc, fixtureWhatsAppNumber) {
			t.Errorf("%s contains the fixture number %s", name, fixtureWhatsAppNumber)
		}
	}
}
