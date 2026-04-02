package agent

import (
	"strconv"
	"strings"
)

type manageWebhooksArgs struct {
	Operation string
	ID        string
	Name      string
	Slug      string
	Enabled   bool
	TokenID   string
}

type imageAnalysisArgs struct {
	FilePath string
	Prompt   string
}

type meshCentralArgs struct {
	Operation   string
	MeshID      string
	NodeID      string
	PowerAction int
	Command     string
}

type webDAVArgs struct {
	Operation   string
	Path        string
	Content     string
	Destination string
}

type s3Args struct {
	Operation         string
	Bucket            string
	Key               string
	LocalPath         string
	Prefix            string
	DestinationBucket string
	DestinationKey    string
}

type homeAssistantArgs struct {
	Operation   string
	Domain      string
	EntityID    string
	Service     string
	ServiceData map[string]interface{}
}

type mediaRegistryArgs struct {
	Operation   string
	Query       string
	MediaType   string
	Description string
	Tags        []string
	TagMode     string
	ID          int64
	Limit       int
	Offset      int
	Filename    string
	FilePath    string
}

type homepageRegistryArgs struct {
	Operation   string
	Query       string
	Name        string
	Description string
	Framework   string
	ProjectDir  string
	URL         string
	Status      string
	Reason      string
	Problem     string
	Notes       string
	Tags        []string
	ID          int64
	Limit       int
	Offset      int
}

func toolArgInt64(args map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok {
			continue
		}
		switch value := raw.(type) {
		case int:
			return int64(value)
		case int32:
			return int64(value)
		case int64:
			return value
		case float32:
			return int64(value)
		case float64:
			return int64(value)
		case string:
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func toolArgTags(args map[string]interface{}, fallback string, keys ...string) []string {
	if values := toolArgStringSlice(args, keys...); len(values) > 0 {
		return values
	}
	if strings.TrimSpace(fallback) == "" {
		return nil
	}
	parts := strings.Split(fallback, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func decodeManageWebhooksArgs(tc ToolCall) manageWebhooksArgs {
	req := manageWebhooksArgs{
		Operation: firstNonEmptyToolString(tc.Operation, tc.SubOperation, toolArgString(tc.Params, "operation", "action")),
		ID:        firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		Name:      firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Slug:      firstNonEmptyToolString(tc.Slug, toolArgString(tc.Params, "slug")),
		TokenID:   firstNonEmptyToolString(tc.TokenID, toolArgString(tc.Params, "token_id")),
		Enabled:   tc.Enabled,
	}
	if enabled, ok := toolArgBool(tc.Params, "enabled"); ok {
		req.Enabled = enabled
	}
	return req
}

func decodeImageAnalysisArgs(tc ToolCall) imageAnalysisArgs {
	return imageAnalysisArgs{
		FilePath: firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		Prompt:   firstNonEmptyToolString(tc.Prompt, toolArgString(tc.Params, "prompt")),
	}
}

func normalizeMeshCentralOp(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "meshes":
		return "list_groups"
	case "nodes":
		return "list_devices"
	case "wakeonlan":
		return "wake"
	default:
		return strings.ToLower(strings.TrimSpace(op))
	}
}

func decodeMeshCentralArgs(tc ToolCall) meshCentralArgs {
	return meshCentralArgs{
		Operation:   normalizeMeshCentralOp(firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))),
		MeshID:      firstNonEmptyToolString(tc.MeshID, toolArgString(tc.Params, "mesh_id")),
		NodeID:      firstNonEmptyToolString(tc.NodeID, toolArgString(tc.Params, "node_id")),
		PowerAction: firstNonEmptyInt(tc.PowerAction, toolArgInt(tc.Params, 0, "power_action")),
		Command:     firstNonEmptyToolString(tc.Command, toolArgString(tc.Params, "command")),
	}
}

func decodeWebDAVArgs(tc ToolCall) webDAVArgs {
	req := webDAVArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Path:        firstNonEmptyToolString(tc.Path, tc.RemotePath, tc.FilePath, toolArgString(tc.Params, "path", "remote_path", "file_path")),
		Content:     firstNonEmptyToolString(tc.Content, tc.Body, toolArgString(tc.Params, "content", "body")),
		Destination: firstNonEmptyToolString(tc.Destination, tc.Dest, toolArgString(tc.Params, "destination", "dest")),
	}
	return req
}

