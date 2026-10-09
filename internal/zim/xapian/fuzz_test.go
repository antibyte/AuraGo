package xapian

import (
	"bytes"
	"context"
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
// Any error is fine; panics, hangs and unbounded allocations are not.
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
		db, err := Open(bytes.NewReader(blob), int64(len(blob)))
		if err != nil {
			return
		}
		ctx := context.Background()
		terms, _ := db.TermsWithPrefix("", 40)
		for _, term := range terms {
			if it, err := db.Postings(term); err == nil {
				for i := 0; i < 2000 && it.Next(); i++ {
					_ = it.DocID()
				}
			}
		}
		for did := uint32(1); did <= 3; did++ {
			_, _ = db.DocLength(did)
			_, _ = db.Data(did)
			_, _ = db.Value(did, 0)
			_, _ = db.Value(did, 1)
		}
		_, _ = db.Metadata("language")
		_, _, _ = Search(ctx, db, terms, OpOr, 0, 5)
		if len(terms) > 1 {
			_, _, _ = Search(ctx, db, terms[:2], OpAnd, 0, 5)
		}
		_, _ = Suggest(ctx, db, NewAnalyzer("deu"), "b", 5)
		_, _ = Suggest(ctx, db, NewAnalyzer("jpn"), "東京", 5)
	})
}
