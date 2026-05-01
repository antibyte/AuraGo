package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

type githubArgs struct {
	Operation   string
	Owner       string
	Repo        string
	Description string
	Title       string
	Body        string
	Path        string
	Content     string
	Query       string
	Value       string
	ID          string
	Limit       int
	Label       string
}

type coAgentArgs struct {
	Operation    string
	Task         string
	ContextHints []string
	Priority     int
	Specialist   string
	CoAgentID    string
}

type netlifyArgs struct {
	Operation    string
	SiteID       string
	DeployID     string
	EnvKey       string
	EnvValue     string
	EnvContext   string
	FormID       string
	HookID       string
	HookType     string
	HookEvent    string
	URL          string
	Value        string
	SiteName     string
	CustomDomain string
}

type vercelArgs struct {
	Operation       string
	ProjectID       string
	ProjectName     string
	DeploymentID    string
	EnvKey          string
	EnvValue        string
	EnvTarget       string
	Domain          string
	Alias           string
	Target          string
	Framework       string
	RootDirectory   string
	OutputDirectory string
}

type proxmoxArgs struct {
	Operation    string
	Hostname     string
	Name         string
	ID           string
	VMID         string
	VMType       string
	ResourceType string
	Description  string
	UPID         string
}

type ollamaArgs struct {
	Operation   string
	Model       string
	Name        string
	Source      string
	Destination string
	Dest        string
}

type tailscaleArgs struct {
	Operation string
	Query     string
	Value     string
	Hostname  string
	ID        string
	Name      string
}

type cloudflareTunnelArgs struct {
	Operation string
	Port      int
}

type ansibleArgs struct {
	Operation string
	Hostname  string
	HostLimit string
	Query     string
	Inventory string
	Body      string
	Module    string
	Package   string
	Command   string
	Name      string
	Tags      string
	SkipTags  string
	Preview   bool
}

type trueNASArgs struct {
	Action    string
	Name      string
	FilePath  string
	Path      string
	Query     string
	Port      int
	Content   string
	Limit     int
	Recursive bool
	Force     bool
}

type firewallArgs struct {
	Operation string
	Command   string
}

type mcpCallArgs struct {
	Operation string
	Server    string
	ToolName  string
	Args      map[string]interface{}
}

type adGuardArgs struct {
	Operation string
	Query     string
	Limit     int
	Offset    int
	Services  []string
	Enabled   bool
	URL       string
	Name      string
	Rules     string
	Domain    string
	Answer    string
	Content   string
	MAC       string
	IP        string
	Hostname  string
}

type uptimeKumaArgs struct {
	Operation   string
	MonitorName string
}

type grafanaArgs struct {
	Operation    string
	Query        string
	UID          string
	DatasourceID int64
}

type sqlQueryArgs struct {
	Operation      string
	ConnectionName string
	SQLQuery       string
	TableName      string
}

type mqttArgs struct {
	Topic   string
	Payload string
	QoS     int
	Retain  bool
	Limit   int
}

type mdnsScanArgs struct {
	ServiceType       string
	Timeout           int
	AutoRegister      bool
	RegisterType      string
	RegisterTags      []string
	OverwriteExisting bool
}

type macLookupArgs struct {
	IP string
}

type ttsArgs struct {
	Text     string
	Language string
}

type chromecastArgs struct {
	Operation   string
	DeviceAddr  string
	DeviceName  string
	DevicePort  int
	URL         string
	LocalPath   string
	ContentType string
	Text        string
	Language    string
	Volume      float64
}

type obsidianArgs struct {
	Operation     string
	Path          string
	Content       string
	Query         string
	TargetType    string
	Target        string
	PatchOp       string
	Period        string
	CommandID     string
	Directory     string
	ContextLength int
}

type jellyfinArgs struct {
	Operation string
	Query     string
	MediaType string
	ItemID    string
	LibraryID string
	SessionID string
	Command   string
	Limit     int
}

func toolArgFloat64(args map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok {
			continue
		}
		switch value := raw.(type) {
		case float64:
			return value
		case float32:
			return float64(value)
		case int:
			return float64(value)
		case int32:
			return float64(value)
		case int64:
			return float64(value)
		}
	}
	return 0
}

