//go:build windows

package localwiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// Windows ignores trailing dots and spaces in path components and accepts 8.3
// short names, so "<protected>.\new" and "PROTEC~1\new" are inside the
// protected tree although they do not look like it. prepareDataDir refuses
// them before it creates anything.
func TestPrepareDataDirResolvesWindowsAliases(t *testing.T) {
	base := t.TempDir()
	protected := filepath.Join(base, "Protected Tree Long Name")
	if err := os.MkdirAll(protected, 0o755); err != nil {
		t.Fatal(err)
	}
	resolvedProtected, err := resolveExistingDir(protected)
	if err != nil {
		t.Fatal(err)
	}
	within := func(p, root string) bool {
		p, root = strings.ToLower(filepath.Clean(p)), strings.ToLower(filepath.Clean(root))
		return p == root || strings.HasPrefix(p, root+`\`)
	}
	// Component-wise, like tools.IsSensitiveHostDirectory without its
	// trailing-dot handling: the lexical forms below pass it.
	sensitive := func(p string) bool { return within(p, protected) || within(p, resolvedProtected) }

	cases := map[string]string{
		"trailing dot":           protected + `.\new\wiki`,
		"trailing space and dot": protected + ` .\new\wiki`,
		"existing alias":         protected + `.`,
	}
	if short := shortPathName(t, protected); !strings.EqualFold(short, protected) {
		cases["short name"] = short + `\new\wiki`
	} else {
		t.Log("8.3 short names are disabled on this volume; the short-name case is skipped")
	}
	for name, dir := range cases {
		if sensitive(dir) {
			t.Fatalf("%s: %s is already refused lexically; the case proves nothing", name, dir)
		}
		if err := prepareDataDir(dir, sensitive); ErrorCode(err) != CodeDataDirInvalid {
			t.Errorf("%s: prepareDataDir(%s) = %v", name, dir, err)
		}
	}
	if entries, _ := os.ReadDir(protected); len(entries) != 0 {
		t.Fatalf("prepareDataDir created %d entries in the protected tree, first %s", len(entries), entries[0].Name())
	}

	harmless := filepath.Join(base, "harmless", "wiki")
	if err := prepareDataDir(harmless, sensitive); err != nil {
		t.Fatalf("harmless directory: %v", err)
	}
}

// Read-only check against a system folder: new folders often get no 8.3 name
// (creation is disabled on many volumes), Program Files usually has one.
func TestResolveExistingDirExpandsShortNames(t *testing.T) {
	long := filepath.Join(os.Getenv("SystemDrive")+`\`, "Program Files")
	short := shortPathName(t, long)
	if strings.EqualFold(short, long) {
		t.Skipf("%s has no 8.3 short name here", long)
	}
	for _, alias := range []string{short, long + `.`, long + ` .`} {
		if got, err := resolveExistingDir(alias); err != nil || !strings.EqualFold(got, long) {
			t.Errorf("resolveExistingDir(%s) = %q, %v; want %s", alias, got, err, long)
		}
	}
}

// A path that cannot be inspected (here an invalid name, which is not "does
// not exist") is refused instead of walking past it.
func TestPrepareDataDirRefusesUninspectablePaths(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "bad<name", "wiki")
	if _, _, err := nearestExistingAncestor(dir); err == nil {
		t.Fatal("nearestExistingAncestor walked past an invalid name")
	}
	if err := prepareDataDir(dir, func(string) bool { return false }); ErrorCode(err) != CodeDataDirInvalid {
		t.Fatalf("prepareDataDir = %v", err)
	}
}

func TestStripFinalPathPrefix(t *testing.T) {
	for in, want := range map[string]string{
		`\\?\C:\Windows`:             `C:\Windows`,
		`\\?\UNC\nas\share\wiki`:     `\\nas\share\wiki`,
		`C:\data`:                    `C:\data`,
		`\\?\Volume{1234}\wikipedia`: `\\?\Volume{1234}\wikipedia`,
	} {
		if got := stripFinalPathPrefix(in); got != want {
			t.Errorf("stripFinalPathPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func shortPathName(t *testing.T, long string) string {
	t.Helper()
	in, err := windows.UTF16PtrFromString(long)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 1024)
	n, err := windows.GetShortPathName(in, &buf[0], uint32(len(buf)))
	if err != nil || int(n) >= len(buf) {
		return long
	}
	return windows.UTF16ToString(buf[:n])
}
