package tools

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/config"
)

// c103Install is an AuraGo install: a config file, a data directory with the vault and a
// database, a workspace and a documents folder, plus the config that describes them.
type c103Install struct {
	root      string
	config    string
	data      string
	workspace string
	docs      string
	cfg       *config.Config
}

func c103BuildInstall(t *testing.T, root, configDir, dataDir, workspace, docs string) c103Install {
	t.Helper()
	in := c103Install{
		root:      root,
		config:    filepath.Join(configDir, "config.yaml"),
		data:      dataDir,
		workspace: workspace,
		docs:      docs,
		cfg:       &config.Config{},
	}
	c103Write(t, in.config, "SECRET-CONFIG")
	c103Write(t, filepath.Join(dataDir, "vault.bin"), "SECRET-VAULT")
	c103Write(t, filepath.Join(dataDir, "short_term.db"), "SECRET-DB")
	c103Write(t, filepath.Join(docs, "news.pdf"), "public-news")
	c103Write(t, filepath.Join(workspace, "bericht.pdf"), "public-bericht")
	in.cfg.ConfigPath = in.config
	in.cfg.Directories.DataDir = dataDir
	in.cfg.Directories.WorkspaceDir = workspace
	in.cfg.Tools.DocumentCreator.OutputDir = docs
	in.cfg.SQLite.ShortTermPath = filepath.Join(dataDir, "short_term.db")
	return in
}

func c103CanonicalTempDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	return root
}

// c103DefaultInstall is the layout of a plain install: everything below one folder.
func c103DefaultInstall(t *testing.T) c103Install {
	t.Helper()
	root := c103CanonicalTempDir(t)
	return c103BuildInstall(t, root, root,
		filepath.Join(root, "data"),
		filepath.Join(root, "agent_workspace", "workdir"),
		filepath.Join(root, "data", "documents"))
}

// c103DockerInstall is the layout in the container: the config lives in /app/data, which is
// also the parent of data_dir, the documents folder and the workspace.
func c103DockerInstall(t *testing.T, dataDirIsConfigDir bool) c103Install {
	t.Helper()
	root := c103CanonicalTempDir(t)
	configDir := filepath.Join(root, "app", "data")
	dataDir := filepath.Join(configDir, "data")
	if dataDirIsConfigDir {
		dataDir = configDir
	}
	return c103BuildInstall(t, root, configDir, dataDir,
		filepath.Join(configDir, "agent_workspace", "workdir"),
		filepath.Join(configDir, "data", "documents"))
}

// c103Opens asserts that OpenOutgoingAttachment accepts path and reads want.
func c103Opens(t *testing.T, path string, cfg *config.Config, want string) {
	t.Helper()
	file, _, err := OpenOutgoingAttachment(path, cfg)
	if err != nil {
		t.Errorf("%q must qualify: %v", path, err)
		return
	}
	defer file.Close()
	if data, _ := io.ReadAll(file); string(data) != want {
		t.Errorf("%q read %q, want %q", path, data, want)
	}
}

// c103RefusedWith asserts the refusal of c103Refused and that both error messages contain every part.
func c103RefusedWith(t *testing.T, path string, cfg *config.Config, parts ...string) {
	t.Helper()
	c103Refused(t, path, cfg)
	_, resolveErr := ResolveOutgoingAttachmentPath(path, cfg)
	_, _, openErr := OpenOutgoingAttachment(path, cfg)
	for _, err := range []error{resolveErr, openErr} {
		if err == nil {
			continue
		}
		for _, part := range parts {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("the error for %q must contain %q: %v", path, part, err)
			}
		}
	}
}

func TestOutgoingAttachmentRefusesProtectedFilesInsideTheRoots(t *testing.T) {
	in := c103DefaultInstall(t)
	in.cfg.SQLite.PlannerPath = filepath.Join(in.workspace, "planner.sqlite")
	protected := []string{
		".env", "prod.env", "aurago_master.key", "vault.bin", "VAULT.BIN", "vault.bin.lock",
		"planner.sqlite", "planner.sqlite-wal", "planner.sqlite-shm",
	}
	for _, name := range protected {
		c103Write(t, filepath.Join(in.workspace, name), "SECRET")
	}

	for _, name := range protected {
		t.Run(name, func(t *testing.T) {
			c103RefusedWith(t, name, in.cfg, "protected")
			c103RefusedWith(t, filepath.Join(in.workspace, name), in.cfg, "protected")
		})
	}
	c103Opens(t, "bericht.pdf", in.cfg, "public-bericht")
}