func decodeCoAgentArgs(tc ToolCall) coAgentArgs {
	req := coAgentArgs{
		Operation:  firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Task:       firstNonEmptyToolString(tc.Task, tc.Content, toolArgString(tc.Params, "task", "content")),
		Priority:   firstNonEmptyInt(tc.Priority, toolArgInt(tc.Params, 0, "priority")),
		Specialist: firstNonEmptyToolString(tc.Specialist, toolArgString(tc.Params, "specialist")),
		CoAgentID:  firstNonEmptyToolString(tc.CoAgentID, tc.ID, toolArgString(tc.Params, "co_agent_id", "id")),
	}
	if len(tc.ContextHints) > 0 {
		req.ContextHints = append([]string(nil), tc.ContextHints...)
	} else {
		req.ContextHints = toolArgStringSlice(tc.Params, "context_hints")
	}
	return req
}

func decodeMDNSScanArgs(tc ToolCall) mdnsScanArgs {
	req := mdnsScanArgs{
		ServiceType:       firstNonEmptyToolString(tc.ServiceType, toolArgString(tc.Params, "service_type")),
		Timeout:           firstNonEmptyInt(tc.Timeout, toolArgInt(tc.Params, 0, "timeout")),
		AutoRegister:      tc.AutoRegister,
		RegisterType:      firstNonEmptyToolString(tc.RegisterType, toolArgString(tc.Params, "register_type")),
		RegisterTags:      append([]string(nil), tc.RegisterTags...),
		OverwriteExisting: tc.OverwriteExisting,
	}
	if autoRegister, ok := toolArgBool(tc.Params, "auto_register"); ok {
		req.AutoRegister = autoRegister
	}
	if len(req.RegisterTags) == 0 {
		req.RegisterTags = toolArgStringSlice(tc.Params, "register_tags")
	}
	if overwriteExisting, ok := toolArgBool(tc.Params, "overwrite_existing"); ok {
		req.OverwriteExisting = overwriteExisting
	}
	return req
}

func decodeMACLookupArgs(tc ToolCall) macLookupArgs {
	return macLookupArgs{
		IP: toolArgString(tc.Params, "ip", "ip_address"),
	}
}

func decodeTTSArgs(tc ToolCall) ttsArgs {
	return ttsArgs{
		Text:     firstNonEmptyToolString(tc.Text, tc.Content, toolArgString(tc.Params, "text", "content")),
		Language: firstNonEmptyToolString(tc.Language, toolArgString(tc.Params, "language")),
	}
}

func decodeChromecastArgs(tc ToolCall) chromecastArgs {
	return chromecastArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		DeviceAddr:  firstNonEmptyToolString(tc.DeviceAddr, toolArgString(tc.Params, "device_addr")),
		DeviceName:  firstNonEmptyToolString(tc.DeviceName, toolArgString(tc.Params, "device_name")),
		DevicePort:  firstNonEmptyInt(tc.DevicePort, toolArgInt(tc.Params, 0, "device_port")),
		URL:         firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		LocalPath:   firstNonEmptyToolString(tc.LocalPath, toolArgString(tc.Params, "local_path")),
		ContentType: firstNonEmptyToolString(tc.ContentType, toolArgString(tc.Params, "content_type")),
		Text:        firstNonEmptyToolString(tc.Text, tc.Content, toolArgString(tc.Params, "text", "content")),
		Language:    firstNonEmptyToolString(tc.Language, toolArgString(tc.Params, "language")),
		Volume:      max(tc.Volume, toolArgFloat64(tc.Params, "volume")),
	}
}

func decodeUptimeKumaArgs(tc ToolCall) uptimeKumaArgs {
	return uptimeKumaArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		MonitorName: firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "monitor_name", "name")),
	}
}

func decodeGrafanaArgs(tc ToolCall) grafanaArgs {
	return grafanaArgs{
		Operation:    firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Query:        firstNonEmptyToolString(tc.Query, tc.Content, toolArgString(tc.Params, "query", "expr")),
		UID:          firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "uid", "dashboard_uid")),
		DatasourceID: int64(firstNonEmptyInt(toolArgInt(tc.Params, 0, "datasource_id", "ds_id"))),
	}
}

