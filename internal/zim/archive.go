package zim

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync/atomic"
)

const (
	maxRedirectDepth = 8
	direntReadSize   = 256
	maxDirentSize    = 64 << 10
)

// Options tunes an Archive. The zero value is ready to use.
type Options struct {
	ClusterCacheBytes int64 // 0 → DefaultClusterCacheBytes (64 MiB)
}

// limits holds guards that tests can lower.
type limits struct {
	maxClusterBytes int64
}

func defaultLimits() limits { return limits{maxClusterBytes: maxClusterBytes} }

// Archive is an open ZIM file. All methods are safe for concurrent use.
type Archive struct {
	path      string
	r         io.ReaderAt
	closer    io.Closer
	size      int64
	dataEnd   int64
	hdr       header
	mimeTypes []string
	contentNS byte
	lim       limits
	cache     *clusterCache
	titles    titleList
	closed    atomic.Bool
	loads     atomic.Int64 // decompressed cluster loads, for tests
}

// Open opens the ZIM file at path and validates its header and indexes.
func Open(path string, opts Options) (*Archive, error) {
	// The *fs.PathError from os.Open and Stat already names the operation and path.
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("zim: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("zim: %w", err)
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrNotZIM, path)
	}
	a, err := newArchive(f, info.Size(), opts, defaultLimits())
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	a.path = path
	a.closer = f
	return a, nil
}

// newArchive reads the archive structure from r. Tests and fuzzers use it
// with in-memory readers.
func newArchive(r io.ReaderAt, size int64, opts Options, lim limits) (*Archive, error) {
	if size < headerSize {
		return nil, fmt.Errorf("%w: file too small for a ZIM header", ErrNotZIM)
	}
	a := &Archive{size: size, lim: lim, cache: newClusterCache(opts.ClusterCacheBytes)}
	a.r = a.guard(r)
	head := make([]byte, headerSize)
	if err := a.readAt(head, 0); err != nil {
		return nil, err
	}
	h, err := parseHeader(head, size)
	if err != nil {
		return nil, err
	}
	a.hdr = h
	a.dataEnd = h.dataEnd(size)
	listLen := min(int64(maxMimeListBytes), a.dataEnd-int64(h.mimeListPos))
	if listLen <= 0 {
		return nil, errCorrupt("no room for the MIME type list")
	}
	list := make([]byte, listLen)
	if err := a.readAt(list, int64(h.mimeListPos)); err != nil {
		return nil, err
	}
	if a.mimeTypes, err = parseMimeList(list); err != nil {
		return nil, err
	}
	if a.contentNS, err = a.detectContentNamespace(); err != nil {
		return nil, err
	}
	if a.titles, err = a.loadTitleList(); err != nil {
		return nil, err
	}
	return a, nil
}

// detectContentNamespace follows libzim (minor version >= 1 means the new
// C/M/W/X scheme) but falls back to 'A' when an archive has no C entries
// and does have A entries (early 5.1 files).
func (a *Archive) detectContentNamespace() (byte, error) {
	if a.hdr.minor < 1 {
		return 'A', nil
	}
	lo, hi, err := a.namespaceRange('C')
	if err != nil || lo < hi {
		return 'C', err
	}
	lo, hi, err = a.namespaceRange('A')
	if err != nil {
		return 0, err
	}
	if lo < hi {
		return 'A', nil
	}
	return 'C', nil
}

// Close releases the file and the cluster cache. Section readers returned by
// Open fail with ErrClosed afterwards.
func (a *Archive) Close() error {
	if a.closed.Swap(true) {
		return nil
	}
	a.cache.clear()
	if a.closer != nil {
		return a.closer.Close()
	}
	return nil
}

func (a *Archive) Path() string { return a.path }

// Size is the file size in bytes.
func (a *Archive) Size() int64 { return a.size }

func (a *Archive) UUID() [16]byte { return a.hdr.uuid }

func (a *Archive) EntryCount() uint32 { return a.hdr.entryCount }

// ContentNamespace is 'C' for new-scheme archives and 'A' for legacy ones.
func (a *Archive) ContentNamespace() byte { return a.contentNS }

// guardedReaderAt reports ErrClosed for every read once the archive is closed
// and maps the file's own closed-file error (a read that races Close) to
// ErrClosed. io.EOF and other errors pass through unchanged.
type guardedReaderAt struct {
	r      io.ReaderAt
	closed *atomic.Bool
}

func (a *Archive) guard(r io.ReaderAt) io.ReaderAt {
	return guardedReaderAt{r: r, closed: &a.closed}
}

func (g guardedReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if g.closed.Load() {
		return 0, ErrClosed
	}
	n, err := g.r.ReadAt(p, off)
	if err != nil && !errors.Is(err, io.EOF) && (g.closed.Load() || errors.Is(err, fs.ErrClosed)) {
		return n, ErrClosed
	}
	return n, err
}

// readAt fills p from offset off or reports a classified error.
func (a *Archive) readAt(p []byte, off int64) error {
	if a.closed.Load() {
		return ErrClosed
	}
	n, err := a.r.ReadAt(p, off)
	if n == len(p) {
		return nil
	}
	switch {
	case a.closed.Load() || errors.Is(err, ErrClosed):
		return ErrClosed
	case err == nil || errors.Is(err, io.EOF):
		return errCorrupt("unexpected end of file at offset %d", off)
	}
	return fmt.Errorf("zim: read at offset %d: %w", off, err)
}

