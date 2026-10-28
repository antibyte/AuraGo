package dockerstacks

import (
	"reflect"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"blanks", "a b  c\td", []string{"a", "b", "c", "d"}},
		{"quotes", `a "b c" 'd e'`, []string{"a", "b c", "d e"}},
		{"empty quotes are a word", `a ""`, []string{"a", ""}},
		{"adjacent quotes join", `a"b"'c'`, []string{"abc"}},
		{"backslash continuation", "a \\\n  b", []string{"a", "b"}},
		{"backslash continuation with blanks and CRLF", "a \\  \r\n  b", []string{"a", "b"}},
		{"backtick continuation", "a `\n b", []string{"a", "b"}},
		{"escaped blank", `a\ b`, []string{"a b"}},
		{"escaped quote in double quotes", `"a\"b"`, []string{`a"b`}},
		{"powershell escaped quote", "\"say `\"hi`\"\"", []string{`say "hi"`}},
		{"windows path stays", `-v C:\data:/data`, []string{"-v", `C:\data:/data`}},
		{"single quoted dollar is literal", `'pa$word'`, []string{"pa$$word"}},
		{"double quoted dollar is a variable", `"pa$word"`, []string{"pa$word"}},
		{"escaped dollar is literal", `\$HOME`, []string{"$$HOME"}},
		{"command substitution is one piece", `-u $(id -u):$(id -g)`, []string{"-u", "$(id -u):$(id -g)"}},
		{"comment", "a # comment\nb", []string{"a", "b"}},
		{"hash inside a word", "a#b", []string{"a#b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := splitCommand(tc.in)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("splitCommand(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
			}
		})
	}
	for _, in := range []string{`a "b`, `a 'b`, `"a\"`} {
		if got, err := splitCommand(in); err == nil {
			t.Errorf("splitCommand(%q) = %q, want an unbalanced quote error", in, got)
		}
	}
}
