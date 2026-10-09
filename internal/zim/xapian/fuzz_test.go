package xapian

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

func FuzzUnpack(f *testing.F) {
	f.Add([]byte{0xac, 0x02})
	f.Add([]byte{0xc0, 0x40, 0x00, 0x00})
	f.Add([]byte("ab\x00\xffc\x00\x01"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if v, n, err := unpackUint(b); err == nil {
			if n < 1 || n > len(b) {
				t.Fatalf("unpackUint consumed %d of %d", n, len(b))
			}
			if back, _, err := unpackUint(appendUint(nil, v)); err != nil || back != v {
				t.Fatalf("varint round trip %d -> %d, %v", v, back, err)
			}
		}
		if v, n, err := unpackSortableUint(b); err == nil {
			if n < 2 || n > len(b) {
				t.Fatalf("unpackSortableUint consumed %d of %d", n, len(b))
			}
			if back, _, err := unpackSortableUint(appendSortableUint(nil, v)); err != nil || back != v {
				t.Fatalf("sortable round trip %d -> %d, %v", v, back, err)
			}
		}
		if _, n, _ := unpackSortPreservingString(b); n > len(b) {
			t.Fatalf("unpackSortPreservingString consumed %d of %d", n, len(b))
		}
	})
}

func FuzzParseVersion(f *testing.F) {
	f.Add(versionBlock(glassMagic, glassFormatVersion, liveRoots, [8]uint64{4, 0, 12, 4, 12, 0, 56, 0})[:200])
	f.Add([]byte(glassMagic))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = parseVersion(b)
	})
}

// FuzzDatabase mutates real libzim-built indexes and drives every read path.
// Errors must belong to the package's classes (ErrCorrupt, ErrDocNotFound,
// ErrUnsupportedFormat); panics, hangs and unbounded allocations are bugs.
func FuzzDatabase(f *testing.F) {
	for _, src := range [][2]string{{"de", "fulltext"}, {"de", "title"}, {"ja", "title"}} {
		_, sr := openFixtureBlob(f, src[0], src[1])
		blob, err := io.ReadAll(io.NewSectionReader(sr, 0, sr.Size()))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(blob)
	}
	f.Fuzz(func(t *testing.T, blob []byte) {
		check := func(what string, err error) {
			t.Helper()
			if err != nil && !errors.Is(err, ErrCorrupt) && !errors.Is(err, ErrDocNotFound) && !errors.Is(err, ErrUnsupportedFormat) {
				t.Fatalf("%s: unclassified error %v", what, err)
			}
		}
		db, err := Open(bytes.NewReader(blob), int64(len(blob)))
		if err != nil {
			check("Open", err)
			return
		}
		ctx := context.Background()
		terms, err := db.TermsWithPrefix("", 40)
		check("TermsWithPrefix", err)
		for _, term := range terms {
			it, err := db.Postings(term)
			check("Postings", err)
			if err == nil {
				for i := 0; i < 2000 && it.Next(); i++ {
					_ = it.DocID()
				}
				check("Postings.Next", it.Err())
			}
		}
		for did := uint32(1); did <= 3; did++ {
			_, err := db.DocLength(did)
			check("DocLength", err)
			_, err = db.Data(did)
			check("Data", err)
			_, err = db.Value(did, 0)
			check("Value 0", err)
			_, err = db.Value(did, 1)
			check("Value 1", err)
		}
		_, err = db.Metadata("language")
		check("Metadata", err)
		_, _, err = Search(ctx, db, terms, OpOr, 0, 5)
		check("Search OR", err)
		if len(terms) > 1 {
			_, _, err = Search(ctx, db, terms[:2], OpAnd, 0, 5)
			check("Search AND", err)
		}
		de, ja := NewAnalyzer("deu"), NewAnalyzer("jpn")
		for _, q := range []struct {
			a     Analyzer
			query string
		}{
			{de, "b"},            // partial word
			{de, "berlin"},       // single word: phrase and anchor checks
			{de, "berlin mitte"}, // phrase and anchored phrase
			{de, "spree-ufer b"}, // AND-part phrase plus partial word
			{de, "!"},            // no word characters: wildcard only
			{de, "-"},            // no word characters: wildcard only
			{ja, "東京"},           // CJK n-grams
		} {
			_, err := Suggest(ctx, db, q.a, q.query, 5)
			check("Suggest "+q.query, err)
		}
	})
}
