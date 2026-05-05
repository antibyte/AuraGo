package tools

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	stdhtml "html"
	"os"
	"path"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/office"
)

type officeToolVersion struct {
	Path     string `json:"path"`
	Modified string `json:"modified"`
	ModTime  string `json:"mod_time"`
	Size     int64  `json:"size"`
	ETag     string `json:"etag"`
}

// ExecuteOfficeDocument performs agent-requested document operations inside the virtual desktop workspace.
func ExecuteOfficeDocument(ctx context.Context, cfg *config.Config, args map[string]interface{}) VirtualDesktopExecution {
	op := strings.ToLower(strings.TrimSpace(virtualDesktopString(args, "operation", "action_type")))
	if op == "" {
		op = "read"
	}
	if err := officeToolAllowed(cfg, "document", op); err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	svc, err := officeToolService(ctx, cfg)
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	defer svc.Close()
	return executeOfficeDocumentOperation(ctx, svc, args, op)
}

// ExecuteOfficeWorkbook performs agent-requested workbook operations inside the virtual desktop workspace.
func ExecuteOfficeWorkbook(ctx context.Context, cfg *config.Config, args map[string]interface{}) VirtualDesktopExecution {
	op := strings.ToLower(strings.TrimSpace(virtualDesktopString(args, "operation", "action_type")))
	if op == "" {
		op = "read"
	}
	if err := officeToolAllowed(cfg, "workbook", op); err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	svc, err := officeToolService(ctx, cfg)
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	defer svc.Close()
	return executeOfficeWorkbookOperation(ctx, svc, args, op)
}

func officeToolAllowed(cfg *config.Config, kind, op string) error {
	if cfg == nil {
		return fmt.Errorf("configuration is unavailable")
	}
	if !cfg.VirtualDesktop.Enabled {
		return fmt.Errorf("virtual desktop is disabled in config")
	}
	if !cfg.VirtualDesktop.AllowAgentControl {
		return fmt.Errorf("agent control for the virtual desktop is disabled in config")
	}
	switch kind {
	case "document":
		if !cfg.Tools.OfficeDocument.Enabled {
			return fmt.Errorf("office_document tool is disabled in config")
		}
		if cfg.Tools.OfficeDocument.ReadOnly && officeToolMutates(kind, op) {
			return fmt.Errorf("office_document tool is in read-only mode")
		}
	case "workbook":
		if !cfg.Tools.OfficeWorkbook.Enabled {
			return fmt.Errorf("office_workbook tool is disabled in config")
		}
		if cfg.Tools.OfficeWorkbook.ReadOnly && officeToolMutates(kind, op) {
			return fmt.Errorf("office_workbook tool is in read-only mode")
		}
	}
	return nil
}

func officeToolMutates(kind, op string) bool {
	op = strings.ToLower(strings.TrimSpace(op))
	switch kind {
	case "document":
		return op != "read" && op != "read_document"
	case "workbook":
		return op != "read" && op != "read_workbook" && op != "evaluate_formula"
	default:
		return true
	}
}

func officeToolService(ctx context.Context, cfg *config.Config) (*desktop.Service, error) {
	svc, err := desktop.NewService(desktop.ConfigFromAuraConfig(cfg))
	if err != nil {
		return nil, err
	}
	if err := svc.Init(ctx); err != nil {
		_ = svc.Close()
		return nil, err
	}
	return svc, nil
}

