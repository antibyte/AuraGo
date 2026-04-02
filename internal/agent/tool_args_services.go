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

type dockerArgs struct {
	Operation   string
	ContainerID string
	Name        string
	Image       string
	Env         []string
	Ports       map[string]string
	Volumes     []string
	Command     string
	Restart     string
	All         bool
	Force       bool
	Tail        int
	User        string
	Source      string
	Destination string
	Direction   string
	Driver      string
	Network     string
	File        string
}

type homepageArgs struct {
	Operation    string
	Command      string
	Framework    string
	Name         string
	Template     string
	ProjectDir   string
	AutoFix      bool
	Packages     []string
	URL          string
	Viewport     string
	Path         string
	Content      string
	SubOperation string
	Old          string
	New          string
	Marker       string
	StartLine    int
	EndLine      int
	JsonPath     string
	SetValue     interface{}
	Xpath        string
	Port         int
	BuildDir     string
	SiteID       string
	Title        string
	Draft        bool
	GitMessage   string
	Message      string
	Count        int
}

type manageSQLConnectionsArgs struct {
	Operation      string
	ConnectionName string
	Driver         string
	Host           string
	Port           int
	DatabaseName   string
	Description    string
	SSLMode        string
	AllowRead      *bool
	AllowWrite     *bool
	AllowChange    *bool
	AllowDelete    *bool
	Username       string
	Password       string
	DockerTemplate string
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

func decodeDockerArgs(tc ToolCall) dockerArgs {
	req := dockerArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ContainerID: firstNonEmptyToolString(tc.ContainerID, toolArgString(tc.Params, "container_id")),
		Name:        firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Image:       firstNonEmptyToolString(tc.Image, toolArgString(tc.Params, "image")),
		Command:     firstNonEmptyToolString(tc.Command, toolArgString(tc.Params, "command")),
		Restart:     firstNonEmptyToolString(tc.Restart, toolArgString(tc.Params, "restart")),
		Tail:        firstNonEmptyInt(tc.Tail, toolArgInt(tc.Params, 0, "tail")),
		User:        firstNonEmptyToolString(tc.User, toolArgString(tc.Params, "user")),
		Source:      firstNonEmptyToolString(tc.Source, toolArgString(tc.Params, "source")),
		Destination: firstNonEmptyToolString(tc.Destination, toolArgString(tc.Params, "destination")),
		Direction:   firstNonEmptyToolString(tc.Direction, toolArgString(tc.Params, "direction")),
		Driver:      firstNonEmptyToolString(tc.Driver, toolArgString(tc.Params, "driver")),
		Network:     firstNonEmptyToolString(tc.Network, toolArgString(tc.Params, "network")),
		File:        firstNonEmptyToolString(tc.File, toolArgString(tc.Params, "file")),
		All:         tc.All,
		Force:       tc.Force,
	}
	if all, ok := toolArgBool(tc.Params, "all"); ok {
		req.All = all
	}
	if force, ok := toolArgBool(tc.Params, "force"); ok {
		req.Force = force
	}
	if len(tc.Env) > 0 {
		req.Env = append([]string(nil), tc.Env...)
	} else {
		req.Env = toolArgStringSlice(tc.Params, "env")
	}
	if len(tc.Ports) > 0 {
		req.Ports = make(map[string]string, len(tc.Ports))
		for key, value := range tc.Ports {
			req.Ports[key] = value
		}
	} else {
		req.Ports = toolArgStringMap(tc.Params, "ports")
	}
	if len(tc.Volumes) > 0 {
		req.Volumes = append([]string(nil), tc.Volumes...)
	} else {
		req.Volumes = toolArgStringSlice(tc.Params, "volumes")
	}
	return req
}

func (req dockerArgs) targetContainerID() string {
	if req.ContainerID != "" {
		return req.ContainerID
	}
	return req.Name
}

