package agent

import "testing"

func TestDecodeCallWebhookArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "call_webhook",
		Params: map[string]interface{}{
			"webhook_name": "Deploy Hook",
			"parameters": map[string]interface{}{
				"branch": "main",
			},
		},
	}

	req := decodeCallWebhookArgs(tc)
	if req.WebhookName != "Deploy Hook" {
		t.Fatalf("WebhookName = %q, want Deploy Hook", req.WebhookName)
	}
	if got, _ := req.Parameters["branch"].(string); got != "main" {
		t.Fatalf("Parameters[branch] = %q, want main", got)
	}
}

func TestDecodeSaveToolArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "save_tool",
		Params: map[string]interface{}{
			"name":        "demo_tool",
			"description": "Demo",
			"code":        "print('ok')",
		},
	}

	req := decodeSaveToolArgs(tc)
	if req.Name != "demo_tool" {
		t.Fatalf("Name = %q, want demo_tool", req.Name)
	}
	if req.Code != "print('ok')" {
		t.Fatalf("Code = %q, want print('ok')", req.Code)
	}
}

func TestDecodeRunToolArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "run_tool",
		Params: map[string]interface{}{
			"name":           "worker.py",
			"args":           []interface{}{"--limit", "5"},
			"background":     true,
			"vault_keys":     []interface{}{"API_KEY"},
			"credential_ids": []interface{}{"cred-1"},
		},
	}

	req := decodeRunToolArgs(tc)
	if req.Name != "worker.py" {
		t.Fatalf("Name = %q, want worker.py", req.Name)
	}
	if len(req.Args) != 2 || req.Args[0] != "--limit" || req.Args[1] != "5" {
		t.Fatalf("Args = %v, want [--limit 5]", req.Args)
	}
	if !req.Background {
		t.Fatal("expected Background to be true")
	}
	if len(req.VaultKeys) != 1 || req.VaultKeys[0] != "API_KEY" {
		t.Fatalf("VaultKeys = %v, want [API_KEY]", req.VaultKeys)
	}
	if len(req.CredentialIDs) != 1 || req.CredentialIDs[0] != "cred-1" {
		t.Fatalf("CredentialIDs = %v, want [cred-1]", req.CredentialIDs)
	}
}

func TestDecodeSandboxExecutionArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "execute_sandbox",
		Params: map[string]interface{}{
			"code":           "print('ok')",
			"sandbox_lang":   "python",
			"libraries":      []interface{}{"requests", "pydantic"},
			"vault_keys":     []interface{}{"API_KEY"},
			"credential_ids": []interface{}{"cred-1"},
		},
	}

	req := decodeSandboxExecutionArgs(tc)
	if req.Code != "print('ok')" || req.Language != "python" {
		t.Fatalf("unexpected sandbox decode: %+v", req)
	}
	if len(req.Libraries) != 2 || req.Libraries[0] != "requests" || req.Libraries[1] != "pydantic" {
		t.Fatalf("Libraries = %v, want [requests pydantic]", req.Libraries)
	}
	if len(req.VaultKeys) != 1 || req.VaultKeys[0] != "API_KEY" {
		t.Fatalf("VaultKeys = %v, want [API_KEY]", req.VaultKeys)
	}
	if len(req.CredentialIDs) != 1 || req.CredentialIDs[0] != "cred-1" {
		t.Fatalf("CredentialIDs = %v, want [cred-1]", req.CredentialIDs)
	}
}

func TestDecodePythonExecutionArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "execute_python",
		Params: map[string]interface{}{
			"code":           "print('hello')",
			"background":     true,
			"vault_keys":     []interface{}{"SECRET"},
			"credential_ids": []interface{}{"cred-2"},
		},
	}

	req := decodePythonExecutionArgs(tc)
	if req.Code != "print('hello')" || !req.Background {
		t.Fatalf("unexpected python decode: %+v", req)
	}
	if len(req.VaultKeys) != 1 || req.VaultKeys[0] != "SECRET" {
		t.Fatalf("VaultKeys = %v, want [SECRET]", req.VaultKeys)
	}
	if len(req.CredentialIDs) != 1 || req.CredentialIDs[0] != "cred-2" {
		t.Fatalf("CredentialIDs = %v, want [cred-2]", req.CredentialIDs)
	}
}

