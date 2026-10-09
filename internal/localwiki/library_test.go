package localwiki

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"aurago/internal/zim/zimtest"
)

// fixtureZIMPath is the German libzim fixture with a real full-text index from
// slice 2 (scripts/localwiki/fixtures).
func fixtureZIMPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "zim", "testdata", "fixture_de.zim")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("slice-2 fixture %s is missing: %v", path, err)
	}
	return path
}

func fixtureZIMBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(fixtureZIMPath(t))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestOpenLibraryReadsFixture(t *testing.T) {
	edition := Edition{Name: "wikipedia_de_all_nopic_2026-10"}
	lib, err := OpenLibrary(fixtureZIMPath(t), edition)
	if err != nil {
		t.Fatalf("OpenLibrary: %v", err)
	}
	defer lib.Close()
	if lib.Edition() != edition {
		t.Fatalf("Edition() = %+v", lib.Edition())
	}
	if !lib.Fulltext() || lib.fulltextNote != "" {
		t.Fatalf("German fixture must offer full-text search (note %q)", lib.fulltextNote)
	}
	if lib.fulltext == nil || lib.archive == nil || lib.analyzer.Language() != "de" {
		t.Fatalf("library fields = fulltext %v archive %v analyzer %q", lib.fulltext, lib.archive, lib.analyzer.Language())
	}
	if uuid := lib.uuid(); len(uuid) != 36 {
		t.Fatalf("uuid = %q", uuid)
	}
}

// Every shipped fixture opens, and the analyzer follows the language of the
// full-text index (the ZIM's M/Language is only the fallback).
func TestOpenLibraryAnalyzerLanguagePerFixture(t *testing.T) {
	for file, want := range map[string]string{
		"fixture_de.zim": "de",
		"fixture_en.zim": "en",
		"fixture_pl.zim": "pl",
		"fixture_ja.zim": "ja",
	} {
		lib, err := OpenLibrary(filepath.Join("..", "zim", "testdata", file), Edition{})
		if err != nil {
			t.Fatalf("%s: OpenLibrary: %v", file, err)
		}
		if got := lib.analyzer.Language(); got != want {
			t.Errorf("%s: analyzer language = %q, want %q", file, got, want)
		}
		if !lib.Fulltext() {
			t.Errorf("%s: Fulltext() = false (note %q)", file, lib.fulltextNote)
		}
		lib.Close()
	}
}

// A damaged, unsupported or missing index only disables that index; the
// library stays usable with title search and the ZIM's M/Language.
func TestOpenLibraryKeepsWorkingWhenAnIndexCannotBeOpened(t *testing.T) {
	build := func(withIndexes bool) string {
		b := zimtest.New()
		z := b.AddCluster(zimtest.CompressionZstd, false)
		u := b.AddCluster(zimtest.CompressionNone, false)
		b.AddArticle(z, 'C', "index", "Hauptseite", "<html><body><p>Willkommen</p></body></html>")
		b.AddMetadata(z, "Language", "deu")
		b.AddRedirect('W', "mainPage", "", "C/index")
		if withIndexes {
			b.Add(zimtest.Entry{Namespace: 'X', Path: "fulltext/xapian", MimeType: "application/octet-stream+xapian", Data: []byte("not a glass database"), Cluster: u})
			b.Add(zimtest.Entry{Namespace: 'X', Path: "title/xapian", MimeType: "application/octet-stream+xapian", Data: []byte("not a glass database either"), Cluster: u})
		}
		path, _ := b.WriteFile(t)
		return path
	}
	for name, withIndexes := range map[string]bool{"corrupt indexes": true, "no indexes": false} {
		lib, err := OpenLibrary(build(withIndexes), Edition{Name: "x"})
		if err != nil {
			t.Fatalf("%s: OpenLibrary: %v", name, err)
		}
		if lib.Fulltext() || lib.fulltext != nil || lib.title != nil || lib.fulltextNote == "" {
			t.Errorf("%s: fulltext %v title %v note %q", name, lib.fulltext, lib.title, lib.fulltextNote)
		}
		if lib.analyzer.Language() != "de" {
			t.Errorf("%s: analyzer language = %q, want the ZIM's de", name, lib.analyzer.Language())
		}
		lib.Close()
	}
}

func TestOpenLibraryRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.zim")
	if err := os.WriteFile(path, []byte("definitely not a ZIM archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	if lib, err := OpenLibrary(path, Edition{}); err == nil {
		lib.Close()
		t.Fatal("garbage was accepted as a ZIM")
	}
	if err := (*Library)(nil).Close(); err != nil {
		t.Fatalf("nil Close = %v", err)
	}
}

