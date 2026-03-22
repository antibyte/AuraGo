package services

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

// ── Binary file detection ────────────────────────────────────────────────────

// knownBinaryExtensions lists extensions that are always considered binary/executable.
var knownBinaryExtensions = map[string]bool{
	// Executables & libraries
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true,
	".elf": true, ".com": true, ".msi": true, ".sys": true, ".drv": true,
	".o": true, ".a": true, ".lib": true, ".obj": true,
	// Archives (non-document)
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true,
	".7z": true, ".rar": true, ".zst": true, ".lz4": true,
	// Media (non-image; images handled separately)
	".mp3": true, ".mp4": true, ".avi": true, ".mkv": true, ".mov": true,
	".flac": true, ".wav": true, ".ogg": true, ".aac": true, ".wma": true,
	".wmv": true, ".webm": true, ".m4a": true, ".m4v": true,
	// Compiled / bytecode
	".class": true, ".pyc": true, ".pyo": true, ".wasm": true,
	// Databases
	".db": true, ".sqlite": true, ".sqlite3": true, ".mdb": true,
	// Disk images
	".iso": true, ".img": true, ".vmdk": true, ".qcow2": true,
	// Fonts
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,
}

// IsBinaryFile checks if a file is a binary/executable that should not be indexed.
// It checks extension first, then samples file content for non-text bytes.
func IsBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	// Explicit binary extensions
	if knownBinaryExtensions[ext] {
		return true
	}

	// Office/PDF documents are "binary" containers but contain extractable text
	if IsDocumentFile(ext) || IsImageFile(ext) {
		return false
	}

	// Sample first 8KB to check for binary content
	f, err := os.Open(path)
	if err != nil {
		return true // assume binary if can't read
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return true
	}
	buf = buf[:n]

	if n == 0 {
		return false // empty file is not binary
	}

	// Check for null bytes (strong binary indicator)
	if bytes.ContainsRune(buf, 0) {
		return true
	}

	// Check if content is valid UTF-8 text
	if !utf8.Valid(buf) {
		// Allow some invalid bytes (might be Latin-1 text)
		invalidCount := 0
		for i := 0; i < len(buf); {
			r, size := utf8.DecodeRune(buf[i:])
			if r == utf8.RuneError && size == 1 {
				invalidCount++
			}
			i += size
		}
		// If >10% of chars are invalid, treat as binary
		if float64(invalidCount)/float64(len(buf)) > 0.1 {
			return true
		}
	}

	return false
}

// ── Document type detection ──────────────────────────────────────────────────

// documentExtensions lists extensions that contain extractable text.
var documentExtensions = map[string]bool{
	".pdf": true, ".docx": true, ".xlsx": true, ".pptx": true,
	".odt": true, ".ods": true, ".odp": true,
	".rtf": true,
}

// IsDocumentFile returns true if the extension indicates a document with extractable text.
func IsDocumentFile(ext string) bool {
	return documentExtensions[strings.ToLower(ext)]
}

// imageExtensions lists image file extensions.
var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".webp": true, ".bmp": true, ".tiff": true, ".tif": true,
	".svg": true, ".ico": true,
}

// IsImageFile returns true if the extension indicates an image file.
func IsImageFile(ext string) bool {
	return imageExtensions[strings.ToLower(ext)]
}

// ── Text extraction ──────────────────────────────────────────────────────────

// ExtractText extracts text content from a document file based on its extension.
// Returns the extracted text or an error if extraction fails.
func ExtractText(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return extractPDFText(path)
	case ".docx":
		return extractDOCXText(path)
	case ".xlsx":
		return extractXLSXText(path)
	case ".pptx":
		return extractPPTXText(path)
	case ".odt", ".ods", ".odp":
		return extractODFText(path)
	case ".rtf":
		return extractRTFText(path)
	default:
		return "", fmt.Errorf("unsupported document format: %s", ext)
	}
}

// ── PDF text extraction (ledongthuc/pdf) ────────────────────────────────────

// extractPDFText extracts text from a PDF file using ledongthuc/pdf,
// which properly parses PDF cross-reference tables and content streams.
func extractPDFText(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		sb.WriteString("\n")
	}

	result := cleanExtractedText(strings.TrimSpace(sb.String()))
	if len(result) < 10 {
		return "", fmt.Errorf("PDF contains no extractable text (possibly scanned/image-based)")
	}
	return result, nil
}

// ── DOCX text extraction ─────────────────────────────────────────────────────

// extractDOCXText reads text from word/document.xml inside the DOCX ZIP archive.
func extractDOCXText(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("failed to open DOCX: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("failed to read document.xml: %w", err)
			}
			defer rc.Close()
			return extractXMLText(rc, "t")
		}
	}

	return "", fmt.Errorf("word/document.xml not found in DOCX")
}

// ── XLSX text extraction ─────────────────────────────────────────────────────

