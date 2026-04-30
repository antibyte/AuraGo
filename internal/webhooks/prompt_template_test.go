package webhooks

import (
	"strings"
	"testing"
)

func TestValidatePromptTemplateRejectsTemplateLogic(t *testing.T) {
	t.Parallel()

	err := ValidatePromptTemplate(`{{if .Payload}}payload{{end}}`)
	if err == nil || !strings.Contains(err.Error(), "unsupported placeholder") {
		t.Fatalf("ValidatePromptTemplate() error = %v, want unsupported placeholder", err)
	}
}

func TestRenderPromptTemplateSupportsExplicitPlaceholders(t *testing.T) {
	t.Parallel()

	rendered, err := renderPromptTemplate(`Name={{webhook_name}} Slug={{slug}} F={{field.repo}} H={{header.X-Test}}`, PromptData{
		WebhookName: "Deploy Hook",
		Slug:        "deploy-hook",
		Payload:     "{}",
		Timestamp:   "2026-04-18T00:00:00Z",
		Fields: map[string]interface{}{
			"repo": "antibyte/AuraGo",
		},
		Headers: map[string]string{
			"X-Test": "ok",
		},
	})
	if err != nil {
		t.Fatalf("renderPromptTemplate() error = %v", err)
	}
	if rendered != "Name=Deploy Hook Slug=deploy-hook F=antibyte/AuraGo H=ok" {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestRenderPromptTemplateRejectsUnknownFieldPlaceholder(t *testing.T) {
	t.Parallel()

	_, err := renderPromptTemplate(`{{field.repo}}`, PromptData{
		WebhookName: "Deploy Hook",
		Slug:        "deploy-hook",
		Payload:     "{}",
		Timestamp:   "2026-04-18T00:00:00Z",
		Fields:      map[string]interface{}{},
		Headers:     map[string]string{},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown field placeholder") {
		t.Fatalf("renderPromptTemplate() error = %v, want missing field error", err)
	}
}

func TestPresetPromptHintsValidate(t *testing.T) {
	t.Parallel()

	for _, preset := range Presets() {
		preset := preset
		t.Run(preset.Key, func(t *testing.T) {
			t.Parallel()
			if err := ValidatePromptTemplate(preset.PromptHint); err != nil {
				t.Fatalf("ValidatePromptTemplate(%s) error = %v", preset.Key, err)
			}
		})
	}
}
