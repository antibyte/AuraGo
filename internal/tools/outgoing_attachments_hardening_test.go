package tools

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/config"
)

// c103Env is a workspace and a documents folder next to each other, plus a config that points at them.
type c103Env struct {
	root      string
	workspace string
	docs      string
	cfg       *config.Config
}

func c103NewEnv(t *testing.T) c103Env {
	t.Helper()
	root := t.TempDir()
	env := c103Env{
		root:      root,
		workspace: filepath.Join(root, "workspace"),
		docs:      filepath.Join(root, "data", "documents"),
		cfg:       &config.Config{},
	}
	for _, dir := range []string{env.workspace, env.docs} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env.cfg.Directories.WorkspaceDir = env.workspace
	env.cfg.Tools.DocumentCreator.OutputDir = env.docs
	return env
}

func c103Write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// c103LinkFile makes link a symlink to the file target. Windows needs a privilege for that,
// so the test is skipped where the OS refuses.
func c103LinkFile(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create a symlink (Windows needs a privilege for it): %v", err)
	}
}

// c103LinkDir makes link a directory that leads to the directory target: a symlink, or on
// Windows a junction, which needs no privilege. The test is skipped where neither works.
func c103LinkDir(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
			t.Skipf("cannot create a junction: %v: %s", err, out)
		}
		t.Cleanup(func() { _ = os.Remove(link) })
		return
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
}

// c103Refused asserts that OpenOutgoingAttachment and ResolveOutgoingAttachmentPath both refuse
// path, and that OpenOutgoingAttachment leaves no file behind.
func c103Refused(t *testing.T, path string, cfg *config.Config) {
	t.Helper()
	if got, err := ResolveOutgoingAttachmentPath(path, cfg); err == nil {
		t.Errorf("Resolve(%q) = %q, must be rejected", path, got)
	}
	file, got, err := OpenOutgoingAttachment(path, cfg)
	if err == nil {
		file.Close()
		t.Errorf("Open(%q) opened %q, must be rejected", path, got)
		return
	}
	if file != nil || got != "" {
		t.Errorf("Open(%q) returned file %v and path %q together with an error", path, file, got)
	}
}

func TestOpenOutgoingAttachmentOpensWorkspaceAndDocumentsFiles(t *testing.T) {
	env := c103NewEnv(t)
	inWorkspace := filepath.Join(env.workspace, "bericht.pdf")
	inSubfolder := filepath.Join(env.workspace, "reports", "q3.pdf")
	inDocs := filepath.Join(env.docs, "news.pdf")
	c103Write(t, inWorkspace, "workspace content")
	c103Write(t, inSubfolder, "subfolder content")
	c103Write(t, inDocs, "documents content")

	for _, tc := range []struct {
		name    string
		request string
		want    string
		content string
	}{
		{"absolute workspace path", inWorkspace, inWorkspace, "workspace content"},
		{"relative workspace path", "bericht.pdf", inWorkspace, "workspace content"},
		{"relative subfolder path", filepath.Join("reports", "q3.pdf"), inSubfolder, "subfolder content"},
		{"absolute documents path", inDocs, inDocs, "documents content"},
		{"relative documents path", "news.pdf", inDocs, "documents content"},
		{"surrounding whitespace", "  bericht.pdf \n", inWorkspace, "workspace content"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, resolved, err := OpenOutgoingAttachment(tc.request, env.cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			want, err := filepath.EvalSymlinks(tc.want)
			if err != nil {
				t.Fatal(err)
			}
			if !filepath.IsAbs(resolved) || resolved != want {
				t.Errorf("resolved = %q, want %q", resolved, want)
			}
			plain, err := ResolveOutgoingAttachmentPath(tc.request, env.cfg)
			if err != nil || plain != resolved {
				t.Errorf("Resolve = %q, %v; Open resolved %q", plain, err, resolved)
			}
			data, err := io.ReadAll(file)
			if err != nil || string(data) != tc.content {
				t.Errorf("read %q, %v; want %q", data, err, tc.content)
			}
			if _, err := file.Write([]byte("x")); err == nil {
				t.Error("the attachment must be opened read-only")
			}
		})
	}
}

