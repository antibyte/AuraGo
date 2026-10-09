package zim

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"aurago/internal/zim/zimtest"
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

func TestSectionsFailWithErrClosedAfterClose(t *testing.T) {
	path, _ := sampleBuilder().WriteFile(t)
	a, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	sections := map[string]*io.SectionReader{}
	for name, e := range map[string]Entry{
		"uncompressed": mustEntry(t, a, 'X', "title/xapian"),
		"zstd":         mustEntry(t, a, 'C', "Berlin"),
		"xz":           mustEntry(t, a, 'C', "Hamburg"),
	} {
		r, err := a.Open(e)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.ReadAt(make([]byte, 4), 0); err != nil {
			t.Fatalf("%s: read before Close: %v", name, err)
		}
		sections[name] = r
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	for name, r := range sections {
		buf := make([]byte, 4)
		_, err := r.ReadAt(buf, 0)
		wantErr(t, err, ErrClosed)
		_, err = r.Read(buf)
		wantErr(t, err, ErrClosed)
		_, err = io.ReadAll(r)
		wantErr(t, err, ErrClosed)
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			t.Fatalf("%s: read after Close leaks the file error %v", name, err)
		}
	}
}

// scriptedReaderAt returns a fixed result; onRead runs first.
type scriptedReaderAt struct {
	n      int
	err    error
	onRead func()
}

func (s scriptedReaderAt) ReadAt([]byte, int64) (int, error) {
	if s.onRead != nil {
		s.onRead()
	}
	return s.n, s.err
}

func TestGuardedReaderAt(t *testing.T) {
	var closed atomic.Bool
	guard := func(r io.ReaderAt) guardedReaderAt { return guardedReaderAt{r: r, closed: &closed} }
	buf := make([]byte, 8)
	diskErr := errors.New("disk on fire")

	// The file was closed underneath a read (Close racing a read).
	_, err := guard(scriptedReaderAt{err: &fs.PathError{Op: "read", Path: "x.zim", Err: fs.ErrClosed}}).ReadAt(buf, 0)
	wantErr(t, err, ErrClosed)

	// io.EOF and ordinary I/O errors pass through while the archive is open.
	if n, err := guard(scriptedReaderAt{n: 3, err: io.EOF}).ReadAt(buf, 0); n != 3 || err != io.EOF {
		t.Fatalf("EOF read = %d, %v; want 3, io.EOF", n, err)
	}
	if _, err := guard(scriptedReaderAt{err: diskErr}).ReadAt(buf, 0); !errors.Is(err, diskErr) || errors.Is(err, ErrClosed) {
		t.Fatalf("I/O error = %v, want the original error", err)
	}

	// A failure that happens while the archive is being closed is a close error.
	_, err = guard(scriptedReaderAt{err: diskErr, onRead: func() { closed.Store(true) }}).ReadAt(buf, 0)
	wantErr(t, err, ErrClosed)

	// Once closed, even a read that would succeed is refused; EOF is not special-cased.
	_, err = guard(scriptedReaderAt{n: len(buf)}).ReadAt(buf, 0)
	wantErr(t, err, ErrClosed)
	_, err = guard(scriptedReaderAt{n: 1, err: io.EOF}).ReadAt(buf, 0)
	wantErr(t, err, ErrClosed)
}

// failLenReaderAt simulates the file being closed underneath reads of one length.
type failLenReaderAt struct {
	r       io.ReaderAt
	failLen int
}

func (f failLenReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if len(p) == f.failLen {
		return 0, &fs.PathError{Op: "read", Path: "race.zim", Err: fs.ErrClosed}
	}
	return f.r.ReadAt(p, off)
}

func TestMetadataReadRacingCloseReportsErrClosed(t *testing.T) {
	value := strings.Repeat("v", 300) // no other read in this archive has this length
	b := zimtest.New()
	b.AddMetadata(b.AddCluster(zimtest.CompressionNone, false), "Blob", value)
	data, _, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	open := func(r io.ReaderAt) *Archive {
		a, err := newArchive(r, int64(len(data)), Options{}, defaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	if got, err := open(bytes.NewReader(data)).Metadata("Blob"); err != nil || got != value {
		t.Fatalf("Metadata without a failing reader = %d bytes, %v", len(got), err)
	}
	_, err = open(failLenReaderAt{r: bytes.NewReader(data), failLen: len(value)}).Metadata("Blob")
	wantErr(t, err, ErrClosed)
}
