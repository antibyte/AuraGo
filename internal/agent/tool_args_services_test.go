package agent

import "testing"

func TestDecodeImageAnalysisArgsUsesPathFallback(t *testing.T) {
	tc := ToolCall{
		Action: "analyze_image",
		Params: map[string]interface{}{
			"path":   "media/demo.png",
			"prompt": "Summarize the image",
		},
	}

	req := decodeImageAnalysisArgs(tc)
	if req.FilePath != "media/demo.png" {
		t.Fatalf("FilePath = %q, want media/demo.png", req.FilePath)
	}
	if req.Prompt != "Summarize the image" {
		t.Fatalf("Prompt = %q, want custom prompt", req.Prompt)
	}
}

func TestDecodeMeshCentralArgsNormalizesOperationAndUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "meshcentral",
		Params: map[string]interface{}{
			"operation":    "wakeonlan",
			"mesh_id":      "mesh//group",
			"node_id":      "node//abc",
			"power_action": float64(4),
			"command":      "hostname",
		},
	}

	req := decodeMeshCentralArgs(tc)
	if req.Operation != "wake" {
		t.Fatalf("Operation = %q, want wake", req.Operation)
	}
	if req.MeshID != "mesh//group" {
		t.Fatalf("MeshID = %q, want mesh//group", req.MeshID)
	}
	if req.NodeID != "node//abc" {
		t.Fatalf("NodeID = %q, want node//abc", req.NodeID)
	}
	if req.PowerAction != 4 {
		t.Fatalf("PowerAction = %d, want 4", req.PowerAction)
	}
	if req.Command != "hostname" {
		t.Fatalf("Command = %q, want hostname", req.Command)
	}
}

func TestDecodeWebDAVArgsUsesAliasesAndFallbacks(t *testing.T) {
	tc := ToolCall{
		Action: "webdav",
		Body:   "fallback body",
		Params: map[string]interface{}{
			"operation":   "move",
			"remote_path": "/docs/source.txt",
			"dest":        "/docs/archive.txt",
		},
	}

	req := decodeWebDAVArgs(tc)
	if req.Operation != "move" {
		t.Fatalf("Operation = %q, want move", req.Operation)
	}
	if req.Path != "/docs/source.txt" {
		t.Fatalf("Path = %q, want /docs/source.txt", req.Path)
	}
	if req.Destination != "/docs/archive.txt" {
		t.Fatalf("Destination = %q, want /docs/archive.txt", req.Destination)
	}
	if req.Content != "fallback body" {
		t.Fatalf("Content = %q, want fallback body", req.Content)
	}
}

func TestDecodeS3ArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "s3_storage",
		Params: map[string]interface{}{
			"operation":          "copy",
			"bucket":             "source-bucket",
			"key":                "images/logo.png",
			"local_path":         "downloads/logo.png",
			"prefix":             "images/",
			"destination_bucket": "backup-bucket",
			"destination_key":    "archive/logo.png",
		},
	}

	req := decodeS3Args(tc)
	if req.Operation != "copy" {
		t.Fatalf("Operation = %q, want copy", req.Operation)
	}
	if req.Bucket != "source-bucket" || req.Key != "images/logo.png" {
		t.Fatalf("unexpected bucket/key decode: %+v", req)
	}
	if req.LocalPath != "downloads/logo.png" || req.Prefix != "images/" {
		t.Fatalf("unexpected local/prefix decode: %+v", req)
	}
	if req.DestinationBucket != "backup-bucket" || req.DestinationKey != "archive/logo.png" {
		t.Fatalf("unexpected destination decode: %+v", req)
	}
}

func TestDecodeManageSQLConnectionsArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "manage_sql_connections",
		Params: map[string]interface{}{
			"operation":       "create",
			"connection_name": "analytics",
			"driver":          "postgres",
			"host":            "db.internal",
			"port":            float64(5432),
			"database_name":   "events",
			"description":     "warehouse",
			"ssl_mode":        "require",
			"allow_read":      true,
			"allow_write":     false,
			"allow_change":    true,
			"allow_delete":    false,
			"username":        "reporter",
			"password":        "secret",
			"docker_template": "postgres",
		},
	}

	req := decodeManageSQLConnectionsArgs(tc)
	if req.Operation != "create" {
		t.Fatalf("Operation = %q, want create", req.Operation)
	}
	if req.ConnectionName != "analytics" || req.Driver != "postgres" {
		t.Fatalf("unexpected connection metadata: %+v", req)
	}
	if req.Host != "db.internal" || req.Port != 5432 || req.DatabaseName != "events" {
		t.Fatalf("unexpected endpoint decode: %+v", req)
	}
	if req.Description != "warehouse" || req.SSLMode != "require" {
		t.Fatalf("unexpected description/ssl decode: %+v", req)
	}
	if req.AllowRead == nil || *req.AllowRead != true {
		t.Fatalf("AllowRead = %#v, want true", req.AllowRead)
	}
	if req.AllowWrite == nil || *req.AllowWrite != false {
		t.Fatalf("AllowWrite = %#v, want false", req.AllowWrite)
	}
	if req.AllowChange == nil || *req.AllowChange != true {
		t.Fatalf("AllowChange = %#v, want true", req.AllowChange)
	}
	if req.AllowDelete == nil || *req.AllowDelete != false {
		t.Fatalf("AllowDelete = %#v, want false", req.AllowDelete)
	}
	if req.Username != "reporter" || req.Password != "secret" {
		t.Fatalf("unexpected credentials decode: %+v", req)
	}
	if req.DockerTemplate != "postgres" {
		t.Fatalf("DockerTemplate = %q, want postgres", req.DockerTemplate)
	}
}

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
