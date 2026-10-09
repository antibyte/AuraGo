package localwiki

import (
	"os"
	"path/filepath"
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
	if ref.acquire() {
		t.Fatal("acquire after retire succeeded")
	}
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ref.lib.Edition()
			ref.release()
		}()
	}
	wg.Wait()
	if closed.Load() != 1 {
		t.Fatalf("afterClose ran %d times, want 1", closed.Load())
	}
	ref.retire(func() { closed.Add(1) })
	if closed.Load() != 1 {
		t.Fatal("a second retire must not run afterClose again")
	}
}

func TestDataDirChecksAndDiskMath(t *testing.T) {
	if diskMargin(10<<30) != 1<<30 || diskMargin(200_000_000_000) != 2_000_000_000 {
		t.Fatalf("diskMargin wrong: %d %d", diskMargin(10<<30), diskMargin(200_000_000_000))
	}
	if requiredBytes(100, 40) != 60+(1<<30) || requiredBytes(100, 400) != 1<<30 {
		t.Fatal("requiredBytes wrong")
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
