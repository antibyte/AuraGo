package pdftext

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"strings"
	"testing"

	"aurago/internal/pdftext/pdftest"
)

func TestExtractBoundsAndCancellation(t *testing.T) {
	valid := pdftest.Document("1", []byte("BT (Synthetic PDF text) Tj ET"), "")
	compressed := func(content string) []byte {
		var b bytes.Buffer
		w := zlib.NewWriter(&b)
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	for _, tc := range []struct {
		name string
		data []byte
		want error
	}{
		{"plain text", valid, nil},
		{"compressed text", pdftest.Document("1", compressed("BT (Synthetic PDF text) Tj ET"), "/Filter /FlateDecode"), nil},
		{"page count", pdftest.Document("2147483647", nil, ""), ErrLimit},
		{"page cycle", bytes.Replace(valid, []byte("/Kids [3 0 R]"), []byte("/Kids [2 0 R]"), 1), ErrLimit},
		{"parent cycle", bytes.Replace(bytes.Replace(valid, []byte("/Resources <<>>"), []byte("/Resources null"), 1), []byte("/Parent 2 0 R"), []byte("/Parent 3 0 R"), 1), ErrLimit},
		{"declared xref size", bytes.Replace(valid, []byte("/Size 5"), []byte("/Size 9223372036854775807"), 1), ErrLimit},
		{"predictor allocation", pdftest.Document("1", compressed(""), "/Filter /FlateDecode /DecodeParms << /Predictor 12 /Columns 9223372036854775807 >>"), ErrLimit},
		{"compressed stream limit", pdftest.Document("1", compressed(strings.Repeat(" ", (16<<20)+1)), "/Filter /FlateDecode"), ErrLimit},
		{"text limit", pdftest.Document("1", compressed("BT ("+strings.Repeat("a", (8<<20)+1)+") Tj ET"), "/Filter /FlateDecode"), ErrLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text, err := Extract(context.Background(), bytes.NewReader(tc.data), int64(len(tc.data)))
			if !errors.Is(err, tc.want) || (tc.want != nil && text != "") || (tc.want == nil && text != "Synthetic PDF text") {
				t.Fatalf("text bytes=%d, err=%v, want %v", len(text), err, tc.want)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if text, err := Extract(ctx, bytes.NewReader(valid), int64(len(valid))); !errors.Is(err, context.Canceled) || text != "" {
		t.Fatalf("cancelled extraction: %q, %v", text, err)
	}
	if _, err := Extract(context.Background(), bytes.NewReader(valid), MaxInputBytes+1); !errors.Is(err, ErrLimit) {
		t.Fatalf("oversized input: %v", err)
	}
}
