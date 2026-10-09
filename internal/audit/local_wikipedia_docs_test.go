package audit

import (
	"strings"
	"testing"
)

func requireAll(t *testing.T, name, doc string, wants []string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(doc, want) {
			t.Errorf("%s is missing %q", name, want)
		}
	}
}

func TestLocalWikipediaGuideCoversOperatorTopics(t *testing.T) {
	t.Parallel()
	doc := readRepoFile(t, "documentation/local-wikipedia.md")
	requireAll(t, "documentation/local-wikipedia.md", doc, []string{
		"# Local Wikipedia",
		"## Sizes",
		"## Install, update and disk space",
		"## Where the files live",
		"## The agent tool",
		"## The desktop app",
		"## Dashboard",
		"## Permissions and API",
		"## Troubleshooting",
		"## Privacy and network",
		"## Licenses",
		"## Limits",
		"`insufficient_disk_space`", "`free_space_unknown`", "`checksum_mismatch`", "`download_failed`",
		"`catalog_unreachable`", "`zim_unreadable`", "`state_unreadable`", "`fulltext_unsupported`", "`busy`", "`disabled`",
		"`data_dir_invalid`", "`already_installed`",
		"opds.library.kiwix.org", "download.kiwix.org", "HTTPS",
		"CC BY-SA 4.0", "Wikimedia Foundation",
		"ReadWritePaths", "/app/data/wikipedia", "aurago_data",
		"max(1 GiB, 1 % of the edition size)",
		"`local_wikipedia`", "ui/js/desktop/apps/AGENTS.md",
		"/api/local-wikipedia/install", "/api/desktop/local-wikipedia/content/",
		"`readable`", "`loading`",
	})
}

func TestLocalWikipediaManualIntegrationSections(t *testing.T) {
	t.Parallel()
	en := readRepoFile(t, "documentation/manual/en/08-integrations.md")
	requireAll(t, "manual/en/08-integrations.md", en, []string{
		"| Knowledge offline | [Local Wikipedia](#local-wikipedia) |",
		"## Local Wikipedia",
		"### Web UI Setup",
		"local_wikipedia:",
		"(../../local-wikipedia.md)",
		"(07-configuration.md#local-wikipedia)",
	})
	de := readRepoFile(t, "documentation/manual/de/08-integrations.md")
	requireAll(t, "manual/de/08-integrations.md", de, []string{
		"| Wissen offline | [Lokale Wikipedia](#lokale-wikipedia) |",
		"## Lokale Wikipedia",
		"### Einrichtung in der Web-UI",
		"local_wikipedia:",
		"(../../local-wikipedia.md)",
		"(07-konfiguration.md#lokale-wikipedia)",
	})
	if strings.Index(en, "## Local Wikipedia") > strings.Index(en, "## Testing Integrations") {
		t.Error("English Local Wikipedia section must come before Testing Integrations")
	}
	if strings.Index(de, "## Lokale Wikipedia") > strings.Index(de, "## Integrationen testen") {
		t.Error("German Lokale Wikipedia section must come before Integrationen testen")
	}
}
