package tools

import (
	"fmt"
	"path"
	"strings"

	"aurago/internal/office"
)

func virtualDesktopDocument(args map[string]interface{}) (office.Document, error) {
	var doc office.Document
	if raw, ok := args["document"]; ok {
		if err := mapToStruct(raw, &doc); err != nil {
			return doc, fmt.Errorf("invalid document: %w", err)
		}
	}
	if doc.Text == "" {
		doc.Text = virtualDesktopString(args, "content", "text")
	}
	if doc.HTML == "" {
		doc.HTML = virtualDesktopString(args, "html")
	}
	if doc.Title == "" {
		doc.Title = virtualDesktopString(args, "title", "name")
	}
	if doc.Path == "" {
		doc.Path = virtualDesktopString(args, "path", "file_path")
	}
	return doc, nil
}

func virtualDesktopWorkbook(args map[string]interface{}) (office.Workbook, error) {
	if raw, ok := args["workbook"]; ok {
		return office.MarshalWorkbook(raw)
	}
	sheet := virtualDesktopString(args, "sheet", "sheet_name")
	if sheet == "" {
		sheet = "Sheet1"
	}
	workbook := office.Workbook{
		Path: virtualDesktopString(args, "path", "file_path"),
		Sheets: []office.Sheet{{
			Name: sheet,
		}},
	}
	return workbook, nil
}

func virtualDesktopEncodeWorkbookForPath(rawPath string, workbook office.Workbook) ([]byte, error) {
	switch strings.ToLower(path.Ext(cleanVirtualDesktopSlashPath(rawPath))) {
	case ".csv":
		return office.EncodeCSV(workbook, "")
	case ".xlsx", ".xlsm", ".xltx", ".xltm", "":
		return office.EncodeWorkbook(workbook)
	default:
		return nil, fmt.Errorf("unsupported workbook type %q", path.Ext(rawPath))
	}
}

func virtualDesktopExportOffice(sourceName string, data []byte, outputPath, format string) ([]byte, error) {
	outputExt := strings.ToLower(path.Ext(cleanVirtualDesktopSlashPath(outputPath)))
	if format != "" {
		outputExt = "." + format
	}
	if outputExt == "" {
		switch strings.ToLower(path.Ext(cleanVirtualDesktopSlashPath(sourceName))) {
		case ".xlsx", ".xlsm", ".xltx", ".xltm", ".csv":
			outputExt = ".xlsx"
		case ".docx", ".html", ".htm", ".md", ".txt", "":
			outputExt = ".docx"
		}
	}
	switch outputExt {
	case ".docx", ".html", ".htm", ".md", ".txt":
		doc, err := office.DecodeDocument(sourceName, data)
		if err != nil {
			return nil, err
		}
		exportName := sourceName
		if outputExt != "" {
			exportName = strings.TrimSuffix(sourceName, path.Ext(sourceName)) + outputExt
		}
		exported, _, err := office.EncodeDocument(exportName, doc)
		return exported, err
	case ".xlsx", ".xlsm", ".csv":
		workbook, err := office.DecodeWorkbook(sourceName, data)
		if err != nil {
			return nil, err
		}
		if outputExt == ".csv" {
			return office.EncodeCSV(workbook, "")
		}
		return office.EncodeWorkbook(workbook)
	default:
		return nil, fmt.Errorf("unsupported export format %q", strings.TrimPrefix(outputExt, "."))
	}
}