func TestDecodeShellExecutionArgsUsesParamsFallback(t *testing.T) {
	req := decodeShellExecutionArgs(ToolCall{
		Action: "execute_shell",
		Params: map[string]interface{}{
			"command":    "dir",
			"background": true,
		},
	})

	if req.Command != "dir" || !req.Background {
		t.Fatalf("unexpected shell decode: %+v", req)
	}
}

func TestDecodeSudoExecutionArgsUsesParamsFallback(t *testing.T) {
	req := decodeSudoExecutionArgs(ToolCall{
		Action: "execute_sudo",
		Params: map[string]interface{}{
			"command": "systemctl status ssh",
		},
	})

	if req.Command != "systemctl status ssh" {
		t.Fatalf("Command = %q, want systemctl status ssh", req.Command)
	}
}

func TestDecodeInstallPackageArgsUsesParamsFallback(t *testing.T) {
	req := decodeInstallPackageArgs(ToolCall{
		Action: "install_package",
		Params: map[string]interface{}{
			"package": "ffmpeg",
		},
	})

	if req.Package != "ffmpeg" {
		t.Fatalf("Package = %q, want ffmpeg", req.Package)
	}
}

func TestDecodeProcessControlArgsUsesParamsFallback(t *testing.T) {
	req := decodeProcessControlArgs(ToolCall{
		Action: "stop_process",
		Params: map[string]interface{}{
			"pid": float64(77),
		},
	})

	if req.PID != 77 {
		t.Fatalf("PID = %d, want 77", req.PID)
	}
}

func TestDecodeUpdateManagementArgsUsesParamsFallback(t *testing.T) {
	req := decodeUpdateManagementArgs(ToolCall{
		Action: "manage_updates",
		Params: map[string]interface{}{
			"operation": "check",
		},
	})

	if req.Operation != "check" {
		t.Fatalf("Operation = %q, want check", req.Operation)
	}
}

func TestDecodeArchiveArgsUsesParamsFallback(t *testing.T) {
	req := decodeArchiveArgs(ToolCall{
		Action: "archive",
		Params: map[string]interface{}{
			"operation":    "extract",
			"path":         "bundle.zip",
			"dest":         "out",
			"source_files": "a.txt,b.txt",
			"format":       "zip",
		},
	})

	if req.Operation != "extract" || req.FilePath != "bundle.zip" || req.Destination != "out" {
		t.Fatalf("unexpected archive decode: %+v", req)
	}
	if req.SourceFiles != "a.txt,b.txt" || req.Format != "zip" {
		t.Fatalf("unexpected archive decode: %+v", req)
	}
}

func TestDecodePDFOperationArgsUsesParamsFallback(t *testing.T) {
	req := decodePDFOperationArgs(ToolCall{
		Action: "pdf_operations",
		Params: map[string]interface{}{
			"operation":      "watermark",
			"path":           "input.pdf",
			"destination":    "output.pdf",
			"pages":          "1-2",
			"password":       "secret",
			"watermark_text": "AuraGo",
			"source_files":   "a.pdf,b.pdf",
		},
	})

	if req.Operation != "watermark" || req.FilePath != "input.pdf" || req.OutputFile != "output.pdf" {
		t.Fatalf("unexpected pdf decode: %+v", req)
	}
	if req.Pages != "1-2" || req.Password != "secret" || req.WatermarkText != "AuraGo" || req.SourceFiles != "a.pdf,b.pdf" {
		t.Fatalf("unexpected pdf decode: %+v", req)
	}
}

func TestDecodeImageProcessingArgsUsesParamsFallback(t *testing.T) {
	req := decodeImageProcessingArgs(ToolCall{
		Action: "image_processing",
		Params: map[string]interface{}{
			"operation":     "resize",
			"path":          "input.png",
			"destination":   "output.jpg",
			"output_format": "jpeg",
			"width":         float64(800),
			"height":        float64(600),
			"quality_pct":   float64(82),
			"crop_x":        float64(10),
			"crop_y":        float64(12),
			"crop_width":    float64(500),
			"crop_height":   float64(400),
			"angle":         float64(90),
		},
	})

	if req.Operation != "resize" || req.FilePath != "input.png" || req.OutputFile != "output.jpg" || req.OutputFormat != "jpeg" {
		t.Fatalf("unexpected image decode: %+v", req)
	}
	if req.Width != 800 || req.Height != 600 || req.QualityPct != 82 || req.CropX != 10 || req.CropY != 12 || req.CropWidth != 500 || req.CropHeight != 400 || req.Angle != 90 {
		t.Fatalf("unexpected image decode: %+v", req)
	}
}

