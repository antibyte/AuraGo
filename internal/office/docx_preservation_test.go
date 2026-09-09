package office

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"
)

func TestDOCXLegacyRewriteRequiresLosslessRoundTrip(t *testing.T) {
	original, err := EncodeDOCX(Document{Text: "Simple content"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateDOCX(original); err != nil {
		t.Fatal(err)
	}
	if err := CheckLegacyDocumentRewrite("simple.docx", original); err != nil {
		t.Fatal(err)
	}
	parts, _ := ReadDOCXParts(original)
	parts["word/footnotes.xml"] = []byte("<notes/>")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range parts {
		w, _ := zw.Create(name)
		w.Write(data)
	}
	zw.Close()
	if !errors.Is(CheckLegacyDocumentRewrite("notes.docx", buf.Bytes()), ErrDOCXRequiresNativeEditor) {
		t.Fatal("Unknown parts were not protected")
	}
	for _, invalid := range [][]byte{nil, []byte("not docx")} {
		if ValidateDOCX(invalid) == nil {
			t.Fatal("Invalid DOCX accepted")
		}
	}
	buf.Reset()
	zw = zip.NewWriter(&buf)
	for _, name := range []string{"../outside.xml", "word/document.xml", "[Content_Types].xml"} {
		w, _ := zw.Create(name)
		w.Write([]byte("<x/>"))
	}
	zw.Close()
	if ValidateDOCX(buf.Bytes()) == nil {
		t.Fatal("Traversal package accepted")
	}
}

func TestDOCXRejectsLossyMetadataAndMultipleRoots(t *testing.T) {
	original, err := EncodeDOCX(Document{Title: "Metadata", Text: "Content"})
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"foreign-author", "multiple-roots"} {
		t.Run(scenario, func(t *testing.T) {
			parts, err := ReadDOCXParts(original)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "foreign-author" {
				parts["docProps/core.xml"] = bytes.Replace(parts["docProps/core.xml"], []byte("<dc:creator>AuraGo</dc:creator>"), []byte("<dc:creator>Original author</dc:creator>"), 1)
			} else {
				parts["word/document.xml"] = append(parts["word/document.xml"], []byte("<extra/>")...)
			}
			var buffer bytes.Buffer
			zw := zip.NewWriter(&buffer)
			for name, data := range parts {
				w, err := zw.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = w.Write(data); err != nil {
					t.Fatal(err)
				}
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			if scenario == "foreign-author" && !errors.Is(CheckLegacyDocumentRewrite("document.docx", buffer.Bytes()), ErrDOCXRequiresNativeEditor) {
				t.Fatal("Foreign metadata would be erased")
			}
			if scenario == "multiple-roots" && ValidateDOCX(buffer.Bytes()) == nil {
				t.Fatal("Multiple document roots accepted")
			}
		})
	}
}
