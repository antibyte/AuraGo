package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/form"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

type pdfOpsResult struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Files   []string `json:"files,omitempty"`
	Pages   int      `json:"pages,omitempty"`
}

func pdfOpsJSON(r pdfOpsResult) string {
	b, _ := json.Marshal(r)
	return string(b)
}

// ExecutePDFOperations dispatches PDF manipulation operations via pdfcpu.
func ExecutePDFOperations(operation, inputFile, outputFile, pages, password, watermarkText, sourceFiles string) string {
	switch strings.ToLower(operation) {
	case "merge":
		return pdfMerge(sourceFiles, outputFile)
	case "split":
		return pdfSplit(inputFile, outputFile, pages)
	case "watermark":
		return pdfWatermark(inputFile, outputFile, watermarkText, pages)
	case "compress":
		return pdfCompress(inputFile, outputFile)
	case "encrypt":
		return pdfEncrypt(inputFile, outputFile, password)
	case "decrypt":
		return pdfDecrypt(inputFile, outputFile, password)
	case "metadata":
		return pdfMetadata(inputFile)
	case "page_count":
		return pdfPageCount(inputFile)
	case "form_fields":
		return pdfFormFields(inputFile)
	case "fill_form":
		return pdfFillForm(inputFile, outputFile, sourceFiles)
	case "export_form":
		return pdfExportForm(inputFile, outputFile)
	case "reset_form":
		return pdfResetForm(inputFile, outputFile)
	case "lock_form":
		return pdfLockForm(inputFile, outputFile)
	default:
		return pdfOpsJSON(pdfOpsResult{
			Status:  "error",
			Message: fmt.Sprintf("unknown operation: %s (valid: merge, split, watermark, compress, encrypt, decrypt, metadata, page_count, form_fields, fill_form, export_form, reset_form, lock_form)", operation),
		})
	}
}

func pdfMerge(sourceFilesJSON, outputFile string) string {
	if sourceFilesJSON == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "source_files is required (JSON array of PDF paths)"})
	}
	if outputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "output_file is required"})
	}

	var files []string
	if err := json.Unmarshal([]byte(sourceFilesJSON), &files); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("invalid source_files JSON: %v", err)})
	}
	if len(files) < 2 {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "at least 2 PDF files required for merge"})
	}

	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("file not found: %s", f)})
		}
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0o750); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot create output directory: %v", err)})
	}

	conf := model.NewDefaultConfiguration()
	if err := api.MergeCreateFile(files, outputFile, false, conf); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("merge failed: %v", err)})
	}

	return pdfOpsJSON(pdfOpsResult{
		Status:  "success",
		Message: fmt.Sprintf("merged %d PDFs into %s", len(files), outputFile),
		Files:   []string{outputFile},
	})
}

func pdfSplit(inputFile, outputDir, pages string) string {
	if inputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "file_path is required"})
	}
	if outputDir == "" {
		outputDir = filepath.Dir(inputFile)
	}

	if _, err := os.Stat(inputFile); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("file not found: %s", inputFile)})
	}

	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot create output directory: %v", err)})
	}

	conf := model.NewDefaultConfiguration()

	if pages != "" {
		// Split at specific page numbers
		pageNrs, err := parsePageNumbers(pages)
		if err != nil {
			return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("invalid pages: %v", err)})
		}
		if err := api.SplitByPageNrFile(inputFile, outputDir, pageNrs, conf); err != nil {
			return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("split failed: %v", err)})
		}
	} else {
		// Split into individual pages (span=1)
		if err := api.SplitFile(inputFile, outputDir, 1, conf); err != nil {
			return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("split failed: %v", err)})
		}
	}

	return pdfOpsJSON(pdfOpsResult{
		Status:  "success",
		Message: fmt.Sprintf("split %s into %s", filepath.Base(inputFile), outputDir),
		Files:   []string{outputDir},
	})
}

