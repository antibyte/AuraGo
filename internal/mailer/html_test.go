package mailer

import (
	"strings"
	"testing"
)

func TestMailerHTMLSanitizesActiveContentAndKeepsMailStructure(t *testing.T) {
	source := `<base href="https://evil.example"><p onclick="steal()">Hello <strong>World</strong></p><script>credential()</script><style>body{background:url(https://tracker.example)}</style><form action="https://evil.example"><input value="send"></form><svg><a href="javascript:steal()">attack</a></svg><table width="100%"><tr><td colspan="2">Details</td></tr></table><img src="https://images.example/p.png" onerror="steal()"><img src="cid:photo@example"><a href="javascript&#58;alert(1)">bad</a><a href="https://example.test">good</a><img srcset="https://tracker.example 2x" src="data:image/svg+xml,bad">`
	got, err := SanitizeHTML(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"credential", "steal", "onclick", "onerror", "javascript", "<svg", "<form", "<style", "srcset", "data:image", "<base", "tracker.example"} {
		if strings.Contains(got, bad) {
			t.Fatalf("active HTML survived %q", bad)
		}
	}
	for _, keep := range []string{"<strong>World</strong>", "<table", "colspan=\"2\"", "https://images.example/p.png", "cid:photo@example", "noopener noreferrer"} {
		if !strings.Contains(got, keep) {
			t.Fatalf("mail structure lost %q: %s", keep, got)
		}
	}
}

func TestMailerHTMLRejectsDangerousURLVariants(t *testing.T) {
	for _, raw := range []string{"//tracker.example/a", " JaVaScRiPt:alert(1)", "java\nscript:alert(1)", "file:///etc/passwd", "https://name:pass@example.test/image", "data:text/html,hello", "blob:untrusted"} {
		if safeMailURL(raw, true) {
			t.Fatalf("allowed unsafe image URI %q", raw)
		}
	}
	if _, err := SanitizeHTML(strings.Repeat("x", maxMailHTMLBytes+1)); err == nil {
		t.Fatal("unbounded mail HTML accepted")
	}
	if _, err := SanitizeHTML(strings.Repeat("<div>", 70) + "deep" + strings.Repeat("</div>", 70)); err == nil {
		t.Fatal("unbounded nesting accepted")
	}
}

func TestMailerHTMLSanitizationIsStable(t *testing.T) {
	first, err := SanitizeHTML(`<p title="A &amp; B">A &lt; B <a href="https://example.test">open</a></p>`)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SanitizeHTML(first)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("sanitization changes stable mail: %q / %q", first, second)
	}
}
