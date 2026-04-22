package agent

import (
	"context"
	"strings"

	"aurago/internal/tools"
)

func dispatchFilesystem(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	cfg := dc.Cfg
	logger := dc.Logger

	switch tc.Action {
	case "archive":
		req := decodeArchiveArgs(tc)
		if !cfg.Agent.AllowFilesystemWrite && (strings.EqualFold(req.Operation, "create") || strings.EqualFold(req.Operation, "extract")) {
			return "Tool Output: [PERMISSION DENIED] archive create/extract operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
		}
		logger.Info("LLM requested archive operation", "op", req.Operation, "path", req.FilePath, "target_dir", req.Destination)
		return "Tool Output: " + tools.ExecuteArchive(cfg.Directories.WorkspaceDir, req.Operation, req.FilePath, req.Destination, req.SourceFiles, req.Format)

	case "pdf_operations":
		req := decodePDFOperationArgs(tc)
		op := strings.ToLower(req.Operation)
		if !cfg.Agent.AllowFilesystemWrite && (op == "merge" || op == "split" || op == "watermark" || op == "compress" || op == "encrypt" || op == "decrypt" || op == "fill_form" || op == "export_form" || op == "reset_form" || op == "lock_form") {
			return "Tool Output: [PERMISSION DENIED] pdf_operations write operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
		}
		logger.Info("LLM requested PDF operation", "op", req.Operation, "path", req.FilePath)
		return "Tool Output: " + tools.ExecutePDFOperations(cfg.Directories.WorkspaceDir, req.Operation, req.FilePath, req.OutputFile, req.Pages, req.Password, req.WatermarkText, req.SourceFiles)

	case "image_processing":
		req := decodeImageProcessingArgs(tc)
		op := strings.ToLower(req.Operation)
		if !cfg.Agent.AllowFilesystemWrite && op != "info" {
			return "Tool Output: [PERMISSION DENIED] image_processing write operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
		}
		logger.Info("LLM requested image processing", "op", req.Operation, "path", req.FilePath)
		return "Tool Output: " + tools.ExecuteImageProcessing(cfg.Directories.WorkspaceDir, req.Operation, req.FilePath, req.OutputFile, req.OutputFormat, req.Width, req.Height, req.QualityPct, req.CropX, req.CropY, req.CropWidth, req.CropHeight, req.Angle)

	case "media_conversion":
		req := decodeMediaConversionArgs(tc)
		op := strings.ToLower(req.Operation)
		if !cfg.Agent.AllowFilesystemWrite && op != "info" {
			return "Tool Output: [PERMISSION DENIED] media_conversion write operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
		}
		logger.Info("LLM requested media conversion", "op", req.Operation, "path", req.FilePath, "output", req.OutputFile)
		return "Tool Output: " + tools.ExecuteMediaConversion(cfg.Directories.WorkspaceDir, &cfg.Tools.MediaConversion, tools.MediaConversionRequest{
			Operation:    req.Operation,
			FilePath:     req.FilePath,
			OutputFile:   req.OutputFile,
			OutputFormat: req.OutputFormat,
			VideoCodec:   req.VideoCodec,
			AudioCodec:   req.AudioCodec,
			VideoBitrate: req.VideoBitrate,
			AudioBitrate: req.AudioBitrate,
			Width:        req.Width,
			Height:       req.Height,
			FPS:          req.FPS,
			SampleRate:   req.SampleRate,
			QualityPct:   req.QualityPct,
		})

	case "filesystem", "filesystem_op":
		req := decodeFilesystemArgs(tc)
		// Parameter robustness: handle 'path' and 'dest' aliases frequently hallucinated by LLMs
		fpath := req.FilePath
		fdest := req.Destination

		op := strings.TrimSpace(strings.ToLower(req.Operation))
		if op == "list" || op == "ls" {
			op = "list_dir"
		}

		// For path-only ops (write_file, read_file, append, etc.) LLMs sometimes supply
		// the target path as 'dest' / 'destination' instead of 'path'. Recover silently.
		if fpath == "" && fdest != "" {
			switch op {
			case "write_file", "write", "read_file", "read", "append", "delete", "remove",
				"mkdir", "create_dir", "create", "exists", "stat":
				fpath = fdest
				fdest = ""
			}
		}

		// Block access to system-sensitive files (config, vault, databases, .env)
		wsDir := cfg.Directories.WorkspaceDir
		for _, checkPath := range []string{fpath, fdest} {
			if isProtectedSystemPath(checkPath, wsDir, cfg) {
				logger.Warn("LLM attempted filesystem access to protected system file — blocked",
					"op", op, "path", checkPath)
				return "Tool Output: [PERMISSION DENIED] Access to this file is not allowed. System configuration, database and credential files are off-limits."
			}
		}
		for _, item := range req.Items {
			for _, checkPath := range []string{
				stringValueFromMap(item, "file_path", "path"),
				stringValueFromMap(item, "destination", "dest"),
			} {
				if isProtectedSystemPath(checkPath, wsDir, cfg) {
					logger.Warn("LLM attempted filesystem batch access to protected system file — blocked",
						"op", op, "path", checkPath)
					return "Tool Output: [PERMISSION DENIED] Access to this file is not allowed. System configuration, database and credential files are off-limits."
				}
			}
		}

		if !cfg.Agent.AllowFilesystemWrite {
			writeOps := map[string]bool{
				"write":            true,
				"write_file":       true,
				"append":           true,
				"delete":           true,
				"remove":           true,
				"copy":             true,
				"move":             true,
				"rename":           true,
				"mkdir":            true,
				"create_dir":       true,
				"create":           true,
				"copy_batch":       true,
				"move_batch":       true,
				"delete_batch":     true,
				"create_dir_batch": true,
			}
			if writeOps[op] {
				return "Tool Output: [PERMISSION DENIED] filesystem write operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
			}
		}
		logger.Info("LLM requested filesystem operation", "op", op, "path", fpath, "dest", fdest)
		return tools.ExecuteFilesystem(op, fpath, fdest, req.Content, req.Items, cfg.Directories.WorkspaceDir, req.Limit, req.Offset)

	case "file_editor":
		req := decodeFileEditorArgs(tc)
		fpath := req.FilePath

		op := strings.TrimSpace(strings.ToLower(req.Operation))

		// Block access to system-sensitive files
		wsDir := cfg.Directories.WorkspaceDir
		if isProtectedSystemPath(fpath, wsDir, cfg) {
			logger.Warn("LLM attempted file_editor access to protected system file — blocked",
				"op", op, "path", fpath)
			return "Tool Output: [PERMISSION DENIED] Access to this file is not allowed. System configuration, database and credential files are off-limits."
		}

		if !cfg.Agent.AllowFilesystemWrite {
			return "Tool Output: [PERMISSION DENIED] file_editor operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
		}
		logger.Info("LLM requested file_editor operation", "op", op, "path", fpath)
		return tools.ExecuteFileEditor(op, fpath, req.Old, req.New, req.Marker, req.Content, req.StartLine, req.EndLine, req.LineCount, cfg.Directories.WorkspaceDir)

	case "json_editor":
		req := decodeJSONEditorArgs(tc)
		fpath := req.FilePath
		op := strings.TrimSpace(strings.ToLower(req.Operation))
		wsDir := cfg.Directories.WorkspaceDir
		if isProtectedSystemPath(fpath, wsDir, cfg) {
			logger.Warn("LLM attempted json_editor access to protected system file — blocked",
				"op", op, "path", fpath)
			return "Tool Output: [PERMISSION DENIED] Access to this file is not allowed. System configuration, database and credential files are off-limits."
		}
		// Write operations need permission
		switch op {
		case "set", "delete", "format":
			if !cfg.Agent.AllowFilesystemWrite {
				return "Tool Output: [PERMISSION DENIED] json_editor write operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
			}
		}
		logger.Info("LLM requested json_editor operation", "op", op, "path", fpath)
		return tools.ExecuteJsonEditor(op, fpath, req.JsonPath, req.SetValue, req.Content, wsDir)

	case "yaml_editor":
		req := decodeYAMLEditorArgs(tc)
		fpath := req.FilePath
		op := strings.TrimSpace(strings.ToLower(req.Operation))
		wsDir := cfg.Directories.WorkspaceDir
		if isProtectedSystemPath(fpath, wsDir, cfg) {
			logger.Warn("LLM attempted yaml_editor access to protected system file — blocked",
				"op", op, "path", fpath)
			return "Tool Output: [PERMISSION DENIED] Access to this file is not allowed. System configuration, database and credential files are off-limits."
		}
		switch op {
		case "set", "delete":
			if !cfg.Agent.AllowFilesystemWrite {
				return "Tool Output: [PERMISSION DENIED] yaml_editor write operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
			}
		}
		logger.Info("LLM requested yaml_editor operation", "op", op, "path", fpath)
		return tools.ExecuteYamlEditor(op, fpath, req.JsonPath, req.SetValue, wsDir)

	case "xml_editor":
		req := decodeXMLEditorArgs(tc)
		fpath := req.FilePath
		op := strings.TrimSpace(strings.ToLower(req.Operation))
		wsDir := cfg.Directories.WorkspaceDir
		if isProtectedSystemPath(fpath, wsDir, cfg) {
			logger.Warn("LLM attempted xml_editor access to protected system file — blocked",
				"op", op, "path", fpath)
			return "Tool Output: [PERMISSION DENIED] Access to this file is not allowed. System configuration, database and credential files are off-limits."
		}
		switch op {
		case "set_text", "set_attribute", "add_element", "delete", "format":
			if !cfg.Agent.AllowFilesystemWrite {
				return "Tool Output: [PERMISSION DENIED] xml_editor write operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
			}
		}
		logger.Info("LLM requested xml_editor operation", "op", op, "path", fpath)
		return tools.ExecuteXmlEditor(op, fpath, req.XPath, req.SetValue, wsDir)

	case "text_diff":
		req := decodeTextDiffArgs(tc)
		op := strings.TrimSpace(strings.ToLower(req.Operation))
		logger.Info("LLM requested text_diff", "op", op)
		return tools.ExecuteTextDiff(op, req.File1, req.File2, req.Text1, req.Text2, cfg.Directories.WorkspaceDir)

	case "file_search":
		req := decodeFileSearchArgs(tc)
		op := strings.TrimSpace(strings.ToLower(req.Operation))
		logger.Info("LLM requested file_search", "op", op, "pattern", req.Pattern)
		return tools.ExecuteFileSearch(op, req.Pattern, req.FilePath, req.Glob, req.OutputMode, cfg.Directories.WorkspaceDir)

	case "file_reader_advanced":
		req := decodeAdvancedFileReadArgs(tc)
		op := strings.TrimSpace(strings.ToLower(req.Operation))
		logger.Info("LLM requested file_reader_advanced", "op", op, "path", req.FilePath)
		return tools.ExecuteFileReaderAdvanced(op, req.FilePath, req.Pattern, req.StartLine, req.EndLine, req.LineCount, cfg.Directories.WorkspaceDir)

	case "smart_file_read":
		req := decodeSmartFileReadArgs(tc)
		op := strings.TrimSpace(strings.ToLower(req.Operation))
		logger.Info("LLM requested smart_file_read", "op", op, "path", req.FilePath, "strategy", req.SamplingStrategy)
		return tools.ExecuteSmartFileRead(ctx, tools.ResolveSummaryLLMConfig(cfg, tools.SummaryLLMConfig{
			APIKey:  cfg.LLM.APIKey,
			BaseURL: cfg.LLM.BaseURL,
			Model:   cfg.LLM.Model,
		}), logger, op, req.FilePath, req.Query, req.SamplingStrategy, req.MaxTokens, req.LineCount, cfg.Directories.WorkspaceDir)

	default:
		return ""
	}
}