// extractXLSXText reads shared strings and sheet text from an XLSX file.
func extractXLSXText(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("failed to open XLSX: %w", err)
	}
	defer r.Close()

	var parts []string

	// 1. Read shared strings
	sharedStrings := make(map[int]string)
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				break
			}
			data, _ := io.ReadAll(rc)
			rc.Close()

			// Parse <si><t>...</t></si> elements
			decoder := xml.NewDecoder(bytes.NewReader(data))
			idx := 0
			inSI := false
			for {
				tok, err := decoder.Token()
				if err != nil {
					break
				}
				switch t := tok.(type) {
				case xml.StartElement:
					if t.Name.Local == "si" {
						inSI = true
					}
				case xml.EndElement:
					if t.Name.Local == "si" {
						idx++
						inSI = false
					}
				case xml.CharData:
					if inSI {
						s := strings.TrimSpace(string(t))
						if s != "" {
							sharedStrings[idx] = s
						}
					}
				}
			}
			break
		}
	}

	// 2. Read sheet data
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			text, _ := extractXMLText(rc, "v")
			rc.Close()
			if text != "" {
				parts = append(parts, text)
			}
		}
	}

	// Combine shared strings and sheet values
	var all []string
	for i := 0; i < len(sharedStrings); i++ {
		if s, ok := sharedStrings[i]; ok && s != "" {
			all = append(all, s)
		}
	}
	all = append(all, parts...)

	result := strings.Join(all, "\n")
	if len(strings.TrimSpace(result)) == 0 {
		return "", fmt.Errorf("no text content found in XLSX")
	}
	return cleanExtractedText(result), nil
}

// ── PPTX text extraction ────────────────────────────────────────────────────

// extractPPTXText extracts text from all slides in a PPTX file.
func extractPPTXText(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("failed to open PPTX: %w", err)
	}
	defer r.Close()

	var parts []string
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			text, _ := extractXMLText(rc, "t")
			rc.Close()
			if text != "" {
				parts = append(parts, text)
			}
		}
	}

	result := strings.Join(parts, "\n")
	if len(strings.TrimSpace(result)) == 0 {
		return "", fmt.Errorf("no text content found in PPTX")
	}
	return cleanExtractedText(result), nil
}

// ── ODF text extraction (ODT/ODS/ODP) ──────────────────────────────────────

// extractODFText extracts text from content.xml inside an ODF ZIP archive.
func extractODFText(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("failed to open ODF: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "content.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("failed to read content.xml: %w", err)
			}
			defer rc.Close()
			// ODF uses <text:p> elements, but extracting all text nodes works too
			return extractAllXMLText(rc)
		}
	}

	return "", fmt.Errorf("content.xml not found in ODF file")
}

// ── RTF text extraction ─────────────────────────────────────────────────────

// extractRTFText extracts plain text from an RTF file by stripping control words.
func extractRTFText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read RTF: %w", err)
	}

	content := string(data)
	if !strings.HasPrefix(content, "{\\rtf") {
		return "", fmt.Errorf("not a valid RTF file")
	}

	// Remove RTF control words and groups
	// Step 1: Remove \' hex escapes
	hexEsc := regexp.MustCompile(`\\'([0-9a-fA-F]{2})`)
	content = hexEsc.ReplaceAllStringFunc(content, func(m string) string {
		var b byte
		fmt.Sscanf(m[2:], "%02x", &b)
		return string(rune(b))
	})

	// Step 2: Remove control words
	ctrlWord := regexp.MustCompile(`\\[a-z]+[-]?[0-9]*\s?`)
	content = ctrlWord.ReplaceAllString(content, "")

	// Step 3: Remove remaining control characters
	content = strings.ReplaceAll(content, "\\", "")
	content = strings.ReplaceAll(content, "{", "")
	content = strings.ReplaceAll(content, "}", "")

	return cleanExtractedText(content), nil
}

// ── Shared XML helpers ──────────────────────────────────────────────────────

// extractXMLText extracts text from elements with the given local name in an XML stream.
func extractXMLText(r io.Reader, elementName string) (string, error) {
	decoder := xml.NewDecoder(r)
	var parts []string
	inElement := false

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == elementName {
				inElement = true
			}
		case xml.EndElement:
			if t.Name.Local == elementName {
				inElement = false
			}
		case xml.CharData:
			if inElement {
				s := strings.TrimSpace(string(t))
				if s != "" {
					parts = append(parts, s)
				}
			}
		}
	}

	return strings.Join(parts, " "), nil
}

// extractAllXMLText extracts all text content from an XML stream regardless of element names.
func extractAllXMLText(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)
	var parts []string

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		if charData, ok := tok.(xml.CharData); ok {
			s := strings.TrimSpace(string(charData))
			if s != "" {
				parts = append(parts, s)
			}
		}
	}

	return strings.Join(parts, " "), nil
}

// ── Text cleanup ────────────────────────────────────────────────────────────

// cleanExtractedText normalizes whitespace and removes garbage characters from extracted text.
func cleanExtractedText(text string) string {
	// Replace multiple whitespace with single space
	multiSpace := regexp.MustCompile(`[^\S\n]+`)
	text = multiSpace.ReplaceAllString(text, " ")

	// Replace excessive newlines (3+) with double newline
	multiNewline := regexp.MustCompile(`\n{3,}`)
	text = multiNewline.ReplaceAllString(text, "\n\n")

	// Remove common garbage characters from PDF extraction
	text = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return -1 // drop control chars
		}
		return r
	}, text)

	return strings.TrimSpace(text)
}
