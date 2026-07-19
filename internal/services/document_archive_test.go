package services

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type testDocumentArchiveEntry struct {
	name string
	body string
	size int64
	mode os.FileMode
}

type zeroArchiveReader struct{}

func (zeroArchiveReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = 0
	}
	return len(buffer), nil
}

func writeTestDocumentArchive(t *testing.T, extension string, entries []testDocumentArchiveEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "document"+extension)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		member, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("create archive member %q: %v", entry.name, err)
		}
		if entry.size > 0 {
			if _, err := io.CopyN(member, zeroArchiveReader{}, entry.size); err != nil {
				t.Fatalf("write archive member %q: %v", entry.name, err)
			}
		} else if _, err := io.WriteString(member, entry.body); err != nil {
			t.Fatalf("write archive member %q: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close archive writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}
	return path
}

func TestDocumentArchiveValidationAndExtraction(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		required  string
		mimetype  string
		entries   []testDocumentArchiveEntry
		wantText  string
	}{
		{
			name:      "docx",
			extension: ".docx",
			required:  "word/document.xml",
			entries: []testDocumentArchiveEntry{{
				name: "word/document.xml",
				body: `<w:document xmlns:w="urn:w"><w:body><w:p><w:t>Hello DOCX</w:t></w:p></w:body></w:document>`,
			}},
			wantText: "Hello DOCX",
		},
		{
			name:      "xlsx",
			extension: ".xlsx",
			required:  "xl/workbook.xml",
			entries: []testDocumentArchiveEntry{
				{name: "xl/workbook.xml", body: `<workbook/>`},
				{name: "xl/worksheets/sheet1.xml", body: `<worksheet><sheetData><v>42</v></sheetData></worksheet>`},
			},
			wantText: "42",
		},
		{
			name:      "pptx",
			extension: ".pptx",
			required:  "ppt/presentation.xml",
			entries: []testDocumentArchiveEntry{
				{name: "ppt/presentation.xml", body: `<presentation/>`},
				{name: "ppt/slides/slide1.xml", body: `<p:sld xmlns:p="urn:p" xmlns:a="urn:a"><a:t>Hello PPTX</a:t></p:sld>`},
			},
			wantText: "Hello PPTX",
		},
		{
			name:      "odf",
			extension: ".odt",
			required:  "content.xml",
			mimetype:  "application/vnd.oasis.opendocument.text",
			entries: []testDocumentArchiveEntry{
				{name: "mimetype", body: "application/vnd.oasis.opendocument.text"},
				{name: "content.xml", body: `<office:document-content xmlns:office="urn:office" xmlns:text="urn:text"><text:p>Hello ODF</text:p></office:document-content>`},
			},
			wantText: "Hello ODF",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeTestDocumentArchive(t, test.extension, test.entries)
			if err := ValidateDocumentArchive(path, test.required, test.mimetype); err != nil {
				t.Fatalf("ValidateDocumentArchive: %v", err)
			}
			text, err := ExtractText(path)
			if err != nil {
				t.Fatalf("ExtractText: %v", err)
			}
			if !strings.Contains(text, test.wantText) {
				t.Fatalf("ExtractText = %q, want content %q", text, test.wantText)
			}
		})
	}
}

func TestDocumentArchiveRejectsUnsafeMembers(t *testing.T) {
	tests := []testDocumentArchiveEntry{
		{name: "../word/document.xml", body: `<document/>`},
		{name: "/word/document.xml", body: `<document/>`},
		{name: "C:/word/document.xml", body: `<document/>`},
		{name: `word\document.xml`, body: `<document/>`},
		{name: "word/document.xml", body: `<document/>`, mode: os.ModeSymlink | 0o777},
	}
	for _, entry := range tests {
		t.Run(entry.name, func(t *testing.T) {
			path := writeTestDocumentArchive(t, ".docx", []testDocumentArchiveEntry{entry})
			err := ValidateDocumentArchive(path, "word/document.xml", "")
			if !errors.Is(err, ErrDocumentArchiveUnsafe) {
				t.Fatalf("ValidateDocumentArchive error = %v, want ErrDocumentArchiveUnsafe", err)
			}
		})
	}

	encrypted := &zip.File{FileHeader: zip.FileHeader{Name: "word/document.xml", Flags: 0x1}}
	if _, err := validateDocumentArchiveMember(encrypted); !errors.Is(err, ErrDocumentArchiveUnsafe) {
		t.Fatalf("encrypted member error = %v, want ErrDocumentArchiveUnsafe", err)
	}
	overflow := &zip.File{FileHeader: zip.FileHeader{Name: "word/document.xml", UncompressedSize64: ^uint64(0)}}
	if _, err := readDocumentArchiveMember(overflow, newDocumentArchiveBudget()); !errors.Is(err, ErrDocumentArchiveTooLarge) {
		t.Fatalf("overflow member error = %v, want ErrDocumentArchiveTooLarge", err)
	}
}

func TestDocumentArchiveEnforcesEntryAndExpansionLimits(t *testing.T) {
	t.Run("entry count", func(t *testing.T) {
		entries := make([]testDocumentArchiveEntry, 0, maxDocumentArchiveEntries+1)
		entries = append(entries, testDocumentArchiveEntry{name: "word/document.xml", body: `<document/>`})
		for index := 1; index <= maxDocumentArchiveEntries; index++ {
			entries = append(entries, testDocumentArchiveEntry{name: "media/item-" + strconv.Itoa(index) + ".bin"})
		}
		path := writeTestDocumentArchive(t, ".docx", entries)
		err := ValidateDocumentArchive(path, "word/document.xml", "")
		if !errors.Is(err, ErrDocumentArchiveTooLarge) {
			t.Fatalf("entry-count error = %v, want ErrDocumentArchiveTooLarge", err)
		}
	})

	t.Run("single relevant member", func(t *testing.T) {
		path := writeTestDocumentArchive(t, ".docx", []testDocumentArchiveEntry{{
			name: "word/document.xml",
			size: maxDocumentArchiveMemberBytes + 1,
		}})
		err := ValidateDocumentArchive(path, "word/document.xml", "")
		if !errors.Is(err, ErrDocumentArchiveTooLarge) {
			t.Fatalf("member-size error = %v, want ErrDocumentArchiveTooLarge", err)
		}
	})

	t.Run("cumulative relevant members", func(t *testing.T) {
		entries := []testDocumentArchiveEntry{{name: "xl/workbook.xml", body: `<workbook/>`}}
		for index := 1; index <= 8; index++ {
			entries = append(entries, testDocumentArchiveEntry{
				name: "xl/worksheets/sheet" + string(rune('0'+index)) + ".xml",
				size: maxDocumentArchiveMemberBytes,
			})
		}
		path := writeTestDocumentArchive(t, ".xlsx", entries)
		err := ValidateDocumentArchive(path, "xl/workbook.xml", "")
		if !errors.Is(err, ErrDocumentArchiveTooLarge) {
			t.Fatalf("cumulative-size error = %v, want ErrDocumentArchiveTooLarge", err)
		}
	})
}