func decodeObsidianArgs(tc ToolCall) obsidianArgs {
	return obsidianArgs{
		Operation:     firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Path:          firstNonEmptyToolString(tc.Path, tc.FilePath, toolArgString(tc.Params, "path")),
		Content:       firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		Query:         firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		TargetType:    firstNonEmptyToolString(toolArgString(tc.Params, "target_type")),
		Target:        firstNonEmptyToolString(tc.Target, toolArgString(tc.Params, "target")),
		PatchOp:       firstNonEmptyToolString(toolArgString(tc.Params, "patch_op")),
		Period:        firstNonEmptyToolString(toolArgString(tc.Params, "period")),
		CommandID:     firstNonEmptyToolString(tc.Command, toolArgString(tc.Params, "command_id")),
		Directory:     firstNonEmptyToolString(toolArgString(tc.Params, "directory")),
		ContextLength: firstNonEmptyInt(toolArgInt(tc.Params, 0, "context_length")),
	}
}

func (req obsidianArgs) params() map[string]string {
	params := map[string]string{}
	if req.Path != "" {
		params["path"] = req.Path
	}
	if req.Content != "" {
		params["content"] = req.Content
	}
	if req.Query != "" {
		params["query"] = req.Query
	}
	if req.TargetType != "" {
		params["target_type"] = req.TargetType
	}
	if req.Target != "" {
		params["target"] = req.Target
	}
	if req.PatchOp != "" {
		params["patch_op"] = req.PatchOp
	}
	if req.Period != "" {
		params["period"] = req.Period
	}
	if req.CommandID != "" {
		params["command_id"] = req.CommandID
	}
	if req.Directory != "" {
		params["directory"] = req.Directory
	}
	if req.ContextLength > 0 {
		params["context_length"] = fmt.Sprintf("%d", req.ContextLength)
	}
	return params
}

func decodeJellyfinArgs(tc ToolCall) jellyfinArgs {
	return jellyfinArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Query:     firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		MediaType: firstNonEmptyToolString(tc.MediaType, toolArgString(tc.Params, "media_type")),
		ItemID:    firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "item_id")),
		LibraryID: firstNonEmptyToolString(toolArgString(tc.Params, "library_id")),
		SessionID: firstNonEmptyToolString(toolArgString(tc.Params, "session_id")),
		Command:   firstNonEmptyToolString(tc.Command, toolArgString(tc.Params, "command")),
		Limit:     firstNonEmptyInt(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
	}
}

func (req jellyfinArgs) params() map[string]string {
	params := map[string]string{}
	if req.Query != "" {
		params["query"] = req.Query
	}
	if req.MediaType != "" {
		params["media_type"] = req.MediaType
	}
	if req.ItemID != "" {
		params["item_id"] = req.ItemID
	}
	if req.LibraryID != "" {
		params["library_id"] = req.LibraryID
	}
	if req.SessionID != "" {
		params["session_id"] = req.SessionID
	}
	if req.Command != "" {
		params["command"] = req.Command
	}
	if req.Limit > 0 {
		params["limit"] = fmt.Sprintf("%d", req.Limit)
	}
	return params
}

func decodeGitHubArgs(tc ToolCall) githubArgs {
	return githubArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Owner:       firstNonEmptyToolString(tc.Owner, toolArgString(tc.Params, "owner")),
		Repo:        firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Description: firstNonEmptyToolString(tc.Description, toolArgString(tc.Params, "description")),
		Title:       firstNonEmptyToolString(tc.Title, toolArgString(tc.Params, "title")),
		Body:        firstNonEmptyToolString(tc.Body, toolArgString(tc.Params, "body")),
		Path:        firstNonEmptyToolString(tc.Path, toolArgString(tc.Params, "path")),
		Content:     firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		Query:       firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		Value:       firstNonEmptyToolString(tc.Value, toolArgString(tc.Params, "value")),
		ID:          firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		Limit:       firstNonEmptyInt(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
		Label:       firstNonEmptyToolString(tc.Label, toolArgString(tc.Params, "label")),
	}
}

func (req githubArgs) issueNumber() int {
	if req.ID == "" {
		return 0
	}
	var issueNum int
	_, _ = fmt.Sscanf(req.ID, "%d", &issueNum)
	return issueNum
}

func (req githubArgs) labels() []string {
	return splitCSV(req.Label)
}

