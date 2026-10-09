package localwiki

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"aurago/internal/zim"
	"aurago/internal/zim/xapian"
)

// Library is one open edition: the ZIM archive and its embedded Xapian indexes.
// Shared users get it through Manager.Acquire; OpenLibrary is for code and tests
// that own the file.
type Library struct {
	edition  Edition
	archive  *zim.Archive
	fulltext *xapian.Database
	title    *xapian.Database
	analyzer xapian.Analyzer
	// fulltextNote says why full-text search is unavailable ("" when it works).
	fulltextNote string
}

// OpenLibrary opens a ZIM and checks what Local Wikipedia needs: a readable
// header and a main page. A missing or unsupported full-text index is not an
// error; the library then offers title search only.
func OpenLibrary(path string, edition Edition) (*Library, error) {
	archive, err := zim.Open(path, zim.Options{})
	if err != nil {
		return nil, fmt.Errorf("open ZIM: %w", err)
	}
	if _, err := archive.MainEntry(); err != nil {
		_ = archive.Close()
		return nil, fmt.Errorf("ZIM main page: %w", err)
	}
	language, _ := archive.Metadata("Language")
	lib := &Library{edition: edition, archive: archive}
	lib.fulltext, lib.fulltextNote = openIndex(archive, "fulltext/xapian")
	lib.title, _ = openIndex(archive, "title/xapian")
	if lib.fulltext != nil {
		// libzim's Searcher analyses queries in the index's own language
		// (slice 2 contract note 6); M/Language is the fallback.
		if indexed, err := lib.fulltext.Metadata("language"); err == nil && strings.TrimSpace(indexed) != "" {
			language = indexed
		}
	}
	lib.analyzer = xapian.NewAnalyzer(language)
	if lib.fulltextNote == "" && !lib.analyzer.FulltextSupported() {
		lib.fulltextNote = "no full-text analyzer for ZIM language " + language
	}
	return lib, nil
}

// openIndex opens X/<path> as a Xapian glass database; note says why not.
func openIndex(archive *zim.Archive, path string) (*xapian.Database, string) {
	entry, err := archive.EntryByPath('X', path)
	if err != nil {
		return nil, "the ZIM has no X/" + path + " index"
	}
	if entry, err = archive.Resolve(entry); err != nil {
		return nil, "X/" + path + ": " + err.Error()
	}
	section, err := archive.Open(entry)
	if err != nil {
		return nil, "X/" + path + ": " + err.Error()
	}
	db, err := xapian.Open(section, section.Size())
	if err != nil {
		// Any Open error (ErrUnsupportedFormat, ErrCorrupt, an I/O error, …)
		// only disables this index; title search keeps working.
		if errors.Is(err, xapian.ErrUnsupportedFormat) {
			return nil, "X/" + path + " uses an unsupported index format"
		}
		return nil, "X/" + path + ": " + err.Error()
	}
	return db, ""
}

// Edition returns the edition this library was opened for.
func (l *Library) Edition() Edition { return l.edition }

// Fulltext reports whether full-text search is available (else title search only).
func (l *Library) Fulltext() bool { return l.fulltext != nil && l.analyzer.FulltextSupported() }

// Close closes the archive. Libraries obtained from Manager.Acquire are closed
// by the manager; callers only release them.
func (l *Library) Close() error {
	if l == nil || l.archive == nil {
		return nil
	}
	return l.archive.Close()
}

func (l *Library) uuid() string {
	u := l.archive.UUID()
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

// libraryRef counts the users of an open library. A retired library is closed
// when its last user releases it; afterClose then runs (e.g. deleting the old
// edition file, which Windows refuses while it is open).
type libraryRef struct {
	lib        *Library
	mu         sync.Mutex
	refs       int
	retired    bool
	closed     bool
	afterClose func()
}

func newLibraryRef(lib *Library) *libraryRef { return &libraryRef{lib: lib} }

func (r *libraryRef) acquire() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.retired {
		return false
	}
	r.refs++
	return true
}

func (r *libraryRef) release() {
	r.mu.Lock()
	r.refs--
	closeNow := r.retired && r.refs == 0 && !r.closed
	if closeNow {
		r.closed = true
	}
	after := r.afterClose
	r.mu.Unlock()
	if closeNow {
		r.finish(after)
	}
}

func (r *libraryRef) retire(afterClose func()) {
	r.mu.Lock()
	if r.retired {
		r.mu.Unlock()
		return
	}
	r.retired = true
	r.afterClose = afterClose
	closeNow := r.refs == 0 && !r.closed
	if closeNow {
		r.closed = true
	}
	r.mu.Unlock()
	if closeNow {
		r.finish(afterClose)
	}
}

func (r *libraryRef) finish(after func()) {
	_ = r.lib.Close()
	if after != nil {
		after()
	}
}
