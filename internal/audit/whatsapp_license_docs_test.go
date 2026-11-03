package audit

import (
	"strings"
	"testing"
)

// The WhatsApp stack links GPL-3.0 code. AuraGo's source stays MIT, and
// distributed binaries are GPL-3.0 combined works. Every inventory says so.
func TestWhatsAppLicenseNoticesStayAligned(t *testing.T) {
	t.Parallel()
	license := readRepoFile(t, "LICENSE")
	if !strings.Contains(license, "MIT License") || strings.Contains(license, "GNU GENERAL PUBLIC LICENSE") {
		t.Fatal("LICENSE must stay the MIT text")
	}
	requireAll(t, "THIRD_PARTY_NOTICES.md", readRepoFile(t, "licences/THIRD_PARTY_NOTICES.md"), []string{
		"## WhatsApp channel (whatsmeow and libsignal)",
		"`go.mau.fi/libsignal`",
		"GNU General Public License version 3",
		"`licences/texts/GPL-3.0-only.txt`",
	})
	requireAll(t, "THIRD_PARTY_LIBRARIES.md", readRepoFile(t, "licences/THIRD_PARTY_LIBRARIES.md"), []string{
		"| `go.mau.fi/whatsmeow` | v0.0.0-20261030195514-56e5ad0581ce | MPL-2.0 |",
		"| go.mau.fi/libsignal | GPL-3.0 |",
	})
	requireAll(t, "licences/UEBERSICHT.md", readRepoFile(t, "licences/UEBERSICHT.md"), []string{
		"`go.mau.fi/whatsmeow` v0.0.0-20261030195514-56e5ad0581ce",
		"GPL-3.0-Gesamtwerke",
	})
	requireAll(t, "README.md", readRepoFile(t, "README.md"), []string{
		"**Source code: [MIT licensed](LICENSE).**",
		"GPL-3.0 combined works",
	})
}
