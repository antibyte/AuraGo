package xapian

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/zim"
)

// fixtureNames are the generated ZIMs in internal/zim/testdata.
var fixtureNames = []string{"de", "en", "pl", "ja", "bulk"}

// fixtureLanguage is the indexing language each fixture was built with.
var fixtureLanguage = map[string]string{"de": "deu", "en": "eng", "pl": "pol", "ja": "jpn", "bulk": "eng"}

// openFixture opens X/<kind>/xapian ("fulltext" or "title") of fixture_<name>.zim.
func openFixture(t testing.TB, name, kind string) *Database {
	t.Helper()
	db, _ := openFixtureBlob(t, name, kind)
	return db
}

// openFixtureBlob also returns the raw database section for fuzz seeding.
func openFixtureBlob(t testing.TB, name, kind string) (*Database, *io.SectionReader) {
	t.Helper()
	a, err := zim.Open(filepath.Join("..", "testdata", "fixture_"+name+".zim"), zim.Options{})
	if err != nil {
		t.Fatalf("open fixture %s: %v (regenerate with scripts/localwiki/fixtures/generate.sh)", name, err)
	}
	t.Cleanup(func() { a.Close() })
	e, err := a.EntryByPath('X', kind+"/xapian")
	if err != nil {
		t.Fatalf("fixture %s has no X/%s/xapian: %v", name, kind, err)
	}
	sr, err := a.Open(e)
	if err != nil {
		t.Fatalf("open X/%s/xapian blob: %v", kind, err)
	}
	db, err := Open(sr, sr.Size())
	if err != nil {
		t.Fatalf("xapian.Open %s/%s: %v", name, kind, err)
	}
	return db, sr
}

func loadJSON(t testing.TB, name string, v any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden %s: %v (regenerate with scripts/localwiki/fixtures/generate.sh)", name, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("parse golden %s: %v", name, err)
	}
}

// goldenDB mirrors db_<name>_<kind>.json written by make_goldens.py.
type goldenDB struct {
	Fixture       string            `json:"fixture"`
	Kind          string            `json:"kind"`
	Delve         string            `json:"delve"`
	DocCount      uint32            `json:"doccount"`
	LastDocID     uint32            `json:"lastdocid"`
	TotalLength   uint64            `json:"total_length"`
	AvLength      float64           `json:"avlength"`
	Metadata      map[string]string `json:"metadata"`
	TermCount     int               `json:"term_count"`
	TermsComplete bool              `json:"terms_complete"`
	Terms         []struct {
		Term     string      `json:"term"`
		TF       uint32      `json:"tf"`
		CF       uint64      `json:"cf"`
		Postings [][2]uint32 `json:"postings"`
	} `json:"terms"`
	DocLens [][2]uint32                     `json:"doclens"`
	Data    [][2]json.RawMessage            `json:"data"`
	Values  map[string][][2]json.RawMessage `json:"values"`
}

func loadGoldenDB(t testing.TB, name, kind string) goldenDB {
	var g goldenDB
	loadJSON(t, "db_"+name+"_"+kind+".json", &g)
	return g
}

// pairs decodes [[docid, "string"], ...] rows.
func pairs(t testing.TB, rows [][2]json.RawMessage) map[uint32]string {
	t.Helper()
	out := make(map[uint32]string, len(rows))
	for _, r := range rows {
		var did uint32
		var s string
		if err := json.Unmarshal(r[0], &did); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(r[1], &s); err != nil {
			t.Fatal(err)
		}
		out[did] = s
	}
	return out
}

// goldenQueries mirrors queries_<name>.json.
type goldenQueries struct {
	Fixture  string `json:"fixture"`
	Language string `json:"language"`
	Fulltext []struct {
		Query  string   `json:"query"`
		Terms  []string `json:"terms"`
		Libzim struct {
			Estimated int      `json:"estimated"`
			Paths     []string `json:"paths"`
		} `json:"libzim"`
		Estimated int                  `json:"estimated"`
		And       [][3]json.RawMessage `json:"and"`
		Or        [][3]json.RawMessage `json:"or"`
	} `json:"fulltext"`
	Suggest []struct {
		Query       string `json:"query"`
		XapianQuery string `json:"xapian_query"`
		Libzim      struct {
			Paths []string `json:"paths"`
		} `json:"libzim"`
		Replica [][4]json.RawMessage `json:"replica"`
	} `json:"suggest"`
}

type weightedHit struct {
	DocID  uint32
	Weight float64
	Data   string
}

func weighted(t testing.TB, rows [][3]json.RawMessage) []weightedHit {
	t.Helper()
	out := make([]weightedHit, len(rows))
	for i, r := range rows {
		if err := json.Unmarshal(r[0], &out[i].DocID); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(r[1], &out[i].Weight); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(r[2], &out[i].Data); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func weighted4(t testing.TB, rows [][4]json.RawMessage) []weightedHit {
	t.Helper()
	three := make([][3]json.RawMessage, len(rows))
	for i, r := range rows {
		three[i] = [3]json.RawMessage{r[0], r[1], r[2]}
	}
	return weighted(t, three)
}

// goldenAnalysis mirrors analysis.json.
type goldenAnalysis struct {
	Normalize [][2]string `json:"normalize"`
	Tokenize  []struct {
		Text  string `json:"text"`
		Terms map[string]struct {
			WDF       int      `json:"wdf"`
			Positions []uint32 `json:"positions"`
		} `json:"terms"`
	} `json:"tokenize"`
	QueryParse []struct {
		Language string   `json:"language"`
		Query    string   `json:"query"`
		Terms    []string `json:"terms"`
	} `json:"queryparse"`
	Stems      map[string][][2]string `json:"stems"`
	IndexWords map[string][]string    `json:"index_words"`
}

func closeEnough(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	m := b
	if m < 0 {
		m = -m
	}
	return d <= 1e-9*(1+m)
}

func shortPath(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return strings.TrimSpace(s)
}
