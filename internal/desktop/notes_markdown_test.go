package desktop

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

type notesFixtureExtract struct {
	Name    string   `json:"name"`
	Content string   `json:"content"`
	Title   string   `json:"title"`
	Aliases []string `json:"aliases"`
	Tags    []string `json:"tags"`
	Links   []struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
		Embed bool   `json:"embed"`
	} `json:"links"`
}

type notesFixtureBlock struct {
	Name    string   `json:"name"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
	Links   []struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
		Embed bool   `json:"embed"`
	} `json:"links"`
}

type notesFixtureResolveCase struct {
	From  string `json:"from"`
	Kind  string `json:"kind"`
	Value string `json:"value"`
	Embed bool   `json:"embed"`
	Path  string `json:"path"`
}

type notesFixture struct {
	Extract  []notesFixtureExtract `json:"extract"`
	Blocks   []notesFixtureBlock   `json:"blocks"`
	NotLinks []notesFixtureBlock   `json:"notLinks"`
	Resolve  struct {
		Notes       []string                  `json:"notes"`
		Aliases     map[string][]string       `json:"aliases"`
		Attachments []string                  `json:"attachments"`
		Cases       []notesFixtureResolveCase `json:"cases"`
	} `json:"resolve"`
}

func loadNotesFixture(t *testing.T) notesFixture {
	t.Helper()
	data, err := os.ReadFile("../../ui/testdata/notes-links-fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture notesFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestExtractNoteRefsFixture(t *testing.T) {
	for _, tc := range loadNotesFixture(t).Extract {
		t.Run(tc.Name, func(t *testing.T) {
			refs := ExtractNoteRefs(tc.Content)
			if refs.Title != tc.Title || !sameStrings(refs.Aliases, tc.Aliases) || !sameStrings(refs.Tags, tc.Tags) {
				t.Fatalf("got title=%q aliases=%v tags=%v", refs.Title, refs.Aliases, refs.Tags)
			}
			if len(refs.Links) != len(tc.Links) {
				t.Fatalf("links: got %+v", refs.Links)
			}
			for i, want := range tc.Links {
				got := refs.Links[i]
				if got.Kind != want.Kind || got.Value != want.Value || got.Embed != want.Embed {
					t.Fatalf("link %d: got %+v want %+v", i, got, want)
				}
				if tc.Content[got.Start:got.End] != want.Value {
					t.Fatalf("link %d span %d:%d = %q", i, got.Start, got.End, tc.Content[got.Start:got.End])
				}
			}
		})
	}
}

func TestExtractNoteRefsEdgeCases(t *testing.T) {
	refs := ExtractNoteRefs("[[a\\|b]] [[bad\nx]] [[x[[y]]\n~~~\n[[no]]\n~~~\n<!-- [[c]] #c -->\n[l](#top) [i](<my file.png>) ![x](pic.PNG) (#paren) \\#esc\n")
	var values []string
	for _, l := range refs.Links {
		values = append(values, l.Kind+":"+l.Value)
	}
	want := []string{"wiki:a\\|b", "wiki:y", "markdown:my file.png", "markdown:pic.PNG"}
	if !sameStrings(values, want) {
		t.Fatalf("links %v want %v", values, want)
	}
	if !refs.Links[3].Embed || refs.Links[2].Embed {
		t.Fatalf("image embed flags wrong: %+v", refs.Links)
	}
	if !sameStrings(refs.Tags, []string{"paren"}) {
		t.Fatalf("tags %v", refs.Tags)
	}
	meta := ExtractNoteRefs("---\nalias: Solo\ntags: \"a, #B c\"\n---\nbody #in\n")
	if !sameStrings(meta.Aliases, []string{"Solo"}) || !sameStrings(meta.Tags, []string{"a", "b", "c", "in"}) {
		t.Fatalf("frontmatter %+v", meta)
	}
}

func TestParseWikiValue(t *testing.T) {
	cases := map[string]WikiValue{
		"Note":           {Target: "Note"},
		"Note#Sub|Alias": {Target: "Note", Subpath: "Sub", Alias: "Alias"},
		"Note\\|Alias":   {Target: "Note", Alias: "Alias"},
		"#Local":         {Subpath: "Local"},
	}
	for in, want := range cases {
		if got := ParseWikiValue(in); got != want {
			t.Errorf("%q: got %+v want %+v", in, got, want)
		}
	}
	if got := ReplaceWikiTarget("Old#Sub|shown", "New/Path"); got != "New/Path#Sub|shown" {
		t.Fatalf("got %q", got)
	}
	if got := ReplaceWikiTarget("Old\\|shown", "New"); got != "New\\|shown" {
		t.Fatalf("got %q", got)
	}
}

func TestRewriteNoteLinksKeepsAliasAndCode(t *testing.T) {
	in := "See [[Old|shown]] and [[Old#Part]] and [x](Old.md#y)\n```\n[[Old]]\n```\n"
	out := RewriteNoteLinks(in, func(link NoteRefLink) (string, bool) {
		if link.Kind == "wiki" {
			v := ParseWikiValue(link.Value)
			if v.Target == "Old" {
				return ReplaceWikiTarget(link.Value, "New"), true
			}
		}
		if link.Kind == "markdown" && strings.HasPrefix(link.Value, "Old.md") {
			return "New.md" + strings.TrimPrefix(link.Value, "Old.md"), true
		}
		return "", false
	})
	want := "See [[New|shown]] and [[New#Part]] and [x](New.md#y)\n```\n[[Old]]\n```\n"
	if out != want {
		t.Fatalf("got %q", out)
	}
}

// The block and "not a link" tables are shared with the JS mirror and with the editor's micromark layer
// (scripts/notes-obsidian-markdown.test.mjs), so all three agree on what a link is.
func TestExtractNoteRefsBlocksFixture(t *testing.T) {
	fixture := loadNotesFixture(t)
	for _, tc := range fixture.Blocks {
		t.Run(tc.Name, func(t *testing.T) {
			refs := ExtractNoteRefs(tc.Content)
			if !sameStrings(refs.Tags, tc.Tags) {
				t.Fatalf("tags %v want %v", refs.Tags, tc.Tags)
			}
			if len(refs.Links) != len(tc.Links) {
				t.Fatalf("links: got %+v want %+v", refs.Links, tc.Links)
			}
			for i, want := range tc.Links {
				got := refs.Links[i]
				if got.Kind != want.Kind || got.Value != want.Value || got.Embed != want.Embed || tc.Content[got.Start:got.End] != want.Value {
					t.Fatalf("link %d: got %+v want %+v", i, got, want)
				}
			}
		})
	}
	for _, tc := range fixture.NotLinks {
		t.Run("not a link: "+tc.Name, func(t *testing.T) {
			if refs := ExtractNoteRefs(tc.Content); len(refs.Links) != 0 {
				t.Fatalf("found links %+v", refs.Links)
			}
		})
	}
}

func TestRewriteNoteLinksLeavesCodeAlone(t *testing.T) {
	in := "> ```\n> [[Old]]\n> ```\n> [[Old]]\n- ```\n  [[Old]]\n  ```\n\nx\n\n    [[Old]]\n\n`a\n[[Old]]` [[Old]]\n<div>[[Old]]</div>\n"
	out := RewriteNoteLinks(in, func(link NoteRefLink) (string, bool) { return "New", link.Value == "Old" })
	want := "> ```\n> [[Old]]\n> ```\n> [[New]]\n- ```\n  [[Old]]\n  ```\n\nx\n\n    [[Old]]\n\n`a\n[[Old]]` [[New]]\n<div>[[Old]]</div>\n"
	if out != want {
		t.Fatalf("got %q", out)
	}
}

func TestExtractNoteRefsAdversarialInputStaysFast(t *testing.T) {
	const size = 2 << 20
	cases := map[string]string{
		"](":                strings.Repeat("](", size/2),
		"](<":               strings.Repeat("](<", size/3),
		"](a ":              strings.Repeat("](a ", size/4),
		"[[":                strings.Repeat("[[", size/2),
		"![[x":              strings.Repeat("![[x", size/4),
		"[[x]":              strings.Repeat("[[x]", size/4),
		"#":                 strings.Repeat("#", size),
		"# #":               strings.Repeat("# #\n", size/4),
		"`":                 strings.Repeat("`", size),
		"`a":                strings.Repeat("`a", size/2),
		"``` mix":           strings.Repeat("`a``b```c", size/9),
		"<!--":              strings.Repeat("<!--", size/4),
		"]](a.md)":          strings.Repeat("]](a.md)", size/8),
		"[a](b ":            strings.Repeat("[a](b ", size/6),
		"> - ":              strings.Repeat("> - ", size/4),
		"lines of ](":       strings.Repeat("x](\n", size/4),
		"distinct backtick": distinctBacktickRuns(size),
	}
	for name, content := range cases {
		start := time.Now()
		ExtractNoteRefs(content)
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Errorf("%s: %d bytes took %v", name, len(content), elapsed)
		}
	}
}

func distinctBacktickRuns(size int) string {
	var b strings.Builder
	for i := 1; b.Len() < size; i++ {
		b.WriteString(strings.Repeat("`", i%1500+1))
		b.WriteByte('a')
	}
	return b.String()
}

func TestExtractNoteRefsContextCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ExtractNoteRefsContext(ctx, strings.Repeat("a [[b]]\n", 100000)); err == nil {
		t.Fatal("expected cancellation")
	}
}

func TestExtractNoteRefsLongLinkURLIsNotALink(t *testing.T) {
	long := "[x](" + strings.Repeat("a", maxNoteLinkURL+1) + ".md)"
	if refs := ExtractNoteRefs(long); len(refs.Links) != 0 {
		t.Fatalf("got %+v", refs.Links)
	}
}
