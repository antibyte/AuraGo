package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"aurago/internal/config"

	"golang.org/x/net/html"
)

// c102DecodeResult decodes a document_creator result and fails the test when it is not valid JSON.
func c102DecodeResult(t *testing.T, raw string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	return out
}

func c102MarotoConfig(outputDir string) *config.DocumentCreatorConfig {
	return &config.DocumentCreatorConfig{Enabled: true, Backend: "maroto", OutputDir: outputDir}
}

func TestDocumentCreatorErrorKeepsQuotesAndBackslashesValid(t *testing.T) {
	const message = `C:\data\x "y"`
	out := c102DecodeResult(t, documentCreatorError(message))
	if out["status"] != "error" || out["message"] != message {
		t.Fatalf("decoded = %+v", out)
	}
}

func TestDocumentCreatorUnknownOperationIsValidAndBounded(t *testing.T) {
	operation := `C:\data\x "y"` + strings.Repeat("ä", 500)
	out := c102DecodeResult(t, ExecuteDocumentCreator(context.Background(), c102MarotoConfig(t.TempDir()), operation, "", "", "", "", "", false, "", ""))
	message, _ := out["message"].(string)
	if out["status"] != "error" || !strings.HasPrefix(message, `unknown operation: C:\data\x "y"`) {
		t.Fatalf("decoded = %+v", out)
	}
	echoed := strings.TrimPrefix(message, "unknown operation: ")
	echoed = echoed[:strings.Index(echoed, ". Valid:")]
	if got := utf8.RuneCountInString(echoed); got > maxEchoedDocumentOperationRunes+1 {
		t.Fatalf("echoed operation has %d runes, want at most %d plus the ellipsis", got, maxEchoedDocumentOperationRunes)
	}
}

func TestDocumentCreatorCreateOutputDirErrorIsValidJSON(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(blocker, `sub "x"\dir`)
	out := c102DecodeResult(t, ExecuteDocumentCreator(context.Background(), c102MarotoConfig(outputDir), "create_pdf", "T", "Body", "", "", "", false, "", ""))
	if out["status"] != "error" || !strings.Contains(out["message"].(string), "create output dir") {
		t.Fatalf("decoded = %+v", out)
	}
}

func TestDocumentCreatorMarotoErrorsAreValidJSON(t *testing.T) {
	t.Run("invalid sections", func(t *testing.T) {
		out := c102DecodeResult(t, createPDFMaroto(t.TempDir(), "T", "", "x", "A4", false, `{"type":"te\x"}`))
		if out["status"] != "error" || !strings.Contains(out["message"].(string), "invalid sections JSON") {
			t.Fatalf("decoded = %+v", out)
		}
	})
	t.Run("save fails", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), `missing "dir"`, `sub\dir`)
		out := c102DecodeResult(t, createPDFMaroto(missing, "T", "Body", "x", "A4", false, ""))
		if out["status"] != "error" || !strings.Contains(out["message"].(string), "save PDF") {
			t.Fatalf("decoded = %+v", out)
		}
	})
}

func TestDocumentCreatorInvalidSourcePathIsValidJSON(t *testing.T) {
	cfg := &config.DocumentCreatorConfig{
		Enabled:   true,
		Backend:   "gotenberg",
		OutputDir: t.TempDir(),
		Gotenberg: config.GotenbergConfig{URL: "http://127.0.0.1:1"},
	}
	sources, _ := json.Marshal([]string{`..\..\..\C:\data\x "y".docx`})
	out := c102DecodeResult(t, ExecuteDocumentCreatorInWorkspace(context.Background(), cfg, t.TempDir(), "convert_document", "", "", "", "", "", false, "", string(sources)))
	if out["status"] != "error" || !strings.Contains(out["message"].(string), "invalid source path") {
		t.Fatalf("decoded = %+v", out)
	}
}

var c102DefaultNamePattern = regexp.MustCompile(`^doc_\d+_[0-9a-f]{6}$`)

