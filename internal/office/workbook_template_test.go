package office

import (
	"bytes"
	"strings"
	"testing"
)

func TestLegacyWorkbookRewriteRejectsTemplates(t *testing.T) {
	original := editorFixture(t)
	for _, name := range []string{"template.xltx", "template.xltm", "TEMPLATE.XLTM"} {
		before := bytes.Clone(original)
		if _, err := DecodeWorkbook(name, original); err != nil {
			t.Fatalf("template must remain readable: %v", err)
		}
		if err := CheckLegacyWorkbookRewrite(name, original); err == nil || !strings.Contains(err.Error(), "templates") {
			t.Fatalf("%s rewrite was accepted: %v", name, err)
		}
		if !bytes.Equal(original, before) {
			t.Fatal("original template changed")
		}
	}
}