func TestDecodeAPIRequestArgsUsesParamsFallback(t *testing.T) {
	req := decodeAPIRequestArgs(ToolCall{
		Action: "api_request",
		Params: map[string]interface{}{
			"method": "POST",
			"url":    "https://example.com/api",
			"body":   "{\"ok\":true}",
			"headers": map[string]interface{}{
				"Authorization": "Bearer token",
				"Content-Type":  "application/json",
			},
		},
	})

	if req.Method != "POST" || req.URL != "https://example.com/api" || req.Body != "{\"ok\":true}" {
		t.Fatalf("unexpected api request decode: %+v", req)
	}
	if req.Headers["Authorization"] != "Bearer token" || req.Headers["Content-Type"] != "application/json" {
		t.Fatalf("unexpected headers decode: %+v", req.Headers)
	}
}

func TestDecodeFilesystemArgsUsesParamsFallback(t *testing.T) {
	req := decodeFilesystemArgs(ToolCall{
		Action: "filesystem",
		Params: map[string]interface{}{
			"operation": "copy_batch",
			"path":      "source.txt",
			"dest":      "dest.txt",
			"content":   "payload",
			"items": []interface{}{
				map[string]interface{}{"path": "a.txt", "dest": "b.txt"},
			},
		},
	})

	if req.Operation != "copy_batch" || req.FilePath != "source.txt" || req.Destination != "dest.txt" || req.Content != "payload" {
		t.Fatalf("unexpected filesystem decode: %+v", req)
	}
	if len(req.Items) != 1 || req.Items[0]["path"] != "a.txt" || req.Items[0]["dest"] != "b.txt" {
		t.Fatalf("unexpected filesystem items: %+v", req.Items)
	}
}

func TestDecodeFileEditorArgsUsesParamsFallback(t *testing.T) {
	req := decodeFileEditorArgs(ToolCall{
		Action: "file_editor",
		Params: map[string]interface{}{
			"operation":  "replace",
			"path":       "notes.txt",
			"old":        "before",
			"new":        "after",
			"marker":     "anchor",
			"content":    "body",
			"start_line": float64(2),
			"end_line":   float64(5),
			"line_count": float64(3),
		},
	})

	if req.Operation != "replace" || req.FilePath != "notes.txt" || req.Old != "before" || req.New != "after" || req.Marker != "anchor" || req.Content != "body" {
		t.Fatalf("unexpected file editor decode: %+v", req)
	}
	if req.StartLine != 2 || req.EndLine != 5 || req.LineCount != 3 {
		t.Fatalf("unexpected file editor decode: %+v", req)
	}
}

func TestDecodeStructuredEditorArgsUseParamsFallback(t *testing.T) {
	jsonReq := decodeJSONEditorArgs(ToolCall{
		Action: "json_editor",
		Params: map[string]interface{}{
			"operation": "set",
			"path":      "config.json",
			"json_path": "features.enabled",
			"set_value": true,
			"content":   "{\"features\":{}}",
		},
	})
	if jsonReq.Operation != "set" || jsonReq.FilePath != "config.json" || jsonReq.JsonPath != "features.enabled" || jsonReq.Content != "{\"features\":{}}" {
		t.Fatalf("unexpected json editor decode: %+v", jsonReq)
	}
	if value, ok := jsonReq.SetValue.(bool); !ok || !value {
		t.Fatalf("unexpected json set value: %#v", jsonReq.SetValue)
	}

	yamlReq := decodeYAMLEditorArgs(ToolCall{
		Action: "yaml_editor",
		Params: map[string]interface{}{
			"operation": "delete",
			"path":      "config.yaml",
			"json_path": "spec.replicas",
			"set_value": "gone",
		},
	})
	if yamlReq.Operation != "delete" || yamlReq.FilePath != "config.yaml" || yamlReq.JsonPath != "spec.replicas" {
		t.Fatalf("unexpected yaml editor decode: %+v", yamlReq)
	}
	if yamlReq.SetValue != "gone" {
		t.Fatalf("unexpected yaml set value: %#v", yamlReq.SetValue)
	}

	xmlReq := decodeXMLEditorArgs(ToolCall{
		Action: "xml_editor",
		Params: map[string]interface{}{
			"operation": "set_text",
			"path":      "doc.xml",
			"xpath":     "/root/title",
			"set_value": "AuraGo",
		},
	})
	if xmlReq.Operation != "set_text" || xmlReq.FilePath != "doc.xml" || xmlReq.XPath != "/root/title" || xmlReq.SetValue != "AuraGo" {
		t.Fatalf("unexpected xml editor decode: %+v", xmlReq)
	}
}