func executeOfficeDocumentOperation(ctx context.Context, svc *desktop.Service, args map[string]interface{}, op string) VirtualDesktopExecution {
	switch op {
	case "read", "read_document":
		path := virtualDesktopString(args, "path", "file_path")
		doc, entry, data, err := officeReadDocument(ctx, svc, path)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		return virtualDesktopJSON("ok", "desktop document read", map[string]interface{}{
			"path":           entry.Path,
			"entry":          entry,
			"document":       doc,
			"office_version": officeToolVersionForEntry(entry, data),
		}, nil)
	case "write", "write_document":
		path := virtualDesktopString(args, "path", "file_path")
		if strings.TrimSpace(path) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		doc, err := virtualDesktopDocument(args)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		if doc.Title == "" {
			doc.Title = virtualDesktopString(args, "title", "name")
		}
		if doc.Path == "" {
			doc.Path = path
		}
		return officeWriteDocument(ctx, svc, path, doc, op)
	case "patch", "patch_document":
		path := virtualDesktopString(args, "path", "file_path")
		if strings.TrimSpace(path) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		doc, _, _, err := officeReadDocument(ctx, svc, path)
		if err != nil {
			readErr := err
			doc, err = virtualDesktopDocument(args)
			if err != nil {
				return virtualDesktopJSON("error", err.Error(), nil, nil)
			}
			if doc.Text == "" && doc.HTML == "" {
				return virtualDesktopJSON("error", readErr.Error(), nil, nil)
			}
		}
		if title := virtualDesktopString(args, "title", "name"); title != "" {
			doc.Title = title
		}
		doc.Path = path
		doc.Text = officePatchText(doc.Text, args)
		doc.HTML = officePatchHTML(doc.HTML, doc.Text, args)
		return officeWriteDocument(ctx, svc, path, doc, op)
	case "export", "export_file":
		sourcePath := virtualDesktopString(args, "path", "file_path", "source_path")
		outputPath := virtualDesktopString(args, "output_path", "target_path")
		format := strings.ToLower(strings.TrimPrefix(virtualDesktopString(args, "format"), "."))
		if strings.TrimSpace(sourcePath) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		if strings.TrimSpace(outputPath) == "" {
			return virtualDesktopJSON("error", "output_path is required", nil, nil)
		}
		data, entry, err := svc.ReadFileBytes(ctx, sourcePath)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		exported, err := virtualDesktopExportOffice(entry.Name, data, outputPath, format)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		outEntry, err := svc.WriteFileBytesConditional(ctx, outputPath, exported, desktop.SourceAgent, nil)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": op, "path": sourcePath, "output_path": outputPath})
		return virtualDesktopJSON("ok", "desktop document exported", map[string]interface{}{
			"path":           entry.Path,
			"output_path":    outEntry.Path,
			"entry":          outEntry,
			"office_version": officeToolVersionForEntry(outEntry, exported),
		}, event)
	default:
		return virtualDesktopJSON("error", fmt.Sprintf("unsupported office_document operation %q", op), nil, nil)
	}
}

func executeOfficeWorkbookOperation(ctx context.Context, svc *desktop.Service, args map[string]interface{}, op string) VirtualDesktopExecution {
	switch op {
	case "read", "read_workbook":
		path := virtualDesktopString(args, "path", "file_path")
		workbook, entry, data, err := officeReadWorkbook(ctx, svc, path)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		return virtualDesktopJSON("ok", "desktop workbook read", map[string]interface{}{
			"path":           entry.Path,
			"entry":          entry,
			"workbook":       workbook,
			"office_version": officeToolVersionForEntry(entry, data),
		}, nil)
	case "write", "write_workbook":
		path := virtualDesktopString(args, "path", "file_path")
		if strings.TrimSpace(path) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		workbook, err := virtualDesktopWorkbook(args)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		workbook.Path = path
		return officeWriteWorkbook(ctx, svc, path, workbook, op)
	case "set_cell":
		path := virtualDesktopString(args, "path", "file_path")
		cellRef := virtualDesktopString(args, "cell", "cell_ref", "address")
		if strings.TrimSpace(path) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		if strings.TrimSpace(cellRef) == "" {
			return virtualDesktopJSON("error", "cell is required", nil, nil)
		}
		workbook, err := officeReadWorkbookOrNew(ctx, svc, path)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		cell := office.Cell{
			Value:   virtualDesktopString(args, "value", "content"),
			Formula: virtualDesktopString(args, "formula"),
		}
		if err := office.SetCell(&workbook, virtualDesktopString(args, "sheet", "sheet_name"), cellRef, cell); err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		workbook.Path = path
		return officeWriteWorkbook(ctx, svc, path, workbook, op)
	case "set_range":
		path := virtualDesktopString(args, "path", "file_path")
		startCell := virtualDesktopString(args, "start_cell", "cell", "cell_ref", "address")
		if strings.TrimSpace(path) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		rows, err := officeToolRangeRows(args["values"])
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		workbook, err := officeReadWorkbookOrNew(ctx, svc, path)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		if err := office.SetRange(&workbook, virtualDesktopString(args, "sheet", "sheet_name"), startCell, rows); err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		workbook.Path = path
		return officeWriteWorkbook(ctx, svc, path, workbook, op)
	case "evaluate_formula":
		path := virtualDesktopString(args, "path", "file_path")
		formula := virtualDesktopString(args, "formula")
		if strings.TrimSpace(path) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		if strings.TrimSpace(formula) == "" {
			return virtualDesktopJSON("error", "formula is required", nil, nil)
		}
		workbook, entry, _, err := officeReadWorkbook(ctx, svc, path)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		sheet, err := officeToolSheet(workbook, virtualDesktopString(args, "sheet", "sheet_name"))
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		result, err := office.EvaluateFormulaForSheet(sheet, formula)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		return virtualDesktopJSON("ok", "desktop workbook formula evaluated", map[string]interface{}{
			"path":    entry.Path,
			"sheet":   sheet.Name,
			"formula": strings.TrimPrefix(strings.TrimSpace(formula), "="),
			"result":  result,
		}, nil)
	case "export", "export_file":
		sourcePath := virtualDesktopString(args, "path", "file_path", "source_path")
		outputPath := virtualDesktopString(args, "output_path", "target_path")
		format := strings.ToLower(strings.TrimPrefix(virtualDesktopString(args, "format"), "."))
		if strings.TrimSpace(sourcePath) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		if strings.TrimSpace(outputPath) == "" {
			return virtualDesktopJSON("error", "output_path is required", nil, nil)
		}
		data, entry, err := svc.ReadFileBytes(ctx, sourcePath)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		exported, err := officeToolExportWorkbook(entry.Name, data, outputPath, format, virtualDesktopString(args, "sheet", "sheet_name"))
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		outEntry, err := svc.WriteFileBytesConditional(ctx, outputPath, exported, desktop.SourceAgent, nil)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": op, "path": sourcePath, "output_path": outputPath})
		return virtualDesktopJSON("ok", "desktop workbook exported", map[string]interface{}{
			"path":           entry.Path,
			"output_path":    outEntry.Path,
			"entry":          outEntry,
			"office_version": officeToolVersionForEntry(outEntry, exported),
		}, event)
	default:
		return virtualDesktopJSON("error", fmt.Sprintf("unsupported office_workbook operation %q", op), nil, nil)
	}
}

