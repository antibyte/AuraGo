package flows

import (
	"regexp/syntax"
	"runtime"
	"strings"
	"testing"
)

// A "matches" pattern is judged by the size of its parse tree before it is simplified and
// compiled: the 200-byte cap alone does not bound the program (a group of 180 bytes
// repeated 1000 times is 182,000 instructions), and the pattern can come from trigger data.
// A pattern that is too complex is refused as "too complex", without the work of compiling
// it, and the node fails with FLOW_CONDITION_FAILED, which is not retried.
func TestMatchesPatternSizeIsBoundedBeforeCompiling(t *testing.T) {
	heavy := "(" + strings.Repeat("abcdefghij", 18) + "){1000}" // 188 bytes, 182,002 instructions when compiled
	if len(heavy) > maxPatternLength {
		t.Fatalf("the test pattern has %d bytes", len(heavy))
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err := compilePattern(heavy)
	runtime.ReadMemStats(&after)
	if err == nil || !strings.Contains(err.Error(), "too complex") {
		t.Fatalf("compilePattern(heavy) = %v, want the too complex error", err)
	}
	// Compiling it allocates about 43 MB; refusing it from the parse tree needs a few KB.
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 8<<20 {
		t.Errorf("refusing the pattern allocated %d KB: it was compiled before it was judged", alloc>>10)
	}

	// The bound never undercounts what Compile emits, and stays close to it, so a pattern
	// within maxPatternInsts is not refused by mistake.
	for _, pattern := range []string{
		`a`, `abc`, `(?i)abc`, `[a-z]+`, `.*`, `a|b|c`, `(a)(b)`, `(?:ab)*c?`, `a{3}`, `a{0}`, `a{2,}`, `a{0,}`, `a{2,5}`, `a{0,3}`, `(ab){3,7}`, `(a|bc){2,}`,
		`^\d{4}-\d{2}-\d{2}$`, `\b(foo|bar)\b`, `(?s).{0,40}`, `(\w+)@(\w+)\.com`, `a{500}`, `a{999}`, `(ab){300}`, `(a?){20}`, `(a*){10}`, `(a+|b*){5}`,
		`(\pL\pN){100}`, `(abcdefghij){100}`, `x{1,1000}`, `[^a-c]{10,20}`, `(?:a{10}b{10}){5}`, `$^`, ``, `()`, `(?:)`, `\x{10ffff}`,
	} {
		parsed, err := syntax.Parse(pattern, syntax.Perl)
		if err != nil {
			t.Fatalf("%q: %v", pattern, err)
		}
		prog, err := syntax.Compile(parsed.Simplify())
		if err != nil {
			t.Fatalf("%q: %v", pattern, err)
		}
		actual, bound := len(prog.Inst), patternInstBound(parsed)
		if bound < min(actual, maxPatternInsts+1) {
			t.Errorf("%q compiles to %d instructions but the bound is %d", pattern, actual, bound)
		}
		if actual <= maxPatternInsts && bound > actual+actual/2+4 {
			t.Errorf("%q compiles to %d instructions but the bound is %d: too loose", pattern, actual, bound)
		}
	}
	// A tree of counts that multiply beyond any int saturates instead of overflowing
	// (the parser refuses such patterns; a bound must not depend on that).
	tree := &syntax.Regexp{Op: syntax.OpLiteral, Rune: []rune("abc")}
	for i := 0; i < 8; i++ {
		tree = &syntax.Regexp{Op: syntax.OpRepeat, Min: 1 << 30, Max: 1 << 30, Sub: []*syntax.Regexp{tree}}
	}
	if bound := patternInstBound(tree); bound != maxPatternInsts+1 {
		t.Errorf("bound of a huge tree = %d, want %d", bound, maxPatternInsts+1)
	}

	// The pattern of the report is refused as well (the parser already rejects repeat
	// counts that multiply past 1000), and a refused pattern ends the node without a retry.
	for _, pattern := range []string{"((a{1000}){100}){30}", heavy, "(a{1000}){25}"} {
		_, err := execLogic(t, TypeIf, map[string]any{"condition": cond("aaa", "matches", pattern)}, nil, nil)
		if ne := asNodeError(err); ne == nil || ne.Code != "FLOW_CONDITION_FAILED" || !hopeless(err) {
			t.Errorf("pattern %.30q: error = %v, want a non-retryable FLOW_CONDITION_FAILED", pattern, err)
		}
	}
}
