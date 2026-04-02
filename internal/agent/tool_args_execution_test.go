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
