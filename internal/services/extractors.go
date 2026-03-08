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

// ── PDF text extraction (pure Go, stream-based) ─────────────────────────────

// extractPDFText extracts text from a PDF file by parsing content streams.
// This is a lightweight implementation that handles common PDF text encodings.
func extractPDFText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read PDF: %w", err)
	}

	content := string(data)
	var textParts []string

	// Strategy 1: Extract text between BT (Begin Text) and ET (End Text) operators
	btPattern := regexp.MustCompile(`BT\s(.*?)\sET`)
	matches := btPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 {
			extracted := extractPDFTextOperands(m[1])
			if extracted != "" {
				textParts = append(textParts, extracted)
			}
		}
	}

	// Strategy 2: Extract text from parenthesized strings (Tj/TJ operators)
	if len(textParts) == 0 {
		tjPattern := regexp.MustCompile(`\(([^)]+)\)\s*Tj`)
		matches := tjPattern.FindAllStringSubmatch(content, -1)
		for _, m := range matches {
			if len(m) > 1 {
				textParts = append(textParts, decodePDFString(m[1]))
			}
		}
	}

	// Strategy 3: Look for /Contents streams between stream/endstream
	if len(textParts) == 0 {
		streamPattern := regexp.MustCompile(`stream\r?\n([\s\S]*?)endstream`)
		streamMatches := streamPattern.FindAllStringSubmatch(content, -1)
		for _, sm := range streamMatches {
			if len(sm) > 1 {
				extracted := extractPDFTextOperands(sm[1])
				if extracted != "" {
					textParts = append(textParts, extracted)
				}
			}
		}
	}

	result := strings.Join(textParts, "\n")
	result = cleanExtractedText(result)

	if len(strings.TrimSpace(result)) < 10 {
		return "", fmt.Errorf("PDF text extraction yielded insufficient content (possibly scanned/image-based PDF)")
	}

	return result, nil
}

// extractPDFTextOperands extracts readable text from PDF text operators.
func extractPDFTextOperands(block string) string {
	var parts []string

	// Match text in parentheses (Tj, ', " operators)
	parenPattern := regexp.MustCompile(`\(([^)]*)\)`)
	matches := parenPattern.FindAllStringSubmatch(block, -1)
	for _, m := range matches {
		if len(m) > 1 {
			decoded := decodePDFString(m[1])
			if decoded != "" {
				parts = append(parts, decoded)
			}
		}
	}

	// Match hex strings <...>
	hexPattern := regexp.MustCompile(`<([0-9A-Fa-f]+)>`)
	hexMatches := hexPattern.FindAllStringSubmatch(block, -1)
	for _, m := range hexMatches {
		if len(m) > 1 {
			decoded := decodePDFHexString(m[1])
			if decoded != "" {
				parts = append(parts, decoded)
			}
		}
	}

	return strings.Join(parts, "")
}

// decodePDFString decodes a PDF literal string, handling common escape sequences.
func decodePDFString(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\r", "\r")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\(", "(")
	s = strings.ReplaceAll(s, "\\)", ")")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}

// decodePDFHexString decodes a PDF hex string like <48656C6C6F>.
func decodePDFHexString(hex string) string {
	if len(hex)%2 != 0 {
		hex += "0" // pad
	}
	var buf bytes.Buffer
	for i := 0; i+1 < len(hex); i += 2 {
		var b byte
		fmt.Sscanf(hex[i:i+2], "%02x", &b)
		if b >= 32 && b < 127 { // printable ASCII only
			buf.WriteByte(b)
		} else if b == 0 {
			// skip null
		} else {
			buf.WriteByte(b)
		}
	}
	return buf.String()
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
