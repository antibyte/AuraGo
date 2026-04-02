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
