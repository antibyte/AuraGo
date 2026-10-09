package zim

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
)

func readEntry(t *testing.T, a *Archive, e Entry) string {
	t.Helper()
	r, err := a.Open(e)
	if err != nil {
		t.Fatalf("Open(%c/%s) error = %v", e.Namespace, e.Path, err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read %c/%s: %v", e.Namespace, e.Path, err)
	}
	return string(b)
}

func TestOpenReadsEveryClusterKind(t *testing.T) {
	a := openSample(t)
	cases := map[string]string{
		"Berlin":   "<html><body><p>Berlin ist die Hauptstadt.</p></body></html>", // zstd
		"Hamburg":  "<html><body><p>Hamburg liegt an der Elbe.</p></body></html>", // xz
		"Köln":     "<html><body><p>Köln am Rhein.</p></body></html>",             // extended zstd
		"map.webp": "RIFF-webp-bytes",                                             // uncompressed
		"big.bin":  strings.Repeat("\xab", 4096),                                  // extended uncompressed
	}
	for path, want := range cases {
		if got := readEntry(t, a, mustEntry(t, a, 'C', path)); got != want {
			t.Fatalf("C/%s = %q, want %q", path, got, want)
		}
	}
}

func TestOpenUncompressedBlobIsFileSection(t *testing.T) {
	a := openSample(t)
	for _, path := range []string{"fulltext/xapian", "title/xapian"} {
		e := mustEntry(t, a, 'X', path)
		r, err := a.Open(e)
		if err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 5)
		if _, err := r.ReadAt(buf, r.Size()-5); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(readEntry(t, a, e), string(buf)) {
			t.Fatalf("X/%s ReadAt tail = %q", path, buf)
		}
	}
	if got := a.loads.Load(); got != 0 {
		t.Fatalf("uncompressed reads decompressed %d clusters, want 0", got)
	}
}

func TestOpenRedirectNeedsResolve(t *testing.T) {
	a := openSample(t)
	_, err := a.Open(mustEntry(t, a, 'C', "Berlin_(Stadt)"))
	wantErr(t, err, ErrIsRedirect)
}

func TestMetadata(t *testing.T) {
	a := openSample(t)
	for name, want := range map[string]string{"Language": "deu", "Title": "Wikipedia (Test)", "Date": "2026-10-01", "Alias": "Wikipedia (Test)"} {
		got, err := a.Metadata(name)
		if err != nil || got != want {
			t.Fatalf("Metadata(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	_, err := a.Metadata("Creator")
	wantErr(t, err, ErrNotFound)
}

func TestConcurrentReadsShareOneDecompression(t *testing.T) {
	a := openSample(t)
	paths := []string{"Berlin", "Bern", "index"} // all in the same zstd cluster
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Go(func() {
			e, err := a.EntryByPath('C', paths[i%len(paths)])
			if err != nil {
				t.Error(err)
				return
			}
			r, err := a.Open(e)
			if err != nil {
				t.Error(err)
				return
			}
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil || !strings.HasPrefix(buf.String(), "<html>") {
				t.Errorf("read %s: %q, %v", e.Path, buf.String(), err)
			}
		})
	}
	wg.Wait()
	if got := a.loads.Load(); got != 1 {
		t.Fatalf("cluster loads = %d, want 1", got)
	}
}

func TestClusterCacheBudgetIsHonoured(t *testing.T) {
	path, _ := sampleBuilder().WriteFile(t)
	a, err := Open(path, Options{ClusterCacheBytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for range 3 {
		readEntry(t, a, mustEntry(t, a, 'C', "Berlin"))
	}
	if got := a.loads.Load(); got != 3 {
		t.Fatalf("loads with a 1-byte cache = %d, want 3", got)
	}
	if n, used := a.cache.usage(); n != 0 || used != 0 {
		t.Fatalf("cache usage = %d/%d, want empty", n, used)
	}
}

func TestCloseStopsReads(t *testing.T) {
	path, _ := sampleBuilder().WriteFile(t)
	a, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	e := mustEntry(t, a, 'C', "Berlin")
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("second Close = %v", err)
	}
	_, err = a.Open(e)
	wantErr(t, err, ErrClosed)
	_, err = a.EntryByPath('C', "Berlin")
	wantErr(t, err, ErrClosed)
}
