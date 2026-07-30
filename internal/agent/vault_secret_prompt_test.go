package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/vaultprompt"

	openai "github.com/sashabaranov/go-openai"
)

type vaultPromptDispatchBroker struct {
	manager *vaultprompt.Manager
	secret  string
	events  []string
}

func (b *vaultPromptDispatchBroker) Send(string, string) {}
func (b *vaultPromptDispatchBroker) SendJSON(string)     {}
func (b *vaultPromptDispatchBroker) SendLLMStreamDelta(string, string, string, int, string) {
}
func (b *vaultPromptDispatchBroker) SendLLMStreamDone(string) {}
func (b *vaultPromptDispatchBroker) SendTokenUpdate(int, int, int, int, int, bool, bool, string) {
}
func (b *vaultPromptDispatchBroker) SendThinkingBlock(string, string, string) {}

func (b *vaultPromptDispatchBroker) SendTyped(event string, payload interface{}) bool {
	b.events = append(b.events, event)
	if event != vaultprompt.EventPrompt {
		return true
	}
	raw, _ := json.Marshal(payload)
	var prompt vaultprompt.PromptPayload
	if json.Unmarshal(raw, &prompt) != nil {
		return false
	}
	_, _ = b.manager.Submit(prompt.SessionID, prompt.RequestID, prompt.VaultKey, b.secret)
	return true
}

func TestRequestVaultSecretDispatchNeverReturnsValue(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	manager := vaultprompt.NewManager(vault, time.Second)
	const sentinel = "agent-must-never-see-this-value"
	broker := &vaultPromptDispatchBroker{manager: manager, secret: sentinel}
	cfg := &config.Config{}
	cfg.Tools.SecretsVault.Enabled = true

	output, handled := dispatchComm(context.Background(), ToolCall{
		Action: "request_vault_secret",
		Prompt: "Enter the service credential.",
		Key:    "service_api_key",
		IsTool: true,
		Params: map[string]interface{}{"replace": true},
	}, &DispatchContext{
		Cfg:                 cfg,
		Broker:              broker,
		VaultSecretPrompter: manager,
		VaultSecretTarget: vaultprompt.Target{
			Channel:         "agodesk",
			ClientSessionID: "agodesk:device-test",
			ConversationID:  "conversation-test",
		},
	})

	if !handled {
		t.Fatal("request_vault_secret was not handled")
	}
	if strings.Contains(output, sentinel) {
		t.Fatal("tool output contains the secret")
	}
	if !strings.Contains(output, `"status":"stored"`) ||
		!strings.Contains(output, `"vault_key":"SERVICE_API_KEY"`) ||
		!strings.Contains(output, `"present":true`) {
		t.Fatalf("unexpected tool output: %s", output)
	}
	present, readable, infoErr := vault.AgentSecretInfo("SERVICE_API_KEY")
	if infoErr != nil || !present || readable {
		t.Fatalf("agent secret info = present %v readable %v err %v", present, readable, infoErr)
	}
}

func TestRequestVaultSecretDispatchRejectsInvalidKeyBeforePrompt(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("c", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	manager := vaultprompt.NewManager(vault, time.Second)
	broker := &vaultPromptDispatchBroker{manager: manager, secret: "unused"}

	output, handled := dispatchComm(context.Background(), ToolCall{
		Action: "request_vault_secret",
		Prompt: "Enter a provider credential.",
		Key:    "provider_main_api_key",
		IsTool: true,
	}, &DispatchContext{
		Cfg:                 &config.Config{},
		Broker:              broker,
		VaultSecretPrompter: manager,
		VaultSecretTarget: vaultprompt.Target{
			Channel:         "web_chat",
			ClientSessionID: "default",
			ConversationID:  "default",
		},
	})

	if !handled || !strings.Contains(output, vaultprompt.ErrorKeyInvalid) {
		t.Fatalf("output = %q, handled = %v", output, handled)
	}
	if len(broker.events) != 0 {
		t.Fatalf("invalid key emitted events: %v", broker.events)
	}
}

func TestRequestVaultSecretSchemaIsExplicitAndVaultGated(t *testing.T) {
	find := func(schemas []openai.Tool) *openai.FunctionDefinition {
		for _, schema := range schemas {
			if schema.Function != nil && schema.Function.Name == "request_vault_secret" {
				return schema.Function
			}
		}
		return nil
	}
	if got := find(builtinToolSchemas(ToolFeatureFlags{})); got != nil {
		t.Fatal("request_vault_secret schema exists while the Vault tool is disabled")
	}
	definition := find(builtinToolSchemas(ToolFeatureFlags{SecretsVaultEnabled: true}))
	if definition == nil {
		t.Fatal("request_vault_secret schema missing while the Vault tool is enabled")
	}
	parameters, ok := definition.Parameters.(map[string]interface{})
	if !ok {
		t.Fatalf("parameters type = %T", definition.Parameters)
	}
	properties, ok := parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties = %#v", parameters["properties"])
	}
	for _, name := range []string{"prompt", "vault_key", "replace"} {
		if _, exists := properties[name]; !exists {
			t.Fatalf("schema missing %q", name)
		}
	}
	promptSchema, _ := properties["prompt"].(map[string]interface{})
	keySchema, _ := properties["vault_key"].(map[string]interface{})
	replaceSchema, _ := properties["replace"].(map[string]interface{})
	if promptSchema["maxLength"] != 2000 || keySchema["maxLength"] != 64 ||
		keySchema["pattern"] != "^[A-Z0-9_]{1,64}$" || replaceSchema["default"] != true {
		t.Fatalf("schema constraints = prompt %#v key %#v replace %#v", promptSchema, keySchema, replaceSchema)
	}
	required, ok := parameters["required"].([]string)
	if !ok || len(required) != 2 || required[0] != "prompt" || required[1] != "vault_key" {
		t.Fatalf("required = %#v", parameters["required"])
	}
}
