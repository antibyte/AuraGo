package pdftext

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"testing"

	"aurago/internal/pdftext/pdftest"
)

func TestExtractDecoderLimits(t *testing.T) {
	compressedPDF := func(content []byte, params string) []byte {
		t.Helper()
		var compressed bytes.Buffer
		w := zlib.NewWriter(&compressed)
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		return pdftest.Document("1", compressed.Bytes(), "/Filter /FlateDecode /DecodeParms << "+params+" >>")
	}
	predictorRow := make([]byte, 65)
	predictorRow[0] = 2
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"zero predictor columns", compressedPDF(bytes.Repeat([]byte{2}, (16<<20)+1), "/Predictor 12 /Columns 0")},
		{"predictor hides inflated row bytes", compressedPDF(bytes.Repeat(predictorRow, (16<<20)/len(predictorRow)+1), "/Predictor 12 /Columns 64")},
		{"nested aggregate decoding", pdftest.NestedDecoderLimit()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text, err := Extract(t.Context(), bytes.NewReader(tc.data), int64(len(tc.data)))
			if !errors.Is(err, ErrLimit) || text != "" {
				t.Fatalf("input=%d bytes, text=%d bytes, err=%v; want terminal limit", len(tc.data), len(text), err)
			}
		})
	}

	const content = "BT (Synthetic PDF text) Tj ET"
	var predicted []byte
	var previous byte
	for _, c := range []byte(content) {
		predicted = append(predicted, 2, c-previous)
		previous = c
	}
	for _, tc := range []struct {
		name, params string
		content      []byte
	}{
		{"default columns", "/Predictor 12", predicted},
		{"no prediction", "/Predictor 1", []byte(content)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := compressedPDF(tc.content, tc.params)
			text, err := Extract(context.Background(), bytes.NewReader(data), int64(len(data)))
			if err != nil || text != "Synthetic PDF text" {
				t.Fatalf("text=%q err=%v", text, err)
			}
		})
	}
}
