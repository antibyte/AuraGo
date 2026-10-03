package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// pdfcpu's File APIs reopen paths. Keep those paths in a private directory,
// then publish successful outputs through the workspace root.
func executePDFOperationsRooted(workspaceDir, operation, inputFile, outputFile, pages, password, watermarkText, sourceFiles string) string {
	op := strings.ToLower(operation)
	switch op {
	case "merge", "split", "watermark", "compress", "encrypt", "decrypt", "metadata", "page_count", "form_fields", "fill_form", "export_form", "reset_form", "lock_form":
	default:
		return executePDFOperationsStaged(workspaceDir, operation, inputFile, outputFile, pages, password, watermarkText, sourceFiles)
	}
	if op != "merge" && inputFile == "" || op == "merge" && (sourceFiles == "" || outputFile == "") || op == "fill_form" && sourceFiles == "" {
		return executePDFOperationsStaged(workspaceDir, operation, inputFile, outputFile, pages, password, watermarkText, sourceFiles)
	}
	stagingDir, err := os.MkdirTemp("", "aurago-pdf-*")
	if err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot stage PDF operation: %v", err)})
	}
	defer os.RemoveAll(stagingDir)
	var resolvedInput string
	if op == "merge" {
		var paths []string
		if err := json.Unmarshal([]byte(sourceFiles), &paths); err != nil {
			return executePDFOperationsStaged(workspaceDir, operation, inputFile, outputFile, pages, password, watermarkText, sourceFiles)
		}
		stagedPaths := make([]string, 0, len(paths))
		for i, path := range paths {
			resolved, err := securePDFPath(workspaceDir, path)
			if err != nil {
				return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("invalid source path %q: %v", path, err)})
			}
			staged := filepath.Join(stagingDir, fmt.Sprintf("source-%d.pdf", i))
			if err := stageRootedToolFile(resolved, staged); err != nil {
				return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot stage source PDF: %v", err)})
			}
			stagedPaths = append(stagedPaths, staged)
		}
		encoded, _ := json.Marshal(stagedPaths)
		sourceFiles = string(encoded)
	} else {
		resolvedInput, err = securePDFPath(workspaceDir, inputFile)
		if err != nil {
			return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("invalid file path: %v", err)})
		}
		stagedInput := filepath.Join(stagingDir, filepath.Base(resolvedInput))
		if err := stageRootedToolFile(resolvedInput, stagedInput); err != nil {
			return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot stage input PDF: %v", err)})
		}
		inputFile = stagedInput
	}
	if op == "metadata" || op == "page_count" || op == "form_fields" {
		return executePDFOperationsStaged(stagingDir, operation, inputFile, "", pages, password, watermarkText, sourceFiles)
	}
	if outputFile == "" {
		switch op {
		case "split":
			outputFile = filepath.Dir(resolvedInput)
		case "watermark":
			outputFile = addSuffix(resolvedInput, "_watermarked")
		case "compress":
			outputFile = addSuffix(resolvedInput, "_compressed")
		case "encrypt":
			outputFile = addSuffix(resolvedInput, "_encrypted")
		case "decrypt":
			outputFile = addSuffix(resolvedInput, "_decrypted")
		case "fill_form":
			outputFile = addSuffix(resolvedInput, "_filled")
		case "export_form":
			outputFile = strings.TrimSuffix(addSuffix(resolvedInput, "_formdata")+".json", ".pdf.json") + ".json"
		case "reset_form":
			outputFile = addSuffix(resolvedInput, "_reset")
		case "lock_form":
			outputFile = addSuffix(resolvedInput, "_locked")
		}
	}
	destination, err := securePDFPath(workspaceDir, outputFile)
	if err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("invalid output path: %v", err)})
	}
	if err := requireUnprotectedNotesPath(destination, true); err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: err.Error()})
	}
	stagedOutput := filepath.Join(stagingDir, "result"+filepath.Ext(destination))
	if op == "split" {
		stagedOutput = filepath.Join(stagingDir, "pages")
	}
	result := executePDFOperationsStaged(stagingDir, operation, inputFile, stagedOutput, pages, password, watermarkText, sourceFiles)
	var parsed pdfOpsResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil || parsed.Status != "success" {
		return result
	}
	if op == "split" {
		err = filepath.WalkDir(stagedOutput, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("PDF output contains a symlink")
			}
			rel, err := filepath.Rel(stagedOutput, path)
			if err != nil {
				return err
			}
			target, err := securePDFPath(workspaceDir, filepath.Join(destination, rel))
			if err != nil {
				return err
			}
			if err := requireUnprotectedNotesPath(target, true); err != nil {
				return err
			}
			if entry.IsDir() {
				return rootedToolMkdirAll(target, 0o750)
			}
			return publishStagedPDF(path, target)
		})
	} else {
		err = publishStagedPDF(stagedOutput, destination)
	}
	if err != nil {
		return pdfOpsJSON(pdfOpsResult{Status: "error", Message: fmt.Sprintf("cannot save PDF output: %v", err)})
	}
	parsed.Message = strings.ReplaceAll(parsed.Message, stagedOutput, destination)
	parsed.Message = strings.ReplaceAll(parsed.Message, filepath.Base(stagedOutput), filepath.Base(destination))
	parsed.Files = []string{destination}
	return pdfOpsJSON(parsed)
}

func publishStagedPDF(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("PDF output is not a regular file")
	}
	return rootedToolWriteFromReaderAtomic(destination, input)
}
