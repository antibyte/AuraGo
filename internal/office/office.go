package office

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	stdhtml "html"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	xhtml "golang.org/x/net/html"
)

// Document is AuraGo's minimal agent-friendly word-processing model.
type Document struct {
	Path  string `json:"path,omitempty"`
	Title string `json:"title,omitempty"`
	Text  string `json:"text"`
	HTML  string `json:"html,omitempty"`
	Delta Delta  `json:"delta"`
}

// Delta is the subset of Quill's Delta format AuraGo needs for v1.
type Delta struct {
	Ops []DeltaOp `json:"ops"`
}

type DeltaOp struct {
	Insert     string                 `json:"insert"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// Workbook is AuraGo's minimal agent-friendly spreadsheet model.
type Workbook struct {
	Path   string  `json:"path,omitempty"`
	Sheets []Sheet `json:"sheets"`
}

type Sheet struct {
	Name string   `json:"name"`
	Rows [][]Cell `json:"rows"`
}

type Cell struct {
	Value   string `json:"value,omitempty"`
	Formula string `json:"formula,omitempty"`
}

func DecodeDocument(name string, data []byte) (Document, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".docx":
		return DecodeDOCX(data)
	case ".html", ".htm":
		text := htmlToText(string(data))
		return documentFromText(strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)), text, string(data)), nil
	case ".md", ".txt", "":
		text := string(data)
		return documentFromText(strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)), text, ""), nil
	default:
		return Document{}, fmt.Errorf("unsupported document type %q", filepath.Ext(name))
	}
}

func EncodeDocument(name string, doc Document) ([]byte, string, error) {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		ext = ".docx"
	}
	switch ext {
	case ".docx":
		data, err := EncodeDOCX(doc)
		return data, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", err
	case ".html", ".htm":
		htmlText := doc.HTML
		if strings.TrimSpace(htmlText) == "" {
			htmlText = textToHTML(doc.Text)
		}
		return []byte(htmlText), "text/html; charset=utf-8", nil
	case ".md":
		return []byte(doc.Text), "text/markdown; charset=utf-8", nil
	case ".txt":
		return []byte(doc.Text), "text/plain; charset=utf-8", nil
	default:
		return nil, "", fmt.Errorf("unsupported document type %q", ext)
	}
}

func DecodeDOCX(data []byte) (Document, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Document{}, fmt.Errorf("open docx: %w", err)
	}
	var documentXML []byte
	var title string
	for _, file := range reader.File {
		switch file.Name {
		case "word/document.xml":
			documentXML, err = readZipFile(file)
			if err != nil {
				return Document{}, err
			}
		case "docProps/core.xml":
			coreXML, readErr := readZipFile(file)
			if readErr != nil {
				return Document{}, readErr
			}
			title = parseCoreTitle(coreXML)
		}
	}
	if len(documentXML) == 0 {
		return Document{}, fmt.Errorf("docx is missing word/document.xml")
	}
	text, err := parseWordDocumentText(documentXML)
	if err != nil {
		return Document{}, err
	}
	return documentFromText(title, text, ""), nil
}

func EncodeDOCX(doc Document) ([]byte, error) {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	now := time.Now().UTC().Format(time.RFC3339)
	files := map[string]string{
		"[Content_Types].xml": contentTypesXML,
		"_rels/.rels":         packageRelsXML,
		"docProps/core.xml":   coreXML(doc.Title, now),
		"word/document.xml":   documentXML(doc.Text),
		"word/styles.xml":     stylesXML,
	}
	order := []string{"[Content_Types].xml", "_rels/.rels", "docProps/core.xml", "word/document.xml", "word/styles.xml"}
	for _, name := range order {
		w, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("create docx part %s: %w", name, err)
		}
		if _, err := io.WriteString(w, files[name]); err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("write docx part %s: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close docx: %w", err)
	}
	return buf.Bytes(), nil
}

func DecodeWorkbook(name string, data []byte) (Workbook, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".xlsx", ".xlsm", ".xltx", ".xltm", "":
		return decodeXLSX(data)
	case ".csv":
		return decodeCSV(data)
	default:
		return Workbook{}, fmt.Errorf("unsupported workbook type %q", filepath.Ext(name))
	}
}

func EncodeWorkbook(workbook Workbook) ([]byte, error) {
	if len(workbook.Sheets) == 0 {
		workbook.Sheets = []Sheet{{Name: "Sheet1"}}
	}
	f := excelize.NewFile()
	defer f.Close()
	for i, sheet := range workbook.Sheets {
		name := cleanSheetName(sheet.Name, i)
		if i == 0 {
			if err := f.SetSheetName("Sheet1", name); err != nil {
				return nil, fmt.Errorf("rename sheet: %w", err)
			}
		} else if _, err := f.NewSheet(name); err != nil {
			return nil, fmt.Errorf("create sheet %q: %w", name, err)
		}
		for r, row := range sheet.Rows {
			for c, cell := range row {
				addr, err := excelize.CoordinatesToCellName(c+1, r+1)
				if err != nil {
					return nil, fmt.Errorf("cell address: %w", err)
				}
				if strings.TrimSpace(cell.Formula) != "" {
					formula := strings.TrimPrefix(strings.TrimSpace(cell.Formula), "=")
					if _, err := EvaluateFormulaForSheet(sheet, formula); err != nil {
						return nil, fmt.Errorf("invalid formula %s!%s: %w", name, addr, err)
					}
					formula = normalizeFormulaForXLSX(formula)
					if err := f.SetCellFormula(name, addr, formula); err != nil {
						return nil, fmt.Errorf("set formula %s!%s: %w", name, addr, err)
					}
					continue
				}
				if err := f.SetCellStr(name, addr, cell.Value); err != nil {
					return nil, fmt.Errorf("set value %s!%s: %w", name, addr, err)
				}
			}
		}
	}
	buffer, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write workbook: %w", err)
	}
	return buffer.Bytes(), nil
}

func EncodeCSV(workbook Workbook, sheetName string) ([]byte, error) {
	if len(workbook.Sheets) == 0 {
		return []byte{}, nil
	}
	sheet := workbook.Sheets[0]
	if strings.TrimSpace(sheetName) != "" {
		for _, candidate := range workbook.Sheets {
			if strings.EqualFold(candidate.Name, sheetName) {
				sheet = candidate
				break
			}
		}
	}
	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)
	sheetLabel := defaultSheetName(sheet.Name, 0)
	for r, row := range sheet.Rows {
		record := make([]string, len(row))
		for i, cell := range row {
			addr, err := excelize.CoordinatesToCellName(i+1, r+1)
			if err != nil {
				return nil, fmt.Errorf("cell address: %w", err)
			}
			if cell.Formula != "" {
				formula := strings.TrimPrefix(strings.TrimSpace(cell.Formula), "=")
				if _, err := EvaluateFormulaForSheet(sheet, formula); err != nil {
					return nil, fmt.Errorf("invalid formula %s!%s: %w", sheetLabel, addr, err)
				}
				record[i] = neutralizeCSVFormulaCell("=" + formula)
			} else {
				record[i] = neutralizeCSVFormulaCell(cell.Value)
			}
		}
		if err := writer.Write(record); err != nil {
			return nil, fmt.Errorf("write csv row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}
	return buf.Bytes(), nil
}

func normalizeFormulaForXLSX(formula string) string {
	var b strings.Builder
	changed := false
	for i := 0; i < len(formula); {
		if isFormulaLetter(formula[i]) {
			start := i
			for i < len(formula) && isFormulaLetter(formula[i]) {
				i++
			}
			name := formula[start:i]
			next := i
			for next < len(formula) && isFormulaWhitespace(formula[next]) {
				next++
			}
			if strings.EqualFold(name, "AVG") && next < len(formula) && formula[next] == '(' {
				b.WriteString("AVERAGE")
				changed = true
			} else {
				b.WriteString(name)
			}
			continue
		}
		b.WriteByte(formula[i])
		i++
	}
	if !changed {
		return formula
	}
	return b.String()
}

func isFormulaWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'
}

func neutralizeCSVFormulaCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func SetCell(workbook *Workbook, sheetName, ref string, cell Cell) error {
	if workbook == nil {
		return fmt.Errorf("workbook is required")
	}
	if len(workbook.Sheets) == 0 {
		workbook.Sheets = []Sheet{{Name: defaultSheetName(sheetName, 0)}}
	}
	sheetIndex := 0
	if strings.TrimSpace(sheetName) != "" {
		sheetIndex = -1
		for i := range workbook.Sheets {
			if strings.EqualFold(workbook.Sheets[i].Name, sheetName) {
				sheetIndex = i
				break
			}
		}
		if sheetIndex < 0 {
			workbook.Sheets = append(workbook.Sheets, Sheet{Name: sheetName})
			sheetIndex = len(workbook.Sheets) - 1
		}
	}
	col, row, err := excelize.CellNameToCoordinates(strings.ToUpper(strings.TrimSpace(ref)))
	if err != nil {
		return fmt.Errorf("invalid cell reference %q: %w", ref, err)
	}
	sheet := &workbook.Sheets[sheetIndex]
	for len(sheet.Rows) < row {
		sheet.Rows = append(sheet.Rows, []Cell{})
	}
	for len(sheet.Rows[row-1]) < col {
		sheet.Rows[row-1] = append(sheet.Rows[row-1], Cell{})
	}
	sheet.Rows[row-1][col-1] = cell
	return nil
}

func documentFromText(title, text, htmlText string) Document {
	if strings.TrimSpace(title) == "" {
		title = "Document"
	}
	return Document{
		Title: title,
		Text:  text,
		HTML:  htmlText,
		Delta: textToDelta(text),
	}
}

func textToDelta(text string) Delta {
	if text == "" {
		return Delta{Ops: []DeltaOp{{Insert: "\n"}}}
	}
	return Delta{Ops: []DeltaOp{{Insert: text + trailingNewline(text)}}}
}

func trailingNewline(text string) string {
	if strings.HasSuffix(text, "\n") {
		return ""
	}
	return "\n"
}

func decodeXLSX(data []byte) (Workbook, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return Workbook{}, fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	var workbook Workbook
	for _, name := range f.GetSheetList() {
		rows, err := f.GetRows(name, excelize.Options{RawCellValue: true})
		if err != nil {
			return Workbook{}, fmt.Errorf("read sheet %q: %w", name, err)
		}
		sheet := Sheet{Name: name, Rows: make([][]Cell, len(rows))}
		for r, row := range rows {
			sheet.Rows[r] = make([]Cell, len(row))
			for c, value := range row {
				addr, _ := excelize.CoordinatesToCellName(c+1, r+1)
				formula, _ := f.GetCellFormula(name, addr)
				sheet.Rows[r][c] = Cell{Value: value, Formula: formula}
			}
		}
		workbook.Sheets = append(workbook.Sheets, sheet)
	}
	if len(workbook.Sheets) == 0 {
		workbook.Sheets = []Sheet{{Name: "Sheet1"}}
	}
	return workbook, nil
}

func decodeCSV(data []byte) (Workbook, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return Workbook{}, fmt.Errorf("read csv: %w", err)
	}
	rows := make([][]Cell, len(records))
	for r, record := range records {
		rows[r] = make([]Cell, len(record))
		for c, value := range record {
			if strings.HasPrefix(value, "=") {
				rows[r][c] = Cell{Formula: strings.TrimPrefix(value, "=")}
			} else {
				rows[r][c] = Cell{Value: value}
			}
		}
	}
	return Workbook{Sheets: []Sheet{{Name: "Sheet1", Rows: rows}}}, nil
}

func cleanSheetName(name string, index int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultSheetName("", index)
	}
	replacer := strings.NewReplacer("[", " ", "]", " ", ":", " ", "*", " ", "?", " ", "/", " ", "\\", " ")
	name = strings.TrimSpace(replacer.Replace(name))
	if name == "" {
		name = defaultSheetName("", index)
	}
	if len([]rune(name)) > 31 {
		runes := []rune(name)
		name = string(runes[:31])
	}
	return name
}

func defaultSheetName(name string, index int) string {
	if strings.TrimSpace(name) != "" {
		return cleanSheetName(name, index)
	}
	if index == 0 {
		return "Sheet1"
	}
	return "Sheet" + strconv.Itoa(index+1)
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open zip part %s: %w", file.Name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read zip part %s: %w", file.Name, err)
	}
	return data, nil
}

func parseCoreTitle(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var inTitle bool
	for {
		tok, err := decoder.Token()
		if err != nil {
			return ""
		}
		switch token := tok.(type) {
		case xml.StartElement:
			inTitle = token.Name.Local == "title"
		case xml.CharData:
			if inTitle {
				return strings.TrimSpace(string(token))
			}
		case xml.EndElement:
			if token.Name.Local == "title" {
				inTitle = false
			}
		}
	}
}

func parseWordDocumentText(data []byte) (string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var paragraphs []string
	var current strings.Builder
	var inParagraph bool
	var inText bool
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parse document.xml: %w", err)
		}
		switch token := tok.(type) {
		case xml.StartElement:
			switch token.Name.Local {
			case "p":
				inParagraph = true
				current.Reset()
			case "t":
				inText = true
			case "tab":
				if inParagraph {
					current.WriteString("\t")
				}
			case "br":
				if inParagraph {
					current.WriteString("\n")
				}
			}
		case xml.CharData:
			if inParagraph && inText {
				current.WriteString(string(token))
			}
		case xml.EndElement:
			switch token.Name.Local {
			case "t":
				inText = false
			case "p":
				paragraphs = append(paragraphs, current.String())
				current.Reset()
				inParagraph = false
			}
		}
	}
	for len(paragraphs) > 0 && paragraphs[len(paragraphs)-1] == "" {
		paragraphs = paragraphs[:len(paragraphs)-1]
	}
	return strings.Join(paragraphs, "\n"), nil
}

func documentXML(text string) string {
	var paragraphs []string
	if text == "" {
		paragraphs = []string{""}
	} else {
		paragraphs = strings.Split(text, "\n")
	}
	var body strings.Builder
	for _, paragraph := range paragraphs {
		body.WriteString(`<w:p><w:r><w:t xml:space="preserve">`)
		body.WriteString(xmlEscape(paragraph))
		body.WriteString(`</w:t></w:r></w:p>`)
	}
	body.WriteString(`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>`)
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body.String() + `</w:body></w:document>`
}

func coreXML(title, timestamp string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:title>` + xmlEscape(title) + `</dc:title><dc:creator>AuraGo</dc:creator><cp:lastModifiedBy>AuraGo</cp:lastModifiedBy><dcterms:created xsi:type="dcterms:W3CDTF">` + timestamp + `</dcterms:created><dcterms:modified xsi:type="dcterms:W3CDTF">` + timestamp + `</dcterms:modified></cp:coreProperties>`
}

