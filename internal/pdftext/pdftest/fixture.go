// Package pdftest builds small synthetic PDFs for extraction-boundary tests.
package pdftest

import (
	"bytes"
	"fmt"
)

func Document(pageCount string, content []byte, streamOptions string) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Count %s /Kids [3 0 R] >>", pageCount),
		"<< /Type /Page /Parent 2 0 R /Resources <<>> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d %s >>\nstream\n%s\nendstream", len(content), streamOptions, content),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}
