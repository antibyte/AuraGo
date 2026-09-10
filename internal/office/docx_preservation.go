package office

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
)

const DOCXMIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

var ErrDOCXRequiresNativeEditor = errors.New("DOCX contains formatting or review data that the legacy editor cannot preserve; edit it in Autor or save an explicit copy")

// ReadDOCXParts bounds package expansion before XML parsing or preservation checks.
func ReadDOCXParts(data []byte) (map[string][]byte, error) {
	return readOfficeParts(data, "word/document.xml")
}

func readOfficeParts(data []byte, mainPart string) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("read DOCX package: %w", err)
	}
	if len(reader.File) > 4096 {
		return nil, fmt.Errorf("DOCX contains too many parts")
	}
	parts := make(map[string][]byte, len(reader.File))
	remaining := int64(128 << 20)
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if strings.Contains(file.Name, "\\") || path.Clean(file.Name) != file.Name || strings.HasPrefix(file.Name, "/") || strings.HasPrefix(file.Name, "../") {
			return nil, fmt.Errorf("invalid DOCX part path")
		}
		if _, exists := parts[file.Name]; exists {
			return nil, fmt.Errorf("duplicate DOCX part")
		}
		if file.UncompressedSize64 > uint64(remaining) {
			return nil, fmt.Errorf("DOCX expanded size exceeds limit")
		}
		stream, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open DOCX part: %w", err)
		}
		content, readErr := io.ReadAll(io.LimitReader(stream, remaining+1))
		stream.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read DOCX part: %w", readErr)
		}
		remaining -= int64(len(content))
		if remaining < 0 {
			return nil, fmt.Errorf("DOCX expanded size exceeds limit")
		}
		parts[file.Name] = content
	}
	if len(parts["[Content_Types].xml"]) == 0 || len(parts[mainPart]) == 0 {
		return nil, fmt.Errorf("DOCX document parts are missing")
	}
	return parts, nil
}

func ValidateDOCX(data []byte) error {
	parts, err := ReadDOCXParts(data)
	if err != nil {
		return err
	}
	decoder := xml.NewDecoder(bytes.NewReader(parts["word/document.xml"]))
	root, depth := false, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid DOCX document XML: %w", err)
		}
		if start, ok := token.(xml.StartElement); ok {
			if depth == 0 && root {
				return fmt.Errorf("DOCX document has multiple roots")
			}
			depth++
			if root {
				continue
			}
			if start.Name.Local != "document" || start.Name.Space != "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				return fmt.Errorf("invalid DOCX document root")
			}
			root = true
		}
		if _, ok := token.(xml.EndElement); ok {
			depth--
		}
		if data, ok := token.(xml.CharData); ok && depth == 0 && len(bytes.TrimSpace(data)) != 0 {
			return fmt.Errorf("text outside DOCX document root")
		}
	}
	if !root {
		return fmt.Errorf("empty DOCX document XML")
	}
	return nil
}

var coreTimestamps = regexp.MustCompile(`<dcterms:(created|modified)\b[^>]*>[^<]*</dcterms:(created|modified)>`)

// The legacy model may only rewrite packages it can reproduce structurally.
// Native Writer saves bypass this projection and preserve the complete package.
func CheckLegacyDocumentRewrite(name string, original []byte) error {
	if !strings.EqualFold(path.Ext(name), ".docx") {
		return nil
	}
	parts, err := ReadDOCXParts(original)
	if err != nil {
		return err
	}
	doc, err := DecodeDOCX(original)
	if err != nil {
		return err
	}
	doc.Title = parseCoreTitle(parts["docProps/core.xml"])
	rebuilt, err := EncodeDOCX(doc)
	if err != nil {
		return err
	}
	expected, err := ReadDOCXParts(rebuilt)
	if err != nil {
		return err
	}
	if len(parts) != len(expected) {
		return ErrDOCXRequiresNativeEditor
	}
	for name, content := range parts {
		// Ignore only generated timestamps; foreign authors and metadata must survive.
		if name == "docProps/core.xml" {
			if !bytes.Equal(coreTimestamps.ReplaceAll(content, []byte("<timestamp/>")), coreTimestamps.ReplaceAll(expected[name], []byte("<timestamp/>"))) {
				return ErrDOCXRequiresNativeEditor
			}
			continue
		}
		if !bytes.Equal(content, expected[name]) {
			return ErrDOCXRequiresNativeEditor
		}
	}
	return nil
}
