package xapian

import (
	"fmt"
	"io"
	"strings"
)

// Database is a read-only single-file glass database (as embedded by libzim at
// X/fulltext/xapian and X/title/xapian). It is safe for concurrent use.
type Database struct {
	r        io.ReaderAt
	size     int64
	info     versionInfo
	postlist *table
	docdata  *table
	avgLen   float64
}

// Open reads the version block at offset 0 of r and validates the postlist
// root. size is the byte length of the embedded database.
func Open(r io.ReaderAt, size int64) (*Database, error) {
	if size < minBlockSize {
		return nil, unsupportedf("%d bytes is too small for a single-file glass database", size)
	}
	head := make([]byte, minBlockSize)
	if err := readFull(r, head, 0); err != nil {
		return nil, fmt.Errorf("xapian: read version block: %w", err)
	}
	info, err := parseVersion(head)
	if err != nil {
		return nil, err
	}
	bs := info.roots[tablePostlist].blockSize
	for t := 0; t < tableCount; t++ {
		if !info.roots[t].rootIsFake && info.roots[t].blockSize != bs {
			return nil, unsupportedf("mixed block sizes (%s %d, postlist %d)", tableNames[t], info.roots[t].blockSize, bs)
		}
	}
	nblocks := size / int64(bs)
	if nblocks > 0xffffffff {
		return nil, unsupportedf("database too large")
	}
	cache := newBlockCache(defaultCacheBytes / bs)
	mk := func(t int) *table {
		ri := info.roots[t]
		return &table{
			name: tableNames[t], r: r, blockSize: bs, nblocks: uint32(nblocks),
			root: ri.root, level: ri.level, empty: ri.rootIsFake, cache: cache,
		}
	}
	d := &Database{r: r, size: size, info: info, postlist: mk(tablePostlist), docdata: mk(tableDocdata)}
	if info.docCount > 0 {
		if d.postlist.empty {
			return nil, corruptf("documents present but postlist table is empty")
		}
		d.avgLen = float64(info.totalLength) / float64(info.docCount)
	}
	for _, t := range []*table{d.postlist, d.docdata} {
		if !t.empty {
			if _, err := t.readBlock(t.root, t.level); err != nil {
				return nil, err
			}
		}
	}
	return d, nil
}

// DocCount is the number of documents.
func (d *Database) DocCount() uint32 { return d.info.docCount }

// LastDocID is the highest document id ever used.
func (d *Database) LastDocID() uint32 { return d.info.lastDocID }

// TotalLength is the sum of all document lengths.
func (d *Database) TotalLength() uint64 { return d.info.totalLength }

// AverageLength is TotalLength/DocCount (0 for an empty database).
func (d *Database) AverageLength() float64 { return d.avgLen }

// Metadata returns user metadata (libzim stores "language", "kind", "data",
// "valuesmap" and, when non-empty, "stopwords"). Missing keys yield "".
func (d *Database) Metadata(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	tag, ok, err := d.postlist.get(metadataKey(name))
	if err != nil || !ok {
		return "", err
	}
	return string(tag), nil
}

// termStats returns the term frequency and collection frequency of term.
func (d *Database) termStats(term string) (tf uint32, cf uint64, err error) {
	if term == "" {
		return 0, 0, nil
	}
	tag, ok, err := d.postlist.get(postlistKey(term))
	if err != nil || !ok {
		return 0, 0, err
	}
	tf, n, err := unpackUint32(tag)
	if err != nil {
		return 0, 0, err
	}
	cf, _, err = unpackUint(tag[n:])
	return tf, cf, err
}

// TermFreq is the number of documents indexed by term (0 if absent).
func (d *Database) TermFreq(term string) (uint32, error) {
	tf, _, err := d.termStats(term)
	return tf, err
}

// Data returns the document data ("C/<path>" in libzim indexes). Documents
// without stored data yield "".
func (d *Database) Data(did uint32) (string, error) {
	if did == 0 || did > d.info.lastDocID {
		return "", ErrDocNotFound
	}
	tag, ok, err := d.docdata.get(docdataKey(did))
	if err != nil || !ok {
		return "", err
	}
	return string(tag), nil
}

// TermsWithPrefix lists terms starting with prefix in byte order; limit <= 0
// means no limit.
func (d *Database) TermsWithPrefix(prefix string, limit int) ([]string, error) {
	var out []string
	err := d.walkTerms(prefix, func(term string, _ *cursor) (bool, error) {
		out = append(out, term)
		return limit <= 0 || len(out) < limit, nil
	})
	return out, err
}

// walkTerms calls fn for each term with the given prefix, positioned on the
// term's first posting chunk. fn returns false to stop.
func (d *Database) walkTerms(prefix string, fn func(term string, c *cursor) (bool, error)) error {
	start := keyFirstTerm
	if prefix != "" {
		start = postlistKey(prefix)
	}
	c := d.postlist.cursor()
	ok, err := c.seekGE(start)
	for ; ok && err == nil; ok, err = c.next() {
		key, kerr := c.key()
		if kerr != nil {
			return kerr
		}
		if len(key) >= 2 && key[0] == 0 && key[1] != 0xff {
			continue // special key; cannot follow keyFirstTerm, but be defensive
		}
		term, _, terminated := unpackSortPreservingString(key)
		if terminated {
			continue // later chunk of a posting list
		}
		if !strings.HasPrefix(term, prefix) {
			return nil
		}
		more, ferr := fn(term, c)
		if ferr != nil || !more {
			return ferr
		}
	}
	return err
}

// stripNamespace turns libzim document data "C/Berlin" into "Berlin".
func stripNamespace(data string) string {
	if len(data) > 2 && data[1] == '/' {
		return data[2:]
	}
	return data
}
