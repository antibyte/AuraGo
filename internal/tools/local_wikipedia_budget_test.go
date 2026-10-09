package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/localwiki"
)

// Sample prose in multi-byte scripts (3 bytes per character in UTF-8), with
// quotes and an ampersand, which the Guardian escapes once more.
const (
	hindiSentence = `भारत (आधिकारिक नाम: "भारत गणराज्य") दक्षिण एशिया में स्थित भारतीय उपमहाद्वीप का सबसे बड़ा देश है & इसकी राजधानी नई दिल्ली है। `
	cjkSentence   = `中华人民共和国是位于东亚的社会主义国家，首都为"北京"，人口约十四亿。`
)

func repeatRunes(sentence string, runes int) string {
	var b strings.Builder
	for utf8.RuneCountInString(b.String()) < runes {
		b.WriteString(sentence)
	}
	return string([]rune(b.String())[:runes])
}

// pagedWikiLibrary pages one article as the library does: PageRunes runes per
// page (8,000 when unset or larger), offsets and next_offset in runes.
type pagedWikiLibrary struct {
	fakeWikiLibrary
	title    string
	text     string
	sections []localwiki.Section
	pages    []int
}

func (p *pagedWikiLibrary) Read(_ context.Context, req localwiki.ReadRequest) (localwiki.Article, error) {
	size := req.PageRunes
	if size <= 0 || size > localWikipediaPageRunes {
		size = localWikipediaPageRunes
	}
	p.pages = append(p.pages, size)
	runes := []rune(p.text)
	if req.Offset > len(runes) {
		return localwiki.Article{}, localwiki.ErrOffsetOutOfRange
	}
	end := min(req.Offset+size, len(runes))
	var next *int
	if end < len(runes) {
		n := end
		next = &n
	}
	return localwiki.Article{
		Ref:        localwiki.Ref{Title: p.title, Path: p.title},
		Sections:   p.sections,
		Content:    string(runes[req.Offset:end]),
		NextOffset: next,
	}, nil
}

func scriptSections(heading string, n int) []localwiki.Section {
	sections := make([]localwiki.Section, n)
	for i := range sections {
		sections[i] = localwiki.Section{Index: i, Heading: fmt.Sprintf("%s %d", heading, i), Level: 2, Chars: 1200}
	}
	sections[0].Heading, sections[0].Level = "", 0
	return sections
}

func scriptSearchResult(sentence string, hits int) localwiki.SearchResult {
	result := localwiki.SearchResult{Edition: localwiki.Edition{Language: "hi", Variant: localwiki.VariantNoPic, Date: "2026-10"}, Fulltext: true}
	for i := range hits {
		hit := localwiki.SearchHit{
			Ref:     localwiki.Ref{Title: fmt.Sprintf("भारत %d", i), Path: fmt.Sprintf("भारत_%d", i)},
			Snippet: repeatRunes(sentence, 200),
		}
		if i < localWikipediaLeads {
			hit.Lead = repeatRunes(sentence, 2000)
		}
		result.Results = append(result.Results, hit)
	}
	return result
}

// The acceptance case: a Hindi search with three 2,000-character leads was
// 11,905 bytes and got archived. Now the answer fits the default budget as
// the agent measures it, leads shrink first and every result stays.
func TestLocalWikipediaSearchFitsTheInlineBudget(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sentence string
		limit    int
		budget   int
	}{
		{"hindi default", hindiSentence, 5, 0},
		{"hindi ten results", hindiSentence, 10, 0},
		{"cjk default", cjkSentence, 5, 0},
		{"cjk smallest budget", cjkSentence, 10, 1},
		{"hindi custom budget", hindiSentence, 5, 9000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lib := &fakeWikiLibrary{result: scriptSearchResult(tc.sentence, tc.limit)}
			useWikiSource(t, &fakeWikiSource{lib: lib, open: true})
			out := ExecuteLocalWikipedia(context.Background(), wikiConfig(), LocalWikipediaRequest{Operation: "search", Query: "भारत", Limit: tc.limit, OutputBudget: tc.budget})
			budget := localWikipediaBudget(tc.budget)
			if got := localWikipediaModelBytes(out); got > budget {
				t.Fatalf("answer is %d model bytes, budget %d", got, budget)
			}
			m := decodeWikiOutput(t, out)
			results := m["results"].([]any)
			if len(results) == 0 {
				t.Fatalf("no results left: %v", m)
			}
			first := results[0].(map[string]any)
			if tc.budget == 0 && len(results) != tc.limit {
				t.Fatalf("results = %d, want all %d at the default budget", len(results), tc.limit)
			}
			if tc.limit == 5 {
				lead, _ := first["lead"].(string)
				if !strings.Contains(lead, "…") {
					t.Fatalf("first lead must stay, shortened: %q", lead)
				}
			}
			if !strings.Contains(first["snippet"].(string), "भारत") && !strings.Contains(first["snippet"].(string), "中华") {
				t.Fatalf("snippet lost: %v", first)
			}
		})
	}
}

// Leads keep their full length when the budget allows it.
func TestLocalWikipediaSearchKeepsFullLeadsWithinALargeBudget(t *testing.T) {
	lib := &fakeWikiLibrary{result: scriptSearchResult(hindiSentence, 5)}
	useWikiSource(t, &fakeWikiSource{lib: lib, open: true})
	m := decodeWikiOutput(t, ExecuteLocalWikipedia(context.Background(), wikiConfig(), LocalWikipediaRequest{Operation: "search", Query: "भारत", OutputBudget: 1 << 20}))
	results := m["results"].([]any)
	for i, r := range results[:localWikipediaLeads] {
		lead := r.(map[string]any)["lead"].(string)
		payload := strings.TrimSuffix(strings.TrimPrefix(lead, "<external_data>\n"), "\n</external_data>")
		if strings.Contains(payload, "…") || utf8.RuneCountInString(payload) < 2000 {
			t.Fatalf("lead %d shortened within a large budget: %d runes", i, utf8.RuneCountInString(payload))
		}
	}
}