func TestOpenOutgoingAttachmentRefusesFilesOutside(t *testing.T) {
	env := c103NewEnv(t)
	secret := filepath.Join(env.root, "config.yaml")
	other := filepath.Join(env.root, "other", "report.pdf")
	c103Write(t, secret, "api_key: hunter2")
	c103Write(t, other, "elsewhere")
	c103Write(t, filepath.Join(env.workspace, "bericht.pdf"), "x")

	for _, bad := range []string{
		secret,
		other,
		"../config.yaml",
		filepath.Join("..", "other", "report.pdf"),
		filepath.Join(env.workspace, "..", "config.yaml"),
		filepath.Join(env.workspace, "missing.pdf"),
		"missing.pdf",
		env.workspace,
		env.docs,
		"",
		"   ",
	} {
		c103Refused(t, bad, env.cfg)
	}
	if file, _, err := OpenOutgoingAttachment(filepath.Join(env.workspace, "bericht.pdf"), nil); err == nil {
		file.Close()
		t.Error("a nil config must be rejected")
	}
}

func TestOutgoingAttachmentRefusesDirectoriesInsideTheRoots(t *testing.T) {
	env := c103NewEnv(t)
	for _, dir := range []string{filepath.Join(env.workspace, "reports"), filepath.Join(env.docs, "archive")} {
		c103Write(t, filepath.Join(dir, "inner.pdf"), "x")
		c103Refused(t, dir, env.cfg)
		c103Refused(t, filepath.Base(dir), env.cfg)
	}
}

func TestOutgoingAttachmentRefusesSiblingsThatShareARootNamePrefix(t *testing.T) {
	env := c103NewEnv(t)
	evilWorkspace := env.workspace + "-evil"
	evilDocs := env.docs + "-evil"
	c103Write(t, filepath.Join(evilWorkspace, "x.pdf"), "x")
	c103Write(t, filepath.Join(evilDocs, "x.pdf"), "x")
	c103Write(t, filepath.Join(env.workspace, "inside.pdf"), "x")

	for _, bad := range []string{
		filepath.Join(evilWorkspace, "x.pdf"),
		filepath.Join(evilDocs, "x.pdf"),
		filepath.Join("..", "workspace-evil", "x.pdf"),
		filepath.Join("..", "documents-evil", "x.pdf"),
	} {
		c103Refused(t, bad, env.cfg)
	}

	// A file name that merely starts with two dots stays inside the root.
	c103Write(t, filepath.Join(env.workspace, "..notes.pdf"), "dots")
	file, _, err := OpenOutgoingAttachment("..notes.pdf", env.cfg)
	if err != nil {
		t.Fatalf("a file named ..notes.pdf inside the workspace must qualify: %v", err)
	}
	file.Close()
}

func TestOutgoingAttachmentRefusesNULAndInvalidUTF8(t *testing.T) {
	env := c103NewEnv(t)
	c103Write(t, filepath.Join(env.workspace, "bericht.pdf"), "x")
	c103Write(t, filepath.Join(env.workspace, "bericht"), "x")

	for name, bad := range map[string]string{
		"NUL only":                "\x00",
		"NUL in a relative name":  "bericht.pdf\x00.txt",
		"NUL after a real name":   "bericht\x00",
		"NUL in an absolute path": filepath.Join(env.workspace, "bericht.pdf") + "\x00",
		"NUL before the name":     "\x00" + filepath.Join(env.workspace, "bericht.pdf"),
		"invalid UTF-8":           "bericht\xff.pdf",
		"truncated UTF-8":         "bericht\xc3",
	} {
		t.Run(name, func(t *testing.T) {
			c103Refused(t, bad, env.cfg)
			_, err := ResolveOutgoingAttachmentPath(bad, env.cfg)
			if err == nil {
				return
			}
			if strings.ContainsRune(err.Error(), 0) || !utf8.ValidString(err.Error()) {
				t.Errorf("the error message must be printable text, got %q", err.Error())
			}
		})
	}
}

