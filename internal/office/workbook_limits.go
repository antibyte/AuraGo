package office

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Preflight sparse row/cell references before Excelize can expand them into
// enormous slices. Row slots are summed across worksheets because GetRows
// expands every row before the last populated row in each sheet.
func validateWorkbookAllocation(parts map[string][]byte) error {
	allocatedCells, allocatedRows := 0, 0
	sharedStringHasValue, err := workbookSharedStringPresence(parts["xl/sharedStrings.xml"])
	if err != nil {
		return fmt.Errorf("parse shared strings for workbook limits: %w", err)
	}
	for path, body := range parts {
		if !strings.HasPrefix(path, "xl/worksheets/") || !strings.HasSuffix(path, ".xml") {
			continue
		}
		decoder := xml.NewDecoder(bytes.NewReader(body))
		row, rowWidth := 0, 0
		sheetLastPopulatedRow := 0
		finishRow := func() error {
			allocatedCells += rowWidth
			rowWidth = 0
			if allocatedCells > 1000000 {
				return fmt.Errorf("workbook cell limit exceeded")
			}
			return nil
		}
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("parse worksheet limits: %w", err)
			}
			start, ok := token.(xml.StartElement)
			if !ok {
				continue
			}
			if start.Name.Local == "row" {
				if err := finishRow(); err != nil {
					return err
				}
				next := row + 1
				for _, attr := range start.Attr {
					if attr.Name.Local == "r" {
						next, err = strconv.Atoi(attr.Value)
						if err != nil {
							return fmt.Errorf("invalid worksheet row")
						}
					}
				}
				if next <= row || next > 100000 {
					return fmt.Errorf("workbook row limit exceeded")
				}
				row = next
			}
			if start.Name.Local != "c" {
				continue
			}

			column, cellType := rowWidth+1, ""
			for _, attr := range start.Attr {
				switch attr.Name.Local {
				case "r":
					var cellRow int
					column, cellRow, err = excelize.CellNameToCoordinates(attr.Value)
					if err != nil || cellRow != row {
						return fmt.Errorf("invalid worksheet cell reference")
					}
				case "t":
					cellType = attr.Value
				}
			}
			if column <= rowWidth || column > 16384 {
				return fmt.Errorf("workbook column limit exceeded")
			}
			rowWidth = column

			var cell struct {
				Formula *string `xml:"f"`
				Value   string  `xml:"v"`
				Inline  struct {
					Text string `xml:"t"`
					Runs []struct {
						Text string `xml:"t"`
					} `xml:"r"`
				} `xml:"is"`
			}
			if err := decoder.DecodeElement(&cell, &start); err != nil {
				return fmt.Errorf("parse worksheet cell limits: %w", err)
			}
			populated := cell.Formula != nil
			switch cellType {
			case "s":
				index, parseErr := strconv.Atoi(strings.TrimSpace(cell.Value))
				populated = populated || parseErr == nil && index >= 0 && index < len(sharedStringHasValue) && sharedStringHasValue[index]
			case "inlineStr":
				populated = populated || cell.Inline.Text != ""
				for _, run := range cell.Inline.Runs {
					populated = populated || run.Text != ""
				}
			default:
				populated = populated || cell.Value != ""
			}
			if populated {
				sheetLastPopulatedRow = row
			}
		}
		if err := finishRow(); err != nil {
			return err
		}
		// GetRows materializes the complete [][]string through the last row
		// containing a value or formula, including the intervening row slots.
		allocatedRows += sheetLastPopulatedRow
		if allocatedRows > 1000000 {
			return fmt.Errorf("workbook expanded row limit exceeded")
		}
	}
	return nil
}

func workbookSharedStringPresence(data []byte) ([]bool, error) {
	if len(data) == 0 {
		return nil, nil
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var hasValue []bool
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return hasValue, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "si" {
			continue
		}
		var item struct {
			Text string `xml:"t"`
			Runs []struct {
				Text string `xml:"t"`
			} `xml:"r"`
		}
		if err := decoder.DecodeElement(&item, &start); err != nil {
			return nil, err
		}
		nonEmpty := item.Text != ""
		for _, run := range item.Runs {
			nonEmpty = nonEmpty || run.Text != ""
		}
		hasValue = append(hasValue, nonEmpty)
	}
}

func openValidatedXLSX(data []byte) (map[string][]byte, *excelize.File, error) {
	parts, err := readOfficeParts(data, "xl/workbook.xml")
	if err != nil {
		return nil, nil, err
	}
	if err := validateWorkbookAllocation(parts); err != nil {
		return nil, nil, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: 128 << 20, UnzipXMLSizeLimit: 16 << 20})
	if err != nil {
		return nil, nil, fmt.Errorf("open workbook: %w", err)
	}
	return parts, f, nil
}
