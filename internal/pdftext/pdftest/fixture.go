// Package pdftest builds small synthetic PDFs for extraction-boundary tests.
package pdftest

import (
	"bytes"
	"compress/zlib"
	"encoding/ascii85"
	"fmt"
)

func Document(pageCount string, content []byte, streamOptions string) []byte {
	return document(pageCount, content, streamOptions, "4 0 R")
}

// NestedDecoderLimit crosses the aggregate decoding budget using a small input.
func NestedDecoderLimit() []byte {
	raw := bytes.Repeat([]byte(" "), 12<<20)
	encoded := make([]byte, ascii85.MaxEncodedLen(len(raw)))
	encoded = encoded[:ascii85.Encode(encoded, raw)]
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	w.Write(encoded)
	w.Close()
	return document("1", compressed.Bytes(), "/Filter [/FlateDecode /ASCII85Decode]", "[4 0 R 4 0 R 4 0 R 4 0 R 4 0 R]")
}

func document(pageCount string, content []byte, streamOptions, contents string) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Count %s /Kids [3 0 R] >>", pageCount),
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /Resources <<>> /Contents %s >>", contents),
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