func pdfWatermark(inputFile, outputFile, text, pages string) string {
	if inputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "file_path is required"})
	}
	if text == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "watermark_text is required"})
	}
	if outputFile == "" {
		outputFile = addSuffix(inputFile, "_watermarked")
	}

	if _, err := os.Stat(inputFile); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("file not found: %s", inputFile)})
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0o750); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot create output directory: %v", err)})
	}

	// pdfcpu watermark descriptor: diagonal text, semi-transparent
	desc := "font:Helvetica, points:48, col:0.5 0.5 0.5, rot:45, scale:1 rel, opacity:0.3"
	selectedPages := parseSelectedPages(pages)

	conf := model.NewDefaultConfiguration()
	if err := api.AddTextWatermarksFile(inputFile, outputFile, selectedPages, true, text, desc, conf); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("watermark failed: %v", err)})
	}

	return pdfOpsJSON(pdfOpsResult{
		Status:  "success",
		Message: fmt.Sprintf("watermarked %s → %s", filepath.Base(inputFile), filepath.Base(outputFile)),
		Files:   []string{outputFile},
	})
}

func pdfCompress(inputFile, outputFile string) string {
	if inputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "file_path is required"})
	}
	if outputFile == "" {
		outputFile = addSuffix(inputFile, "_compressed")
	}

	if _, err := os.Stat(inputFile); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("file not found: %s", inputFile)})
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0o750); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot create output directory: %v", err)})
	}

	conf := model.NewDefaultConfiguration()
	if err := api.OptimizeFile(inputFile, outputFile, conf); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("compress failed: %v", err)})
	}

	origInfo, _ := os.Stat(inputFile)
	newInfo, _ := os.Stat(outputFile)
	var msg string
	if origInfo != nil && newInfo != nil {
		saved := origInfo.Size() - newInfo.Size()
		pct := float64(saved) / float64(origInfo.Size()) * 100
		msg = fmt.Sprintf("compressed %s: %s → %s (%.1f%% reduction)",
			filepath.Base(inputFile), humanSize(origInfo.Size()), humanSize(newInfo.Size()), pct)
	} else {
		msg = fmt.Sprintf("compressed %s → %s", filepath.Base(inputFile), filepath.Base(outputFile))
	}

	return pdfOpsJSON(pdfOpsResult{
		Status:  "success",
		Message: msg,
		Files:   []string{outputFile},
	})
}

func pdfEncrypt(inputFile, outputFile, password string) string {
	if inputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "file_path is required"})
	}
	if password == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "password is required for encryption"})
	}
	if outputFile == "" {
		outputFile = addSuffix(inputFile, "_encrypted")
	}

	if _, err := os.Stat(inputFile); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("file not found: %s", inputFile)})
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0o750); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot create output directory: %v", err)})
	}

	conf := model.NewDefaultConfiguration()
	conf.UserPW = password
	conf.OwnerPW = password
	conf.EncryptUsingAES = true
	conf.EncryptKeyLength = 256

	if err := api.EncryptFile(inputFile, outputFile, conf); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("encrypt failed: %v", err)})
	}

	return pdfOpsJSON(pdfOpsResult{
		Status:  "success",
		Message: fmt.Sprintf("encrypted %s with AES-256 → %s", filepath.Base(inputFile), filepath.Base(outputFile)),
		Files:   []string{outputFile},
	})
}

func pdfDecrypt(inputFile, outputFile, password string) string {
	if inputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "file_path is required"})
	}
	if password == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "password is required for decryption"})
	}
	if outputFile == "" {
		outputFile = addSuffix(inputFile, "_decrypted")
	}

	if _, err := os.Stat(inputFile); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("file not found: %s", inputFile)})
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0o750); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot create output directory: %v", err)})
	}

	conf := model.NewDefaultConfiguration()
	conf.UserPW = password
	conf.OwnerPW = password

	if err := api.DecryptFile(inputFile, outputFile, conf); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("decrypt failed: %v", err)})
	}

	return pdfOpsJSON(pdfOpsResult{
		Status:  "success",
		Message: fmt.Sprintf("decrypted %s → %s", filepath.Base(inputFile), filepath.Base(outputFile)),
		Files:   []string{outputFile},
	})
}

type pdfMetadataInfo struct {
	Status     string            `json:"status"`
	Pages      int               `json:"pages"`
	Properties map[string]string `json:"properties,omitempty"`
	Keywords   []string          `json:"keywords,omitempty"`
}