func TestDefaultDocumentNamesAreUnique(t *testing.T) {
	first, second := defaultDocumentName(), defaultDocumentName()
	if first == second {
		t.Fatalf("two default names are equal: %q", first)
	}
	for _, name := range []string{first, second} {
		if !c102DefaultNamePattern.MatchString(name) {
			t.Fatalf("default name %q does not match %s", name, c102DefaultNamePattern)
		}
	}
}

func TestDefaultDocumentNamesDoNotOverwriteEachOther(t *testing.T) {
	t.Run("maroto", func(t *testing.T) {
		dir := t.TempDir()
		first := c102DecodeResult(t, createPDFMaroto(dir, "One", "Body", "", "A4", false, ""))
		second := c102DecodeResult(t, createPDFMaroto(dir, "Two", "Body", "", "A4", false, ""))
		if first["status"] != "success" || second["status"] != "success" || first["file_path"] == second["file_path"] {
			t.Fatalf("first = %+v, second = %+v", first, second)
		}
	})
	t.Run("gotenberg", func(t *testing.T) {
		dir := t.TempDir()
		firstPath, _, err := saveGotenbergOutput([]byte("one"), dir, "", ".pdf")
		if err != nil {
			t.Fatal(err)
		}
		secondPath, _, err := saveGotenbergOutput([]byte("two"), dir, "", ".pdf")
		if err != nil {
			t.Fatal(err)
		}
		if firstPath == secondPath {
			t.Fatalf("both outputs went to %s", firstPath)
		}
		if data, err := os.ReadFile(firstPath); err != nil || string(data) != "one" {
			t.Fatalf("first output = %q, %v", data, err)
		}
		if !c102DefaultNamePattern.MatchString(strings.TrimSuffix(filepath.Base(firstPath), ".pdf")) {
			t.Fatalf("output name %q does not match %s", filepath.Base(firstPath), c102DefaultNamePattern)
		}
	})
}

// ── block_remote_content ─────────────────────────────────────────────────────

const c102CSP = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:">`

// c102ParseRendered parses out the way Chromium does: a leading byte order mark is
// consumed by the decoder before the parser sees anything.
func c102ParseRendered(t *testing.T, out string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(strings.TrimPrefix(out, utf8ByteOrderMark)))
	if err != nil {
		t.Fatalf("parse %q: %v", out, err)
	}
	return doc
}

func c102Attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func c102FindElement(n *html.Node, name string) *html.Node {
	if n.Type == html.ElementNode && n.Data == name {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := c102FindElement(c, name); found != nil {
			return found
		}
	}
	return nil
}

// c102AssertPolicyFirstInHead checks that the parsed document's first head element is the policy.
func c102AssertPolicyFirstInHead(t *testing.T, out string) {
	t.Helper()
	head := c102FindElement(c102ParseRendered(t, out), "head")
	if head == nil {
		t.Fatalf("no head in %q", out)
	}
	first := head.FirstChild
	for first != nil && first.Type != html.ElementNode {
		first = first.NextSibling
	}
	if first == nil || first.Data != "meta" || c102Attr(first, "http-equiv") != "Content-Security-Policy" ||
		c102Attr(first, "content") != "default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:" {
		t.Fatalf("first head element of %q is %+v, want the policy meta", out, first)
	}
}

// c102AssertNoMetaRefresh checks that the parsed document holds no meta element with a refresh.
func c102AssertNoMetaRefresh(t *testing.T, out string) {
	t.Helper()
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" && strings.Contains(strings.ToLower(c102Attr(n, "http-equiv")), "refresh") {
			t.Fatalf("meta refresh survived in %q", out)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(c102ParseRendered(t, out))
}

func TestRemoteContentPolicyIsTheSpecifiedOne(t *testing.T) {
	if remoteContentCSPMeta != c102CSP {
		t.Fatalf("policy = %s, want %s", remoteContentCSPMeta, c102CSP)
	}
}

