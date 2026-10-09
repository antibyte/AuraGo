package xapian

import (
	"context"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// titleSyllables build the synthetic title vocabulary: every two- and
// three-syllable combination, 65,600 words.
var titleSyllables = []string{
	"ka", "ro", "lin", "te", "sa", "mo", "ri", "an", "del", "bur", "gen", "sto", "ver", "ham",
	"ton", "ma", "ne", "li", "so", "ter", "ber", "ge", "stan", "mi", "re", "al", "en", "or",
	"us", "is", "pa", "di", "ko", "va", "le", "na", "sch", "ul", "fa", "hel",
}

// titleWord is the vocabulary word of rank r (0 = most frequent).
func titleWord(r int) string {
	n := len(titleSyllables)
	if r < n*n {
		return titleSyllables[r/n] + titleSyllables[r%n]
	}
	r -= n * n
	return titleSyllables[r/(n*n)] + titleSyllables[r/n%n] + titleSyllables[r%n]
}

// synthTitles builds a libzim-like English title index over n generated
// titles of one to four words drawn with Zipf frequencies. Each title is
// indexed as "0posanchor", its words and, for words that start with a
// letter, "Z" + English stem; value slot 0 holds the title, slot 1 the target
// path (every seventh document redirects to an earlier one, so suggestions
// collapse) and the document data is "C/" + the document's own path.
func synthTitles(tb testing.TB, n int) *Database {
	tb.Helper()
	vocab := len(titleSyllables) * len(titleSyllables) * (1 + len(titleSyllables))
	words := make([]string, vocab)
	stems := make([]string, vocab)
	en := NewAnalyzer("eng")
	for r := range words {
		words[r] = titleWord(r)
		stems[r] = "Z" + en.Stem(words[r])
	}
	rng := uint64(7)
	next := func() uint64 {
		rng = rng*6364136223846793005 + 1442695040888963407
		return rng >> 11
	}
	lens := make([]uint32, n)
	terms := map[string][][2]uint32{}
	titles := make([]string, n)
	targets := make([]string, n)
	data := make([]string, n)
	paths := make([]string, n)
	for i := range lens {
		did := uint32(i + 1)
		k := []int{1, 2, 2, 3, 3, 2, 4, 3, 1, 2}[next()%10]
		wdf := map[string]uint32{anchorTerm: 1}
		var sb strings.Builder
		for j := 0; j < k; j++ {
			u := float64(next()%1_000_000) / 1_000_000
			r := int(math.Pow(float64(vocab), u)) - 1
			if j > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(strings.ToUpper(words[r][:1]) + words[r][1:])
			wdf[words[r]]++
			wdf[stems[r]]++
		}
		titles[i] = sb.String()
		paths[i] = strings.ReplaceAll(titles[i], " ", "_") + "_" + strconv.Itoa(i+1)
		data[i] = "C/" + paths[i]
		targets[i] = paths[i]
		if did%7 == 0 {
			targets[i] = paths[next()%uint64(i)]
		}
		for term, f := range wdf {
			terms[term] = append(terms[term], [2]uint32{did, f})
			lens[i] += f
		}
	}
	return synthIndex{
		blockSize: 8192, docLens: lens, terms: terms, data: data,
		values: map[uint32][]string{0: titles, 1: targets},
	}.build(tb)
}

// suggestScaleDocs titles make up BenchmarkSuggestScale's index.
const suggestScaleDocs = 1_000_000

var suggestScale struct {
	sync.Once
	db *Database
}

// BenchmarkSuggestScale runs title suggestions against a synthetic
// 1,000,000-title index: one- and two-letter prefixes that expand to the 100
// most frequent of thousands of terms, a full word, two words and a phrase.
func BenchmarkSuggestScale(b *testing.B) {
	suggestScale.Do(func() { suggestScale.db = synthTitles(b, suggestScaleDocs) })
	db := suggestScale.db
	a := NewAnalyzer("eng")
	ctx := context.Background()
	for _, c := range []struct{ name, query string }{
		{"prefix_s", "s"},
		{"prefix_ka", "ka"},
		{"word_karo", "karo"},
		{"two_words", "karo li"},
		{"phrase", "karo-kalin"},
	} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Suggest(ctx, db, a, c.query, 10); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