func pdfMetadata(inputFile string) string {
	if inputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "file_path is required"})
	}

	if _, err := os.Stat(inputFile); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("file not found: %s", inputFile)})
	}

	conf := model.NewDefaultConfiguration()

	pageCount, err := api.PageCountFile(inputFile)
	if err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot read PDF: %v", err)})
	}

	f, err := os.Open(inputFile)
	if err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot open file: %v", err)})
	}
	defer f.Close()

	props, _ := api.Properties(f, conf)

	// Re-open for keywords (ReadSeeker position may have moved)
	f2, err := os.Open(inputFile)
	if err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot open file: %v", err)})
	}
	defer f2.Close()
	kws, _ := api.Keywords(f2, conf)

	info := pdfMetadataInfo{
		Status:     "success",
		Pages:      pageCount,
		Properties: props,
		Keywords:   kws,
	}
	b, _ := json.Marshal(info)
	return string(b)
}

func pdfPageCount(inputFile string) string {
	if inputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "file_path is required"})
	}

	count, err := api.PageCountFile(inputFile)
	if err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot read PDF: %v", err)})
	}

	return pdfOpsJSON(pdfOpsResult{
		Status:  "success",
		Message: fmt.Sprintf("%s has %d pages", filepath.Base(inputFile), count),
		Pages:   count,
	})
}

// --- form operations ---

type pdfFormFieldInfo struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Type    string `json:"type"`
	Value   string `json:"value,omitempty"`
	Default string `json:"default,omitempty"`
	Options string `json:"options,omitempty"`
	Pages   []int  `json:"pages"`
	Locked  bool   `json:"locked"`
}

type pdfFormFieldsResult struct {
	Status string             `json:"status"`
	Fields []pdfFormFieldInfo `json:"fields"`
	Count  int                `json:"count"`
}

func fieldTypeName(ft form.FieldType) string {
	switch ft {
	case form.FTText:
		return "text"
	case form.FTDate:
		return "date"
	case form.FTCheckBox:
		return "checkbox"
	case form.FTComboBox:
		return "combobox"
	case form.FTListBox:
		return "listbox"
	case form.FTRadioButtonGroup:
		return "radio"
	default:
		return "unknown"
	}
}

func pdfFormFields(inputFile string) string {
	if inputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "file_path is required"})
	}
	f, err := os.Open(inputFile)
	if err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot open file: %v", err)})
	}
	defer f.Close()

	conf := model.NewDefaultConfiguration()
	fields, err := api.FormFields(f, conf)
	if err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("form_fields failed: %v", err)})
	}

	infos := make([]pdfFormFieldInfo, 0, len(fields))
	for _, fld := range fields {
		infos = append(infos, pdfFormFieldInfo{
			Name:    fld.Name,
			ID:      fld.ID,
			Type:    fieldTypeName(fld.Typ),
			Value:   fld.V,
			Default: fld.Dv,
			Options: fld.Opts,
			Pages:   fld.Pages,
			Locked:  fld.Locked,
		})
	}

	result := pdfFormFieldsResult{
		Status: "success",
		Fields: infos,
		Count:  len(infos),
	}
	b, _ := json.Marshal(result)
	return string(b)
}