func officeReadDocument(ctx context.Context, svc *desktop.Service, rawPath string) (office.Document, desktop.FileEntry, []byte, error) {
	if strings.TrimSpace(rawPath) == "" {
		return office.Document{}, desktop.FileEntry{}, nil, fmt.Errorf("path is required")
	}
	data, entry, err := svc.ReadFileBytes(ctx, rawPath)
	if err != nil {
		return office.Document{}, desktop.FileEntry{}, nil, err
	}
	doc, err := office.DecodeDocument(entry.Name, data)
	if err != nil {
		return office.Document{}, desktop.FileEntry{}, nil, err
	}
	doc.Path = entry.Path
	return doc, entry, data, nil
}

func officeWriteDocument(ctx context.Context, svc *desktop.Service, rawPath string, doc office.Document, op string) VirtualDesktopExecution {
	data, _, err := office.EncodeDocument(rawPath, doc)
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	entry, err := svc.WriteFileBytesConditional(ctx, rawPath, data, desktop.SourceAgent, nil)
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	if decoded, err := office.DecodeDocument(entry.Name, data); err == nil {
		doc = decoded
	}
	doc.Path = entry.Path
	event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": op, "path": entry.Path})
	return virtualDesktopJSON("ok", "desktop document written", map[string]interface{}{
		"path":           entry.Path,
		"entry":          entry,
		"document":       doc,
		"office_version": officeToolVersionForEntry(entry, data),
	}, event)
}

func officeReadWorkbook(ctx context.Context, svc *desktop.Service, rawPath string) (office.Workbook, desktop.FileEntry, []byte, error) {
	if strings.TrimSpace(rawPath) == "" {
		return office.Workbook{}, desktop.FileEntry{}, nil, fmt.Errorf("path is required")
	}
	data, entry, err := svc.ReadFileBytes(ctx, rawPath)
	if err != nil {
		return office.Workbook{}, desktop.FileEntry{}, nil, err
	}
	workbook, err := office.DecodeWorkbook(entry.Name, data)
	if err != nil {
		return office.Workbook{}, desktop.FileEntry{}, nil, err
	}
	workbook.Path = entry.Path
	return workbook, entry, data, nil
}

func officeReadWorkbookOrNew(ctx context.Context, svc *desktop.Service, rawPath string) (office.Workbook, error) {
	workbook, _, _, err := officeReadWorkbook(ctx, svc, rawPath)
	if err == nil {
		return workbook, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return office.Workbook{Path: rawPath}, nil
	}
	return office.Workbook{}, err
}

func officeWriteWorkbook(ctx context.Context, svc *desktop.Service, rawPath string, workbook office.Workbook, op string) VirtualDesktopExecution {
	data, err := virtualDesktopEncodeWorkbookForPath(rawPath, workbook)
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	entry, err := svc.WriteFileBytesConditional(ctx, rawPath, data, desktop.SourceAgent, nil)
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	workbook.Path = entry.Path
	event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": op, "path": entry.Path})
	return virtualDesktopJSON("ok", "desktop workbook written", map[string]interface{}{
		"path":           entry.Path,
		"entry":          entry,
		"workbook":       workbook,
		"office_version": officeToolVersionForEntry(entry, data),
	}, event)
}

func officePatchText(text string, args map[string]interface{}) string {
	text = officeToolRawString(args, "prepend_text") + text + officeToolRawString(args, "append_text")
	for _, replacement := range officeToolReplacements(args["replacements"]) {
		text = strings.ReplaceAll(text, replacement.find, replacement.replace)
	}
	return text
}

