package storymaker

import (
	"bytes"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"
)

// readFixture reads a testdata file with line endings normalised to LF (a Windows checkout may
// convert them).
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

func TestBundleAcceptsFixtureScene(t *testing.T) {
	res := BundleCodeScene(readFixture(t, "testdata/scenes/s3.js"), "s3", 0)
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	code := string(res.Code)
	for _, want := range []string{"var __smSceneModule = (() => {", "default: () => scene", "\nself.__smScene=__smSceneModule.default;\n"} {
		if !strings.Contains(code, want) {
			t.Errorf("bundle misses %q", want)
		}
	}
	if !strings.HasSuffix(code, "\nself.__smScene=__smSceneModule.default;\n") {
		t.Error("bundle must end with the registration line")
	}
	if strings.Contains(code, "import ") || strings.Contains(code, "require(") {
		t.Error("bundle must not load other modules")
	}
}

// TestBundleGolden pins the exact bytes: the player test in Node reproduces them with esbuild.
// Regenerate with STORYMAKER_UPDATE_GOLDEN=1.
func TestBundleGolden(t *testing.T) {
	res := BundleCodeScene(readFixture(t, "testdata/scenes/s3.js"), "s3", 0)
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	const golden = "testdata/scenes/s3.bundle.js"
	if os.Getenv("STORYMAKER_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, res.Code, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if want := readFixture(t, golden); !bytes.Equal(res.Code, want) {
		t.Fatalf("bundle differs from %s (rerun with STORYMAKER_UPDATE_GOLDEN=1 after an intended esbuild change)\n got: %q\nwant: %q", golden, res.Code, want)
	}
}

func TestBundleRejects(t *testing.T) {
	cases := []struct {
		name string
		src  string
		line int
		col  int
		msg  string // substring
	}{
		{"static import", "import x from './y.js';\nexport default function scene(api) {}\n", 1, 1, "import"},
		{"side-effect import", "import './y.js';\nexport default function scene(api) {}\n", 1, 1, "import"},
		{"dynamic import", "export default function scene(api) {\n  api.go(import('./y.js'));\n}\n", 2, 10, "import"},
		{"export from", "export { a } from './y.js';\nexport default function scene(api) {}\n", 1, 1, "export ... from"},
		{"export star", "export * from './y.js';\nexport default function scene(api) {}\n", 1, 1, "export ... from"},
		{"eval", "export default function scene(api) {\n  eval('1');\n}\n", 2, 3, "eval"},
		{"Function", "export default function scene(api) {\n  new Function('return 1')();\n}\n", 2, 7, "Function"},
		{"globalThis", "export default function scene(api) { globalThis.x = 1; }\n", 1, 38, "globalThis"},
		{"self", "export default function scene(api) { self.postMessage(1); }\n", 1, 38, "self"},
		{"window", "export default function scene(api) { window.x = 1; }\n", 1, 38, "window"},
		{"document", "export default function scene(api) { document.title = 'x'; }\n", 1, 38, "document"},
		{"importScripts", "export default function scene(api) { importScripts('x'); }\n", 1, 38, "importScripts"},
		{"fetch", "export default function scene(api) { fetch('/x'); }\n", 1, 38, "fetch"},
		{"XMLHttpRequest", "export default function scene(api) { new XMLHttpRequest(); }\n", 1, 42, "XMLHttpRequest"},
		{"WebSocket", "export default function scene(api) { new WebSocket('ws://x'); }\n", 1, 42, "WebSocket"},
		{"dot constructor", "export default function scene(api) {\n  const f = (() => 1).constructor;\n}\n", 2, 23, ".constructor"},
		{"spaced constructor", "export default function scene(api) {\n  const f = (() => 1)\n    . constructor;\n}\n", 3, 7, ".constructor"},
		{"bracket constructor", "export default function scene(api) {\n  const f = (() => 1)['constructor'];\n}\n", 2, 23, "constructor"},
		{"escaped identifier", "export default function scene(api) {\n  \x5cu0065val('1');\n}\n", 2, 3, "eval"},
		{"escaped bracket string", "export default function scene(api) {\n  const f = {}['constr\x5cx75ctor'];\n}\n", 2, 16, "constructor"},
		{"in template substitution", "export default function scene(api) {\n  const s = `a ${eval('1')} b`;\n}\n", 2, 18, "eval"},
		{"missing default export", "export function scene(api) {}\n", 1, 1, "export default"},
		{"empty source", "  \n", 1, 1, "export default"},
		{"syntax error", "export default function scene(api) {\n  api.duration(5;\n}\n", 2, 17, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := BundleCodeScene([]byte(tc.src), "s1", 0)
			if res.Code != nil || len(res.Errors) == 0 {
				t.Fatalf("accepted: %q", res.Code)
			}
			found := false
			for _, e := range res.Errors {
				if e.Line == tc.line && e.Column == tc.col && strings.Contains(e.Message, tc.msg) {
					found = true
				}
			}
			if !found {
				t.Fatalf("want error at %d:%d containing %q, got %+v", tc.line, tc.col, tc.msg, res.Errors)
			}
		})
	}
}

