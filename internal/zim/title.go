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

func (a *Archive) loadTitleList() (titleList, error) {
	e, err := a.EntryByPath('X', "listing/titleOrdered/v1")
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
			return titleList{base: off, count: n / 4, v1: true}, nil
		}
		// The spec requires listings in uncompressed clusters; like libzim,
		// ignore a compressed v1 listing and fall back to v0.
	case err != nil && !errors.Is(err, ErrNotFound):
		return titleList{}, err
	}
	if !a.hdr.hasTitleListV0() {
		return titleList{}, nil
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
	return a.EntryAt(idx)
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