// The acceptance case for read: an 8,000-character Hindi page was 23,039
// bytes. Now every page fits, the section list shrinks to a third of the
// budget, and next_offset chains the pages without losing or repeating text.
func TestLocalWikipediaReadFitsTheInlineBudget(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sentence string
		heading  string
		sections int
		budget   int
	}{
		{"hindi default", hindiSentence, "इतिहास और संस्कृति", 45, 0},
		{"cjk default", cjkSentence, "历史与文化", 120, 0},
		{"hindi smallest budget", hindiSentence, "इतिहास", 30, 1},
		{"latin larger budget", "Berlin ist die Hauptstadt und ein Land der Bundesrepublik Deutschland. ", "Geschichte", 20, 20000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := repeatRunes(tc.sentence, 30_000)
			lib := &pagedWikiLibrary{title: "भारत", text: text, sections: scriptSections(tc.heading, tc.sections)}
			useWikiSource(t, pagedWikiSource{lib})
			budget := localWikipediaBudget(tc.budget)
			var content strings.Builder
			offset, pages := 0, 0
			for {
				out := ExecuteLocalWikipedia(context.Background(), wikiConfig(), LocalWikipediaRequest{Operation: "read", Path: "भारत", Offset: offset, OutputBudget: tc.budget})
				if got := localWikipediaModelBytes(out); got > budget {
					t.Fatalf("page at %d is %d model bytes, budget %d", offset, got, budget)
				}
				m := decodeWikiOutput(t, out)
				if m["status"] != "success" {
					t.Fatalf("page at %d: %v", offset, m)
				}
				listed := len(m["sections"].([]any))
				if listed < tc.sections && (m["sections_truncated"] != true || m["sections_total"] != float64(tc.sections)) {
					t.Fatalf("shortened section list not marked: %d of %d, %v", listed, tc.sections, m["sections_total"])
				}
				page := strings.TrimSuffix(strings.TrimPrefix(m["content"].(string), "<external_data>\n"), "\n</external_data>")
				content.WriteString(page)
				pages++
				next, ok := m["next_offset"].(float64)
				if !ok {
					break
				}
				if int(next) <= offset {
					t.Fatalf("next_offset %v does not advance from %d", next, offset)
				}
				offset = int(next)
			}
			if got := content.String(); got != htmlEscapedForTest(text) {
				t.Fatalf("chained pages differ from the article: %d vs %d bytes", len(got), len(htmlEscapedForTest(text)))
			}
			t.Logf("%d pages, page sizes asked %v", pages, lib.pages[:min(len(lib.pages), 8)])
			if tc.budget == 0 && pages < 10 {
				t.Fatalf("pages = %d, want small pages at the default budget", pages)
			}
			for _, size := range lib.pages {
				if size > budget {
					t.Fatalf("asked for a %d-character page with a %d-byte budget", size, budget)
				}
			}
		})
	}
}

// A short German article still comes back in one piece with every section.
func TestLocalWikipediaReadKeepsShortArticlesWhole(t *testing.T) {
	text := repeatRunes("Berlin ist die Hauptstadt Deutschlands. ", 1500)
	lib := &pagedWikiLibrary{title: "Berlin", text: text, sections: scriptSections("Geschichte", 8)}
	useWikiSource(t, pagedWikiSource{lib})
	m := decodeWikiOutput(t, ExecuteLocalWikipedia(context.Background(), wikiConfig(), LocalWikipediaRequest{Operation: "read", Path: "Berlin"}))
	if m["next_offset"] != nil || len(m["sections"].([]any)) != 8 || len(lib.pages) != 1 {
		t.Fatalf("short article: next %v, %d sections, %d reads", m["next_offset"], len(m["sections"].([]any)), len(lib.pages))
	}
}

type pagedWikiSource struct{ lib *pagedWikiLibrary }

func (s pagedWikiSource) AcquireLibrary() (LocalWikipediaLibrary, func(), bool) {
	return s.lib, func() {}, true
}

// htmlEscapedForTest is the article text as the content field carries it
// (IsolateExternalData escapes it once).
func htmlEscapedForTest(text string) string {
	return strings.NewReplacer("&", "&amp;", "'", "&#39;", "<", "&lt;", ">", "&gt;", `"`, "&#34;").Replace(text)
}

func TestLocalWikipediaClip(t *testing.T) {
	for _, tc := range []struct {
		in   string
		n    int
		want string
	}{
		{"kurz", 10, "kurz"},
		{"eins zwei drei vier", 12, "eins zwei…"},
		{"abcdefghij", 5, "abcd…"},
		{"भारत गणराज्य देश", 10, "भारत…"},
	} {
		if got := localWikipediaClip(tc.in, tc.n); got != tc.want || utf8.RuneCountInString(got) > max(tc.n, utf8.RuneCountInString(tc.in)) {
			t.Errorf("clip(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestLocalWikipediaBudgetDefaults(t *testing.T) {
	if got := localWikipediaBudget(0); got != localWikipediaDefaultBudget-localWikipediaBudgetReserve {
		t.Fatalf("default budget = %d", got)
	}
	if got := localWikipediaBudget(10); got != localWikipediaMinBudget-localWikipediaBudgetReserve {
		t.Fatalf("tiny limit budget = %d", got)
	}
	if got := localWikipediaBudget(20000); got != 20000-localWikipediaBudgetReserve {
		t.Fatalf("custom budget = %d", got)
	}
}
