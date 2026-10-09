package xapian

import (
	"context"
	"sync"
	"testing"
)

// BenchmarkSearchBulk ranks the 1,101-document bulk fixture (every list spans
// two posting chunks): a term in every document, a two-term AND over 1,100
// documents, a three-term AND whose rarest term holds 157 documents and the
// OR of the same three terms.
func BenchmarkSearchBulk(b *testing.B) {
	db := openFixture(b, "bulk", "fulltext")
	a := NewAnalyzer("eng")
	ctx := context.Background()
	for _, c := range []struct {
		name, query string
		op          Op
	}{
		{"and/zebra", "zebra", OpAnd},
		{"and/bulk_item", "bulk item", OpAnd},
		{"and/group_g3_zebra", "group g3 zebra", OpAnd},
		{"or/group_g3_zebra", "group g3 zebra", OpOr},
	} {
		terms := a.QueryTerms(c.query)
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := Search(ctx, db, terms, c.op, 0, 25); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// scaleDocs is the article count of the German Wikipedia edition the
// full-text search budget (warm p95 < 500 ms) is set for.
const scaleDocs = 2_900_000

var scaleOnce struct {
	sync.Once
	db *Database
}

// scaleDB is a synthetic glass index with scaleDocs documents (lengths 50 to
// 1,549) and four terms: "die" in about 90 % of the documents, "und" in 85 %,
// "berlin" in 4 % and "spree" in 0.05 %. It lives in memory, so it measures
// decoding and ranking, not file reads.
func scaleDB(b *testing.B) *Database {
	scaleOnce.Do(func() {
		rng := uint64(1)
		next := func() uint32 {
			rng = rng*6364136223846793005 + 1442695040888963407
			return uint32(rng >> 33)
		}
		lens := make([]uint32, scaleDocs)
		terms := map[string][][2]uint32{}
		for i := range lens {
			did := uint32(i + 1)
			lens[i] = 50 + next()%1500
			for _, t := range []struct {
				term     string
				perMille uint32
				maxWdf   uint32
			}{{"die", 900, 40}, {"und", 850, 30}, {"berlin", 40, 5}, {"spree", 1, 3}} {
				if next()%1000 < t.perMille {
					terms[t.term] = append(terms[t.term], [2]uint32{did, 1 + next()%t.maxWdf})
				}
			}
		}
		scaleOnce.db = synthDB(b, 8192, lens, terms)
	})
	return scaleOnce.db
}

// BenchmarkSearchScale ranks queries against scaleDB, from a single rare term
// to an AND of the two most common terms.
func BenchmarkSearchScale(b *testing.B) {
	db := scaleDB(b)
	ctx := context.Background()
	for _, c := range []struct {
		name  string
		terms []string
		op    Op
	}{
		{"and/spree_berlin", []string{"spree", "berlin"}, OpAnd},
		{"and/berlin", []string{"berlin"}, OpAnd},
		{"and/berlin_die", []string{"berlin", "die"}, OpAnd},
		{"and/die", []string{"die"}, OpAnd},
		{"and/die_und", []string{"die", "und"}, OpAnd},
		{"or/berlin_spree", []string{"berlin", "spree"}, OpOr},
		{"or/die_und", []string{"die", "und"}, OpOr},
	} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := Search(ctx, db, c.terms, c.op, 0, 25); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
