package office

import (
	"fmt"
	"strings"
	"testing"
)

func TestWorkbookPreflightRejectsSparseAllocationBeforeDecode(t *testing.T) {
	for _, body := range []string{
		`<worksheet><sheetData><row r="100001"><c r="A100001"/></row></sheetData></worksheet>`,
		`<worksheet><sheetData><row r="1"><c r="XFE1"/></row></sheetData></worksheet>`,
		`<worksheet><sheetData><row r="1"><c r="A2"/></row></sheetData></worksheet>`,
	} {
		if err := validateWorkbookAllocation(map[string][]byte{"xl/worksheets/sheet1.xml": []byte(body)}); err == nil {
			t.Fatal("oversized/malformed sheet accepted")
		}
	}
	var body strings.Builder
	body.WriteString("<worksheet><sheetData>")
	for i := 1; i <= 62; i++ {
		fmt.Fprintf(&body, `<row r="%d"><c r="XFD%d"/></row>`, i, i)
	}
	body.WriteString("</sheetData></worksheet>")
	if err := validateWorkbookAllocation(map[string][]byte{"xl/worksheets/sheet1.xml": []byte(body.String())}); err == nil {
		t.Fatal("expanded cell budget accepted")
	}
}
