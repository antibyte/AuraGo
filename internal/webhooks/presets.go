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
			PromptHint: "[Webhook: {{webhook_name}}]\nPayload:\n{{payload}}",
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
			PromptHint: "[GitHub Event: {{webhook_name}}]\nRepo: {{field.repo}}\nAction: {{field.action}}\nUser: {{field.user}}\nFull payload: {{payload}}",
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
			PromptHint: "[GitLab Event: {{webhook_name}}]\nProject: {{field.project}}\nType: {{field.event_type}}\nUser: {{field.user}}",
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
			PromptHint: "[Home Assistant: {{webhook_name}}]\nEntity: {{field.entity}}\nState: {{field.old_state}} -> {{field.new_state}}",
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
			PromptHint: "[Alert: {{webhook_name}}]\nMonitor: {{field.monitor}}\nType: {{field.alert_type}}\nMessage: {{field.message}}",
		},
		{
			Key:   "plain_text",
			Label: "Plain Text",
			Format: WebhookFormat{
				AcceptedContentTypes: []string{"text/plain", "application/json"},
				Description:          "Accept plain text or JSON body and forward as-is.",
			},
			PromptHint: "[Webhook: {{webhook_name}}]\n{{payload}}",
		},
	}
}