func decodeHomepageArgs(tc ToolCall) homepageArgs {
	req := homepageArgs{
		Operation:    firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Command:      firstNonEmptyToolString(tc.Command, toolArgString(tc.Params, "command")),
		Framework:    firstNonEmptyToolString(tc.Framework, toolArgString(tc.Params, "framework")),
		Name:         firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Template:     firstNonEmptyToolString(tc.Template, toolArgString(tc.Params, "template")),
		ProjectDir:   firstNonEmptyToolString(tc.ProjectDir, toolArgString(tc.Params, "project_dir")),
		AutoFix:      tc.AutoFix,
		URL:          firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		Viewport:     firstNonEmptyToolString(tc.Viewport, toolArgString(tc.Params, "viewport")),
		Path:         firstNonEmptyToolString(tc.Path, toolArgString(tc.Params, "path")),
		Content:      firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		SubOperation: firstNonEmptyToolString(tc.SubOperation, toolArgString(tc.Params, "sub_operation")),
		Old:          firstNonEmptyToolString(tc.Old, toolArgString(tc.Params, "old")),
		New:          firstNonEmptyToolString(tc.New, toolArgString(tc.Params, "new")),
		Marker:       firstNonEmptyToolString(tc.Marker, toolArgString(tc.Params, "marker")),
		StartLine:    firstNonEmptyInt(tc.StartLine, toolArgInt(tc.Params, 0, "start_line")),
		EndLine:      firstNonEmptyInt(tc.EndLine, toolArgInt(tc.Params, 0, "end_line")),
		JsonPath:     firstNonEmptyToolString(tc.JsonPath, toolArgString(tc.Params, "json_path")),
		Xpath:        firstNonEmptyToolString(tc.Xpath, toolArgString(tc.Params, "xpath")),
		Port:         firstNonEmptyInt(tc.Port, toolArgInt(tc.Params, 0, "port")),
		BuildDir:     firstNonEmptyToolString(tc.BuildDir, toolArgString(tc.Params, "build_dir")),
		SiteID:       firstNonEmptyToolString(tc.SiteID, toolArgString(tc.Params, "site_id")),
		Title:        firstNonEmptyToolString(tc.Title, toolArgString(tc.Params, "title")),
		Draft:        tc.Draft,
		GitMessage:   firstNonEmptyToolString(tc.GitMessage, toolArgString(tc.Params, "git_message")),
		Message:      firstNonEmptyToolString(tc.Message, toolArgString(tc.Params, "message")),
		Count:        firstNonEmptyInt(tc.Count, toolArgInt(tc.Params, 0, "count")),
	}
	if autoFix, ok := toolArgBool(tc.Params, "auto_fix"); ok {
		req.AutoFix = autoFix
	}
	if draft, ok := toolArgBool(tc.Params, "draft"); ok {
		req.Draft = draft
	}
	if len(tc.Packages) > 0 {
		req.Packages = append([]string(nil), tc.Packages...)
	} else {
		req.Packages = toolArgStringSlice(tc.Params, "packages")
	}
	if tc.SetValue != nil {
		req.SetValue = tc.SetValue
	} else if tc.Params != nil {
		req.SetValue = tc.Params["set_value"]
	}
	return req
}

func toolArgOptionalBool(args map[string]interface{}, keys ...string) *bool {
	if value, ok := toolArgBool(args, keys...); ok {
		result := value
		return &result
	}
	return nil
}

func decodeManageSQLConnectionsArgs(tc ToolCall) manageSQLConnectionsArgs {
	req := manageSQLConnectionsArgs{
		Operation:      firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ConnectionName: firstNonEmptyToolString(tc.ConnectionName, toolArgString(tc.Params, "connection_name")),
		Driver:         firstNonEmptyToolString(tc.Driver, toolArgString(tc.Params, "driver")),
		Host:           firstNonEmptyToolString(tc.Host, toolArgString(tc.Params, "host")),
		Port:           firstNonEmptyInt(tc.Port, toolArgInt(tc.Params, 0, "port")),
		DatabaseName:   firstNonEmptyToolString(tc.DatabaseName, toolArgString(tc.Params, "database_name")),
		Description:    firstNonEmptyToolString(tc.Description, toolArgString(tc.Params, "description")),
		SSLMode:        firstNonEmptyToolString(tc.SSLMode, toolArgString(tc.Params, "ssl_mode")),
		Username:       firstNonEmptyToolString(tc.Username, toolArgString(tc.Params, "username", "user")),
		Password:       firstNonEmptyToolString(tc.Password, toolArgString(tc.Params, "password", "pass")),
		DockerTemplate: firstNonEmptyToolString(tc.DockerTemplate, toolArgString(tc.Params, "docker_template")),
		AllowRead:      tc.AllowRead,
		AllowWrite:     tc.AllowWrite,
		AllowChange:    tc.AllowChange,
		AllowDelete:    tc.AllowDelete,
	}
	if req.AllowRead == nil {
		req.AllowRead = toolArgOptionalBool(tc.Params, "allow_read")
	}
	if req.AllowWrite == nil {
		req.AllowWrite = toolArgOptionalBool(tc.Params, "allow_write")
	}
	if req.AllowChange == nil {
		req.AllowChange = toolArgOptionalBool(tc.Params, "allow_change")
	}
	if req.AllowDelete == nil {
		req.AllowDelete = toolArgOptionalBool(tc.Params, "allow_delete")
	}
	return req
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