func TestLibraryRefClosesAfterLastReleaseOnly(t *testing.T) {
	lib, err := OpenLibrary(fixtureZIMPath(t), Edition{})
	if err != nil {
		t.Fatal(err)
	}
	ref := newLibraryRef(lib)
	const readers = 8
	for i := 0; i < readers; i++ {
		if !ref.acquire() {
			t.Fatal("acquire before retire failed")
		}
	}
	var closed atomic.Int32
	ref.retire(func() { closed.Add(1) })
	if closed.Load() != 0 {
		t.Fatal("retire closed a library that still has readers")
	}
	if ref.acquire() {
		t.Fatal("acquire after retire succeeded")
	}
	var wg sync.WaitGroup
	for i := 0; i < readers-1; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ref.lib.Edition()
			ref.release()
		}()
	}
	wg.Wait()
	if closed.Load() != 0 {
		t.Fatalf("afterClose ran %d times while one reader was left", closed.Load())
	}
	ref.release()
	if closed.Load() != 1 {
		t.Fatalf("afterClose ran %d times, want 1", closed.Load())
	}
	ref.retire(func() { closed.Add(1) })
	if closed.Load() != 1 {
		t.Fatal("a second retire must not run afterClose again")
	}
}

func TestLibraryRefClosesImmediatelyWithoutReaders(t *testing.T) {
	lib, err := OpenLibrary(fixtureZIMPath(t), Edition{})
	if err != nil {
		t.Fatal(err)
	}
	ref := newLibraryRef(lib)
	var closed atomic.Int32
	ref.retire(func() { closed.Add(1) })
	if closed.Load() != 1 || ref.acquire() {
		t.Fatalf("retire without readers: afterClose ran %d times", closed.Load())
	}
}

// A release without a matching acquire is ignored: the count must not go
// negative, or the library would never close and its file never be deleted.
func TestLibraryRefIgnoresUnmatchedRelease(t *testing.T) {
	lib, err := OpenLibrary(fixtureZIMPath(t), Edition{})
	if err != nil {
		t.Fatal(err)
	}
	ref := newLibraryRef(lib)
	ref.release()
	ref.release()
	if ref.refs != 0 {
		t.Fatalf("refs = %d after unmatched releases", ref.refs)
	}
	if !ref.acquire() {
		t.Fatal("acquire failed")
	}
	var closed atomic.Int32
	ref.retire(func() { closed.Add(1) })
	if closed.Load() != 0 {
		t.Fatal("closed while a reader is active")
	}
	ref.release()
	if closed.Load() != 1 {
		t.Fatalf("afterClose ran %d times, want 1", closed.Load())
	}
	ref.release() // after the close: still ignored
	if closed.Load() != 1 || ref.refs != 0 {
		t.Fatalf("a release after the close changed state: closed %d refs %d", closed.Load(), ref.refs)
	}
}

func TestDataDirChecksAndDiskMath(t *testing.T) {
	if diskMargin(10<<30) != 1<<30 || diskMargin(200_000_000_000) != 2_000_000_000 {
		t.Fatalf("diskMargin wrong: %d %d", diskMargin(10<<30), diskMargin(200_000_000_000))
	}
	if requiredBytes(100, 40) != 60+(1<<30) || requiredBytes(100, 400) != 1<<30 {
		t.Fatal("requiredBytes wrong")
	}
	// Hostile or corrupt sizes saturate instead of wrapping around to a small
	// or negative requirement that would pass every disk check.
	for name, got := range map[string]int64{
		"max size":           requiredBytes(math.MaxInt64, 0),
		"max size, negative": requiredBytes(math.MaxInt64, -5),
		"max size, min":      requiredBytes(math.MaxInt64, math.MinInt64),
		"near max":           requiredBytes(math.MaxInt64-100, 0),
		"margin-sized gap":   requiredBytes(math.MaxInt64-(1<<30)+1, 0),
	} {
		if got != math.MaxInt64 {
			t.Errorf("requiredBytes(%s) = %d, want saturation at MaxInt64", name, got)
		}
	}
	if got := requiredBytes(math.MinInt64, 5); got != 1<<30 {
		t.Errorf("requiredBytes(MinInt64, 5) = %d, want the margin", got)
	}
	if got := requiredBytes(1<<40, 1); got <= 1<<40-1 || got < 0 {
		t.Errorf("requiredBytes(cap) = %d", got)
	}
	if got := diskMargin(math.MaxInt64); got <= 0 {
		t.Errorf("diskMargin(MaxInt64) = %d", got)
	}
	base := t.TempDir()
	valid := filepath.Join(base, "nested", "wikipedia")
	if err := prepareDataDir(valid, nil); err != nil {
		t.Fatalf("prepareDataDir(valid) = %v", err)
	}
	if info, err := os.Stat(valid); err != nil || !info.IsDir() {
		t.Fatal("prepareDataDir did not create the directory")
	}
	entries, _ := os.ReadDir(valid)
	if len(entries) != 0 {
		t.Fatalf("write probe left files behind: %v", entries)
	}
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{"relative": "wikipedia", "empty": "", "file": file, "below a file": filepath.Join(file, "wiki")} {
		if err := prepareDataDir(dir, nil); ErrorCode(err) != CodeDataDirInvalid {
			t.Fatalf("%s: prepareDataDir = %v", name, err)
		}
	}
	sensitive := func(string) bool { return true }
	if err := prepareDataDir(valid, sensitive); ErrorCode(err) != CodeDataDirInvalid {
		t.Fatalf("sensitive directory accepted: %v", err)
	}
}

