package audit

import (
	"os"
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

func TestLocalWikipediaManualConfigurationReference(t *testing.T) {
	t.Parallel()
	en := readRepoFile(t, "documentation/manual/en/07-configuration.md")
	requireAll(t, "manual/en/07-configuration.md", en, []string{
		"### Local Wikipedia",
		"| `local_wikipedia` | Offline Wikipedia edition (Kiwix ZIM), agent tool and desktop app. |",
		"update_check: true        # daily catalog check; updates still need a click",
	})
	de := readRepoFile(t, "documentation/manual/de/07-konfiguration.md")
	requireAll(t, "manual/de/07-konfiguration.md", de, []string{
		"### Lokale Wikipedia",
		"| `local_wikipedia` | Offline-Wikipedia (Kiwix-ZIM), Agenten-Tool und Desktop-App |",
		"update_check: true        # tägliche Katalogprüfung; Updates nur per Klick",
	})
	if got := strings.Count(de, "| `local_wikipedia` |"); got != 2 {
		t.Errorf("German chapter 7 must list local_wikipedia in both block tables, found %d rows", got)
	}
	if !strings.Contains(readRepoFile(t, "config_template.yaml"), "\nlocal_wikipedia:\n") {
		t.Error("config_template.yaml must contain the local_wikipedia block (slice 3)")
	}
}

func TestLocalWikipediaManualToolDashboardAndAPIEntries(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"documentation/manual/en/06-tools.md":          {"| **Local Wikipedia** | `local_wikipedia`"},
		"documentation/manual/de/06-tools.md":          {"| **Lokale Wikipedia** | `local_wikipedia`"},
		"documentation/manual/en/13-dashboard.md":      {"Local Wikipedia shows **New edition**"},
		"documentation/manual/de/13-dashboard.md":      {"Lokale Wikipedia zeigt **Neue Ausgabe**"},
		"documentation/manual/en/22-internal-tools.md": {"### `local_wikipedia`", "| `offset` | integer |"},
		"documentation/manual/de/22-interne-tools.md":  {"### `local_wikipedia`"},
		"documentation/manual/en/21-api-reference.md": {
			"87. [Local Wikipedia API](#local-wikipedia-api)", "88. [SSE Events](#sse-events)",
			"## Local Wikipedia API", "POST /api/local-wikipedia/install", "GET  /api/desktop/local-wikipedia/content/{path}",
		},
		"documentation/manual/de/21-api-reference.md": {
			"87. [Local Wikipedia API](#local-wikipedia-api)", "88. [SSE Events](#sse-events)",
			"## Local Wikipedia API", "POST /api/local-wikipedia/install", "GET  /api/desktop/local-wikipedia/content/{path}",
		},
	}
	for path, wants := range cases {
		requireAll(t, path, readRepoFile(t, path), wants)
	}
}

func TestLocalWikipediaReadmeMention(t *testing.T) {
	t.Parallel()
	readme := readRepoFile(t, "README.md")
	requireAll(t, "README.md", readme, []string{
		"- **Wikipedia in your pocket.**",
		"[Local Wikipedia](documentation/local-wikipedia.md)",
	})
}

func TestLocalWikipediaContractIsRouted(t *testing.T) {
	t.Parallel()
	root := readRepoFile(t, "AGENTS.md")
	if got := strings.Count(root, "`internal/localwiki/AGENTS.md`"); got < 2 {
		t.Errorf("root AGENTS.md must route Local Wikipedia in the contract table and the Child DOX Index, found %d references", got)
	}
	requireAll(t, "AGENTS.md", root, []string{"| Local Wikipedia Contract |"})
	contract := readRepoFile(t, "internal/localwiki/AGENTS.md")
	requireAll(t, "internal/localwiki/AGENTS.md", contract, []string{
		"## Purpose", "## Ownership", "## Local Contracts", "### Local Wikipedia Contract", "## Verification", "## Child DOX Index",
		"no CGO", "HTTPS only", "max(1 GiB, 1 % of size)", "refcounted", "`interrupted`", "Content-Security-Policy",
		"`security.IsolateExternalData`", "GPL", "internal/zim", "ui/js/desktop/apps/AGENTS.md",
	})
	// The app section is slice 5's contract; it is only required once the app exists.
	if _, err := os.Stat(repoPath("ui", "js", "desktop", "apps", "local-wikipedia.js")); err == nil {
		if !strings.Contains(readRepoFile(t, "ui/js/desktop/apps/AGENTS.md"), "local-wikipedia") {
			t.Error("ui/js/desktop/apps/AGENTS.md must carry the Local Wikipedia app section (slice 5)")
		}
	}
}
