package webhooks

// Preset defines a pre-configured webhook format for common services.
type Preset struct {
	Key        string        `json:"key"`
	Label      string        `json:"label"`
	Format     WebhookFormat `json:"format"`
	PromptHint string        `json:"prompt_hint"` // suggested prompt template snippet
}

// Presets returns all built-in webhook format presets.
func Presets() []Preset {
	return []Preset{
		{
			Key:   "generic_json",
			Label: "Generic JSON",
			Format: WebhookFormat{
				AcceptedContentTypes: []string{"application/json"},
				Description:          "Accept any JSON payload and forward it entirely.",
			},
			PromptHint: "[Webhook: {{.WebhookName}}]\nPayload:\n{{.Payload}}",
		},
		{
			Key:   "github",
			Label: "GitHub",
			Format: WebhookFormat{
				AcceptedContentTypes: []string{"application/json"},
				Fields: []FieldMapping{
					{Source: "action", Alias: "action"},
					{Source: "repository.full_name", Alias: "repo"},
					{Source: "sender.login", Alias: "user"},
				},
				SignatureHeader: "X-Hub-Signature-256",
				SignatureAlgo:   "sha256",
				Description:     "GitHub webhook events (push, PR, issues, etc.)",
			},
			PromptHint: "[GitHub Event: {{.WebhookName}}]\nRepo: {{.Fields.repo}}\nAction: {{.Fields.action}}\nUser: {{.Fields.user}}\nFull payload: {{.Payload}}",
		},
		{
			Key:   "gitlab",
			Label: "GitLab",
			Format: WebhookFormat{
				AcceptedContentTypes: []string{"application/json"},
				Fields: []FieldMapping{
					{Source: "object_kind", Alias: "event_type"},
					{Source: "project.path_with_namespace", Alias: "project"},
					{Source: "user.username", Alias: "user"},
				},
				SignatureHeader: "X-Gitlab-Token",
				SignatureAlgo:   "plain",
				Description:     "GitLab webhook events.",
			},
			PromptHint: "[GitLab Event: {{.WebhookName}}]\nProject: {{.Fields.project}}\nType: {{.Fields.event_type}}\nUser: {{.Fields.user}}",
		},
		{
			Key:   "home_assistant",
			Label: "Home Assistant",
			Format: WebhookFormat{
				AcceptedContentTypes: []string{"application/json"},
				Fields: []FieldMapping{
					{Source: "entity_id", Alias: "entity"},
					{Source: "new_state.state", Alias: "new_state"},
					{Source: "old_state.state", Alias: "old_state"},
				},
				Description: "Home Assistant automation webhook events.",
			},
			PromptHint: "[Home Assistant: {{.WebhookName}}]\nEntity: {{.Fields.entity}}\nState: {{.Fields.old_state}} → {{.Fields.new_state}}",
		},
		{
			Key:   "uptime",
			Label: "Uptime / Monitoring",
			Format: WebhookFormat{
				AcceptedContentTypes: []string{"application/json"},
				Fields: []FieldMapping{
					{Source: "monitor.name", Alias: "monitor"},
					{Source: "alert_type", Alias: "alert_type"},
					{Source: "msg", Alias: "message"},
				},
				Description: "Uptime monitoring alerts (Uptime Kuma, Hetrix, etc.)",
			},
			PromptHint: "[Alert: {{.WebhookName}}]\nMonitor: {{.Fields.monitor}}\nType: {{.Fields.alert_type}}\nMessage: {{.Fields.message}}",
		},
		{
			Key:   "plain_text",
			Label: "Plain Text",
			Format: WebhookFormat{
				AcceptedContentTypes: []string{"text/plain", "application/json"},
				Description:          "Accept plain text or JSON body and forward as-is.",
			},
			PromptHint: "[Webhook: {{.WebhookName}}]\n{{.Payload}}",
		},
	}
}