// EntryAt returns the entry at index in path order.
func (a *Archive) EntryAt(index uint32) (Entry, error) {
	if index >= a.hdr.entryCount {
		return Entry{}, fmt.Errorf("%w: entry index %d (archive has %d)", ErrNotFound, index, a.hdr.entryCount)
	}
	off, err := a.direntOffset(index)
	if err != nil {
		return Entry{}, err
	}
	e, err := a.readDirent(off)
	if err != nil {
		return Entry{}, err
	}
	e.Index = index
	return e, nil
}

func (a *Archive) direntOffset(index uint32) (int64, error) {
	var b [8]byte
	if err := a.readAt(b[:], int64(a.hdr.pathPtrPos)+8*int64(index)); err != nil {
		return 0, err
	}
	off := readOffset(b[:], 8)
	if off < a.hdr.mimeListPos || off >= uint64(a.dataEnd) {
		return 0, errCorrupt("directory entry %d points to offset %d outside the archive", index, off)
	}
	return int64(off), nil
}

// readDirent reads one directory entry, growing the read window for long
// paths and titles up to maxDirentSize.
func (a *Archive) readDirent(off int64) (Entry, error) {
	for size := int64(direntReadSize); ; size *= 4 {
		n := min(size, a.dataEnd-off)
		buf := make([]byte, n)
		if err := a.readAt(buf, off); err != nil {
			return Entry{}, err
		}
		e, err := parseDirent(buf, a.mimeTypes)
		if err == nil {
			return e, nil
		}
		if !errors.Is(err, errShortDirent) {
			return Entry{}, err
		}
		if n < size || size >= maxDirentSize {
			return Entry{}, errCorrupt("directory entry at offset %d is truncated", off)
		}
	}
}

// namespaceAt reads only the namespace byte of the entry at index.
func (a *Archive) namespaceAt(index uint32) (byte, error) {
	off, err := a.direntOffset(index)
	if err != nil {
		return 0, err
	}
	if off+4 > a.dataEnd {
		return 0, errCorrupt("directory entry at offset %d is truncated", off)
	}
	var b [1]byte
	if err := a.readAt(b[:], off+3); err != nil {
		return 0, err
	}
	return b[0], nil
}

// namespaceRange returns the half-open path-order index range [lo, hi) of
// the entries in namespace ns.
func (a *Archive) namespaceRange(ns byte) (uint32, uint32, error) {
	lo, err := a.firstWithNamespaceAtLeast(ns)
	if err != nil || ns == 0xFF {
		return lo, a.hdr.entryCount, err
	}
	hi, err := a.firstWithNamespaceAtLeast(ns + 1)
	return lo, hi, err
}

func (a *Archive) firstWithNamespaceAtLeast(ns byte) (uint32, error) {
	lo, hi := uint32(0), a.hdr.entryCount
	for lo < hi {
		mid := lo + (hi-lo)/2
		got, err := a.namespaceAt(mid)
		if err != nil {
			return 0, err
		}
		if got < ns {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo, nil
}

// EntryByPath finds an entry by namespace and path (binary search over the
// path pointer list).
func (a *Archive) EntryByPath(ns byte, path string) (Entry, error) {
	lo, hi := uint32(0), a.hdr.entryCount
	for lo < hi {
		mid := lo + (hi-lo)/2
		e, err := a.EntryAt(mid)
		if err != nil {
			return Entry{}, err
		}
		switch c := compareKey(e.Namespace, e.Path, ns, path); {
		case c == 0:
			return e, nil
		case c < 0:
			lo = mid + 1
		default:
			hi = mid
		}
	}
	return Entry{}, fmt.Errorf("%w: %c/%s", ErrNotFound, ns, path)
}

// Resolve follows redirects (at most maxRedirectDepth hops). The zero Entry
// (one that no Archive returned) is rejected with ErrNotFound.
func (a *Archive) Resolve(e Entry) (Entry, error) {
	if e.kind == kindInvalid {
		return Entry{}, fmt.Errorf("%w: entry %c/%s was not returned by an archive", ErrNotFound, e.Namespace, e.Path)
	}
	for hops := 0; e.IsRedirect; hops++ {
		if hops == maxRedirectDepth {
			return Entry{}, fmt.Errorf("%w: %c/%s", ErrRedirectLoop, e.Namespace, e.Path)
		}
		if e.RedirectTo >= a.hdr.entryCount {
			return Entry{}, errCorrupt("redirect %c/%s targets missing entry %d", e.Namespace, e.Path, e.RedirectTo)
		}
		next, err := a.EntryAt(e.RedirectTo)
		if err != nil {
			return Entry{}, err
		}
		e = next
	}
	return e, nil
}

// MainEntry returns the resolved main page: W/mainPage, else the header's
// mainPage index.
func (a *Archive) MainEntry() (Entry, error) {
	e, err := a.EntryByPath('W', "mainPage")
	if err == nil {
		return a.Resolve(e)
	}
	if !errors.Is(err, ErrNotFound) {
		return Entry{}, err
	}
	if a.hdr.mainPage == noMainPage {
		return Entry{}, fmt.Errorf("%w: archive has no main page", ErrNotFound)
	}
	if e, err = a.EntryAt(a.hdr.mainPage); err != nil {
		return Entry{}, err
	}
	return a.Resolve(e)
}