func pdfFillForm(inputFile, outputFile, formDataJSON string) string {
	if inputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "file_path is required"})
	}
	if formDataJSON == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "source_files is required (JSON object mapping field names to values)"})
	}
	if outputFile == "" {
		outputFile = addSuffix(inputFile, "_filled")
	}

	if _, err := os.Stat(inputFile); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("file not found: %s", inputFile)})
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0o750); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot create output directory: %v", err)})
	}

	// pdfcpu FillForm expects JSON in pdfcpu's own format.
	// We accept a simple {field: value} map and convert to pdfcpu format:
	// {"pages": [{"fields": {"fieldname": {"value": "val"}}}]}
	var simpleMap map[string]string
	if err := json.Unmarshal([]byte(formDataJSON), &simpleMap); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("invalid form data JSON (expected {\"field\": \"value\"}): %v", err)})
	}

	// Build pdfcpu-compatible JSON
	fieldsMap := make(map[string]map[string]string, len(simpleMap))
	for k, v := range simpleMap {
		fieldsMap[k] = map[string]string{"value": v}
	}
	pdfcpuData := map[string]interface{}{
		"pages": []map[string]interface{}{
			{"fields": fieldsMap},
		},
	}
	jsonBytes, err := json.Marshal(pdfcpuData)
	if err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("failed to build form data: %v", err)})
	}

	// Write temp JSON and use FillFormFile
	tmpFile, err := os.CreateTemp("", "pdfform-*.json")
	if err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("failed to create temp file: %v", err)})
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(jsonBytes); err != nil {
		tmpFile.Close()
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("failed to write temp file: %v", err)})
	}
	tmpFile.Close()

	conf := model.NewDefaultConfiguration()
	if err := api.FillFormFile(inputFile, tmpPath, outputFile, conf); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("fill_form failed: %v", err)})
	}

	return pdfOpsJSON(pdfOpsResult{
		Status:  "success",
		Message: fmt.Sprintf("filled %d form fields in %s → %s", len(simpleMap), filepath.Base(inputFile), filepath.Base(outputFile)),
		Files:   []string{outputFile},
	})
}

func pdfExportForm(inputFile, outputFile string) string {
	if inputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "file_path is required"})
	}
	if outputFile == "" {
		outputFile = addSuffix(inputFile, "_formdata") + ".json"
		outputFile = strings.TrimSuffix(outputFile, ".pdf.json") + ".json"
	}

	if _, err := os.Stat(inputFile); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("file not found: %s", inputFile)})
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0o750); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot create output directory: %v", err)})
	}

	conf := model.NewDefaultConfiguration()
	if err := api.ExportFormFile(inputFile, outputFile, conf); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("export_form failed: %v", err)})
	}

	return pdfOpsJSON(pdfOpsResult{
		Status:  "success",
		Message: fmt.Sprintf("exported form data from %s → %s", filepath.Base(inputFile), filepath.Base(outputFile)),
		Files:   []string{outputFile},
	})
}

func pdfResetForm(inputFile, outputFile string) string {
	if inputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "file_path is required"})
	}

	if _, err := os.Stat(inputFile); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("file not found: %s", inputFile)})
	}

	if outputFile == "" {
		outputFile = addSuffix(inputFile, "_reset")
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0o750); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot create output directory: %v", err)})
	}

	conf := model.NewDefaultConfiguration()
	if err := api.ResetFormFieldsFile(inputFile, outputFile, nil, conf); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("reset_form failed: %v", err)})
	}

	return pdfOpsJSON(pdfOpsResult{
		Status:  "success",
		Message: fmt.Sprintf("reset all form fields in %s → %s", filepath.Base(inputFile), filepath.Base(outputFile)),
		Files:   []string{outputFile},
	})
}

func pdfLockForm(inputFile, outputFile string) string {
	if inputFile == "" {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: "file_path is required"})
	}

	if _, err := os.Stat(inputFile); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("file not found: %s", inputFile)})
	}

	if outputFile == "" {
		outputFile = addSuffix(inputFile, "_locked")
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0o750); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot create output directory: %v", err)})
	}

	conf := model.NewDefaultConfiguration()
	if err := api.LockFormFieldsFile(inputFile, outputFile, nil, conf); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("lock_form failed: %v", err)})
	}

	return pdfOpsJSON(pdfOpsResult{
		Status:  "success",
		Message: fmt.Sprintf("locked all form fields in %s → %s", filepath.Base(inputFile), filepath.Base(outputFile)),
		Files:   []string{outputFile},
	})
}

// --- helpers ---

func addSuffix(filePath, suffix string) string {
	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filePath, ext)
	return base + suffix + ext
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func parsePageNumbers(s string) ([]int, error) {
	var result []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(part, "%d", &n); err != nil {
			return nil, fmt.Errorf("invalid page number: %s", part)
		}
		if n < 1 {
			return nil, fmt.Errorf("page number must be >= 1: %d", n)
		}
		result = append(result, n)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no valid page numbers provided")
	}
	return result, nil
}

func parseSelectedPages(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

// Ensure types import is used (needed for model.Box in future extensions)
var _ = types.DisplayUnit(0)