func TestOutgoingAttachmentRefusesAHardLinkToProtectedState(t *testing.T) {
	in := c103DefaultInstall(t)
	c103Write(t, filepath.Join(in.root, ".env"), "SECRET-ENV")
	targets := map[string]string{
		"hl-config.pdf": in.config,
		"hl-vault.pdf":  filepath.Join(in.data, "vault.bin"),
		"hl-db.pdf":     filepath.Join(in.data, "short_term.db"),
		"hl-env.pdf":    filepath.Join(in.root, ".env"),
	}
	for name, target := range targets {
		if err := os.Link(target, filepath.Join(in.workspace, name)); err != nil {
			t.Skipf("cannot create a hard link: %v", err)
		}
	}

	for name := range targets {
		t.Run(name, func(t *testing.T) {
			c103RefusedWith(t, name, in.cfg, "protected")
			c103RefusedWith(t, filepath.Join(in.workspace, name), in.cfg, "protected")
		})
	}

	// A hard link between two harmless files stays harmless.
	if err := os.Link(filepath.Join(in.workspace, "bericht.pdf"), filepath.Join(in.workspace, "bericht-copy.pdf")); err != nil {
		t.Fatal(err)
	}
	c103Opens(t, "bericht-copy.pdf", in.cfg, "public-bericht")
}

// The resolver already refuses a hard link. The open handle is checked again because the
// path can be swapped for one after the resolver looked at it.
func TestCheckOpenedOutgoingFileRefusesAHardLinkToProtectedState(t *testing.T) {
	in := c103DefaultInstall(t)
	link := filepath.Join(in.workspace, "swapped.pdf")
	if err := os.Link(in.config, link); err != nil {
		t.Skipf("cannot create a hard link: %v", err)
	}
	file, err := os.Open(link)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if err := checkOpenedOutgoingFile(file, link, outgoingProtectedFiles(in.cfg)); err == nil {
		t.Error("a handle on a hard link to config.yaml must be refused")
	}
	if err := checkOpenedOutgoingFile(file, link, nil); err != nil {
		t.Errorf("without protected files the same handle is an ordinary file: %v", err)
	}
	file2, err := openWithinOutgoingRoot(in.cfg, canonicalExistingRoot(in.workspace), link)
	if err == nil {
		file2.Close()
		t.Error("the open step must refuse a hard link to config.yaml on its own")
	}
}

