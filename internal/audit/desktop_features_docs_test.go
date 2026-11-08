package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The desktop apps that shipped in November 2026 must be discoverable in the manual. The
// registry in internal/desktop/types.go is the source of truth; these checks keep the
// chapter 4 app table, the tool catalog and the configuration chapter from falling behind
// it again.

func TestManualHubChapterCountMatchesTheFiles(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(repoPath("documentation", "manuals", "en"))
	if err != nil {
		t.Fatalf("read manual chapters: %v", err)
	}
	chapter := regexp.MustCompile(`^(\d{2})-[a-z0-9-]+\.md$`)
	numbers := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if match := chapter.FindStringSubmatch(entry.Name()); match != nil {
			numbers[match[1]] = true
		}
	}
	if len(numbers) == 0 {
		t.Fatal("no chapter files found in documentation/manuals/en")
	}

	hub := readRepoFile(t, "documentation/manuals/README.md")
	match := regexp.MustCompile(`same (\d+) chapters`).FindStringSubmatch(hub)
	if match == nil {
		t.Fatal("documentation/manuals/README.md does not state the chapter count")
	}
	count, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatalf("chapter count %q: %v", match[1], err)
	}
	if count != len(numbers) {
		t.Errorf("documentation/manuals/README.md claims %d chapters, the manual has %d", count, len(numbers))
	}

	// The German mirror must ship the same chapters.
	deEntries, err := os.ReadDir(repoPath("documentation", "manuals", "de"))
	if err != nil {
		t.Fatalf("read german manual chapters: %v", err)
	}
	deNumbers := map[string]bool{}
	for _, entry := range deEntries {
		if entry.IsDir() {
			continue
		}
		if match := chapter.FindStringSubmatch(entry.Name()); match != nil {
			deNumbers[match[1]] = true
		}
	}
	for number := range numbers {
		if !deNumbers[number] {
			t.Errorf("documentation/manuals/de is missing chapter %s", number)
		}
	}
	for number := range deNumbers {
		if !numbers[number] {
			t.Errorf("documentation/manuals/en is missing chapter %s", number)
		}
	}
}

func TestNewManualChaptersAreIndexed(t *testing.T) {
	t.Parallel()
	chapters := map[string]string{
		"34-storymaker.md":        "34-storymaker.md",
		"35-goldfish-adventure.md": "35-goldfish-adventure.md",
		"36-synth-studio.md":       "36-synth-studio.md",
	}
	for file := range chapters {
		readRepoFile(t, "documentation/manuals/en/"+file)
		readRepoFile(t, "documentation/manuals/de/"+file)
	}

	hub := readRepoFile(t, "documentation/manuals/README.md")
	enIndex := readRepoFile(t, "documentation/manuals/en/README.md")
	deIndex := readRepoFile(t, "documentation/manuals/de/README.md")
	for file := range chapters {
		requireAll(t, "documentation/manuals/README.md", hub, []string{"en/" + file, "de/" + file})
		requireAll(t, "documentation/manuals/en/README.md", enIndex, []string{file})
		requireAll(t, "documentation/manuals/de/README.md", deIndex, []string{file})
	}
}

func TestStorymakerAndDesktopAppsAreDocumented(t *testing.T) {
	t.Parallel()
	en04 := readRepoFile(t, "documentation/manuals/en/04-webui.md")
	de04 := readRepoFile(t, "documentation/manuals/de/04-webui.md")
	// Every app that ships with the desktop needs a row in the chapter 4 table.
	requireAll(t, "manuals/en/04-webui.md", en04, []string{
		"| **FindIt** |",
		"| **Kaggle** |",
		"| **Synth Studio** |",
		"| **AuraGolf** |",
		"| **Retro Games** |",
		"| **CYD Creator** |",
		"| **Docker Manager** |",
		"| **Please Do Not Feed the Universe** |",
		"| **Wikipedia** |",
		// The newer features of rows that already existed.
		"pet.always_on_top",
		"Punk 3D",
	})
	requireAll(t, "manuals/de/04-webui.md", de04, []string{
		"| **FindIt** |",
		"| **Kaggle** |",
		"| **Synth Studio** |",
		"| **AuraGolf** |",
		"| **Retro Games** |",
		"| **CYD Creator** |",
		"| **Docker Manager** |",
		"| **Please Do Not Feed the Universe** |",
		"| **Wikipedia** |",
		"pet.always_on_top",
		"Punk 3D",
	})
}

