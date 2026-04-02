package agent

import "testing"

func TestDecodeManageWebhooksArgsUsesActionAlias(t *testing.T) {
	tc := ToolCall{
		Action: "manage_webhooks",
		Params: map[string]interface{}{
			"action":   "create",
			"name":     "Inbox Hook",
			"slug":     "inbox-hook",
			"token_id": "token-1",
			"enabled":  true,
		},
	}

	req := decodeManageWebhooksArgs(tc)
	if req.Operation != "create" {
		t.Fatalf("Operation = %q, want create", req.Operation)
	}
	if req.Name != "Inbox Hook" {
		t.Fatalf("Name = %q, want Inbox Hook", req.Name)
	}
	if req.Slug != "inbox-hook" {
		t.Fatalf("Slug = %q, want inbox-hook", req.Slug)
	}
	if req.TokenID != "token-1" {
		t.Fatalf("TokenID = %q, want token-1", req.TokenID)
	}
	if !req.Enabled {
		t.Fatal("expected Enabled to be true")
	}
}

func TestDecodeHomeAssistantArgsUsesServiceDataFallback(t *testing.T) {
	tc := ToolCall{
		Action: "home_assistant",
		Params: map[string]interface{}{
			"operation": "call_service",
			"domain":    "light",
			"service":   "turn_on",
			"entity_id": "light.desk",
			"service_data": map[string]interface{}{
				"brightness": float64(255),
			},
		},
	}

	req := decodeHomeAssistantArgs(tc)
	if req.Operation != "call_service" {
		t.Fatalf("Operation = %q, want call_service", req.Operation)
	}
	if req.Domain != "light" || req.Service != "turn_on" || req.EntityID != "light.desk" {
		t.Fatalf("decoded request = %+v", req)
	}
	if got, _ := req.ServiceData["brightness"].(float64); got != 255 {
		t.Fatalf("ServiceData[brightness] = %v, want 255", got)
	}
}

func TestDecodeMediaRegistryArgsParsesTagsAndID(t *testing.T) {
	tc := ToolCall{
		Action: "media_registry",
		Tags:   "cover, album",
		Params: map[string]interface{}{
			"operation":  "tag",
			"id":         float64(42),
			"media_type": "audio",
			"tag_mode":   "set",
			"limit":      float64(5),
		},
	}

	req := decodeMediaRegistryArgs(tc)
	if req.Operation != "tag" {
		t.Fatalf("Operation = %q, want tag", req.Operation)
	}
	if req.ID != 42 {
		t.Fatalf("ID = %d, want 42", req.ID)
	}
	if req.MediaType != "audio" {
		t.Fatalf("MediaType = %q, want audio", req.MediaType)
	}
	if req.TagMode != "set" {
		t.Fatalf("TagMode = %q, want set", req.TagMode)
	}
	if len(req.Tags) != 2 || req.Tags[0] != "cover" || req.Tags[1] != "album" {
		t.Fatalf("Tags = %v, want [cover album]", req.Tags)
	}
	if req.Limit != 5 {
		t.Fatalf("Limit = %d, want 5", req.Limit)
	}
}

func TestDecodeHomepageRegistryArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "homepage_registry",
		Params: map[string]interface{}{
			"operation":   "update",
			"id":          "7",
			"name":        "Aura Site",
			"status":      "maintenance",
			"reason":      "deploy rollback",
			"problem":     "broken build",
			"notes":       "investigating",
			"project_dir": "sites/aura",
			"tags":        []interface{}{"frontend", "landing"},
		},
	}

	req := decodeHomepageRegistryArgs(tc)
	if req.Operation != "update" {
		t.Fatalf("Operation = %q, want update", req.Operation)
	}
	if req.ID != 7 {
		t.Fatalf("ID = %d, want 7", req.ID)
	}
	if req.Name != "Aura Site" {
		t.Fatalf("Name = %q, want Aura Site", req.Name)
	}
	if req.Status != "maintenance" || req.Reason != "deploy rollback" || req.Problem != "broken build" {
		t.Fatalf("decoded request = %+v", req)
	}
	if len(req.Tags) != 2 || req.Tags[0] != "frontend" || req.Tags[1] != "landing" {
		t.Fatalf("Tags = %v, want [frontend landing]", req.Tags)
	}
}