func TestCheckOpenedOutgoingFileRequiresTheFileThePathNames(t *testing.T) {
	env := c103NewEnv(t)
	first := filepath.Join(env.workspace, "first.pdf")
	second := filepath.Join(env.workspace, "second.pdf")
	c103Write(t, first, "first")
	c103Write(t, second, "second")
	file, err := os.Open(first)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if err := checkOpenedOutgoingFile(file, first, nil); err != nil {
		t.Errorf("the handle belongs to its own path: %v", err)
	}
	if err := checkOpenedOutgoingFile(file, second, nil); err == nil {
		t.Error("a handle on first.pdf must not pass for the path of second.pdf")
	}
	if err := checkOpenedOutgoingFile(file, filepath.Join(env.workspace, "gone.pdf"), nil); err == nil {
		t.Error("a path that no longer exists must be refused")
	}
	dir, err := os.Open(env.workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if err := checkOpenedOutgoingFile(dir, env.workspace, nil); err == nil {
		t.Error("a directory handle must be refused")
	}
}

func TestOutgoingAttachmentDropsRootsThatHoldConfigurationOrData(t *testing.T) {
	setDocs := func(in c103Install, folder string) *config.Config {
		cfg := *in.cfg
		cfg.Tools.DocumentCreator.OutputDir = folder
		return &cfg
	}
	for _, tc := range []struct {
		name   string
		folder func(in c103Install) string
		mutate func(cfg *config.Config)
	}{
		{"install folder", func(in c103Install) string { return in.root }, nil},
		{"data directory", func(in c103Install) string { return in.data }, nil},
		{"install folder named by a dot", func(in c103Install) string { return "." }, nil},
		{"config directory without a data directory", func(in c103Install) string { return in.root },
			func(cfg *config.Config) { cfg.Directories.DataDir = "" }},
		{"data directory without a config file", func(in c103Install) string { return in.data },
			func(cfg *config.Config) { cfg.ConfigPath = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := c103DefaultInstall(t)
			t.Chdir(in.root)
			cfg := setDocs(in, tc.folder(in))
			if tc.mutate != nil {
				tc.mutate(cfg)
			}

			roots := outgoingAttachmentRoots(cfg)
			if len(roots.dropped) != 1 || roots.dropped[0] != outgoingDocumentsSetting {
				t.Fatalf("dropped = %q, want only the documents folder", roots.dropped)
			}
			if len(roots.canonical) != 1 || roots.canonical[0] != canonicalExistingRoot(in.workspace) {
				t.Fatalf("roots = %q, want only the workspace", roots.canonical)
			}
			c103RefusedWith(t, "short_term.db", cfg, outgoingDocumentsSetting, "narrow")
			c103RefusedWith(t, in.config, cfg, outgoingDocumentsSetting, "narrow")
			c103RefusedWith(t, filepath.Join(in.data, "vault.bin"), cfg, outgoingDocumentsSetting)
			c103RefusedWith(t, filepath.Join(in.data, "short_term.db"), cfg)
			c103RefusedWith(t, "config.yaml", cfg)
			c103Opens(t, "bericht.pdf", cfg, "public-bericht")
		})
	}
}

func TestOutgoingAttachmentDropsAWorkspaceThatHoldsConfigurationOrData(t *testing.T) {
	in := c103DefaultInstall(t)
	in.cfg.Directories.WorkspaceDir = in.root

	roots := outgoingAttachmentRoots(in.cfg)
	if len(roots.dropped) != 1 || roots.dropped[0] != outgoingWorkspaceSetting {
		t.Fatalf("dropped = %q, want only the workspace", roots.dropped)
	}
	c103RefusedWith(t, in.config, in.cfg, outgoingWorkspaceSetting, "narrow")
	c103RefusedWith(t, filepath.Join(in.data, "vault.bin"), in.cfg, outgoingWorkspaceSetting)
	c103Opens(t, filepath.Join(in.docs, "news.pdf"), in.cfg, "public-news")
	c103Opens(t, "news.pdf", in.cfg, "public-news")

	// When both folders are wide nothing is left, and the answer still says why.
	in.cfg.Tools.DocumentCreator.OutputDir = in.root
	for _, path := range []string{in.config, "bericht.pdf"} {
		c103RefusedWith(t, path, in.cfg, outgoingWorkspaceSetting, outgoingDocumentsSetting)
	}
}

func TestOutgoingAttachmentDoesNotGuardUnsetConfigAndDataPaths(t *testing.T) {
	// filepath.Dir("") is ".", the working directory. With no config file set, a documents
	// folder that happens to be the working directory must not be dropped for that reason.
	env := c103NewEnv(t)
	c103Write(t, filepath.Join(env.docs, "news.pdf"), "news")
	t.Chdir(env.docs)

	roots := outgoingAttachmentRoots(env.cfg)
	if len(roots.dropped) != 0 || len(roots.canonical) != 2 {
		t.Fatalf("roots = %q, dropped = %q; neither folder holds configuration or data", roots.canonical, roots.dropped)
	}
	c103Opens(t, "news.pdf", env.cfg, "news")
}

func TestOutgoingAttachmentKeepsTheDefaultAndDockerLayouts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(t *testing.T) c103Install
	}{
		{"default install", c103DefaultInstall},
		{"docker, data below the config folder", func(t *testing.T) c103Install { return c103DockerInstall(t, false) }},
		{"docker, data_dir is the config folder", func(t *testing.T) c103Install { return c103DockerInstall(t, true) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.build(t)
			roots := outgoingAttachmentRoots(in.cfg)
			if len(roots.dropped) != 0 {
				t.Fatalf("dropped = %q; neither folder holds configuration or data", roots.dropped)
			}
			if len(roots.canonical) != 2 {
				t.Fatalf("roots = %q, want the workspace and the documents folder", roots.canonical)
			}
			// news.pdf lives below data_dir. The data-directory prefix rule must not block it.
			c103Opens(t, "news.pdf", in.cfg, "public-news")
			c103Opens(t, filepath.Join(in.docs, "news.pdf"), in.cfg, "public-news")
			c103Opens(t, "bericht.pdf", in.cfg, "public-bericht")
			c103Opens(t, filepath.Join(in.workspace, "bericht.pdf"), in.cfg, "public-bericht")

			c103Refused(t, in.config, in.cfg)
			c103Refused(t, filepath.Join(in.data, "vault.bin"), in.cfg)
			c103Refused(t, filepath.Join(in.data, "short_term.db"), in.cfg)
			c103Refused(t, filepath.Join("..", "..", "config.yaml"), in.cfg)
		})
	}
}

