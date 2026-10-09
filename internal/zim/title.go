package zim

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
)

// titleList is the title-ordered article listing: X/listing/titleOrdered/v1
// (front articles only), or the header's v0 list restricted to the content
// namespace. base is the file offset of the first 4-byte entry index.
type titleList struct {
	base  int64
	count int64
	v1    bool
}

// loadTitleList picks the title listing. A v1 listing that cannot be used
// is skipped in favour of the header's v0 list, the tolerant choice libzim
// also makes for a compressed v1 listing:
//   - a compressed v1 listing (the spec requires an uncompressed cluster), or
//   - a v1 listing with more positions than the archive has entries (every
//     position is an entry index and a listing never repeats an entry, so a
//     longer one is damaged).
//
// A damaged v1 listing in an archive without a v0 list is ErrCorrupt rather
// than silently listing nothing. Positions inside an accepted listing are
// validated lazily, when they are read (titleEntry).
func (a *Archive) loadTitleList() (titleList, error) {
	e, err := a.EntryByPath('X', "listing/titleOrdered/v1")
	var damagedV1 error
	switch {
	case err == nil && e.kind == kindContent:
		ci, err := a.clusterInfo(e.cluster)
		if err != nil {
			return titleList{}, err
		}
		if !ci.compressed() {
			off, n, err := a.uncompressedBlob(ci, e.blob)
			if err != nil {
				return titleList{}, err
			}
			if n/4 <= int64(a.hdr.entryCount) {
				return titleList{base: off, count: n / 4, v1: true}, nil
			}
			damagedV1 = errCorrupt("title listing X/listing/titleOrdered/v1 has %d positions, archive has %d entries", n/4, a.hdr.entryCount)
		}
		// Otherwise (compressed) ignore the v1 listing and fall back to v0.
	case err != nil && !errors.Is(err, ErrNotFound):
		return titleList{}, err
	}
	if !a.hdr.hasTitleListV0() {
		return titleList{}, damagedV1
	}
	if !tableFits(a.hdr.titlePtrPos, uint64(a.hdr.entryCount), 4, a.hdr.mimeListPos, uint64(a.dataEnd)) {
		return titleList{}, errCorrupt("title pointer list at %d outside the archive", a.hdr.titlePtrPos)
	}
	// v0 is ordered by namespace then title, so the content namespace is one
	// contiguous block that starts after all entries of lower namespaces.
	lo, hi, err := a.namespaceRange(a.contentNS)
	if err != nil {
		return titleList{}, err
	}
	return titleList{base: int64(a.hdr.titlePtrPos) + 4*int64(lo), count: int64(hi - lo)}, nil
}

// ArticleCount is the number of entries in the title-ordered article list.
func (a *Archive) ArticleCount() int {
	if a.titles.count > math.MaxInt {
		return math.MaxInt
	}
	return int(a.titles.count)
}

// ArticleAt returns the i-th entry of the title-ordered list (not resolved).
func (a *Archive) ArticleAt(i int) (Entry, error) {
	if i < 0 || int64(i) >= a.titles.count {
		return Entry{}, fmt.Errorf("%w: article %d (archive lists %d)", ErrNotFound, i, a.titles.count)
	}
	return a.titleEntry(int64(i))
}

func (a *Archive) titleEntry(i int64) (Entry, error) {
	var b [4]byte
	if err := a.readAt(b[:], a.titles.base+4*i); err != nil {
		return Entry{}, err
	}
	idx := binary.LittleEndian.Uint32(b[:])
	if idx >= a.hdr.entryCount {
		return Entry{}, errCorrupt("title list position %d points to missing entry %d", i, idx)
	}
	e, err := a.EntryAt(idx)
	if err != nil {
		return Entry{}, err
	}
	// The listing promises articles: content or redirect entries of the content
	// namespace. A damaged listing must not hand out X/fulltext/xapian or M/ entries.
	if e.Namespace != a.contentNS || (e.kind != kindContent && e.kind != kindRedirect) {
		return Entry{}, errCorrupt("title list position %d points to %c/%s, which is not a %c article", i, e.Namespace, e.Path, a.contentNS)
	}
	return e, nil
}

// TitlePrefix returns up to limit listed entries whose title starts with
// prefix (byte-wise, case-sensitive), in title order.
func (a *Archive) TitlePrefix(prefix string, limit int) ([]Entry, error) {
	if limit <= 0 || a.titles.count == 0 {
		return nil, nil
	}
	lo, hi := int64(0), a.titles.count
	for lo < hi {
		mid := lo + (hi-lo)/2
		e, err := a.titleEntry(mid)
		if err != nil {
			return nil, err
		}
		if e.Title < prefix {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	var out []Entry
	for i := lo; i < a.titles.count && len(out) < limit; i++ {
		e, err := a.titleEntry(i)
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(e.Title, prefix) {
			break
		}
		out = append(out, e)
	}
	return out, nil
}