func TestOutgoingAttachmentErrorsEchoABoundedPath(t *testing.T) {
	env := c103NewEnv(t)
	for name, long := range map[string]string{
		"ascii":      strings.Repeat("a", 10*1024),
		"multi-byte": strings.Repeat("ä", 5*1024),
		"absolute":   filepath.Join(env.workspace, strings.Repeat("d", 10*1024), "x.pdf"),
	} {
		t.Run(name, func(t *testing.T) {
			_, resolveErr := ResolveOutgoingAttachmentPath(long, env.cfg)
			file, _, openErr := OpenOutgoingAttachment(long, env.cfg)
			if file != nil {
				file.Close()
			}
			echoed := fmt.Sprintf("%q", truncateStr(long, maxEchoedAttachmentPathRunes))
			for _, err := range []error{resolveErr, openErr} {
				if err == nil {
					t.Fatal("a path that does not exist must be rejected")
				}
				if !strings.Contains(err.Error(), echoed) {
					t.Errorf("the error does not echo the cut path %s: %.300q", echoed, err.Error())
				}
				if got := utf8.RuneCountInString(err.Error()); got > maxEchoedAttachmentPathRunes+200 {
					t.Errorf("the error has %d runes, the path echo is not bounded", got)
				}
			}
		})
	}
}

func TestOutgoingAttachmentDefaultsToTheDocumentsFolderOfTheWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	c103Write(t, filepath.Join(root, "data", "documents", "news.pdf"), "news")
	c103Write(t, filepath.Join(root, "data", "vault.bin"), "vault")
	c103Write(t, filepath.Join(root, "config.yaml"), "api_key: hunter2")
	t.Chdir(root)

	// An unset output folder means data/documents below the working directory, and a relative
	// path is tried against the working directory first.
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspace
	for _, ok := range []string{filepath.Join("data", "documents", "news.pdf"), "news.pdf"} {
		file, _, err := OpenOutgoingAttachment(ok, cfg)
		if err != nil {
			t.Errorf("%s: %v", ok, err)
			continue
		}
		data, _ := io.ReadAll(file)
		file.Close()
		if string(data) != "news" {
			t.Errorf("%s: read %q", ok, data)
		}
	}
	for _, bad := range []string{"config.yaml", filepath.Join("data", "vault.bin")} {
		c103Refused(t, bad, cfg)
	}
}

func TestOpenOutgoingAttachmentRefusesASymlinkLeadingOutside(t *testing.T) {
	env := c103NewEnv(t)
	secret := filepath.Join(env.root, "config.yaml")
	c103Write(t, secret, "api_key: hunter2")
	link := filepath.Join(env.workspace, "innocent.pdf")
	c103LinkFile(t, secret, link)

	c103Refused(t, link, env.cfg)
	c103Refused(t, "innocent.pdf", env.cfg)
}

func TestOpenOutgoingAttachmentRefusesALinkedFolderLeadingOutside(t *testing.T) {
	env := c103NewEnv(t)
	outside := filepath.Join(env.root, "outside")
	c103Write(t, filepath.Join(outside, "secret.pdf"), "secret")
	linked := filepath.Join(env.workspace, "linked")
	c103LinkDir(t, outside, linked)

	c103Refused(t, filepath.Join(linked, "secret.pdf"), env.cfg)
	c103Refused(t, filepath.Join("linked", "secret.pdf"), env.cfg)
}

func TestOpenOutgoingAttachmentFollowsALinkThatStaysInsideTheRoot(t *testing.T) {
	env := c103NewEnv(t)
	c103Write(t, filepath.Join(env.workspace, "real.pdf"), "real")
	link := filepath.Join(env.workspace, "alias.pdf")
	c103LinkFile(t, filepath.Join(env.workspace, "real.pdf"), link)

	file, resolved, err := OpenOutgoingAttachment("alias.pdf", env.cfg)
	if err != nil {
		t.Fatalf("a link between two files of the workspace is harmless: %v", err)
	}
	defer file.Close()
	if filepath.Base(resolved) != "real.pdf" {
		t.Errorf("resolved = %q, want the link target real.pdf", resolved)
	}
	if data, _ := io.ReadAll(file); string(data) != "real" {
		t.Errorf("read %q", data)
	}
}

