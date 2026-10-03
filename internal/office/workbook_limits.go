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
// enormous slices. The bound applies across all worksheets, including blanks.
func validateWorkbookAllocation(parts map[string][]byte) error {
	allocated := 0
	for path, body := range parts {
		if !strings.HasPrefix(path, "xl/worksheets/") || !strings.HasSuffix(path, ".xml") {
			continue
		}
		decoder := xml.NewDecoder(bytes.NewReader(body))
		row, rowWidth := 0, 0
		finishRow := func() error {
			allocated += rowWidth
			rowWidth = 0
			if allocated > 1000000 {
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
			if start.Name.Local == "c" {
				column := rowWidth + 1
				for _, attr := range start.Attr {
					if attr.Name.Local == "r" {
						var cellRow int
						column, cellRow, err = excelize.CellNameToCoordinates(attr.Value)
						if err != nil || cellRow != row {
							return fmt.Errorf("invalid worksheet cell reference")
						}
					}
				}
				if column <= rowWidth || column > 16384 {
					return fmt.Errorf("workbook column limit exceeded")
				}
				rowWidth = column
			}
		}
		if err := finishRow(); err != nil {
			return err
		}
	}
	return nil
}