func decodeNetlifyArgs(tc ToolCall) netlifyArgs {
	return netlifyArgs{
		Operation:    firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		SiteID:       firstNonEmptyToolString(tc.SiteID, toolArgString(tc.Params, "site_id")),
		DeployID:     firstNonEmptyToolString(tc.DeployID, toolArgString(tc.Params, "deploy_id")),
		EnvKey:       firstNonEmptyToolString(tc.EnvKey, toolArgString(tc.Params, "env_key")),
		EnvValue:     firstNonEmptyToolString(tc.EnvValue, toolArgString(tc.Params, "env_value")),
		EnvContext:   firstNonEmptyToolString(tc.EnvContext, toolArgString(tc.Params, "env_context")),
		FormID:       firstNonEmptyToolString(tc.FormID, toolArgString(tc.Params, "form_id")),
		HookID:       firstNonEmptyToolString(tc.HookID, toolArgString(tc.Params, "hook_id")),
		HookType:     firstNonEmptyToolString(tc.HookType, toolArgString(tc.Params, "hook_type")),
		HookEvent:    firstNonEmptyToolString(tc.HookEvent, toolArgString(tc.Params, "hook_event")),
		URL:          firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		Value:        firstNonEmptyToolString(tc.Value, toolArgString(tc.Params, "value")),
		SiteName:     firstNonEmptyToolString(tc.SiteName, toolArgString(tc.Params, "site_name")),
		CustomDomain: firstNonEmptyToolString(tc.CustomDomain, toolArgString(tc.Params, "custom_domain")),
	}
}

func decodeVercelArgs(tc ToolCall) vercelArgs {
	return vercelArgs{
		Operation:       firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ProjectID:       firstNonEmptyToolString(toolArgString(tc.Params, "project_id"), toolArgString(tc.Params, "id")),
		ProjectName:     firstNonEmptyToolString(toolArgString(tc.Params, "project_name"), toolArgString(tc.Params, "name")),
		DeploymentID:    firstNonEmptyToolString(tc.DeployID, toolArgString(tc.Params, "deployment_id", "deploy_id")),
		EnvKey:          firstNonEmptyToolString(tc.EnvKey, toolArgString(tc.Params, "env_key")),
		EnvValue:        firstNonEmptyToolString(tc.EnvValue, toolArgString(tc.Params, "env_value")),
		EnvTarget:       firstNonEmptyToolString(toolArgString(tc.Params, "env_target"), tc.EnvContext, toolArgString(tc.Params, "env_context")),
		Domain:          firstNonEmptyToolString(tc.CustomDomain, toolArgString(tc.Params, "domain", "custom_domain")),
		Alias:           firstNonEmptyToolString(toolArgString(tc.Params, "alias")),
		Target:          firstNonEmptyToolString(toolArgString(tc.Params, "target")),
		Framework:       firstNonEmptyToolString(tc.Framework, toolArgString(tc.Params, "framework")),
		RootDirectory:   firstNonEmptyToolString(toolArgString(tc.Params, "root_directory")),
		OutputDirectory: firstNonEmptyToolString(toolArgString(tc.Params, "output_directory")),
	}
}

func (req netlifyArgs) hookData() map[string]interface{} {
	hookData := map[string]interface{}{}
	if req.URL != "" {
		hookData["url"] = req.URL
	}
	if req.Value != "" {
		hookData["email"] = req.Value
	}
	return hookData
}

func decodeProxmoxArgs(tc ToolCall) proxmoxArgs {
	return proxmoxArgs{
		Operation:    firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Hostname:     firstNonEmptyToolString(tc.Hostname, toolArgString(tc.Params, "hostname", "node")),
		Name:         firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		ID:           firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		VMID:         firstNonEmptyToolString(tc.VMID, toolArgString(tc.Params, "vmid")),
		VMType:       firstNonEmptyToolString(tc.VMType, toolArgString(tc.Params, "vm_type")),
		ResourceType: firstNonEmptyToolString(tc.ResourceType, toolArgString(tc.Params, "resource_type")),
		Description:  firstNonEmptyToolString(tc.Description, toolArgString(tc.Params, "description")),
		UPID:         firstNonEmptyToolString(tc.UPID, toolArgString(tc.Params, "upid")),
	}
}

func (req proxmoxArgs) node() string {
	if req.Hostname != "" {
		return req.Hostname
	}
	return req.Name
}

