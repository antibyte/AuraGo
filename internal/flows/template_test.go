package flows

import (
	"errors"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func TestParseTemplateParts(t *testing.T) {
	tpl, err := ParseTemplate("Hallo {{ name }}!")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	exprs := tpl.Exprs()
	if len(exprs) != 1 || exprs[0].Root != "name" || exprs[0].Source != "name" {
		t.Fatalf("exprs = %+v", exprs)
	}
	if tpl.IsSingleExpr() {
		t.Fatal("text around the expression: not a single expression")
	}
	single, err := ParseTemplate("{{websuche.results}}")
	if err != nil || !single.IsSingleExpr() {
		t.Fatalf("single expr: %v %v", single, err)
	}
}

func TestParseTemplatePaths(t *testing.T) {
	tpl, err := ParseTemplate(`{{websuche.results[0].title}} {{trigger.data["x-key"]}} {{w.items[-1]}}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	exprs := tpl.Exprs()
	want0 := []PathSeg{{Field: "results"}, {Index: 0, IsIndex: true}, {Field: "title"}}
	if exprs[0].Root != "websuche" || !reflect.DeepEqual(exprs[0].Path, want0) {
		t.Fatalf("expr0 = %+v", exprs[0])
	}
	want1 := []PathSeg{{Field: "data"}, {Field: "x-key"}}
	if exprs[1].Root != "trigger" || !reflect.DeepEqual(exprs[1].Path, want1) {
		t.Fatalf("expr1 = %+v", exprs[1])
	}
	if p := exprs[2].Path; len(p) != 2 || !p[1].IsIndex || p[1].Index != -1 {
		t.Fatalf("expr2 = %+v", exprs[2])
	}
}

func TestParseTemplateFilters(t *testing.T) {
	tpl, err := ParseTemplate(`{{ a | truncate(10) | upper }} {{b | default("}}")}} {{c | replace('a', "b")}}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	exprs := tpl.Exprs()
	if len(exprs) != 3 {
		t.Fatalf("exprs = %d", len(exprs))
	}
	want := []FilterCall{{Name: "truncate", Args: []any{10.0}}, {Name: "upper"}}
	if !reflect.DeepEqual(exprs[0].Filters, want) {
		t.Fatalf("filters0 = %+v", exprs[0].Filters)
	}
	if exprs[1].Filters[0].Args[0] != "}}" {
		t.Fatalf("quoted }} must stay inside the string: %+v", exprs[1].Filters)
	}
	if !reflect.DeepEqual(exprs[2].Filters[0].Args, []any{"a", "b"}) {
		t.Fatalf("single quotes: %+v", exprs[2].Filters)
	}
	escaped, err := ParseTemplate(`{{d | join("\n") | replace("\t", "\"")}}`)
	if err != nil {
		t.Fatalf("parse escapes: %v", err)
	}
	f := escaped.Exprs()[0].Filters
	if f[0].Args[0] != "\n" || f[1].Args[0] != "\t" || f[1].Args[1] != `"` {
		t.Fatalf("string escapes = %#v", f)
	}
}

func TestParseTemplateEscape(t *testing.T) {
	tpl, err := ParseTemplate(`\{{not}} {{x}}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(tpl.Exprs()) != 1 {
		t.Fatalf("escaped braces must not become an expression: %+v", tpl.Exprs())
	}
	if tpl.parts[0].literal != "{{not}} " {
		t.Fatalf("escaped literal = %q", tpl.parts[0].literal)
	}
}

func TestParseTemplateErrors(t *testing.T) {
	for _, src := range []string{
		"{{a",
		"{{}}",
		"{{a.}}",
		"{{a[x]}}",
		"{{a[1.5]}}",
		"{{a | nope}}",
		"{{a | truncate}}",
		"{{a b}}",
		`{{a | default("x}}`,
		"{{a | truncate(1,)}}x{{",
	} {
		_, err := ParseTemplate(src)
		var te *TemplateError
		if !errors.As(err, &te) {
			t.Errorf("ParseTemplate(%q) error = %v, want *TemplateError", src, err)
		}
	}
}

func TestHasTemplate(t *testing.T) {
	if !HasTemplate("a {{b}}") || HasTemplate("plain") || HasTemplate(`\{{x}}`) {
		t.Fatal("HasTemplate mismatch")
	}
}

// The remaining tests go beyond the plan's list: they pin error positions and
// messages and the parser's behaviour on hostile input.

func TestParseTemplateErrorOffsets(t *testing.T) {
	cases := []struct {
		src    string
		offset int
		msg    string
	}{
		{"Hallo {{a b}}", 10, `unexpected "b"`},
		{"ab {{c", 3, "unclosed {{"},
		{"{{a.}}", 4, "expected a field name after '.'"},
		{"{{a | nope}}", 6, `unknown filter "nope"`},
		{"{{a | truncate}}", 6, "filter truncate takes 1 argument(s)"},
		{"{{a[1.5]}}", 4, "index must be a whole number"},
		{"{{a[x]}}", 4, "expected a number or string inside [ ]"},
		{"{{a[1}}", 5, "expected ']'"},
		{"x {{a | truncate(1,)}}", 19, "expected an argument after ','"},
		{"{{a[2147483648]}}", 4, "index too large"},
		{"{{a | truncate(b)}}", 15, "expected a string, number, true, false or null"},
		{"{{a | truncate(1 2)}}", 17, "expected ',' or ')'"},
		{"{{a | truncate(}}", 15, "expected a string, number, true, false or null"},
		{"{{a % b}}", 4, `unexpected character '%'`},
		{"{{a[1.2.3]}}", 4, `invalid number "1.2.3"`},
		{"{{ }}", 3, "expected a name"},
		{"{{a | }}", 6, "expected a filter name after '|'"},
	}
	for _, tc := range cases {
		_, err := ParseTemplate(tc.src)
		var te *TemplateError
		if !errors.As(err, &te) {
			t.Errorf("ParseTemplate(%q) error = %v, want *TemplateError", tc.src, err)
			continue
		}
		if te.Offset != tc.offset || te.Msg != tc.msg {
			t.Errorf("ParseTemplate(%q) = offset %d %q, want offset %d %q", tc.src, te.Offset, te.Msg, tc.offset, tc.msg)
		}
	}
}

func TestTemplateErrorMessage(t *testing.T) {
	_, err := ParseTemplate("Hallo {{a b}}")
	if err == nil || err.Error() != `template error at 10: unexpected "b"` {
		t.Fatalf("error = %v", err)
	}
}

func TestParseTemplateLiteralsAndEscapes(t *testing.T) {
	tpl, err := ParseTemplate(`{{a | default(true) | default(false) | default(null) | default(-2.5)}} {{b | replace("\\", "/") | replace('it\'s', "x")}}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	exprs := tpl.Exprs()
	var args []any
	for _, f := range exprs[0].Filters {
		args = append(args, f.Args[0])
	}
	if want := []any{true, false, nil, -2.5}; !reflect.DeepEqual(args, want) {
		t.Fatalf("literal args = %#v, want %#v", args, want)
	}
	f := exprs[1].Filters
	if f[0].Args[0] != `\` || f[0].Args[1] != "/" || f[1].Args[0] != "it's" {
		t.Fatalf("escaped quote and backslash = %#v", f)
	}

	// An escaped multi-byte character keeps all of its bytes.
	uni, err := ParseTemplate(`{{a | default("\ä")}}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := uni.Exprs()[0].Filters[0].Args[0]; got != "ä" {
		t.Fatalf("escaped multi-byte = %q", got)
	}
}

func TestParseTemplateLayout(t *testing.T) {
	tpl, err := ParseTemplate("a{{x}}b{{y}}c")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var kinds []string
	for _, p := range tpl.parts {
		if p.expr != nil {
			kinds = append(kinds, "{"+p.expr.Root+"}")
		} else {
			kinds = append(kinds, p.literal)
		}
	}
	if want := []string{"a", "{x}", "b", "{y}", "c"}; !reflect.DeepEqual(kinds, want) {
		t.Fatalf("parts = %v, want %v", kinds, want)
	}
	if empty, err := ParseTemplate(""); err != nil || len(empty.Exprs()) != 0 || empty.IsSingleExpr() {
		t.Fatalf("empty template: %+v %v", empty, err)
	}
	if lone, err := ParseTemplate("a }} b {x"); err != nil || len(lone.Exprs()) != 0 {
		t.Fatalf("lone braces are literal text: %+v %v", lone, err)
	}
	if ws, err := ParseTemplate("{{\n\ta\r\n | upper\n}}"); err != nil || ws.Exprs()[0].Source != "a\r\n | upper" {
		t.Fatalf("whitespace inside the braces: %+v %v", ws, err)
	}
}

func TestParseTemplateIndexRange(t *testing.T) {
	tpl, err := ParseTemplate("{{a[2147483647]}} {{a[-2147483647]}}")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	exprs := tpl.Exprs()
	if exprs[0].Path[0].Index != 2147483647 || exprs[1].Path[0].Index != -2147483647 {
		t.Fatalf("indexes = %+v %+v", exprs[0].Path, exprs[1].Path)
	}
	// Beyond int32 the conversion float64 -> int would wrap silently on some
	// values, so the parser refuses them.
	for _, src := range []string{"{{a[2147483648]}}", "{{a[-2147483648]}}", "{{a[99999999999999999999]}}", "{{a[" + strings.Repeat("9", 400) + "]}}"} {
		_, err := ParseTemplate(src)
		var te *TemplateError
		if !errors.As(err, &te) {
			t.Errorf("ParseTemplate(%.40q) error = %v, want *TemplateError", src, err)
		}
	}
}

func TestParseTemplateNonASCIICharacter(t *testing.T) {
	_, err := ParseTemplate("{{größe}}")
	var te *TemplateError
	if !errors.As(err, &te) || te.Msg != "unexpected character 'ö'" || te.Offset != 4 {
		t.Fatalf("error = %v", err)
	}
	_, err = ParseTemplate("{{a\xff}}")
	if !errors.As(err, &te) || te.Msg != "unexpected byte 0xff" {
		t.Fatalf("invalid UTF-8 error = %v", err)
	}
}

func TestParseTemplateErrorEchoIsBounded(t *testing.T) {
	long := strings.Repeat("x", 100000)
	for _, src := range []string{
		`{{a "` + long + `"}}`,
		`{{a ` + long + `}}`,
		`{{a | ` + long + `}}`,
		`{{a[` + strings.Repeat("9", 100000) + `]}}`,
		`{{a[1.` + strings.Repeat("1.", 50000) + `]}}`,
	} {
		_, err := ParseTemplate(src)
		if err == nil {
			t.Fatalf("ParseTemplate(%.30q) succeeded", src)
		}
		if len(err.Error()) > 200 {
			t.Errorf("error for %.30q is %d bytes long; hostile input must not flood messages", src, len(err.Error()))
		}
	}
}

func TestParseTemplateHostileInputs(t *testing.T) {
	seeds := []string{
		`Hallo {{ a.b[0]["k"] | truncate(10) | default("}}") | replace('a', "\"") }} und \{{x}} {{c}}`,
		`{{a | join("\n") | date("YYYY-MM-DD", "Europe/Berlin")}}`,
		`{{ w.items[-1] | pluck('name') | first | json }}`,
		`\\{{x}} \{{ {{y}}} }}`,
		strings.Repeat("{{", 40) + strings.Repeat("}}", 40),
		strings.Repeat(`{{a|b("`, 20),
		strings.Repeat("{{a}}", 200),
		strings.Repeat("{{a", 100) + strings.Repeat("]", 100),
		"{{" + strings.Repeat(".a", 5000) + "}}",
		"{{a" + strings.Repeat("[0]", 5000) + "}}",
		"{{a" + strings.Repeat(` | upper`, 5000) + "}}",
		"{{a | default(" + strings.Repeat("9", 5000) + ")}}",
	}
	check := func(src string) {
		t.Helper()
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseTemplate(%.80q) panicked: %v", src, r)
			}
		}()
		tpl, err := ParseTemplate(src)
		has := HasTemplate(src)
		if err != nil {
			var te *TemplateError
			if !errors.As(err, &te) {
				t.Fatalf("ParseTemplate(%.80q) error = %v, want *TemplateError", src, err)
			}
			if te.Offset < 0 || te.Offset > len(src) {
				t.Fatalf("ParseTemplate(%.80q) offset %d outside the source (len %d)", src, te.Offset, len(src))
			}
			if !has {
				t.Fatalf("ParseTemplate(%.80q) failed although HasTemplate is false", src)
			}
			return
		}
		if has != (len(tpl.Exprs()) > 0) {
			t.Fatalf("ParseTemplate(%.80q): HasTemplate=%v but %d expressions", src, has, len(tpl.Exprs()))
		}
		for _, e := range tpl.Exprs() {
			if e.Root == "" {
				t.Fatalf("ParseTemplate(%.80q) produced an expression without a root", src)
			}
		}
	}
	for _, seed := range seeds {
		check(seed)
		if len(seed) > 600 {
			continue
		}
		// Every prefix and every one-byte deletion of the seed.
		for i := 0; i <= len(seed); i++ {
			check(seed[:i])
		}
		for i := 0; i < len(seed); i++ {
			check(seed[:i] + seed[i+1:])
		}
	}
	// Deterministic byte soup over the characters the lexer treats specially.
	const alphabet = "{{}}\\\"'|()[],.- 0123456789abz_\n\xff"
	rng := rand.New(rand.NewSource(1))
	for n := 0; n < 20000; n++ {
		var b strings.Builder
		for k := rng.Intn(40); k > 0; k-- {
			b.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}
		check(b.String())
	}
}
