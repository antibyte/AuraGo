package dockerstacks

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseEnvFile(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		want      map[string]string
		warnLines []int
	}{
		{"plain", "A=1\nB=two\n", map[string]string{"A": "1", "B": "two"}, nil},
		{"comments, blanks and CRLF", "# c\r\n\r\nA=1\r\nB=2 # trailing\r\nC=3#kept\r\n",
			map[string]string{"A": "1", "B": "2", "C": "3#kept"}, nil},
		{"export prefix", "export A=1\nexport\tB=2\nexported=3\n", map[string]string{"A": "1", "B": "2", "exported": "3"}, nil},
		{"spaces around the equals sign", "A = spaced value  \n", map[string]string{"A": "spaced value"}, nil},
		{"value keeps further equals signs and may be empty", "A=a=b=c\nB=\n", map[string]string{"A": "a=b=c", "B": ""}, nil},
		{"double quotes", `A="x # y"` + "\n" + `B="tab\there \"q\" \\ \$X"` + "\n",
			map[string]string{"A": "x # y", "B": "tab\there \"q\" \\ $X"}, nil},
		{"single quotes are literal", `A='raw $HOME \n'` + "\n", map[string]string{"A": `raw $HOME \n`}, nil},
		{"escaped newline in double quotes", `A="l1\nl2"` + "\n", map[string]string{"A": "l1\nl2"}, nil},
		{"quoted value over several lines", "A=\"first\nsecond\"\nB=5\n", map[string]string{"A": "first\nsecond", "B": "5"}, nil},
		{"comment after a closing quote", `A="x" # why` + "\n", map[string]string{"A": "x"}, nil},
		{"later definition wins", "A=1\nA=2\n", map[string]string{"A": "2"}, nil},
		{"byte order mark", "\xef\xbb\xbfA=1\n", map[string]string{"A": "1"}, nil},
		{"line without equals sign", "just words\nA=1\n", map[string]string{"A": "1"}, []int{1}},
		{"bare key", "KEY\n", map[string]string{}, []int{1}},
		{"invalid name", "1BAD=x\nbad name=y\nOK=1\n", map[string]string{"OK": "1"}, []int{1, 2}},
		{"text after the closing quote", `A="x" y` + "\n", map[string]string{"A": "x"}, []int{1}},
		{"quote that is never closed", "A=\"open\nB=2\n", map[string]string{"B": "2"}, []int{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, diags := parseEnvFile(tc.src)
			if !reflect.DeepEqual(env, tc.want) {
				t.Fatalf("env = %#v, want %#v", env, tc.want)
			}
			var lines []int
			for _, d := range diags {
				if d.Code != "env_syntax" || d.Severity != "warning" || d.Params["file"] != ".env" {
					t.Fatalf("unexpected diagnostic %+v", d)
				}
				lines = append(lines, d.Line)
			}
			if !reflect.DeepEqual(lines, tc.warnLines) {
				t.Fatalf("warning lines = %v, want %v", lines, tc.warnLines)
			}
		})
	}
}

