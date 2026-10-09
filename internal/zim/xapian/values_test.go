package xapian

import "testing"

func TestDatabaseGoldenValues(t *testing.T) {
	for _, name := range fixtureNames {
		for _, kind := range []string{"fulltext", "title"} {
			g := loadGoldenDB(t, name, kind)
			db := openFixture(t, name, kind)
			for slot, rows := range g.Values {
				s := uint32(slot[0] - '0')
				for did, want := range pairs(t, rows) {
					got, err := db.Value(did, s)
					if err != nil || got != want {
						t.Errorf("%s/%s Value(%d,%d) = %q, %v; want %q", name, kind, did, s, shortPath(got), err, shortPath(want))
					}
				}
			}
			// A forward reader returns the same values as random lookups.
			r := db.newValueReader(0)
			for did := uint32(1); did <= db.LastDocID(); did++ {
				fwd, err := r.get(did)
				if err != nil {
					t.Fatal(err)
				}
				if rnd, _ := db.Value(did, 0); fwd != rnd {
					t.Fatalf("%s/%s slot 0 doc %d: forward %q, random %q", name, kind, did, shortPath(fwd), shortPath(rnd))
				}
			}
		}
	}
}
