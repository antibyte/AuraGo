package whatsapp

import (
	"strings"
	"testing"
)

func TestToWhatsAppConvertsMarkdown(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"bold stars", "**b**", "*b*"},
		{"bold underscores", "__b__", "*b*"},
		{"italic star", "*i*", "_i_"},
		{"italic underscore stays", "_i_", "_i_"},
		{"strikethrough", "~~s~~", "~s~"},
		{"two italics", "*a* and *b*", "_a_ and _b_"},
		{"two bolds", "**a** and **b**", "*a* and *b*"},
		{"snake case stays", "snake_case_name", "snake_case_name"},
		{"arithmetic stars stay", "2*3*4", "2*3*4"},
		{"bold with colon", "**Note:** text", "*Note:* text"},
		{"bold around italic", "**a *b* c**", "*a _b_ c*"},
		{"bold italic", "***x***", "*_x_*"},
		{"bold around strikethrough", "**~~x~~**", "*~x~*"},
		{"opener followed by space stays", "** not bold **", "** not bold **"},
		{"empty emphasis stays", "****", "****"},
		{"heading", "### **Title**", "*Title*"},
		{"plain heading", "# Title", "*Title*"},
		{"heading closing hashes", "## Title ##", "*Title*"},
		{"heading keeps trailing hash of the word", "# C#", "*C#*"},
		{"heading keeps one-sided underscore", "## Use _init_", "*Use _init_*"},
		{"link", "[docs](https://example.org)", "docs (https://example.org)"},
		{"link with equal text", "[https://example.org](https://example.org)", "https://example.org"},
		{"image", "![logo](https://example.org/a.png)", "logo (https://example.org/a.png)"},
		{"image without alt", "![](https://example.org/a.png)", "https://example.org/a.png"},
		{"bracket without link stays", "array[0] and [x]", "array[0] and [x]"},
		{"url underscores stay", "see https://example.org/__init__/a_b_ now", "see https://example.org/__init__/a_b_ now"},
		{"bold url", "**https://example.org/a**", "*https://example.org/a*"},
		{"escaped stars stay", `\*not italic\*`, `\*not italic\*`},
		{"star bullet", "* item", "- item"},
		{"plus bullet", "+ item", "- item"},
		{"indented star bullet", "  * item", "  - item"},
		{"bullet with bold", "* **k**: v", "- *k*: v"},
		{"dash bullet stays", "- x", "- x"},
		{"numbered stays", "1. x", "1. x"},
		{"quote stays", "> q", "> q"},
		{"quote with italic", "> *q*", "> _q_"},
		{"carriage return stays", "**b**\r\n*i*", "*b*\r\n_i_"},
		{"multi-line", "# T\n\n* **a**\n* b", "*T*\n\n- *a*\n- b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := toWhatsApp(tc.in); got != tc.want {
				t.Fatalf("toWhatsApp(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestToWhatsAppNeverTouchesCode(t *testing.T) {
	inline := "run `**x**` and ``a ` **y**`` now"
	if got := toWhatsApp(inline); got != inline {
		t.Fatalf("code spans changed: %q", got)
	}
	fenced := "before **b**\n```go\n**x**\n# not a heading\n| a | b |\n|---|---|\n* star\n```\nafter"
	want := strings.Replace(fenced, "before **b**", "before *b*", 1)
	if got := toWhatsApp(fenced); got != want {
		t.Fatalf("fenced block changed:\n got %q\nwant %q", got, want)
	}
	block := "```\n**x**\n# not a heading\n| a | b |\n```"
	if got := toWhatsApp(block); got != block {
		t.Fatalf("a fenced block must come out byte-identical, got %q", got)
	}
	if got := toWhatsApp("**a `b` c**"); got != "*a `b` c*" {
		t.Fatalf("emphasis around a code span: %q", got)
	}
}

func TestToWhatsAppTurnsTablesIntoBullets(t *testing.T) {
	table := "| Name | Role |\n|---|:--:|\n| Ada | **Dev** |\n| Bob | Ops |\nafter"
	want := "• Name — Role\n• Ada — *Dev*\n• Bob — Ops\nafter"
	if got := toWhatsApp(table); got != want {
		t.Fatalf("table:\n got %q\nwant %q", got, want)
	}
	noSeparator := "| a | b |\n| c | d |"
	if got := toWhatsApp(noSeparator); got != noSeparator {
		t.Fatalf("a run of pipe lines without a separator must stay: %q", got)
	}
	single := "| only | row |"
	if got := toWhatsApp(single); got != single {
		t.Fatalf("a single pipe line must stay: %q", got)
	}
}