// Names inside strings, comments, templates and regular expressions are not code.
func TestBundleIgnoresForbiddenNamesInsideText(t *testing.T) {
	src := "// eval self window\n" +
		"/* fetch(document) */\n" +
		"export default function scene(api) {\n" +
		"  api.text('eval and fetch and self', 'title');\n" +
		"  const re = /window|document\\//g;\n" +
		"  const t = `the ${api.duration(1)} document is self-made`;\n" +
		"  const half = (2 + 2) / 2 / 1;\n" +
		"  const selfish = 1, fetched = 2, evaluate = 3;\n" +
		"  api.tween(api.text('x', 'title'), { opacity: 1 }, { start: 0, dur: 1, ease: 'ease-out' });\n" +
		"}\n"
	res := BundleCodeScene([]byte(src), "s1", 0)
	if len(res.Errors) != 0 {
		t.Fatalf("rejected: %+v", res.Errors)
	}
}

// A slash after ) or } may be a division or a regular expression; both readings are checked, so
// a quote inside a regex literal cannot hide the code behind it.
func TestBundleAmbiguousSlashCannotHideCode(t *testing.T) {
	src := "export default function scene(api) {\n  if (api) /\"/.test('x'); eval('1');\n}\n"
	res := BundleCodeScene([]byte(src), "s1", 0)
	if len(res.Errors) == 0 {
		t.Fatal("eval behind an ambiguous slash was accepted")
	}
}

func TestBundleSizeCap(t *testing.T) {
	src := []byte("export default function scene(api) {}\n" + strings.Repeat("// padding\n", 20))
	if res := BundleCodeScene(src, "s1", len(src)); len(res.Errors) != 0 {
		t.Fatalf("source at the limit rejected: %+v", res.Errors)
	}
	res := BundleCodeScene(src, "s1", len(src)-1)
	if res.Code != nil || len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, "limit") {
		t.Fatalf("oversize source: %+v", res)
	}
}

func TestBundleReportsEveryProblemSortedAndColumnsCountCharacters(t *testing.T) {
	src := "export default function scene(api) {\n  api.text('äöü', eval('1'));\n  fetch('x');\n}\n"
	res := BundleCodeScene([]byte(src), "s1", 0)
	if len(res.Errors) != 2 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	if res.Errors[0].Line != 2 || res.Errors[0].Column != 19 || res.Errors[1].Line != 3 || res.Errors[1].Column != 3 {
		t.Fatalf("positions: %+v", res.Errors)
	}
}

func TestBundleOutputIsDeterministic(t *testing.T) {
	src := readFixture(t, "testdata/scenes/s3.js")
	a := BundleCodeScene(src, "s3", 0)
	b := BundleCodeScene(src, "s3", 0)
	if !bytes.Equal(a.Code, b.Code) {
		t.Fatal("two bundles of the same source differ")
	}
}

func TestBundleAmbiguousSlashBothDirections(t *testing.T) {
	// Read as a regular expression, "/2; eval('1'); const b = 2/" would swallow the call; read as
	// a division it is plain code. The check sees it either way.
	src := "export default function scene(api) {\n  const a = (4)/2; eval('1'); const b = 2/1;\n}\n"
	if res := BundleCodeScene([]byte(src), "s1", 0); len(res.Errors) == 0 {
		t.Fatal("eval between two divisions was accepted")
	}
	src = "export default function scene(api) {\n  let n = 1; n++ /2; eval('1'); n /1;\n}\n"
	if res := BundleCodeScene([]byte(src), "s1", 0); len(res.Errors) == 0 {
		t.Fatal("eval after a postfix increment was accepted")
	}
}

func TestPrecheckNeverPanicsOrHangs(t *testing.T) {
	alphabet := []string{"/", "*", "`", "${", "}", "{", "\"", "'", "\\", "\x5cu", "\x5cu{", "\n", " ", "(", ")", "[", "]", "eval", "x", "1", ".", "+", "-", "=>",
		"import", "export", "default", "from", "ä", "\x00", "//", "/*", "*/", "constructor", "\xff"}
	done := make(chan struct{})
	go func() {
		defer close(done)
		rng := rand.New(rand.NewSource(1))
		for i := 0; i < 30000; i++ {
			var b strings.Builder
			for j, n := 0, rng.Intn(40); j < n; j++ {
				b.WriteString(alphabet[rng.Intn(len(alphabet))])
			}
			precheckSource([]byte(b.String()))
		}
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the pre-check did not finish")
	}
}