func TestRestrictRemoteContentPutsThePolicyFirstInHead(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"empty document", "", c102CSP},
		{"no head", `<p>Hello</p>`, c102CSP + `<p>Hello</p>`},
		{"no head after a doctype", `<!DOCTYPE html><html><body><p>x</p></body></html>`,
			`<!DOCTYPE html><html>` + c102CSP + `<body><p>x</p></body></html>`},
		{"upper-case head with attributes", `<html><HEAD lang=de><title>T</title></HEAD><body>x</body></html>`,
			`<html><HEAD lang=de>` + c102CSP + `<title>T</title></HEAD><body>x</body></html>`},
		{"doctype first", "<!DOCTYPE html>\n<html lang=\"de\">\n<head>\n<meta charset=\"utf-8\"><title>T</title></head>",
			"<!DOCTYPE html>\n<html lang=\"de\">\n<head>" + c102CSP + "\n<meta charset=\"utf-8\"><title>T</title></head>"},
		{"existing policy stays behind ours", `<html><head><meta http-equiv="Content-Security-Policy" content="img-src *"></head>`,
			`<html><head>` + c102CSP + `<meta http-equiv="Content-Security-Policy" content="img-src *"></head>`},
		{"head inside a comment before the real head", `<!-- <head> --><html><head><title>T</title></head>`,
			`<!-- <head> --><html><head>` + c102CSP + `<title>T</title></head>`},
		{"comment with a tag inside before the head", `<!-- a > <img src=http://10.0.0.1/x> --><head>`,
			`<!-- a > <img src=http://10.0.0.1/x> --><head>` + c102CSP},
		{"head inside a script before the real head", `<html><script>var s = "<head>";</script><head><title>T</title></head>`,
			`<html>` + c102CSP + `<script>var s = "<head>";</script><head><title>T</title></head>`},
		{"resource before a late head", `<img src="http://192.168.1.1/x.png"><head><title>T</title></head>`,
			c102CSP + `<img src="http://192.168.1.1/x.png"><head><title>T</title></head>`},
		{"self-closing head", `<html><head/><title>T</title>`, `<html><head/>` + c102CSP + `<title>T</title>`},
		{"abrupt empty comment", `<!--><head>`, `<!--><head>` + c102CSP},
		{"quoted > in the html tag", `<html data-x="a>b"><head>`, c102CSP + `<html data-x="a>b"><head>`},
		{"byte order mark stays first", "\xef\xbb\xbf  \n<head>", "\xef\xbb\xbf  \n<head>" + c102CSP},
		{"invalid UTF-8 cannot form a UTF-16 byte order mark", "\xff\xfe<p>x</p>", c102CSP + "\xef\xbf\xbd<p>x</p>"},
		{"ESC and NUL are dropped", "<html><head lang=\"\x1b$B\">\x00<title>T</title>", `<html><head lang="$B">` + c102CSP + `<title>T</title>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := restrictRemoteContent(tc.in)
			if got != tc.want {
				t.Fatalf("restrictRemoteContent(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
			c102AssertPolicyFirstInHead(t, got)
		})
	}
}

func TestRestrictRemoteContentNeutralizesMetaRefresh(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"double quotes", `<meta http-equiv="refresh" content="0;url=http://192.168.1.1/">`,
			`<!meta http-equiv="refresh" content="0;url=http://192.168.1.1/">`},
		{"upper case and single quotes", `<META HTTP-EQUIV='Refresh' CONTENT='0; URL=http://10.0.0.1/'>`,
			`<!META HTTP-EQUIV='Refresh' CONTENT='0; URL=http://10.0.0.1/'>`},
		{"other attribute order and extra attributes", `<meta content="0;url=http://lan/" data-x="1" http-equiv=REFRESH />`,
			`<!meta content="0;url=http://lan/" data-x="1" http-equiv=REFRESH />`},
		{"whitespace around the attribute", "<meta\nhttp-equiv\n=\n\"refresh\"\ncontent=0>", "<!meta\nhttp-equiv\n=\n\"refresh\"\ncontent=0>"},
		{"character reference in the value", `<meta http-equiv="&#114;efresh" content="0;url=http://lan/">`,
			`<!meta http-equiv="&#114;efresh" content="0;url=http://lan/">`},
		{"NUL inside the tag name", "<me\x00ta http-equiv=refresh content=0>", "<!meta http-equiv=refresh content=0>"},
		{"inside an SVG style element", `<svg><style><meta http-equiv="refresh" content="0;url=http://lan/"></style></svg>`,
			`<svg><style><!meta http-equiv="refresh" content="0;url=http://lan/"></style></svg>`},
		{"after a title that ends inside an attribute", `<title><meta a="</title><meta http-equiv=refresh content=0;url=http://lan/>">`,
			`<title><meta a="</title><!meta http-equiv=refresh content=0;url=http://lan/>">`},
		{"inside a comment", `<!-- <meta http-equiv=refresh content=0> --><p>x</p>`, `<!-- <!meta http-equiv=refresh content=0> --><p>x</p>`},
		{"unterminated at the end", `<p>x</p><meta http-equiv=refresh content=0`, `<p>x</p><!meta http-equiv=refresh content=0`},
		{"charset meta kept", `<meta charset="utf-8">`, `<meta charset="utf-8">`},
		{"viewport meta kept", `<meta name="viewport" content="width=device-width">`, `<meta name="viewport" content="width=device-width">`},
		{"content-type meta kept", `<meta http-equiv="Content-Type" content="text/html; charset=utf-8">`,
			`<meta http-equiv="Content-Type" content="text/html; charset=utf-8">`},
		{"refresh only in another attribute", `<meta name="description" content="refresh daily">`, `<meta name="description" content="refresh daily">`},
		{"metadata is another tag", `<metadata http-equiv="refresh">`, `<metadata http-equiv="refresh">`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := neutralizeMetaRefresh(sanitizeRenderText(tc.in)); got != tc.want {
				t.Fatalf("neutralizeMetaRefresh(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
			out := restrictRemoteContent("<html><head></head><body>" + tc.in + "</body></html>")
			c102AssertNoMetaRefresh(t, out)
			c102AssertPolicyFirstInHead(t, out)
		})
	}
}

func TestNeutralizeMetaRefreshFailsClosed(t *testing.T) {
	t.Run("tag longer than the scan window", func(t *testing.T) {
		in := `<meta data-pad="` + strings.Repeat("x", metaScanWindow) + `" http-equiv="refresh" content="0;url=http://lan/">`
		if got := neutralizeMetaRefresh(in); got != "<!"+in[1:] {
			t.Fatalf("oversized refresh tag kept: %.80q", got)
		}
	})
	t.Run("more tags than the scan limit", func(t *testing.T) {
		harmless := `<meta charset="utf-8">`
		in := strings.Repeat(harmless, metaScanLimit+1)
		want := strings.Repeat(harmless, metaScanLimit) + "<!" + harmless[1:]
		if got := neutralizeMetaRefresh(in); got != want {
			t.Fatalf("tags beyond the scan limit were kept")
		}
	})
	t.Run("hostile input", func(t *testing.T) {
		in := strings.Repeat(`<meta a="`, 200000)
		if got := strings.Count(neutralizeMetaRefresh(in), `<!meta a="`); got != 200000 {
			t.Fatalf("neutralized %d of 200000 unterminated tags", got)
		}
	})
}

