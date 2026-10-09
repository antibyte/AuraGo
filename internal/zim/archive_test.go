package zim

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/zim/zimtest"
)

func openSample(t *testing.T) *Archive {
	t.Helper()
	path, _ := sampleBuilder().WriteFile(t)
	return openPath(t, path)
}

func openPath(t *testing.T, path string) *Archive {
	t.Helper()
	a, err := Open(path, Options{})
	if err != nil {
		t.Fatalf("Open(%s) error = %v", path, err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func mustEntry(t *testing.T, a *Archive, ns byte, path string) Entry {
	t.Helper()
	e, err := a.EntryByPath(ns, path)
	if err != nil {
		t.Fatalf("EntryByPath(%c, %q) error = %v", ns, path, err)
	}
	return e
}

func TestOpenSyntheticArchive(t *testing.T) {
	path, layout := sampleBuilder().WriteFile(t)
	a := openPath(t, path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.Path() != path || a.Size() != info.Size() {
		t.Fatalf("Path/Size = %q/%d, want %q/%d", a.Path(), a.Size(), path, info.Size())
	}
	if int(a.EntryCount()) != len(layout.EntryIndex) {
		t.Fatalf("EntryCount = %d, want %d", a.EntryCount(), len(layout.EntryIndex))
	}
	if uuid := a.UUID(); string(uuid[:]) != "aurago-zimtest-1" {
		t.Fatalf("UUID = %q", uuid[:])
	}
	if a.ContentNamespace() != 'C' {
		t.Fatalf("ContentNamespace = %c, want C", a.ContentNamespace())
	}
}

func TestOpenRejectsNonZIMFiles(t *testing.T) {
	small := zimtest.WriteBytes(t, []byte("hello"))
	_, err := Open(small, Options{})
	wantErr(t, err, ErrNotZIM)
	_, err = Open(t.TempDir(), Options{})
	wantErr(t, err, ErrNotZIM)
	_, err = Open(small+".missing", Options{})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file error = %v, want not-exist", err)
	}
}

func TestEntryByPathAndEntryAt(t *testing.T) {
	path, layout := sampleBuilder().WriteFile(t)
	a := openPath(t, path)
	e := mustEntry(t, a, 'C', "Köln")
	if e.Index != layout.EntryIndex["C/Köln"] || e.Title != "Köln" || e.MimeType != "text/html" || e.IsRedirect {
		t.Fatalf("entry = %+v", e)
	}
	again, err := a.EntryAt(e.Index)
	if err != nil || again.Path != "Köln" {
		t.Fatalf("EntryAt(%d) = %+v, %v", e.Index, again, err)
	}
	for _, missing := range []struct {
		ns   byte
		path string
	}{{'C', "Köl"}, {'C', "Zürich"}, {'A', "Berlin"}, {'X', "listing/titleOrdered/v0"}} {
		_, err := a.EntryByPath(missing.ns, missing.path)
		wantErr(t, err, ErrNotFound)
	}
	_, err = a.EntryAt(a.EntryCount())
	wantErr(t, err, ErrNotFound)
}

func TestResolveFollowsRedirects(t *testing.T) {
	a := openSample(t)
	e, err := a.Resolve(mustEntry(t, a, 'C', "Berlin_(Stadt)"))
	if err != nil || e.Path != "Berlin" || e.IsRedirect {
		t.Fatalf("Resolve = %+v, %v", e, err)
	}
	plain := mustEntry(t, a, 'C', "Bern")
	if got, err := a.Resolve(plain); err != nil || got.Index != plain.Index {
		t.Fatalf("Resolve(non-redirect) = %+v, %v", got, err)
	}
}

func redirectChain(hops int) *zimtest.Builder {
	b := zimtest.New()
	c := b.AddCluster(zimtest.CompressionZstd, false)
	b.AddArticle(c, 'C', "Target", "Target", "<p>target</p>")
	for i := 1; i <= hops; i++ {
		next := fmt.Sprintf("C/R%d", i+1)
		if i == hops {
			next = "C/Target"
		}
		b.AddRedirect('C', fmt.Sprintf("R%d", i), "", next)
	}
	return b
}

func TestResolveRedirectDepthLimit(t *testing.T) {
	path, _ := redirectChain(maxRedirectDepth).WriteFile(t)
	a := openPath(t, path)
	e, err := a.Resolve(mustEntry(t, a, 'C', "R1"))
	if err != nil || e.Path != "Target" {
		t.Fatalf("Resolve(8 hops) = %+v, %v", e, err)
	}
	path, _ = redirectChain(maxRedirectDepth + 1).WriteFile(t)
	a = openPath(t, path)
	_, err = a.Resolve(mustEntry(t, a, 'C', "R1"))
	wantErr(t, err, ErrRedirectLoop)
}

func TestResolveRedirectLoop(t *testing.T) {
	b := zimtest.New()
	b.AddCluster(zimtest.CompressionNone, false) // archives with entries need a cluster
	b.AddRedirect('C', "A", "", "C/B")
	b.AddRedirect('C', "B", "", "C/A")
	b.AddRedirect('C', "Self", "", "C/Self")
	path, _ := b.WriteFile(t)
	a := openPath(t, path)
	for _, p := range []string{"A", "Self"} {
		_, err := a.Resolve(mustEntry(t, a, 'C', p))
		wantErr(t, err, ErrRedirectLoop)
	}
}

func TestRedirectToMissingEntryIsCorrupt(t *testing.T) {
	missing := uint32(1000)
	b := zimtest.New()
	b.AddCluster(zimtest.CompressionNone, false)
	b.Add(zimtest.Entry{Namespace: 'C', Path: "Dangling", RawRedirectIndex: &missing})
	path, _ := b.WriteFile(t)
	a := openPath(t, path)
	_, err := a.Resolve(mustEntry(t, a, 'C', "Dangling"))
	wantErr(t, err, ErrCorrupt)
}

func TestMainEntry(t *testing.T) {
	a := openSample(t)
	e, err := a.MainEntry()
	if err != nil || e.Path != "index" || e.Title != "Hauptseite" {
		t.Fatalf("MainEntry = %+v, %v", e, err)
	}

	b := zimtest.New()
	c := b.AddCluster(zimtest.CompressionZstd, false)
	b.AddArticle(c, 'C', "Start", "Start", "<p>start</p>")
	b.SetHeaderMainPage("C/Start")
	path, _ := b.WriteFile(t)
	if e, err := openPath(t, path).MainEntry(); err != nil || e.Path != "Start" {
		t.Fatalf("header MainEntry = %+v, %v", e, err)
	}

	b = zimtest.New()
	c = b.AddCluster(zimtest.CompressionZstd, false)
	b.AddArticle(c, 'C', "Only", "Only", "<p>only</p>")
	path, _ = b.WriteFile(t)
	_, err = openPath(t, path).MainEntry()
	wantErr(t, err, ErrNotFound)
}

func TestContentNamespaceDetection(t *testing.T) {
	for _, tc := range []struct {
		minor uint16
		ns    byte
		want  byte
	}{{0, 'A', 'A'}, {1, 'A', 'A'}, {1, 'C', 'C'}, {2, 'C', 'C'}} {
		b := zimtest.New()
		b.Minor = tc.minor
		b.TitleListV1 = false
		c := b.AddCluster(zimtest.CompressionZstd, false)
		b.AddArticle(c, tc.ns, "Page", "Page", "<p>page</p>")
		path, _ := b.WriteFile(t)
		if got := openPath(t, path).ContentNamespace(); got != tc.want {
			t.Fatalf("minor %d with %c entries: ContentNamespace = %c, want %c", tc.minor, tc.ns, got, tc.want)
		}
	}
}

func TestOpenErrorsNameThePathOnce(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.zim")
	_, err := Open(missing, Options{})
	if err == nil || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open(missing) error = %v, want not-exist", err)
	}
	notZIM := zimtest.WriteBytes(t, []byte("hello"))
	_, notZIMErr := Open(notZIM, Options{})
	wantErr(t, notZIMErr, ErrNotZIM)
	for path, err := range map[string]error{missing: err, notZIM: notZIMErr} {
		if got := strings.Count(err.Error(), path); got != 1 {
			t.Fatalf("error %q names %q %d times, want once", err, path, got)
		}
	}
}

func TestOpenRejectsMainPageOutsideArchive(t *testing.T) {
	data, layout, err := sampleBuilder().Build()
	if err != nil {
		t.Fatal(err)
	}
	entries := uint32(len(layout.EntryIndex))
	for _, tc := range []struct {
		name     string
		mainPage uint32
		want     error
	}{
		{"first index past the end", entries, ErrCorrupt},
		{"far past the end", 0xFFFFFFFE, ErrCorrupt},
		{"last entry", entries - 1, nil},
		{"no main page sentinel", 0xFFFFFFFF, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patched := append([]byte(nil), data...)
			binary.LittleEndian.PutUint32(patched[64:], tc.mainPage)
			a, err := Open(zimtest.WriteBytes(t, patched), Options{})
			if tc.want != nil {
				wantErr(t, err, tc.want)
				return
			}
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			_ = a.Close()
		})
	}
}

func TestZeroEntryIsRejected(t *testing.T) {
	a := openSample(t)
	_, err := a.Open(Entry{})
	wantErr(t, err, ErrNotFound)
	_, err = a.Resolve(Entry{})
	wantErr(t, err, ErrNotFound)
	_, err = a.Resolve(Entry{IsRedirect: true, RedirectTo: 1})
	wantErr(t, err, ErrNotFound)
	if got := a.loads.Load(); got != 0 {
		t.Fatalf("a zero Entry loaded %d clusters, want 0", got)
	}
	// A copy of a real entry is still fine.
	copied := mustEntry(t, a, 'C', "Berlin")
	if got := readEntry(t, a, copied); !strings.Contains(got, "Berlin") {
		t.Fatalf("copied entry content = %q", got)
	}
}
