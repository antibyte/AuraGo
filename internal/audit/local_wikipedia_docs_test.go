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