func TestRestrictRemoteMarkdownNeutralizesRefreshWithoutAPolicy(t *testing.T) {
	got := restrictRemoteMarkdown("# Hi\n\n<meta http-equiv=\"refresh\" content=\"0;url=http://lan/\">\n\n![x](http://lan/x.png)\n")
	want := "# Hi\n\n<!meta http-equiv=\"refresh\" content=\"0;url=http://lan/\">\n\n![x](http://lan/x.png)\n"
	if got != want {
		t.Fatalf("restrictRemoteMarkdown = %q, want %q", got, want)
	}
}

// c102GotenbergCall is one request the fake Gotenberg received.
type c102GotenbergCall struct {
	route string
	files map[string]string
}

// c102FakeGotenberg starts a Gotenberg stand-in that records the uploaded files and answers
// every conversion with a tiny PDF, and returns a gotenberg-backed document_creator config.
func c102FakeGotenberg(t *testing.T) (*config.DocumentCreatorConfig, func() []c102GotenbergCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []c102GotenbergCall
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		call := c102GotenbergCall{route: r.URL.Path, files: map[string]string{}}
		for _, header := range r.MultipartForm.File["files"] {
			file, err := header.Open()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			data, _ := io.ReadAll(file)
			file.Close()
			call.files[header.Filename] = string(data)
		}
		mu.Lock()
		calls = append(calls, call)
		mu.Unlock()
		_, _ = w.Write([]byte("%PDF-1.4 fake"))
	}))
	t.Cleanup(server.Close)
	cfg := &config.DocumentCreatorConfig{
		Enabled:   true,
		Backend:   "gotenberg",
		OutputDir: t.TempDir(),
		Gotenberg: config.GotenbergConfig{URL: server.URL},
	}
	return cfg, func() []c102GotenbergCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]c102GotenbergCall(nil), calls...)
	}
}

