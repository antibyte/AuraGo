package xapian

import (
	"reflect"
	"testing"
)

func TestIndexTokensGolden(t *testing.T) {
	var g goldenAnalysis
	loadJSON(t, "analysis.json", &g)
	for _, c := range g.Tokenize {
		type stat struct {
			wdf int
			pos []uint32
		}
		got := map[string]*stat{}
		var pos uint32
		for _, tok := range indexTokens(c.Text) {
			if len(tok.text) > 64 { // TermGenerator default max_word_length
				continue
			}
			s := got[tok.text]
			if s == nil {
				s = &stat{}
				got[tok.text] = s
			}
			s.wdf++
			if tok.positional {
				pos++
				s.pos = append(s.pos, pos)
			}
		}
		if len(got) != len(c.Terms) {
			t.Errorf("indexTokens(%q): %d distinct terms, want %d (%v)", c.Text, len(got), len(c.Terms), got)
			continue
		}
		for term, want := range c.Terms {
			s := got[term]
			if s == nil || s.wdf != want.WDF || !reflect.DeepEqual(append([]uint32{}, s.pos...), append([]uint32{}, want.Positions...)) {
				t.Errorf("indexTokens(%q) term %q = %+v, want wdf %d positions %v", c.Text, term, s, want.WDF, want.Positions)
			}
		}
	}
}

func TestQueryWords(t *testing.T) {
	cases := []struct {
		in   string
		want []queryWord
	}{
		{"berlin", []queryWord{{text: "berlin", atEnd: true, stemOK: true}}},
		{"spree-athen x", []queryWord{{text: "spree", stemOK: true}, {text: "athen", phrased: true, stemOK: true}, {text: "x", atEnd: true, stemOK: true}}},
		{"spree-", []queryWord{{text: "spree", stemOK: true}}},
		{"c++", []queryWord{{text: "c++", atEnd: true, stemOK: true}}},
		{"c+ ", []queryWord{{text: "c+", stemOK: true}}},
		{"at&", []queryWord{{text: "at", stemOK: true}}},
		{"o'brien", []queryWord{{text: "o'brien", atEnd: true, stemOK: true}}},
		{"word(", []queryWord{{text: "word"}}},
		{"東京タワー x", []queryWord{{text: "東京タワー", cjk: true, stemOK: true}, {text: "x", atEnd: true, stemOK: true}}},
		{"東京", []queryWord{{text: "東京", cjk: true, atEnd: true, stemOK: true}}},
	}
	for _, c := range cases {
		if got := queryWords(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("queryWords(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}
