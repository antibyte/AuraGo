package xapian

import (
	"bytes"
)

// PostingIterator walks (docid, wdf) pairs in ascending docid order. Next must
// be called before the first DocID/WDF.
type PostingIterator interface {
	Next() bool
	DocID() uint32
	WDF() uint32
	Err() error
}

// Postings iterates the documents of term in ascending docid order.
func (d *Database) Postings(term string) (PostingIterator, error) {
	if term == "" {
		return emptyPostings{}, nil
	}
	return d.openPostings(term)
}

// DocLength returns the length (sum of wdf) of document did.
func (d *Database) DocLength(did uint32) (uint32, error) {
	if did == 0 || did > d.info.lastDocID {
		return 0, ErrDocNotFound
	}
	pl, err := d.openPostings("")
	if err != nil {
		return 0, err
	}
	ok, err := pl.skipTo(did)
	if err != nil {
		return 0, err
	}
	if !ok || pl.did != did {
		return 0, ErrDocNotFound
	}
	return pl.wdf, nil
}

type emptyPostings struct{}

func (emptyPostings) Next() bool    { return false }
func (emptyPostings) DocID() uint32 { return 0 }
func (emptyPostings) WDF() uint32   { return 0 }
func (emptyPostings) Err() error    { return nil }

// postingList decodes the chunked posting list of one term ("" = the
// document length list, whose "wdf" values are document lengths).
type postingList struct {
	db          *Database
	term        string
	c           *cursor
	data        []byte // undecoded rest of the current chunk (may alias a cached block)
	did         uint32
	wdf         uint32
	lastInChunk uint32
	isLastChunk bool
	started     bool
	done        bool
	err         error
	termFreq    uint32
}

func (d *Database) openPostings(term string) (*postingList, error) {
	p := &postingList{db: d, term: term, c: d.postlist.cursor()}
	_, exact, err := p.c.seekLE(postlistKey(term))
	if err != nil {
		return nil, err
	}
	if !exact {
		p.done = true
		return p, nil
	}
	if err := p.loadChunk(); err != nil {
		return nil, err
	}
	return p, nil
}

// chunkStart parses the cursor key as a chunk of p.term. first is true for the
// initial chunk (whose start docid lives in the tag header).
func (p *postingList) chunkStart(key []byte) (did uint32, first, ok bool, err error) {
	var rest []byte
	if p.term == "" {
		if !bytes.HasPrefix(key, keyPrefixDocLen) {
			return 0, false, false, nil
		}
		rest = key[2:]
	} else {
		if len(key) >= 2 && key[0] == 0 && key[1] != 0xff {
			return 0, false, false, nil
		}
		t, n, terminated := unpackSortPreservingString(key)
		if t != p.term {
			return 0, false, false, nil
		}
		if !terminated {
			return 0, true, true, nil
		}
		rest = key[n:]
	}
	if len(rest) == 0 {
		return 0, true, true, nil
	}
	v, n, err := unpackSortableUint(rest)
	if err != nil {
		return 0, false, false, err
	}
	if n != len(rest) || v == 0 || v > 0xffffffff {
		return 0, false, false, corruptf("bad posting chunk key for %q", p.term)
	}
	return uint32(v), false, true, nil
}

// loadChunk decodes the chunk the cursor is on and positions on its first posting.
func (p *postingList) loadChunk() error {
	key, err := p.c.key()
	if err != nil {
		return err
	}
	did, first, ok, err := p.chunkStart(key)
	if err != nil {
		return err
	}
	if !ok {
		return corruptf("posting list for %q: unexpected key", p.term)
	}
	tag, err := p.c.tagView()
	if err != nil {
		return err
	}
	if first {
		tf, n, err := unpackUint32(tag)
		if err != nil {
			return err
		}
		p.termFreq = tf
		tag = tag[n:]
		if _, n, err = unpackUint(tag); err != nil { // collection frequency
			return err
		}
		tag = tag[n:]
		d0, n, err := unpackUint32(tag)
		if err != nil {
			return err
		}
		tag = tag[n:]
		if d0 == 0xffffffff {
			return corruptf("posting list for %q: first docid overflows", p.term)
		}
		did = d0 + 1
	}
	if len(tag) == 0 || (tag[0] != '0' && tag[0] != '1') {
		return corruptf("posting list for %q: bad chunk header", p.term)
	}
	p.isLastChunk = tag[0] == '1'
	inc, n, err := unpackUint32(tag[1:])
	if err != nil {
		return err
	}
	tag = tag[1+n:]
	if uint64(did)+uint64(inc) > 0xffffffff {
		return corruptf("posting list for %q: chunk end overflows", p.term)
	}
	if p.started && did <= p.did {
		return corruptf("posting list for %q: chunk docids not increasing", p.term)
	}
	p.lastInChunk = did + inc
	wdf, n, err := unpackUint32(tag)
	if err != nil {
		return err
	}
	p.did, p.wdf, p.data = did, wdf, tag[n:]
	return nil
}

