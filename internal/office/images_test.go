package office

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func officeImageFixture(t *testing.T) []byte {
	t.Helper()
	var encoded bytes.Buffer
	pixels := image.NewRGBA(image.Rect(0, 0, 21, 21))
	for y := 0; y < 21; y++ {
		for x := 0; x < 21; x++ {
			pixels.Set(x, y, color.White)
			if (x+y)%3 == 0 {
				pixels.Set(x, y, color.Black)
			}
		}
	}
	if err := png.Encode(&encoded, pixels); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func TestOfficeImageProofWorkbook(t *testing.T) {
	picture := officeImageFixture(t)
	f, err := excelize.OpenReader(bytes.NewReader(editorFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = f.AddPictureFromBytes("Budget", "H1", &excelize.Picture{Extension: ".png", File: picture}); err != nil {
		t.Fatal(err)
	}
	if err = f.AddChart("Budget", "H8", &excelize.Chart{Type: excelize.Col, Series: []excelize.ChartSeries{{Name: "Budget!B1", Categories: "Budget!A2:A3", Values: "Budget!B2:B3"}}}); err != nil {
		t.Fatal(err)
	}
	buffer, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	parts, err := readOfficeParts(buffer.Bytes(), "xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	parts["customXml/vendor.xml"] = []byte(`<vendor xmlns="urn:vendor">opaque bytes</vendor>`)
	parts["xl/vbaProject.bin"] = []byte("opaque VBA: do not execute")
	original, err := writeOfficeParts(parts)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := InsertWorkbookImage(original, picture, WorkbookImageOptions{Sheet: "Budget", Anchor: "D8", Width: 210, Height: 210, Alt: "Test QR"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := readOfficeParts(saved, "xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range parts {
		if name == "[Content_Types].xml" || (strings.HasPrefix(name, "xl/drawings/") && !strings.HasSuffix(name, ".rels")) || strings.HasPrefix(name, "xl/drawings/_rels/") {
			continue
		}
		if !bytes.Equal(content, updated[name]) {
			t.Fatalf("untouched part changed during image insertion: %s", name)
		}
	}
	loaded, err := DecodeEditorWorkbook(saved)
	if err != nil {
		t.Fatal(err)
	}
	roundtrip, err := ApplyWorkbookEditorPatch(saved, WorkbookEditorPatch{SchemaVersion: 2, Workbook: loaded.Workbook, Charts: loaded.Charts})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := excelize.OpenReader(bytes.NewReader(roundtrip))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for _, anchor := range []string{"H1", "D8"} {
		pictures, readErr := reopened.GetPictures("Budget", anchor)
		if readErr != nil || len(pictures) != 1 || !bytes.Equal(pictures[0].File, picture) {
			t.Fatalf("picture at %s did not survive insertion/save/reopen: %d %v", anchor, len(pictures), readErr)
		}
	}
	reloaded, err := readOfficeParts(roundtrip, "xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range parts {
		if strings.HasPrefix(name, "xl/charts/") || strings.HasPrefix(name, "xl/media/") || name == "xl/vbaProject.bin" || strings.HasPrefix(name, "customXml/") {
			if !bytes.Equal(content, reloaded[name]) {
				t.Fatalf("opaque/chart/image part lost in roundtrip: %s", name)
			}
		}
	}
}

func TestOfficeImageProofDOCX(t *testing.T) {
	original, err := EncodeDOCX(Document{Text: "Keep paragraph one\nKeep paragraph two"})
	if err != nil {
		t.Fatal(err)
	}
	parts, err := ReadDOCXParts(original)
	if err != nil {
		t.Fatal(err)
	}
	parts["word/footnotes.xml"] = []byte(`<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:footnote w:id="1"><w:p/></w:footnote></w:footnotes>`)
	parts["customXml/vendor.bin"] = []byte{0, 255, 4, 9}
	original, err = writeOfficeParts(parts)
	if err != nil {
		t.Fatal(err)
	}
	result, err := InsertDOCXImage(original, officeImageFixture(t), DocumentImageOptions{Anchor: "end", Width: 210, Height: 210, Alt: `QR & "caption"`})
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateDOCX(result); err != nil {
		t.Fatal(err)
	}
	updated, err := ReadDOCXParts(result)
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range parts {
		if name == "word/document.xml" || name == "word/_rels/document.xml.rels" || name == "[Content_Types].xml" {
			continue
		}
		if !bytes.Equal(content, updated[name]) {
			t.Fatalf("DOCX opaque part changed: %s", name)
		}
	}
	document := updated["word/document.xml"]
	start := bytes.Index(document, []byte(`<w:p xmlns:w=`))
	end := bytes.Index(document[start:], []byte(`</w:p>`)) + start + len(`</w:p>`)
	if start < 0 || end < start || !bytes.Equal(append(append([]byte{}, document[:start]...), document[end:]...), parts["word/document.xml"]) {
		t.Fatal("DOCX insertion changed original document markup")
	}
	var drawing struct {
		Body struct {
			Paragraphs []struct {
				Run struct {
					Drawing struct {
						Inline struct {
							Extent struct {
								CX int `xml:"cx,attr"`
							} `xml:"extent"`
						} `xml:"inline"`
					} `xml:"drawing"`
				} `xml:"r"`
			} `xml:"p"`
		} `xml:"body"`
	}
	if err = xml.Unmarshal(document, &drawing); err != nil || drawing.Body.Paragraphs[len(drawing.Body.Paragraphs)-1].Run.Drawing.Inline.Extent.CX != 210*9525 {
		t.Fatalf("DOCX inline dimensions were not retained: %v", err)
	}
}

func TestWorkbookEditorImagesRoundTrip(t *testing.T) {
	original := editorFixture(t)
	doc, err := DecodeEditorWorkbook(original)
	if err != nil {
		t.Fatal(err)
	}
	id := doc.Workbook.SheetOrder[0]
	image := EditorImage{ID: "qr-test", Sheet: id, Anchor: "D8", Width: 210, Height: 210, MIME: "image/png", Data: base64.StdEncoding.EncodeToString(officeImageFixture(t)), Alt: "QR code"}
	saved, err := ApplyWorkbookEditorPatch(original, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts, Images: []EditorImage{image}})
	if err != nil {
		t.Fatal(err)
	}
	doc, err = DecodeEditorWorkbook(saved)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Images) != 1 || doc.Images[0] != image || doc.StructuralLocked {
		t.Fatalf("managed image snapshot: %+v locked=%v", doc.Images, doc.StructuralLocked)
	}
	before, _ := readOfficeParts(saved, "xl/workbook.xml")
	doc.Images[0].X, doc.Images[0].Y, doc.Images[0].Width, doc.Images[0].Height = 17, 23, 300, 300
	doc.Charts = []EditorChart{{ID: "with-image", Sheet: id, Anchor: "H2", Width: 480, Height: 280, Type: "column", Range: "A1:B3", Legend: true}}
	moved, err := ApplyWorkbookEditorPatch(saved, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts, Images: doc.Images})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := DecodeEditorWorkbook(moved)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Images) != 1 || loaded.Images[0].X != 17 || loaded.Images[0].Y != 23 || loaded.Images[0].Width != 300 || len(loaded.Charts) != 1 {
		t.Fatalf("move/resize with chart: %+v", loaded)
	}
	// An older client omitting the field may edit cells without deleting images.
	legacy, err := ApplyWorkbookEditorPatch(moved, WorkbookEditorPatch{SchemaVersion: 2, Workbook: loaded.Workbook, Charts: loaded.Charts})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err = DecodeEditorWorkbook(legacy)
	if err != nil || len(loaded.Images) != 1 {
		t.Fatalf("omitted images did not preserve: %+v %v", loaded.Images, err)
	}
	loaded.Workbook.Sheets[id].Name = "Renamed"
	loaded.Images[0].Anchor = "D10"
	structural, err := ApplyWorkbookEditorPatch(legacy, WorkbookEditorPatch{SchemaVersion: 2, Workbook: loaded.Workbook, Charts: loaded.Charts, Images: loaded.Images, Operations: []WorkbookStructureOperation{{Type: "insertRows", Sheet: id, Index: 7, Count: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err = DecodeEditorWorkbook(structural)
	if err != nil || loaded.Images[0].Anchor != "D10" || loaded.Workbook.Sheets[id].Name != "Renamed" {
		t.Fatalf("structural image anchor: %+v %v", loaded.Images, err)
	}
	deleted, err := ApplyWorkbookEditorPatch(structural, WorkbookEditorPatch{SchemaVersion: 2, Workbook: loaded.Workbook, Charts: loaded.Charts, Images: []EditorImage{}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err = DecodeEditorWorkbook(deleted)
	if err != nil || len(loaded.Images) != 0 {
		t.Fatalf("explicit empty images did not delete managed anchor: %+v %v", loaded.Images, err)
	}
	for name, content := range before {
		if strings.HasPrefix(name, "xl/media/") {
			parts, _ := readOfficeParts(moved, "xl/workbook.xml")
			if !bytes.Equal(content, parts[name]) {
				t.Fatalf("move changed image bytes: %s", name)
			}
		}
	}
}

func TestWorkbookEditorPreservesForeignDrawings(t *testing.T) {
	f, err := excelize.OpenReader(bytes.NewReader(editorFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = f.AddPictureFromBytes("Budget", "H1", &excelize.Picture{Extension: ".png", File: officeImageFixture(t)}); err != nil {
		t.Fatal(err)
	}
	buffer, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	parts, err := readOfficeParts(buffer.Bytes(), "xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	drawing := ""
	for name := range parts {
		if strings.HasPrefix(name, "xl/drawings/") && strings.HasSuffix(name, ".xml") {
			drawing = name
			break
		}
	}
	opaque := []byte(`<vendor:opaque xmlns:vendor="urn:vendor">keep drawing bytes</vendor:opaque>`)
	parts[drawing], err = appendOfficeChild(parts[drawing], opaque, "wsDr")
	if err != nil {
		t.Fatal(err)
	}
	original, err := writeOfficeParts(parts)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := DecodeEditorWorkbook(original)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.StructuralLocked || len(doc.Images) != 1 || !doc.Images[0].Readonly {
		t.Fatalf("foreign picture guard: %+v", doc)
	}
	result, err := ApplyWorkbookEditorPatch(original, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts, Images: []EditorImage{}})
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := readOfficeParts(result, "xl/workbook.xml")
	if !bytes.Equal(parts[drawing], saved[drawing]) {
		t.Fatal("foreign drawing was rewritten or removed")
	}
	doc.Images[0].Width++
	if _, err = ApplyWorkbookEditorPatch(original, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts, Images: doc.Images}); err == nil {
		t.Fatal("foreign image edit accepted")
	}
	doc.Workbook.Sheets[doc.Workbook.SheetOrder[0]].Name = "Unsafe rename"
	if _, err = ApplyWorkbookEditorPatch(original, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts}); err == nil {
		t.Fatal("foreign image structural edit accepted")
	}
}

func TestOfficeImageValidation(t *testing.T) {
	docx, err := EncodeDOCX(Document{Text: "Keep content"})
	if err != nil {
		t.Fatal(err)
	}
	xlsx := editorFixture(t)
	for _, data := range [][]byte{nil, []byte("invalid image"), officeImageFixture(t)[:40]} {
		if _, err = InsertDOCXImage(docx, data, DocumentImageOptions{Anchor: "end", Width: 100, Height: 100}); err == nil {
			t.Fatal("invalid DOCX image accepted")
		}
		if _, err = InsertWorkbookImage(xlsx, data, WorkbookImageOptions{Sheet: "Budget", Anchor: "A1", Width: 100, Height: 100}); err == nil {
			t.Fatal("invalid XLSX image accepted")
		}
	}
	if _, err = InsertDOCXImage(docx, officeImageFixture(t), DocumentImageOptions{Anchor: "cursor", Width: 100, Height: 100}); err == nil {
		t.Fatal("unsupported DOCX anchor accepted")
	}
	if _, err = InsertWorkbookImage(xlsx, officeImageFixture(t), WorkbookImageOptions{Sheet: "Budget", Anchor: "XFE1", Width: 100, Height: 100}); err == nil {
		t.Fatal("out-of-bounds cell accepted")
	}
	if _, err = InsertWorkbookImage(xlsx, officeImageFixture(t), WorkbookImageOptions{Sheet: "Budget", Anchor: "A1", Width: 5000, Height: 100}); err == nil {
		t.Fatal("oversized insertion accepted")
	}
}

func TestOfficeImageNestedDrawingRelationships(t *testing.T) {
	pixels := officeImageFixture(t)
	first, err := InsertWorkbookImage(editorFixture(t), pixels, WorkbookImageOptions{Sheet: "Budget", Anchor: "D8", Width: 210, Height: 210})
	if err != nil {
		t.Fatal(err)
	}
	parts, err := readOfficeParts(first, "xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	oldDrawing, newDrawing := "xl/drawings/aurago-drawing-1.xml", "xl/drawings/vendor/drawing.xml"
	parts[newDrawing] = parts[oldDrawing]
	delete(parts, oldDrawing)
	parts[officeRelationshipPart(newDrawing)] = bytes.ReplaceAll(parts[officeRelationshipPart(oldDrawing)], []byte("../media/"), []byte("../../media/"))
	delete(parts, officeRelationshipPart(oldDrawing))
	sheetRel := officeRelationshipPart(sheetPartByName(parts, "Budget"))
	parts[sheetRel] = bytes.ReplaceAll(parts[sheetRel], []byte("../drawings/aurago-drawing-1.xml"), []byte("../drawings/vendor/drawing.xml"))
	parts["[Content_Types].xml"] = bytes.ReplaceAll(parts["[Content_Types].xml"], []byte("/"+oldDrawing), []byte("/"+newDrawing))
	original, err := writeOfficeParts(parts)
	if err != nil {
		t.Fatal(err)
	}
	inserted, err := InsertWorkbookImage(original, pixels, WorkbookImageOptions{Sheet: "Budget", Anchor: "F4", Width: 210, Height: 210})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := DecodeEditorWorkbook(inserted)
	if err != nil || len(doc.Images) != 2 {
		t.Fatalf("nested drawing images: %+v %v", doc.Images, err)
	}
	for _, image := range doc.Images {
		if image.Data != base64.StdEncoding.EncodeToString(pixels) {
			t.Fatal("nested drawing lost embedded pixels")
		}
	}
}

func TestWorkbookEditorTreatsChangedManagedImageAsOpaque(t *testing.T) {
	original, err := InsertWorkbookImage(editorFixture(t), officeImageFixture(t), WorkbookImageOptions{Sheet: "Budget", Anchor: "D8", Width: 210, Height: 210})
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct{ name, old, replacement string }{
		{"aspect-lock", `noChangeAspect="1"`, `noChangeAspect="0"`},
		{"oversized", `cx="2000250"`, `cx="47625000"`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			parts, err := readOfficeParts(original, "xl/workbook.xml")
			if err != nil {
				t.Fatal(err)
			}
			drawing := "xl/drawings/aurago-drawing-1.xml"
			parts[drawing] = bytes.ReplaceAll(parts[drawing], []byte(scenario.old), []byte(scenario.replacement))
			changed, err := writeOfficeParts(parts)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := DecodeEditorWorkbook(changed)
			if err != nil || len(doc.Images) != 1 || !doc.Images[0].Readonly || !doc.StructuralLocked {
				t.Fatalf("changed managed image guard: %+v %v", doc.Images, err)
			}
			saved, err := ApplyWorkbookEditorPatch(changed, WorkbookEditorPatch{SchemaVersion: 2, Workbook: doc.Workbook, Charts: doc.Charts, Images: []EditorImage{}})
			if err != nil {
				t.Fatal(err)
			}
			after, err := readOfficeParts(saved, "xl/workbook.xml")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(parts[drawing], after[drawing]) {
				t.Fatal("changed image drawing was regenerated or dropped")
			}
		})
	}
}