func decodeS3Args(tc ToolCall) s3Args {
	return s3Args{
		Operation:         firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Bucket:            firstNonEmptyToolString(tc.Bucket, toolArgString(tc.Params, "bucket")),
		Key:               firstNonEmptyToolString(tc.Key, toolArgString(tc.Params, "key")),
		LocalPath:         firstNonEmptyToolString(tc.LocalPath, toolArgString(tc.Params, "local_path")),
		Prefix:            firstNonEmptyToolString(tc.Prefix, toolArgString(tc.Params, "prefix")),
		DestinationBucket: firstNonEmptyToolString(tc.DestinationBucket, toolArgString(tc.Params, "destination_bucket")),
		DestinationKey:    firstNonEmptyToolString(tc.DestinationKey, toolArgString(tc.Params, "destination_key")),
	}
}

func decodeHomeAssistantArgs(tc ToolCall) homeAssistantArgs {
	req := homeAssistantArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Domain:    firstNonEmptyToolString(tc.Domain, toolArgString(tc.Params, "domain")),
		EntityID:  firstNonEmptyToolString(tc.EntityID, toolArgString(tc.Params, "entity_id")),
		Service:   firstNonEmptyToolString(tc.Service, toolArgString(tc.Params, "service")),
	}
	if tc.ServiceData != nil {
		req.ServiceData = tc.ServiceData
	} else {
		req.ServiceData = toolArgInterfaceMap(tc.Params, "service_data")
	}
	return req
}

func decodeMediaRegistryArgs(tc ToolCall) mediaRegistryArgs {
	return mediaRegistryArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Query:       firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		MediaType:   firstNonEmptyToolString(tc.MediaType, toolArgString(tc.Params, "media_type")),
		Description: firstNonEmptyToolString(tc.Description, toolArgString(tc.Params, "description")),
		Tags:        toolArgTags(tc.Params, tc.Tags, "tags"),
		TagMode:     firstNonEmptyToolString(tc.TagMode, toolArgString(tc.Params, "tag_mode")),
		ID:          toolArgInt64(tc.Params, "id"),
		Limit:       firstNonEmptyInt(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
		Offset:      firstNonEmptyInt(tc.Offset, toolArgInt(tc.Params, 0, "offset")),
		Filename:    firstNonEmptyToolString(tc.Filename, toolArgString(tc.Params, "filename")),
		FilePath:    firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
	}
}

func decodeHomepageRegistryArgs(tc ToolCall) homepageRegistryArgs {
	return homepageRegistryArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Query:       firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		Name:        firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Description: firstNonEmptyToolString(tc.Description, toolArgString(tc.Params, "description")),
		Framework:   firstNonEmptyToolString(tc.Framework, toolArgString(tc.Params, "framework")),
		ProjectDir:  firstNonEmptyToolString(tc.ProjectDir, toolArgString(tc.Params, "project_dir")),
		URL:         firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		Status:      firstNonEmptyToolString(tc.Status, toolArgString(tc.Params, "status")),
		Reason:      firstNonEmptyToolString(tc.Reason, toolArgString(tc.Params, "reason")),
		Problem:     firstNonEmptyToolString(tc.Problem, toolArgString(tc.Params, "problem")),
		Notes:       firstNonEmptyToolString(tc.Notes, toolArgString(tc.Params, "notes")),
		Tags:        toolArgTags(tc.Params, tc.Tags, "tags"),
		ID:          toolArgInt64(tc.Params, "id"),
		Limit:       firstNonEmptyInt(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
		Offset:      firstNonEmptyInt(tc.Offset, toolArgInt(tc.Params, 0, "offset")),
	}
}

func firstNonEmptyInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