func TestDocumentCreatorBlockRemoteContentReachesEveryChromiumRender(t *testing.T) {
	const page = `<html><head><title>T</title></head><body><img src="http://192.168.1.1/x.png">` +
		`<meta http-equiv="refresh" content="0;url=http://192.168.1.1/"></body></html>`
	const markdown = "# T\n\n<meta http-equiv=\"refresh\" content=\"0;url=http://192.168.1.1/\">\n"
	for _, tc := range []struct {
		operation, content, sections, route string
	}{
		{operation: "html_to_pdf", content: page, route: "/forms/chromium/convert/html"},
		{operation: "screenshot_html", content: page, route: "/forms/chromium/screenshot/html"},
		{operation: "markdown_to_pdf", content: markdown, route: "/forms/chromium/convert/markdown"},
		{operation: "create_pdf", content: "Hallo", sections: `[{"type":"text","header":"H","body":"B"}]`, route: "/forms/chromium/convert/html"},
	} {
		t.Run(tc.operation, func(t *testing.T) {
			for _, block := range []bool{false, true} {
				cfg, calls := c102FakeGotenberg(t)
				var raw string
				if block {
					raw = ExecuteDocumentCreator(context.Background(), cfg, tc.operation, "T", tc.content, "", "", "A4", false, tc.sections, "",
						DocumentCreatorOptions{BlockRemoteContent: true})
				} else {
					raw = ExecuteDocumentCreator(context.Background(), cfg, tc.operation, "T", tc.content, "", "", "A4", false, tc.sections, "")
				}
				if out := c102DecodeResult(t, raw); out["status"] != "success" {
					t.Fatalf("block=%v: result = %+v", block, out)
				}
				got := calls()
				if len(got) != 1 || got[0].route != tc.route {
					t.Fatalf("block=%v: calls = %+v, want one call to %s", block, got, tc.route)
				}
				index := got[0].files["index.html"]
				if !block {
					if strings.Contains(index, "Content-Security-Policy") {
						t.Fatalf("default render got the policy: %q", index)
					}
					if tc.operation == "html_to_pdf" && index != page {
						t.Fatalf("default render changed the page: %q", index)
					}
					continue
				}
				c102AssertPolicyFirstInHead(t, index)
				c102AssertNoMetaRefresh(t, index)
				if tc.operation == "markdown_to_pdf" && !strings.Contains(got[0].files["content.md"], "<!meta http-equiv") {
					t.Fatalf("markdown kept its refresh: %q", got[0].files["content.md"])
				}
			}
		})
	}
}