func officePatchHTML(htmlText, patchedText string, args map[string]interface{}) string {
	if strings.TrimSpace(htmlText) == "" {
		return office.TextToHTML(patchedText)
	}
	if officeToolRawString(args, "prepend_text") != "" || officeToolRawString(args, "append_text") != "" {
		return office.TextToHTML(patchedText)
	}
	patchedHTML := htmlText
	for _, replacement := range officeToolReplacements(args["replacements"]) {
		patchedHTML = strings.ReplaceAll(patchedHTML, replacement.find, stdhtml.EscapeString(replacement.replace))
	}
	return patchedHTML
}

func officeToolRawString(args map[string]interface{}, key string) string {
	raw, ok := args[key]
	if !ok || raw == nil {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return value
}

type officeToolReplacement struct {
	find    string
	replace string
}

func officeToolReplacements(raw interface{}) []officeToolReplacement {
	var replacements []officeToolReplacement
	items, ok := raw.([]interface{})
	if !ok {
		return replacements
	}
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		find := strings.TrimSpace(fmt.Sprint(m["find"]))
		if find == "" || find == "<nil>" {
			continue
		}
		replace := ""
		if rawReplace, ok := m["replace"]; ok && rawReplace != nil {
			replace = fmt.Sprint(rawReplace)
		}
		replacements = append(replacements, officeToolReplacement{find: find, replace: replace})
	}
	return replacements
}

func officeToolRangeRows(raw interface{}) ([][]office.Cell, error) {
	rows, ok := raw.([]interface{})
	if !ok || len(rows) == 0 {
		return nil, fmt.Errorf("values must be a non-empty 2D array")
	}
	result := make([][]office.Cell, len(rows))
	for r, rawRow := range rows {
		rowItems, ok := rawRow.([]interface{})
		if !ok {
			return nil, fmt.Errorf("values row %d must be an array", r+1)
		}
		result[r] = make([]office.Cell, len(rowItems))
		for c, rawCell := range rowItems {
			cell, err := officeToolCell(rawCell)
			if err != nil {
				return nil, fmt.Errorf("values cell %d,%d: %w", r+1, c+1, err)
			}
			result[r][c] = cell
		}
	}
	return result, nil
}

func officeToolCell(raw interface{}) (office.Cell, error) {
	switch v := raw.(type) {
	case office.Cell:
		return v, nil
	case map[string]interface{}:
		cell := office.Cell{
			Value:   virtualDesktopString(v, "value", "content", "text"),
			Formula: virtualDesktopString(v, "formula"),
		}
		return cell, nil
	case string:
		return office.Cell{Value: v}, nil
	case fmt.Stringer:
		return office.Cell{Value: v.String()}, nil
	case nil:
		return office.Cell{}, nil
	default:
		return office.Cell{Value: fmt.Sprint(v)}, nil
	}
}

func officeToolSheet(workbook office.Workbook, sheetName string) (office.Sheet, error) {
	if len(workbook.Sheets) == 0 {
		return office.Sheet{}, fmt.Errorf("workbook has no sheets")
	}
	if strings.TrimSpace(sheetName) == "" {
		return workbook.Sheets[0], nil
	}
	for _, sheet := range workbook.Sheets {
		if strings.EqualFold(sheet.Name, sheetName) {
			return sheet, nil
		}
	}
	return office.Sheet{}, fmt.Errorf("sheet %q not found", sheetName)
}

func officeToolExportWorkbook(sourceName string, data []byte, outputPath, format, sheetName string) ([]byte, error) {
	outputExt := strings.ToLower(path.Ext(cleanVirtualDesktopSlashPath(outputPath)))
	if format != "" {
		outputExt = "." + format
	}
	if outputExt == "" {
		outputExt = ".xlsx"
	}
	workbook, err := office.DecodeWorkbook(sourceName, data)
	if err != nil {
		return nil, err
	}
	switch outputExt {
	case ".csv":
		return office.EncodeCSV(workbook, sheetName)
	case ".xlsx", ".xlsm":
		return office.EncodeWorkbook(workbook)
	default:
		return nil, fmt.Errorf("unsupported export format %q", strings.TrimPrefix(outputExt, "."))
	}
}

func officeToolVersionForEntry(entry desktop.FileEntry, data []byte) officeToolVersion {
	modified := entry.ModTime.UTC().Format(time.RFC3339Nano)
	etagHash := sha256.New()
	_, _ = etagHash.Write([]byte(entry.Path))
	_, _ = etagHash.Write([]byte{0})
	_, _ = etagHash.Write(data)
	return officeToolVersion{
		Path:     entry.Path,
		Modified: modified,
		ModTime:  modified,
		Size:     entry.Size,
		ETag:     fmt.Sprintf("%x", etagHash.Sum(nil)),
	}
}