func (req proxmoxArgs) vmid() string {
	if req.VMID != "" {
		return req.VMID
	}
	return req.ID
}

func (req proxmoxArgs) upid() string {
	if req.UPID != "" {
		return req.UPID
	}
	return req.ID
}

func decodeOllamaArgs(tc ToolCall) ollamaArgs {
	return ollamaArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Model:       firstNonEmptyToolString(tc.Model, toolArgString(tc.Params, "model")),
		Name:        firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Source:      firstNonEmptyToolString(tc.Source, toolArgString(tc.Params, "source")),
		Destination: firstNonEmptyToolString(tc.Destination, toolArgString(tc.Params, "destination")),
		Dest:        firstNonEmptyToolString(tc.Dest, toolArgString(tc.Params, "dest")),
	}
}

func (req ollamaArgs) modelName() string {
	if req.Model != "" {
		return req.Model
	}
	return req.Name
}

func (req ollamaArgs) destinationName() string {
	if req.Destination != "" {
		return req.Destination
	}
	return req.Dest
}

func decodeTailscaleArgs(tc ToolCall) tailscaleArgs {
	return tailscaleArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Query:     firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		Value:     firstNonEmptyToolString(tc.Value, toolArgString(tc.Params, "value", "routes")),
		Hostname:  firstNonEmptyToolString(tc.Hostname, toolArgString(tc.Params, "hostname")),
		ID:        firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		Name:      firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
	}
}

func (req tailscaleArgs) deviceQuery() string {
	if req.Query != "" {
		return req.Query
	}
	if req.Hostname != "" {
		return req.Hostname
	}
	if req.ID != "" {
		return req.ID
	}
	return req.Name
}

func (req tailscaleArgs) routes() []string {
	return splitCSV(req.Value)
}

func decodeCloudflareTunnelArgs(tc ToolCall) cloudflareTunnelArgs {
	return cloudflareTunnelArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Port:      firstNonEmptyInt(tc.Port, toolArgInt(tc.Params, 0, "port")),
	}
}

func decodeAnsibleArgs(tc ToolCall) ansibleArgs {
	req := ansibleArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Hostname:  firstNonEmptyToolString(tc.Hostname, toolArgString(tc.Params, "hostname")),
		HostLimit: firstNonEmptyToolString(tc.HostLimit, toolArgString(tc.Params, "host_limit")),
		Query:     firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		Inventory: firstNonEmptyToolString(tc.Inventory, toolArgString(tc.Params, "inventory")),
		Body:      firstNonEmptyToolString(tc.Body, toolArgString(tc.Params, "body")),
		Module:    firstNonEmptyToolString(tc.Module, toolArgString(tc.Params, "module")),
		Package:   firstNonEmptyToolString(tc.Package, toolArgString(tc.Params, "package")),
		Command:   firstNonEmptyToolString(tc.Command, toolArgString(tc.Params, "command")),
		Name:      firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Tags:      firstNonEmptyToolString(tc.Tags, toolArgString(tc.Params, "tags")),
		SkipTags:  firstNonEmptyToolString(tc.SkipTags, toolArgString(tc.Params, "skip_tags")),
		Preview:   tc.Preview,
	}
	if preview, ok := toolArgBool(tc.Params, "preview"); ok {
		req.Preview = preview
	}
	return req
}

func (req ansibleArgs) hosts() string {
	if req.Hostname != "" {
		return req.Hostname
	}
	if req.HostLimit != "" {
		return req.HostLimit
	}
	return req.Query
}

func (req ansibleArgs) moduleName() string {
	if req.Module != "" {
		return req.Module
	}
	return req.Package
}

func (req ansibleArgs) extraVars() map[string]interface{} {
	if strings.TrimSpace(req.Body) == "" {
		return nil
	}
	var extraVars map[string]interface{}
	_ = json.Unmarshal([]byte(req.Body), &extraVars)
	return extraVars
}

