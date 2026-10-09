package xapian

import (
	"errors"
	"testing"
)

func TestDatabaseGoldenPostings(t *testing.T) {
	for _, name := range fixtureNames {
		for _, kind := range []string{"fulltext", "title"} {
			g := loadGoldenDB(t, name, kind)
			db := openFixture(t, name, kind)
			for _, gt := range g.Terms {
				tf, cf, err := db.termStats(gt.Term)
				if err != nil || tf != gt.TF || cf != gt.CF {
					t.Errorf("%s/%s %q stats = (%d,%d,%v) want (%d,%d)", name, kind, gt.Term, tf, cf, err, gt.TF, gt.CF)
				}
				it, err := db.Postings(gt.Term)
				if err != nil {
					t.Fatalf("%s/%s Postings(%q): %v", name, kind, gt.Term, err)
				}
				var got [][2]uint32
				for it.Next() {
					got = append(got, [2]uint32{it.DocID(), it.WDF()})
				}
				if it.Err() != nil {
					t.Fatalf("%s/%s %q: %v", name, kind, gt.Term, it.Err())
				}
				if len(got) != len(gt.Postings) {
					t.Errorf("%s/%s %q: %d postings want %d", name, kind, gt.Term, len(got), len(gt.Postings))
					continue
				}
				for i := range got {
					if got[i] != gt.Postings[i] {
						t.Errorf("%s/%s %q posting %d = %v want %v", name, kind, gt.Term, i, got[i], gt.Postings[i])
						break
					}
				}
			}
			if g.TermsComplete {
				all, err := db.TermsWithPrefix("", 0)
				if err != nil || len(all) != g.TermCount {
					t.Errorf("%s/%s TermsWithPrefix(\"\") = %d terms, %v; want %d", name, kind, len(all), err, g.TermCount)
				}
			}
		}
	}
}

func TestDatabaseGoldenDocLengths(t *testing.T) {
	for _, name := range fixtureNames {
		for _, kind := range []string{"fulltext", "title"} {
			g := loadGoldenDB(t, name, kind)
			db := openFixture(t, name, kind)
			for _, dl := range g.DocLens {
				got, err := db.DocLength(dl[0])
				if err != nil || got != dl[1] {
					t.Errorf("%s/%s DocLength(%d) = %d, %v; want %d", name, kind, dl[0], got, err, dl[1])
				}
			}
			if _, err := db.DocLength(db.LastDocID() + 1); !errors.Is(err, ErrDocNotFound) {
				t.Errorf("%s/%s DocLength past end: %v", name, kind, err)
			}
		}
	}
}