func textToHTML(text string) string {
	var b strings.Builder
	b.WriteString("<!doctype html><meta charset=\"utf-8\"><body>")
	for _, paragraph := range strings.Split(text, "\n") {
		b.WriteString("<p>")
		b.WriteString(stdhtml.EscapeString(paragraph))
		b.WriteString("</p>")
	}
	b.WriteString("</body>")
	return b.String()
}

func htmlToText(raw string) string {
	node, err := xhtml.Parse(strings.NewReader(raw))
	if err != nil {
		re := regexp.MustCompile(`<[^>]+>`)
		return strings.TrimSpace(stdhtml.UnescapeString(re.ReplaceAllString(raw, " ")))
	}
	var lines []string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			value := strings.TrimSpace(n.Data)
			if value != "" {
				lines = append(lines, value)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.Join(lines, "\n")
}

func xmlEscape(value string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(value)); err != nil {
		return ""
	}
	return b.String()
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>`
const packageRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/></Relationships>`
const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style></w:styles>`

func MarshalWorkbook(raw interface{}) (Workbook, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return Workbook{}, fmt.Errorf("marshal workbook: %w", err)
	}
	var workbook Workbook
	if err := json.Unmarshal(b, &workbook); err != nil {
		return Workbook{}, fmt.Errorf("decode workbook: %w", err)
	}
	return workbook, nil
}