func TestInterpolate(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		env        map[string]string
		want       string
		unresolved bool
		visited    []varRef
	}{
		{"plain text", "hello", nil, "hello", false, nil},
		{"escaped dollar", "pa$$word", nil, "pa$word", false, nil},
		{"named variables", "$A-$B", map[string]string{"A": "1"}, "1-", true,
			[]varRef{{Name: "A", Set: true}, {Name: "B"}}},
		{"braced", "x${A}y", map[string]string{"A": "-"}, "x-y", false, []varRef{{Name: "A", Set: true}}},
		{"default when unset", "${A:-d}", nil, "d", false, []varRef{{Name: "A", Default: true}}},
		{"default when empty with colon", "${A:-d}", map[string]string{"A": ""}, "d", false, []varRef{{Name: "A", Default: true}}},
		{"empty is a value without colon", "${A-d}", map[string]string{"A": ""}, "", false, []varRef{{Name: "A", Default: true, Set: true}}},
		{"required and missing", "${A:?need it}", nil, "", true, []varRef{{Name: "A", Required: true}}},
		{"required and empty with colon", "${A:?x}", map[string]string{"A": ""}, "", true, []varRef{{Name: "A", Required: true}}},
		{"required and empty without colon", "${A?x}", map[string]string{"A": ""}, "", false, []varRef{{Name: "A", Required: true, Set: true}}},
		{"alternative when set", "${A:+yes}", map[string]string{"A": "1"}, "yes", false, []varRef{{Name: "A", Default: true, Set: true}}},
		{"alternative when unset", "${A:+yes}", nil, "", false, []varRef{{Name: "A", Default: true}}},
		{"nested default", "${A:-${B:-z}}", nil, "z", false, []varRef{{Name: "A", Default: true}, {Name: "B", Default: true}}},
		{"nested default is not evaluated when the outer variable is set", "${A:-${B}}", map[string]string{"A": "1"}, "1", false,
			[]varRef{{Name: "A", Default: true, Set: true}}},
		{"text that is not variable syntax stays", "${} ${1A} $ 5 $(x) ${A", nil, "${} ${1A} $ 5 $(x) ${A", false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var visited []varRef
			got, unresolved := interpolate(tc.in, tc.env, func(r varRef) { visited = append(visited, r) })
			if got != tc.want || unresolved != tc.unresolved || !reflect.DeepEqual(visited, tc.visited) {
				t.Fatalf("interpolate(%q) = %q, %v, visited %+v; want %q, %v, %+v", tc.in, got, unresolved, visited, tc.want, tc.unresolved, tc.visited)
			}
			if again, _ := interpolate(tc.in, tc.env, nil); again != tc.want {
				t.Fatalf("interpolate with a nil visitor = %q, want %q", again, tc.want)
			}
		})
	}
}

func TestAnUnclosedQuoteIsScannedOnceNotOncePerFollowingLine(t *testing.T) {
	var b strings.Builder
	b.WriteString("P=\"unclosed\n")
	for i := 0; i < 20000; i++ {
		fmt.Fprintf(&b, "K%d=value%d\n", i, i)
	}
	start := time.Now()
	env, diags := parseEnvFile(b.String())
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("parseEnvFile took %v on %d KB with one unclosed quote", elapsed, b.Len()>>10)
	}
	if len(diags) != 1 || diags[0].Line != 1 || len(env) != 20000 || env["K19999"] != "value19999" {
		t.Fatalf("got %d diagnostics and %d variables, want the one warning on line 1 and every later line parsed", len(diags), len(env))
	}

	// A backslash at the end of a line still escapes the line break the scan carries on over.
	env, diags = parseEnvFile("A=\"x\\\n\"y\nB=1\n")
	if env["A"] != "x\\\n" || env["B"] != "1" || len(diags) != 1 || diags[0].Line != 1 {
		t.Fatalf("env = %#v, diagnostics = %+v", env, diags)
	}
}

func TestNestedDefaultsAreFollowedOnlyToAFixedDepth(t *testing.T) {
	const depth = 24000
	s := strings.Repeat("${A:-", depth) + "x" + strings.Repeat("}", depth)
	start := time.Now()
	got, unresolved := interpolate(s, map[string]string{}, nil)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("interpolate took %v on %d nested defaults", elapsed, depth)
	}
	if !unresolved || len(got) != 6*(depth-maxInterpolateDepth)+1 || !strings.HasPrefix(got, "${A:-") {
		t.Fatalf("a nesting deeper than %d levels must stay as it is and count as unresolved, got unresolved=%v (%d bytes)", maxInterpolateDepth, unresolved, len(got))
	}

	// What a real file nests still expands.
	ok := strings.Repeat("${A:-", maxInterpolateDepth-1) + "x" + strings.Repeat("}", maxInterpolateDepth-1)
	if got, unresolved := interpolate(ok, map[string]string{}, nil); got != "x" || unresolved {
		t.Fatalf("interpolate at %d levels = %q, %v, want x", maxInterpolateDepth-1, got, unresolved)
	}
}