func TestNewDesktopToolsAreInTheToolCatalog(t *testing.T) {
	t.Parallel()
	tools := []string{
		"storymaker", "findit", "code_graph", "remember_learning",
		"kaggle_datasets", "kaggle_models", "kaggle_notebooks", "kaggle_competitions",
		"kaggle_benchmarks", "kaggle_forums", "kaggle_runtime",
		"homepage", "homepage_project", "homepage_file", "homepage_quality",
		"homepage_deploy", "homepage_git",
	}
	for _, lang := range []string{"en", "de"} {
		name := "documentation/manuals/" + lang + "/22-internal-tools.md"
		doc := readRepoFile(t, name)
		for _, tool := range tools {
			if !strings.Contains(doc, "### `"+tool+"`") {
				t.Errorf("%s is missing the tool section ### `%s`", name, tool)
			}
		}
	}
}

func TestNewDesktopAPIsAreInTheAPIReference(t *testing.T) {
	t.Parallel()
	for _, lang := range []string{"en", "de"} {
		name := "documentation/manuals/" + lang + "/21-api-reference.md"
		doc := readRepoFile(t, name)
		requireAll(t, name, doc, []string{
			"/api/desktop/storymaker",
			"/api/ecalc/catalog",
			"/api/kaggle",
			"/api/desktop/findit",
			"/api/homepage/designs",
		})
	}
}

func TestNewDesktopConfigBlocksAreDocumented(t *testing.T) {
	t.Parallel()
	en07 := readRepoFile(t, "documentation/manuals/en/07-configuration.md")
	de07 := readRepoFile(t, "documentation/manuals/de/07-configuration.md")
	requireAll(t, "manuals/en/07-configuration.md", en07, []string{
		"### Storymaker",
		"### FindIt",
		"### Kaggle",
		"max_daily_jobs",
		"agent_access",
		"read_only",
		"| `storymaker` |",
		"| `findit` |",
		"| `kaggle` |",
	})
	requireAll(t, "manuals/de/07-configuration.md", de07, []string{
		"### Storymaker",
		"### FindIt",
		"### Kaggle",
		"max_daily_jobs",
		"agent_access",
		"read_only",
		"| `storymaker` |",
		"| `findit` |",
		"| `kaggle` |",
	})

	reference := readRepoFile(t, "documentation/configuration.md")
	requireAll(t, "documentation/configuration.md", reference, []string{
		"## `storymaker`",
		"## `findit`",
		"## `kaggle`",
	})
}

func TestReadmeMentionsTheNovemberFeatures(t *testing.T) {
	t.Parallel()
	readme := readRepoFile(t, "README.md")
	requireAll(t, "README.md", readme, []string{
		"*November 2026*",
		"**Storymaker.**",
		"**Aura Drift.**",
		"**Please Do Not Feed the Universe.**",
		"**FindIt, ECalc and Kaggle.**",
		"(documentation/manuals/en/34-storymaker.md)",
		"(documentation/manuals/en/35-goldfish-adventure.md)",
		"(documentation/manuals/en/36-synth-studio.md)",
		"(documentation/storymaker.md)",
		"(documentation/aura-drift.md)",
		"(documentation/findit.md)",
		"(documentation/kaggle.md)",
		"(documentation/goldfish-adventure.md)",
	})
}

// The relative links of a document must resolve inside the repository. A new chapter or
// guide that nobody links to is a chapter nobody finds.
func TestDocumentationLinksResolve(t *testing.T) {
	t.Parallel()
	files := []string{
		"README.md",
		"documentation/manuals/README.md",
		"documentation/manuals/en/README.md",
		"documentation/manuals/de/README.md",
		"documentation/manuals/en/04-webui.md",
		"documentation/manuals/de/04-webui.md",
		"documentation/manuals/en/07-configuration.md",
		"documentation/manuals/de/07-configuration.md",
		"documentation/manuals/en/21-api-reference.md",
		"documentation/manuals/de/21-api-reference.md",
		"documentation/manuals/en/22-internal-tools.md",
		"documentation/manuals/de/22-internal-tools.md",
		"documentation/manuals/en/34-storymaker.md",
		"documentation/manuals/de/34-storymaker.md",
		"documentation/manuals/en/35-goldfish-adventure.md",
		"documentation/manuals/de/35-goldfish-adventure.md",
		"documentation/manuals/en/36-synth-studio.md",
		"documentation/manuals/de/36-synth-studio.md",
		"documentation/goldfish-adventure.md",
	}
	link := regexp.MustCompile(`\]\(([^)\s]+)\)`)
	for _, file := range files {
		doc := readRepoFile(t, file)
		base := filepath.Dir(repoPath(filepath.FromSlash(file)))
		for _, match := range link.FindAllStringSubmatch(doc, -1) {
			target := match[1]
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") ||
				strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
				continue
			}
			path := target
			if index := strings.IndexAny(path, "#?"); index >= 0 {
				path = path[:index]
			}
			if path == "" {
				continue
			}
			resolved := filepath.Clean(filepath.Join(base, filepath.FromSlash(path)))
			if _, err := os.Stat(resolved); err != nil {
				t.Errorf("%s links to %s, which does not exist", file, target)
			}
		}
	}
}
