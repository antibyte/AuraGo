package xapian

import (
	"errors"
	"testing"
)

func TestDatabaseGoldenStats(t *testing.T) {
	for _, name := range fixtureNames {
		for _, kind := range []string{"fulltext", "title"} {
			g := loadGoldenDB(t, name, kind)
			db := openFixture(t, name, kind)
			if db.DocCount() != g.DocCount || db.LastDocID() != g.LastDocID || db.TotalLength() != g.TotalLength {
				t.Errorf("%s/%s stats: got (%d,%d,%d) want (%d,%d,%d)", name, kind,
					db.DocCount(), db.LastDocID(), db.TotalLength(), g.DocCount, g.LastDocID, g.TotalLength)
			}
			if !closeEnough(db.AverageLength(), g.AvLength) {
				t.Errorf("%s/%s avlength %v want %v", name, kind, db.AverageLength(), g.AvLength)
			}
			for k, want := range g.Metadata {
				got, err := db.Metadata(k)
				if err != nil || got != want {
					t.Errorf("%s/%s metadata %q = %q, %v; want %q", name, kind, k, got, err, want)
				}
			}
			if got, _ := db.Metadata("no-such-key"); got != "" {
				t.Errorf("%s/%s missing metadata = %q", name, kind, got)
			}
		}
	}
}

func TestDatabaseGoldenData(t *testing.T) {
	for _, name := range fixtureNames {
		for _, kind := range []string{"fulltext", "title"} {
			g := loadGoldenDB(t, name, kind)
			db := openFixture(t, name, kind)
			for did, want := range pairs(t, g.Data) {
				got, err := db.Data(did)
				if err != nil || got != want {
					t.Errorf("%s/%s Data(%d) = %q, %v; want %q", name, kind, did, shortPath(got), err, shortPath(want))
				}
			}
			if _, err := db.Data(db.LastDocID() + 1); !errors.Is(err, ErrDocNotFound) {
				t.Errorf("%s/%s Data past end: %v", name, kind, err)
			}
		}
	}
}

func TestTermsWithPrefix(t *testing.T) {
	db := openFixture(t, "de", "title")
	got, err := db.TermsWithPrefix("ber", 0)
	if err != nil || len(got) != 1 || got[0] != "berlin" {
		t.Fatalf("TermsWithPrefix(ber) = %v, %v", got, err)
	}
	got, err = db.TermsWithPrefix("Z", 3)
	if err != nil || len(got) != 3 {
		t.Fatalf("TermsWithPrefix(Z,3) = %v, %v", got, err)
	}
	for _, term := range got {
		if term[0] != 'Z' {
			t.Fatalf("unexpected term %q", term)
		}
	}
	bulk := openFixture(t, "bulk", "fulltext")
	got, err = bulk.TermsWithPrefix("05", 0)
	if err != nil || len(got) != 100 || got[0] != "0500" || got[99] != "0599" {
		t.Fatalf("bulk TermsWithPrefix(05) = %d terms (%v...), %v", len(got), got[:2], err)
	}
}