func decodeTrueNASArgs(tc ToolCall) trueNASArgs {
	req := trueNASArgs{
		Action:    tc.Action,
		Name:      firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		FilePath:  firstNonEmptyToolString(tc.FilePath, toolArgString(tc.Params, "file_path")),
		Path:      firstNonEmptyToolString(tc.Path, toolArgString(tc.Params, "path")),
		Query:     firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		Port:      firstNonEmptyInt(tc.Port, toolArgInt(tc.Params, 0, "port")),
		Content:   firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		Limit:     firstNonEmptyInt(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
		Recursive: tc.Recursive,
		Force:     tc.Force,
	}
	if recursive, ok := toolArgBool(tc.Params, "recursive"); ok {
		req.Recursive = recursive
	}
	if force, ok := toolArgBool(tc.Params, "force"); ok {
		req.Force = force
	}
	return req
}

func (req trueNASArgs) params() map[string]string {
	params := map[string]string{}
	if req.Name != "" {
		params["name"] = req.Name
	}
	if p := firstNonEmpty(req.FilePath, req.Path); p != "" {
		params["path"] = p
	}
	if req.Query != "" {
		params["pool"] = req.Query
		params["dataset"] = req.Query
	}
	if req.Port != 0 {
		params["pool_id"] = fmt.Sprintf("%d", req.Port)
		params["share_id"] = fmt.Sprintf("%d", req.Port)
	}
	if req.Content != "" {
		params["compression"] = req.Content
	}
	if req.Limit > 0 {
		params["quota_gb"] = fmt.Sprintf("%d", req.Limit)
		params["retention_days"] = fmt.Sprintf("%d", req.Limit)
	}
	params["recursive"] = fmt.Sprintf("%v", req.Recursive)
	params["force"] = fmt.Sprintf("%v", req.Force)
	return params
}

func decodeFirewallArgs(tc ToolCall) firewallArgs {
	return firewallArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Command:   firstNonEmptyToolString(tc.Command, toolArgString(tc.Params, "command")),
	}
}

func decodeMCPCallArgs(tc ToolCall) mcpCallArgs {
	req := mcpCallArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Server:    toolArgString(tc.Params, "server"),
		ToolName:  toolArgString(tc.Params, "tool_name", "name"),
	}
	req.Args = toolArgInterfaceMap(tc.Params, "args", "mcp_args", "parameters")
	if req.Args == nil {
		req.Args = map[string]interface{}{}
	}
	return req
}

func decodeAdGuardArgs(tc ToolCall) adGuardArgs {
	req := adGuardArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Query:     firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		Limit:     toolArgInt(tc.Params, 0, "limit"),
		Offset:    toolArgInt(tc.Params, 0, "offset"),
		URL:       firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		Name:      firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Domain:    toolArgString(tc.Params, "domain"),
		Answer:    toolArgString(tc.Params, "answer"),
		Content:   firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		MAC:       toolArgString(tc.Params, "mac"),
		IP:        toolArgString(tc.Params, "ip"),
		Hostname:  firstNonEmptyToolString(tc.Hostname, toolArgString(tc.Params, "hostname")),
	}
	if enabled, ok := toolArgBool(tc.Params, "enabled"); ok {
		req.Enabled = enabled
	}
	req.Services = toolArgStringSlice(tc.Params, "services")
	if len(req.Services) == 0 {
		req.Services = splitCSV(toolArgString(tc.Params, "services"))
	}
	if rules := toolArgStringSlice(tc.Params, "rules"); len(rules) > 0 {
		req.Rules = strings.Join(rules, "\n")
	} else {
		req.Rules = toolArgString(tc.Params, "rules")
	}
	return req
}

func decodeSQLQueryArgs(tc ToolCall) sqlQueryArgs {
	return sqlQueryArgs{
		Operation:      firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ConnectionName: toolArgString(tc.Params, "connection_name"),
		SQLQuery:       toolArgString(tc.Params, "sql_query"),
		TableName:      toolArgString(tc.Params, "table_name"),
	}
}

func decodeMQTTArgs(tc ToolCall) mqttArgs {
	payload := toolArgString(tc.Params, "payload")
	if payload == "" {
		payload = firstNonEmptyToolString(tc.Message, toolArgString(tc.Params, "message"))
	}
	if payload == "" {
		payload = firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content"))
	}
	req := mqttArgs{
		Topic:   toolArgString(tc.Params, "topic"),
		Payload: payload,
		QoS:     toolArgInt(tc.Params, 0, "qos"),
		Limit:   toolArgInt(tc.Params, 0, "limit"),
	}
	if retain, ok := toolArgBool(tc.Params, "retain"); ok {
		req.Retain = retain
	}
	return req
}