// The check and the open are two steps. A path component swapped for a link between them
// must not send a file from outside the roots, so the open step has to refuse it on its own.
func TestOpenWithinOutgoingRootRefusesAPathSwappedAfterTheCheck(t *testing.T) {
	requireSwapRedirects := func(t *testing.T, resolved string) {
		t.Helper()
		if data, err := os.ReadFile(resolved); err != nil || string(data) != "SECRET" {
			t.Fatalf("test setup: after the swap a plain read of %q gives %q, %v; the test would prove nothing", resolved, data, err)
		}
	}
	requireRefused := func(t *testing.T, cfg *config.Config, root, resolved string) {
		t.Helper()
		file, err := openWithinOutgoingRoot(cfg, root, resolved)
		if err == nil {
			data, _ := io.ReadAll(file)
			file.Close()
			t.Fatalf("the swapped path was opened and gave %q", data)
		}
		if file != nil {
			t.Error("a file was returned together with an error")
		}
		if strings.Contains(err.Error(), filepath.Base(resolved)) {
			t.Errorf("the error repeats the file name: %v", err)
		}
	}

	t.Run("folder swapped for a link", func(t *testing.T) {
		env := c103NewEnv(t)
		outside := filepath.Join(env.root, "outside")
		c103Write(t, filepath.Join(outside, "report.pdf"), "SECRET")
		c103Write(t, filepath.Join(env.workspace, "sub", "report.pdf"), "public")

		resolved, root, err := resolveOutgoingAttachment(filepath.Join(env.workspace, "sub", "report.pdf"), env.cfg)
		if err != nil {
			t.Fatal(err)
		}
		file, err := openWithinOutgoingRoot(env.cfg, root, resolved)
		if err != nil {
			t.Fatalf("before the swap the file must open: %v", err)
		}
		if data, _ := io.ReadAll(file); string(data) != "public" {
			t.Errorf("before the swap read %q", data)
		}
		file.Close()

		if err := os.RemoveAll(filepath.Join(env.workspace, "sub")); err != nil {
			t.Fatal(err)
		}
		c103LinkDir(t, outside, filepath.Join(env.workspace, "sub"))
		requireSwapRedirects(t, resolved)
		requireRefused(t, env.cfg, root, resolved)
	})

	t.Run("file swapped for a symlink", func(t *testing.T) {
		env := c103NewEnv(t)
		outside := filepath.Join(env.root, "outside", "report.pdf")
		c103Write(t, outside, "SECRET")
		target := filepath.Join(env.workspace, "report.pdf")
		c103Write(t, target, "public")

		resolved, root, err := resolveOutgoingAttachment(target, env.cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(target); err != nil {
			t.Fatal(err)
		}
		c103LinkFile(t, outside, target)
		requireSwapRedirects(t, resolved)
		requireRefused(t, env.cfg, root, resolved)
	})
}

func TestOpenWithinOutgoingRootRefusesPathsOutsideItsRoot(t *testing.T) {
	env := c103NewEnv(t)
	secret := filepath.Join(env.root, "config.yaml")
	sibling := filepath.Join(env.workspace+"-evil", "x.pdf")
	c103Write(t, secret, "x")
	c103Write(t, sibling, "x")
	c103Write(t, filepath.Join(env.workspace, "sub"), "a file, not a folder")

	root := canonicalExistingRoot(env.workspace)
	for _, path := range []string{secret, sibling, filepath.Join(root, "sub", "x")} {
		if file, err := openWithinOutgoingRoot(env.cfg, root, path); err == nil {
			file.Close()
			t.Errorf("%q must not open inside %q", path, root)
		}
	}
}

func TestOpenWithinOutgoingRootRequiresARegularFile(t *testing.T) {
	env := c103NewEnv(t)
	c103Write(t, filepath.Join(env.workspace, "sub", "inner.pdf"), "x")
	root := canonicalExistingRoot(env.workspace)

	for _, dir := range []string{root, filepath.Join(root, "sub")} {
		if file, err := openWithinOutgoingRoot(env.cfg, root, dir); err == nil {
			file.Close()
			t.Errorf("the directory %q must not be handed out as an attachment", dir)
		}
	}
}
