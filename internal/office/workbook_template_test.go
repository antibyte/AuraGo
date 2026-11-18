package office

import (
	"bytes"
	"errors"
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

func TestLegacyWorkbookRewriteChecksExtensionlessFiles(t *testing.T) {
	original := editorFixture(t)
	for _, name := range []string{"Workbook", "old.files/Workbook", `old.files\Workbook`} {
		if err := CheckLegacyWorkbookRewrite(name, original); !errors.Is(err, ErrWorkbookRequiresNativeEditor) {
			t.Errorf("legacy rewrite of %s should preserve its native contents: %v", name, err)
		}
	}
	simple, err := EncodeWorkbook(Workbook{})
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{simple, []byte("Plain text notes")} {
		if err := CheckLegacyWorkbookRewrite("Notes", data); err != nil {
			t.Fatalf("simple extensionless file rejected: %v", err)
		}
	}
}