func TestDecodeFileReadArgsUseParamsFallback(t *testing.T) {
	searchReq := decodeFileSearchArgs(ToolCall{
		Action: "file_search",
		Params: map[string]interface{}{
			"operation":   "search",
			"path":        ".",
			"pattern":     "TODO",
			"glob":        "*.go",
			"output_mode": "lines",
		},
	})
	if searchReq.Operation != "search" || searchReq.FilePath != "." || searchReq.Pattern != "TODO" || searchReq.Glob != "*.go" || searchReq.OutputMode != "lines" {
		t.Fatalf("unexpected file search decode: %+v", searchReq)
	}

	advancedReq := decodeAdvancedFileReadArgs(ToolCall{
		Action: "file_reader_advanced",
		Params: map[string]interface{}{
			"operation":  "lines",
			"path":       "README.md",
			"pattern":    "install",
			"start_line": float64(10),
			"end_line":   float64(25),
			"line_count": float64(15),
		},
	})
	if advancedReq.Operation != "lines" || advancedReq.FilePath != "README.md" || advancedReq.Pattern != "install" {
		t.Fatalf("unexpected advanced file read decode: %+v", advancedReq)
	}
	if advancedReq.StartLine != 10 || advancedReq.EndLine != 25 || advancedReq.LineCount != 15 {
		t.Fatalf("unexpected advanced file read decode: %+v", advancedReq)
	}

	smartReq := decodeSmartFileReadArgs(ToolCall{
		Action: "smart_file_read",
		Params: map[string]interface{}{
			"operation":         "semantic",
			"path":              "manual.md",
			"query":             "deployment",
			"sampling_strategy": "distributed",
			"max_tokens":        float64(1200),
			"line_count":        float64(40),
		},
	})
	if smartReq.Operation != "semantic" || smartReq.FilePath != "manual.md" || smartReq.Query != "deployment" || smartReq.SamplingStrategy != "distributed" {
		t.Fatalf("unexpected smart file read decode: %+v", smartReq)
	}
	if smartReq.MaxTokens != 1200 || smartReq.LineCount != 40 {
		t.Fatalf("unexpected smart file read decode: %+v", smartReq)
	}
}

func TestDecodeKnowledgeGraphArgsUsesParamsFallback(t *testing.T) {
	req := decodeKnowledgeGraphArgs(ToolCall{
		Action: "knowledge_graph",
		Params: map[string]interface{}{
			"operation":    "update_edge",
			"id":           "node-1",
			"label":        "Server",
			"source":       "srv-1",
			"target":       "rack-1",
			"relation":     "located_in",
			"new_relation": "runs_in",
			"depth":        float64(3),
			"limit":        float64(25),
			"content":      "search term",
			"properties": map[string]interface{}{
				"role": "db",
			},
		},
	})

	if req.Operation != "update_edge" || req.ID != "node-1" || req.Label != "Server" {
		t.Fatalf("unexpected graph decode: %+v", req)
	}
	if req.Source != "srv-1" || req.Target != "rack-1" || req.Relation != "located_in" || req.NewRelation != "runs_in" {
		t.Fatalf("unexpected edge decode: %+v", req)
	}
	if req.Depth != 3 || req.Limit != 25 || req.Content != "search term" {
		t.Fatalf("unexpected depth/limit/content decode: %+v", req)
	}
	if req.Properties["role"] != "db" {
		t.Fatalf("Properties[role] = %q, want db", req.Properties["role"])
	}
}

