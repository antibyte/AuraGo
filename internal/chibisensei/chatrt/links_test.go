package chatrt

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"aurago/internal/llamacpp"
	"aurago/internal/onnxrt"
)

type tarItem struct {
	name, target string
	kind         byte // tar.TypeReg, TypeSymlink, TypeLink, TypeDir
}

func writeTestArchive(t *testing.T, cacheDir string, spec llamacpp.AssetSpec, items []tarItem) {
	t.Helper()
	archive := onnxrt.ArchivePath(cacheDir, spec)
	if err := os.MkdirAll(filepath.Dir(archive), 0o750); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, it := range items {
		hdr := &tar.Header{Name: it.name, Typeflag: it.kind, Mode: 0o644, Linkname: it.target}
		if it.kind == tar.TypeDir {
			hdr.Mode = 0o755
		}
		if it.kind == tar.TypeReg {
			hdr.Size = 1
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if it.kind == tar.TypeReg {
			if _, err := tw.Write([]byte("x")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func skipWithoutSymlinks(t *testing.T) {
	t.Helper()
	probe := filepath.Join(t.TempDir(), "probe")
	if err := os.Symlink("target", probe); err != nil {
		t.Skipf("symbolic links are not available here: %v", err)
	}
}

func testSpec() llamacpp.AssetSpec {
	return llamacpp.AssetSpec{ID: "rt-test", SHA256: "abc", Kind: onnxrt.AssetTarGzip}
}

func makeRuntimeFiles(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlanArchiveLinksPure(t *testing.T) {
	existing := map[string]bool{"rt/libx.so.1.2.3": true, "rt/dir": false}
	exists := func(rel string) bool { return existing[rel] }
	long := []linkEntry{}
	for i := 0; i < 12; i++ { // a chain of 12 links, longer than the limit
		long = append(long, linkEntry{Name: "rt/c" + string(rune('a'+i)), Target: "c" + string(rune('a'+i+1))})
	}
	long = append(long, linkEntry{Name: "rt/cm", Target: "libx.so.1.2.3"})
	entries := append([]linkEntry{
		{"rt/libx.so.1", "libx.so.1.2.3"},
		{"./rt/libx.so", "libx.so.1"},
		{"rt/abs", "/etc/passwd"},
		{"rt/up", "../../etc/passwd"},
		{"../escape", "rt/libx.so.1.2.3"},
		{"/abs-name", "rt/libx.so.1.2.3"},
		{"rt/missing", "nothing.so"},
		{"rt/todir", "dir"},
		{"rt/back", "..\\x"},
		{"rt/nul", "li\x00bx.so.1.2.3"},
		{"rt/dot/..", "libx.so.1.2.3"},
	}, long...)
	got := planArchiveLinks(entries, exists)
	want := []plannedLink{
		{"rt/libx.so", "rt/libx.so.1.2.3"},
		{"rt/libx.so.1", "rt/libx.so.1.2.3"},
	}
	// The tail of the long chain (cl, cm and anything within 8 steps) may resolve; the head must not.
	var core []plannedLink
	for _, l := range got {
		if !strings.HasPrefix(l.Name, "rt/c") {
			core = append(core, l)
		} else if l.Name == "rt/ca" || l.Name == "rt/cb" {
			t.Fatalf("a chain longer than the limit resolved: %v", l)
		}
	}
	if !reflect.DeepEqual(core, want) {
		t.Fatalf("plan %v, want %v", core, want)
	}
}

func TestEnsureArchiveLinksCreatesSonameLinks(t *testing.T) {
	skipWithoutSymlinks(t)
	cache, rtRoot := t.TempDir(), filepath.Join(t.TempDir(), "runtime")
	spec := testSpec()
	writeTestArchive(t, cache, spec, []tarItem{
		{"rt/", "", tar.TypeDir},
		{"rt/libx.so.1.2.3", "", tar.TypeReg},
		{"rt/libx.so.1", "libx.so.1.2.3", tar.TypeSymlink},
		{"rt/libx.so", "libx.so.1", tar.TypeSymlink},
	})
	makeRuntimeFiles(t, rtRoot, "rt/libx.so.1.2.3")
	root, err := ensureArchiveLinksFor(context.Background(), "linux", cache, spec, rtRoot)
	if err != nil || root == "" {
		t.Fatalf("got %q, %v", root, err)
	}
	if root != filepath.Join(cache, "links", spec.ID) {
		t.Fatalf("link root %q", root)
	}
	want, _ := filepath.EvalSymlinks(filepath.Join(rtRoot, "rt", "libx.so.1.2.3"))
	for _, name := range []string{"libx.so.1", "libx.so"} {
		got, err := filepath.EvalSymlinks(filepath.Join(root, "rt", name))
		if err != nil || got != want {
			t.Fatalf("%s resolves to %q (%v), want %q", name, got, err, want)
		}
	}
	marker := filepath.Join(root, linkMarkerName)
	first, err := os.Stat(marker)
	if err != nil {
		t.Fatal(err)
	}
	old := first.ModTime().Add(-time.Hour)
	if err := os.Chtimes(marker, old, old); err != nil {
		t.Fatal(err)
	}
	again, err := ensureArchiveLinksFor(context.Background(), "linux", cache, spec, rtRoot)
	if err != nil || again != root {
		t.Fatalf("second call %q, %v", again, err)
	}
	second, _ := os.Stat(marker)
	if second.ModTime().After(old.Add(time.Minute)) {
		t.Fatal("the second call rewrote the link root")
	}
}

func TestEnsureArchiveLinksSkipsUnsafeEntries(t *testing.T) {
	skipWithoutSymlinks(t)
	base := t.TempDir()
	cache, rtRoot := filepath.Join(base, "cache"), filepath.Join(base, "runtime")
	spec := testSpec()
	items := []tarItem{
		{"rt/libx.so.1.2.3", "", tar.TypeReg},
		{"rt/ok.so", "libx.so.1.2.3", tar.TypeSymlink},
		{"rt/abs.so", "/etc/passwd", tar.TypeSymlink},
		{"rt/up.so", "../../etc/passwd", tar.TypeSymlink},
		{"../../escape.so", "rt/libx.so.1.2.3", tar.TypeSymlink},
		{"rt/gone.so", "nothing.so", tar.TypeSymlink},
		{"rt/dir.so", "sub", tar.TypeSymlink},
		{"rt/hard.so", "rt/libx.so.1.2.3", tar.TypeLink},
	}
	for i := 0; i < 12; i++ {
		items = append(items, tarItem{"rt/chain" + string(rune('a'+i)), "chain" + string(rune('a'+i+1)), tar.TypeSymlink})
	}
	writeTestArchive(t, cache, spec, items)
	makeRuntimeFiles(t, rtRoot, "rt/libx.so.1.2.3")
	if err := os.MkdirAll(filepath.Join(rtRoot, "rt", "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	root, err := ensureArchiveLinksFor(context.Background(), "linux", cache, spec, rtRoot)
	if err != nil || root == "" {
		t.Fatalf("got %q, %v", root, err)
	}
	var created []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() != linkMarkerName {
			rel, _ := filepath.Rel(root, p)
			created = append(created, filepath.ToSlash(rel))
		}
		return nil
	})
	if !reflect.DeepEqual(created, []string{"rt/ok.so"}) {
		t.Fatalf("created %v", created)
	}
	for _, escaped := range []string{filepath.Join(base, "escape.so"), filepath.Join(cache, "escape.so"), filepath.Join(filepath.Dir(base), "escape.so")} {
		if _, err := os.Lstat(escaped); err == nil {
			t.Fatalf("%s was written outside the link root", escaped)
		}
	}
}

func TestEnsureArchiveLinksHealsAMissingLink(t *testing.T) {
	skipWithoutSymlinks(t)
	cache, rtRoot := t.TempDir(), filepath.Join(t.TempDir(), "runtime")
	spec := testSpec()
	writeTestArchive(t, cache, spec, []tarItem{
		{"rt/libx.so.1.2.3", "", tar.TypeReg},
		{"rt/libx.so.1", "libx.so.1.2.3", tar.TypeSymlink},
		{"rt/libx.so", "libx.so.1", tar.TypeSymlink},
	})
	makeRuntimeFiles(t, rtRoot, "rt/libx.so.1.2.3")
	root, err := ensureArchiveLinksFor(context.Background(), "linux", cache, spec, rtRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "rt", "libx.so.1")); err != nil {
		t.Fatal(err)
	}
	again, err := ensureArchiveLinksFor(context.Background(), "linux", cache, spec, rtRoot)
	if err != nil || again != root {
		t.Fatalf("got %q, %v", again, err)
	}
	if _, err := os.Stat(filepath.Join(root, "rt", "libx.so.1")); err != nil {
		t.Fatalf("the missing link was not restored: %v", err)
	}
}

func TestEnsureArchiveLinksNotNeeded(t *testing.T) {
	cache, rtRoot := t.TempDir(), t.TempDir()
	zipSpec := llamacpp.AssetSpec{ID: "zip-test", Kind: onnxrt.AssetZip}
	if got, err := ensureArchiveLinksFor(context.Background(), "linux", cache, zipSpec, rtRoot); got != "" || err != nil {
		t.Fatalf("zip: %q, %v", got, err)
	}
	spec := testSpec()
	writeTestArchive(t, cache, spec, []tarItem{{"rt/libx.so", "", tar.TypeReg}})
	if got, err := ensureArchiveLinksFor(context.Background(), "windows", cache, spec, rtRoot); got != "" || err != nil {
		t.Fatalf("windows: %q, %v", got, err)
	}
	if got, err := ensureArchiveLinksFor(context.Background(), "linux", cache, spec, rtRoot); got != "" || err != nil {
		t.Fatalf("no links: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(cache, "links")); err == nil {
		t.Fatal("an archive without links created a link directory")
	}
	if got, err := ensureArchiveLinks(context.Background(), cache, zipSpec, rtRoot); got != "" || err != nil {
		t.Fatalf("exported entry: %q, %v", got, err)
	}
}

func TestEnsureArchiveLinksStopsOnCancel(t *testing.T) {
	cache := t.TempDir()
	spec := testSpec()
	writeTestArchive(t, cache, spec, []tarItem{{"rt/a", "", tar.TypeReg}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ensureArchiveLinksFor(ctx, "linux", cache, spec, t.TempDir()); err == nil {
		t.Fatal("a cancelled context did not stop the read")
	}
}

func TestLibraryRootsIncludeLinkRoots(t *testing.T) {
	linkDir := filepath.Join(t.TempDir(), "links", "a")
	if err := os.MkdirAll(linkDir, 0o750); err != nil {
		t.Fatal(err)
	}
	rtDir := t.TempDir()
	n := &nativeLauncher{ensureLinks: func(_ context.Context, _ string, spec llamacpp.AssetSpec, _ string) (string, error) {
		if spec.ID == "a" {
			return linkDir, nil
		}
		return "", nil
	}}
	assets := []llamacpp.AssetSpec{{ID: "a"}, {ID: "b"}}
	roots, err := n.libraryRoots(context.Background(), "cache", assets, []string{rtDir, t.TempDir()})
	if err != nil || len(roots) != 3 || roots[2] != linkDir {
		t.Fatalf("roots %v, %v", roots, err)
	}
	cmd := newServerCommand("llama-server", roots, "k", nil)
	key := "PATH="
	if runtime.GOOS == "linux" {
		key = "LD_LIBRARY_PATH="
	}
	found := false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, key) && strings.Contains(e, linkDir) {
			found = true
		}
	}
	if !found {
		t.Fatalf("%s lacks the link directory", key)
	}
}
