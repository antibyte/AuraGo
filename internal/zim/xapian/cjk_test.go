package xapian

import (
	"context"
	"testing"
)

// TestCJKFulltextDecision is the spec's ja/zh decision test. libzim indexes
// Japanese and Chinese with Xapian 1.4's FLAG_CJK_NGRAM (unigrams + bigrams
// of CJK code points, no stemmer) after its accent removal, which also strips
// kana voicing marks (ガ -> カ). The Go analyzer reproduces this; the ja
// fixture goldens (tokenisation, query terms, search and suggestion order)
// prove it. If this test or any ja golden test fails and cannot be fixed by
// following the plan's Format notes, set cjkFulltext to false in analyzer.go
// (ja and zh then degrade to title search) and record the reason in doc.go.
func TestCJKFulltextDecision(t *testing.T) {
	for _, lang := range []string{"jpn", "zho", "kor", "ja", "zh", "ko"} {
		if got := NewAnalyzer(lang).FulltextSupported(); got != cjkFulltext {
			t.Fatalf("%s: FulltextSupported() = %v, cjkFulltext = %v", lang, got, cjkFulltext)
		}
	}
	if !cjkFulltext {
		t.Skip("CJK full-text disabled (cjkFulltext); the parity checks below justify enabling it")
	}
	a := NewAnalyzer("jpn")
	if got := a.QueryTerms("東京タワー"); len(got) != 9 || got[0] != "東" || got[1] != "東京" {
		t.Fatalf("QueryTerms(東京タワー) = %q", got)
	}
	db := openFixture(t, "ja", "fulltext")
	voiced, _, err := Search(context.Background(), db, a.QueryTerms("ガンダム"), OpAnd, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	unvoiced, _, err := Search(context.Background(), db, a.QueryTerms("カンタム"), OpAnd, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(voiced) != 1 || voiced[0].Path != "ガンダム" || len(unvoiced) != 1 || unvoiced[0].DocID != voiced[0].DocID {
		t.Fatalf("voicing marks not folded: %+v / %+v", voiced, unvoiced)
	}
	title := openFixture(t, "ja", "title")
	hits, err := Suggest(context.Background(), title, a, "東京", 3)
	if err != nil || len(hits) == 0 || hits[0].Path != "東京" {
		t.Fatalf("Suggest(東京) = %+v, %v", hits, err)
	}
}
