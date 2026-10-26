package office

import (
	"bytes"
	"context"
	"image/png"
	"strings"
	"testing"

	sharedqr "aurago/internal/qrcode"
	"github.com/makiuchi-d/gozxing"
	qr "github.com/makiuchi-d/gozxing/qrcode"
)

func TestOfficeImagesEmbedVerifiedSharedQR(t *testing.T) {
	const payload = "https://example.org/AuraGo-office-qr"
	rendered, err := sharedqr.Render(context.Background(), sharedqr.QRSpec{SchemaVersion: 1, Content: sharedqr.Content{Type: "url", URL: payload}, Style: sharedqr.Style{Size: 256}, Output: sharedqr.Output{Format: "png"}})
	if err != nil || !rendered.Metadata.Verified {
		t.Fatalf("shared QR render: %v", err)
	}
	document, err := EncodeDOCX(Document{Text: "Keep the original document"})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"docx", "xlsx"} {
		t.Run(kind, func(t *testing.T) {
			var saved []byte
			mainPart := "word/document.xml"
			if kind == "docx" {
				saved, err = InsertDOCXImage(document, rendered.Data, DocumentImageOptions{Anchor: "end", Width: 192, Height: 192, Alt: "QR code"})
			} else {
				mainPart = "xl/workbook.xml"
				saved, err = InsertWorkbookImage(editorFixture(t), rendered.Data, WorkbookImageOptions{Sheet: "Budget", Anchor: "D8", Width: 192, Height: 192, Alt: "QR code"})
			}
			if err != nil {
				t.Fatal(err)
			}
			parts, err := readOfficeParts(saved, mainPart)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for name, data := range parts {
				if !strings.Contains(name, "/media/aurago-image-") {
					continue
				}
				found = true
				pixels, err := png.Decode(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				bitmap, err := gozxing.NewBinaryBitmapFromImage(pixels)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := qr.NewQRCodeReader().DecodeWithoutHints(bitmap)
				if err != nil || decoded.GetText() != payload {
					t.Fatalf("embedded QR decode: %v", err)
				}
			}
			if !found {
				t.Fatal("embedded image is missing")
			}
		})
	}
}