func TestOutgoingPathLexicallyInside(t *testing.T) {
	env := c103NewEnv(t)
	workspace, docs := canonicalExistingRoot(env.workspace), canonicalExistingRoot(env.docs)
	roots := []string{workspace, docs}
	parent := filepath.Dir(workspace)
	sep := string(filepath.Separator)

	// The UNC and device spellings are written the way Windows reads them. On other systems
	// they are odd relative names, which the predicate refuses as well.
	for _, tc := range []struct {
		name string
		path string
		want bool
	}{
		{"file in the workspace", filepath.Join(workspace, "bericht.pdf"), true},
		{"file in a subfolder", filepath.Join(workspace, "a", "b", "c.pdf"), true},
		{"file in the documents folder", filepath.Join(docs, "news.pdf"), true},
		{"name that starts with two dots", filepath.Join(workspace, "..notes.pdf"), true},
		{"the root itself", workspace, true},
		{"unclean path that stays inside", workspace + sep + "a" + sep + ".." + sep + "x.pdf", true},
		{"sibling that shares the name prefix", workspace + "-evil" + sep + "x.pdf", false},
		{"file in the parent folder", filepath.Join(parent, "config.yaml"), false},
		{"unclean path that leaves the root", workspace + sep + ".." + sep + "config.yaml", false},
		{"relative path", "bericht.pdf", false},
		{"empty path", "", false},
		{"UNC share", `\\host\share\x.pdf`, false},
		{"UNC share with forward slashes", `//host/share/x.pdf`, false},
		{"UNC loopback admin share", `\\127.0.0.1\c$\x.pdf`, false},
		{"extended-length UNC", `\\?\UNC\host\share\x.pdf`, false},
		{"extended-length spelling of a local path", `\\?\` + filepath.Join(workspace, "bericht.pdf"), false},
		{"device namespace pipe", `\\.\pipe\x`, false},
		{"device namespace spelling of a local path", `\\.\` + filepath.Join(workspace, "bericht.pdf"), false},
		{"reserved device name", "NUL", false},
		{"proc environ", "/proc/self/environ", false},
		{"other drive root", `C:\Windows\win.ini`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := outgoingPathLexicallyInside(tc.path, roots); got != tc.want {
				t.Errorf("outgoingPathLexicallyInside(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
	if outgoingPathLexicallyInside(filepath.Join(workspace, "bericht.pdf"), nil) {
		t.Error("with no roots nothing is inside")
	}
}

// The resolver asks the lexical question before it touches the filesystem, so a path that
// only reaches the workspace through a link outside it is not even looked at. That is the
// observable side of keeping UNC and device paths away from EvalSymlinks and os.Stat.
func TestOutgoingAttachmentDoesNotLookAtPathsThatStartOutsideTheRoots(t *testing.T) {
	env := c103NewEnv(t)
	c103Write(t, filepath.Join(env.workspace, "bericht.pdf"), "public-bericht")
	outside := filepath.Join(env.root, "elsewhere")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	reach := filepath.Join(outside, "reach")
	c103LinkDir(t, env.workspace, reach)

	c103Refused(t, filepath.Join(reach, "bericht.pdf"), env.cfg)
	c103Opens(t, filepath.Join(env.workspace, "bericht.pdf"), env.cfg, "public-bericht")
}

func TestOutgoingAttachmentRootsKeepBothFormsOfALinkedRoot(t *testing.T) {
	env := c103NewEnv(t)
	c103Write(t, filepath.Join(env.workspace, "bericht.pdf"), "public-bericht")
	alias := filepath.Join(env.root, "alias")
	c103LinkDir(t, env.workspace, alias)
	env.cfg.Directories.WorkspaceDir = alias

	roots := outgoingAttachmentRoots(env.cfg)
	absAlias, err := filepath.Abs(alias)
	if err != nil {
		t.Fatal(err)
	}
	var hasAlias, hasCanonical bool
	for _, form := range roots.lexical {
		hasAlias = hasAlias || strings.EqualFold(form, absAlias)
		hasCanonical = hasCanonical || strings.EqualFold(form, canonicalExistingRoot(alias))
	}
	if !hasAlias || !hasCanonical {
		t.Fatalf("lexical roots %q must hold the configured form %q and the canonical form %q", roots.lexical, absAlias, canonicalExistingRoot(alias))
	}

	if runtime.GOOS == "windows" {
		// filepath.EvalSymlinks fails on a path through a junction, so everything is refused.
		// That is safe, and documented on ResolveOutgoingAttachmentPath.
		c103Refused(t, filepath.Join(alias, "bericht.pdf"), env.cfg)
		c103Refused(t, "bericht.pdf", env.cfg)
		return
	}
	c103Opens(t, filepath.Join(alias, "bericht.pdf"), env.cfg, "public-bericht")
	c103Opens(t, filepath.Join(env.workspace, "bericht.pdf"), env.cfg, "public-bericht")
	c103Opens(t, "bericht.pdf", env.cfg, "public-bericht")
}

// The workspace swapped for a link between the check and the open: os.Root would only keep
// the path inside the link's target, so the open step has to look at the folder itself.
func TestOpenWithinOutgoingRootRefusesARootSwappedForALink(t *testing.T) {
	env := c103NewEnv(t)
	install := filepath.Join(env.root, "install")
	c103Write(t, filepath.Join(env.workspace, "report.pdf"), "public")
	c103Write(t, filepath.Join(install, "report.pdf"), "SECRET")

	resolved, root, err := resolveOutgoingAttachment(filepath.Join(env.workspace, "report.pdf"), env.cfg)
	if err != nil {
		t.Fatal(err)
	}
	file, err := openWithinOutgoingRoot(env.cfg, root, resolved)
	if err != nil {
		t.Fatalf("before the swap the file must open: %v", err)
	}
	file.Close()

	if err := os.Rename(env.workspace, env.workspace+"-old"); err != nil {
		t.Fatal(err)
	}
	c103LinkDir(t, install, env.workspace)
	if data, err := os.ReadFile(resolved); err != nil || string(data) != "SECRET" {
		t.Fatalf("test setup: after the swap a plain read of %q gives %q, %v; the test would prove nothing", resolved, data, err)
	}

	file, err = openWithinOutgoingRoot(env.cfg, root, resolved)
	if err == nil {
		data, _ := io.ReadAll(file)
		file.Close()
		t.Fatalf("the swapped folder was opened and gave %q", data)
	}
	if file != nil {
		t.Error("a file was returned together with an error")
	}
}

func TestRequirePlainOutgoingRootAcceptsAPlainDirectory(t *testing.T) {
	env := c103NewEnv(t)
	root := canonicalExistingRoot(env.workspace)
	dir, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if err := requirePlainOutgoingRoot(root, dir); err != nil {
		t.Errorf("a plain directory must pass: %v", err)
	}
	if err := requirePlainOutgoingRoot(filepath.Join(root, "missing"), dir); err == nil {
		t.Error("a root that no longer exists must be refused")
	}
	other, err := os.OpenRoot(env.docs)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := requirePlainOutgoingRoot(root, other); err == nil {
		t.Error("a handle on a different directory than the root path must be refused")
	}
}