// advance moves to the next posting; false at the end of the list.
func (p *postingList) advance() (bool, error) {
	if d := p.data; len(d) > 0 {
		// Scoring decodes millions of (gap, wdf) pairs per query; almost all
		// are a one-byte gap with a one- or two-byte wdf (document lengths
		// mostly need two bytes), decoded here without a call.
		var delta, wdf uint32
		var n int
		switch {
		case len(d) >= 2 && d[0] < 0x80 && d[1] < 0x80:
			delta, wdf, n = uint32(d[0]), uint32(d[1]), 2
		case len(d) >= 3 && d[0] < 0x80 && d[2] < 0x80:
			delta, wdf, n = uint32(d[0]), uint32(d[1]&0x7f)|uint32(d[2])<<7, 3
		default:
			var m int
			var err error
			if delta, n, err = unpackUint32(d); err != nil {
				return false, err
			}
			if wdf, m, err = unpackUint32(d[n:]); err != nil {
				return false, err
			}
			n += m
		}
		next := uint64(p.did) + uint64(delta) + 1
		if next > uint64(p.lastInChunk) {
			return false, corruptf("posting list for %q: docid beyond chunk end", p.term)
		}
		p.did, p.wdf, p.data = uint32(next), wdf, d[n:]
		return true, nil
	}
	if p.did != p.lastInChunk {
		return false, corruptf("posting list for %q: chunk ends before its last docid", p.term)
	}
	if p.isLastChunk {
		return false, nil
	}
	ok, err := p.c.next()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, corruptf("posting list for %q: missing continuation chunk", p.term)
	}
	if err := p.loadChunk(); err != nil {
		return false, err
	}
	return true, nil
}

// Next implements PostingIterator.
func (p *postingList) Next() bool {
	if p.done || p.err != nil {
		return false
	}
	if !p.started {
		p.started = true
		return true
	}
	ok, err := p.advance()
	if err != nil {
		p.err, p.done = err, true
		return false
	}
	if !ok {
		p.done = true
	}
	return ok
}

func (p *postingList) DocID() uint32 { return p.did }
func (p *postingList) WDF() uint32   { return p.wdf }
func (p *postingList) Err() error    { return p.err }

// skipTo positions on the first posting with docid >= target.
func (p *postingList) skipTo(target uint32) (bool, error) {
	if p.done || p.err != nil {
		return false, p.err
	}
	p.started = true
	if p.did >= target {
		return true, nil
	}
	if target > p.lastInChunk && !p.isLastChunk {
		if err := p.chunkFor(target); err != nil {
			p.err, p.done = err, true
			return false, err
		}
	}
	for p.did < target {
		ok, err := p.advance()
		if err != nil {
			p.err, p.done = err, true
			return false, err
		}
		if !ok {
			p.done = true
			return false, nil
		}
	}
	return true, nil
}

// chunkFor loads the chunk that may contain target, which lies beyond the
// current chunk of a list that continues. Scoring walks the document length
// list (and common terms) in docid order, so target is usually in the next
// chunk: one cursor step instead of a seek from the root. A target past the
// next chunk falls back to jumpTo.
func (p *postingList) chunkFor(target uint32) error {
	ok, err := p.c.next()
	if err != nil {
		return err
	}
	if !ok {
		return corruptf("posting list for %q: missing continuation chunk", p.term)
	}
	if err := p.loadChunk(); err != nil {
		return err
	}
	if p.lastInChunk >= target || p.isLastChunk {
		return nil
	}
	return p.jumpTo(target)
}

// jumpTo loads the chunk that may contain target: the last chunk starting at
// or before target, or the one after it when that chunk ends before target.
func (p *postingList) jumpTo(target uint32) error {
	if _, _, err := p.c.seekLE(postlistChunkKey(p.term, target)); err != nil {
		return err
	}
	p.started = false
	err := p.loadChunk()
	p.started = true
	if err != nil {
		return err
	}
	if p.lastInChunk >= target || p.isLastChunk {
		return nil
	}
	ok, err := p.c.next()
	if err != nil {
		return err
	}
	if !ok {
		return corruptf("posting list for %q: missing continuation chunk", p.term)
	}
	return p.loadChunk()
}