// The sensitive-path check is repeated on the resolved directory, so a link
// with an innocent name cannot point the edition into a protected tree.
func TestPrepareDataDirChecksResolvedSymlinks(t *testing.T) {
	base := t.TempDir()
	protected := filepath.Join(base, "protected-tree")
	if err := os.MkdirAll(filepath.Join(protected, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "innocent-link")
	symlink := makeDirLink(t, link, protected)
	sensitive := func(p string) bool {
		return strings.Contains(strings.ToLower(filepath.ToSlash(p)), "protected-tree")
	}
	cases := map[string]string{"link": link, "below the link": filepath.Join(link, "inner"), "new dir below the link": filepath.Join(link, "new", "wiki")}
	if !symlink {
		// Go does not resolve a Windows junction in the last path element.
		delete(cases, "link")
	}
	for name, dir := range cases {
		if err := prepareDataDir(dir, sensitive); ErrorCode(err) != CodeDataDirInvalid {
			t.Errorf("%s: prepareDataDir = %v", name, err)
		}
	}
	if entries, _ := os.ReadDir(protected); len(entries) != 1 {
		for _, e := range entries {
			if e.Name() != "inner" && e.Name() != "new" {
				t.Errorf("the write probe was left in the protected tree: %s", e.Name())
			}
		}
	}

	harmless := filepath.Join(base, "harmless")
	if err := os.MkdirAll(harmless, 0o755); err != nil {
		t.Fatal(err)
	}
	harmlessLink := filepath.Join(base, "harmless-link")
	makeDirLink(t, harmlessLink, harmless)
	for name, dir := range map[string]string{"plain directory": harmless, "link to a plain directory": harmlessLink} {
		if err := prepareDataDir(dir, sensitive); err != nil {
			t.Errorf("%s: prepareDataDir = %v", name, err)
		}
	}
}

// macOS keeps /var, /tmp and /etc as links into /private. A directory that
// differs from its resolved form only by that alias is the same place.
func TestIsSystemPathAlias(t *testing.T) {
	for _, tc := range []struct {
		lexical, resolved string
		want              bool
	}{
		{"/var/folders/ab/T/wiki", "/private/var/folders/ab/T/wiki", true},
		{"/tmp/wiki", "/private/tmp/wiki", true},
		{"/etc/aurago", "/private/etc/aurago", true},
		{"/var", "/private/var", true},
		{"/Var/Folders/x", "/private/var/folders/x", true},
		{"/tmp/wiki", "/private/etc/wiki", false},
		{"/tmp/evil", "/usr/lib", false},
		{"/var/x", "/private/var/y", false},
		{"/home/user/wiki", "/private/home/user/wiki", false},
		{"/data/wiki", "/data/wiki", false},
		{"/varnish/x", "/private/varnish/x", false},
	} {
		if got := isSystemPathAlias(tc.lexical, tc.resolved); got != tc.want {
			t.Errorf("isSystemPathAlias(%q, %q) = %v, want %v", tc.lexical, tc.resolved, got, tc.want)
		}
	}
}

// makeDirLink links link to the directory target: a symbolic link, or on
// Windows without the privilege for those a directory junction (symlink is
// then false). It skips the test when neither can be created.
func makeDirLink(t *testing.T, link, target string) (symlink bool) {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return true
	}
	if runtime.GOOS == "windows" {
		if out, jerr := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); jerr == nil {
			return false
		} else {
			err = fmt.Errorf("%w; mklink /J: %v: %s", err, jerr, out)
		}
	}
	t.Skipf("directory links are unavailable here: %v", err)
	return false
}
