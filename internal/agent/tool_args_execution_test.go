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