func TestDecodeCoreMemoryArgsUsesParamsFallback(t *testing.T) {
	req := decodeCoreMemoryArgs(ToolCall{
		Action: "manage_memory",
		Params: map[string]interface{}{
			"operation":    "add",
			"fact":         "primary fact",
			"key":          "profile",
			"value":        "Nova",
			"id":           "12",
			"memory_key":   "nickname",
			"memory_value": "Nova Prime",
			"content":      "fallback content",
		},
	})

	if req.Operation != "add" || req.ID != "12" {
		t.Fatalf("unexpected core memory decode: %+v", req)
	}
	if req.Fact != "primary fact" || req.Key != "profile" || req.Value != "Nova" {
		t.Fatalf("unexpected fact/key/value decode: %+v", req)
	}
	if req.MemoryKey != "nickname" || req.MemoryValue != "Nova Prime" || req.Content != "fallback content" {
		t.Fatalf("unexpected memory alias decode: %+v", req)
	}
}

func TestDecodeCheatsheetArgsUsesParamsFallback(t *testing.T) {
	req := decodeCheatsheetArgs(ToolCall{
		Action: "cheatsheet",
		Params: map[string]interface{}{
			"operation":     "attach",
			"id":            "sheet-1",
			"name":          "Deploy",
			"content":       "steps",
			"active":        true,
			"filename":      "runbook.md",
			"source":        "upload",
			"attachment_id": "att-1",
		},
	})

	if req.Operation != "attach" || req.ID != "sheet-1" || req.Name != "Deploy" {
		t.Fatalf("unexpected cheatsheet decode: %+v", req)
	}
	if req.Content != "steps" || req.Filename != "runbook.md" || req.Source != "upload" || req.AttachmentID != "att-1" {
		t.Fatalf("unexpected attachment decode: %+v", req)
	}
	if req.Active == nil || !*req.Active {
		t.Fatalf("Active = %v, want true", req.Active)
	}
}

func TestDecodeSecretVaultArgsUsesParamsFallback(t *testing.T) {
	req := decodeSecretVaultArgs(ToolCall{
		Action: "set_secret",
		Params: map[string]interface{}{
			"operation": "store",
			"key":       "api_token",
			"value":     "secret-value",
		},
	})

	if req.Action != "set_secret" || req.Operation != "store" {
		t.Fatalf("unexpected secret decode: %+v", req)
	}
	if req.Key != "api_token" || req.Value != "secret-value" {
		t.Fatalf("unexpected key/value decode: %+v", req)
	}
}

func TestDecodeCronScheduleArgsUsesParamsFallback(t *testing.T) {
	req := decodeCronScheduleArgs(ToolCall{
		Action: "manage_schedule",
		Params: map[string]interface{}{
			"operation":   "add",
			"id":          "job-1",
			"cron_expr":   "0 9 * * *",
			"task_prompt": "send daily summary",
		},
	})

	if req.Operation != "add" || req.ID != "job-1" {
		t.Fatalf("unexpected cron decode: %+v", req)
	}
	if req.CronExpr != "0 9 * * *" || req.TaskPrompt != "send daily summary" {
		t.Fatalf("unexpected cron fields: %+v", req)
	}
}

func TestDecodeDocumentCreatorArgsUsesParamsFallback(t *testing.T) {
	req := decodeDocumentCreatorArgs(ToolCall{
		Action: "document_creator",
		Params: map[string]interface{}{
			"operation":    "create_pdf",
			"title":        "Weekly Report",
			"content":      "Hello",
			"url":          "https://example.com",
			"filename":     "weekly-report",
			"paper_size":   "A4",
			"landscape":    true,
			"sections":     "[{\"title\":\"Intro\"}]",
			"source_files": "[\"a.md\",\"b.md\"]",
		},
	})

	if req.Operation != "create_pdf" || req.Title != "Weekly Report" {
		t.Fatalf("unexpected document decode: %+v", req)
	}
	if req.URL != "https://example.com" || req.Filename != "weekly-report" || req.PaperSize != "A4" {
		t.Fatalf("unexpected document metadata: %+v", req)
	}
	if !req.Landscape || req.Sections == "" || req.SourceFiles == "" {
		t.Fatalf("unexpected document options: %+v", req)
	}
}
