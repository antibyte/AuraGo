package agent

import (
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

func decodeMCPCallArgs(tc ToolCall) mcpCallArgs {
	req := mcpCallArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Server:    firstNonEmptyToolString(tc.Server, toolArgString(tc.Params, "server")),
		ToolName:  firstNonEmptyToolString(tc.ToolName, toolArgString(tc.Params, "tool_name", "name")),
	}
	if tc.MCPArgs != nil {
		req.Args = tc.MCPArgs
	} else {
		req.Args = toolArgInterfaceMap(tc.Params, "args", "mcp_args", "parameters")
	}
	if req.Args == nil {
		req.Args = map[string]interface{}{}
	}
	return req
}

func decodeAdGuardArgs(tc ToolCall) adGuardArgs {
	req := adGuardArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Query:     firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		Limit:     firstNonEmptyInt(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
		Offset:    firstNonEmptyInt(tc.Offset, toolArgInt(tc.Params, 0, "offset")),
		Enabled:   tc.Enabled,
		URL:       firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		Name:      firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Domain:    firstNonEmptyToolString(tc.Domain, toolArgString(tc.Params, "domain")),
		Answer:    firstNonEmptyToolString(tc.Answer, toolArgString(tc.Params, "answer")),
		Content:   firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		MAC:       firstNonEmptyToolString(tc.MAC, toolArgString(tc.Params, "mac")),
		IP:        firstNonEmptyToolString(tc.IP, toolArgString(tc.Params, "ip")),
		Hostname:  firstNonEmptyToolString(tc.Hostname, toolArgString(tc.Params, "hostname")),
	}
	if enabled, ok := toolArgBool(tc.Params, "enabled"); ok {
		req.Enabled = enabled
	}
	if len(tc.Services) > 0 {
		req.Services = append([]string(nil), tc.Services...)
	} else {
		req.Services = toolArgStringSlice(tc.Params, "services")
		if len(req.Services) == 0 {
			req.Services = splitCSV(toolArgString(tc.Params, "services"))
		}
	}
	req.Rules = tc.Rules
	if req.Rules == "" {
		if rules := toolArgStringSlice(tc.Params, "rules"); len(rules) > 0 {
			req.Rules = strings.Join(rules, "\n")
		} else {
			req.Rules = toolArgString(tc.Params, "rules")
		}
	}
	return req
}

func decodeSQLQueryArgs(tc ToolCall) sqlQueryArgs {
	return sqlQueryArgs{
		Operation:      firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ConnectionName: firstNonEmptyToolString(tc.ConnectionName, toolArgString(tc.Params, "connection_name")),
		SQLQuery:       firstNonEmptyToolString(tc.SQLQuery, toolArgString(tc.Params, "sql_query")),
		TableName:      firstNonEmptyToolString(tc.TableName, toolArgString(tc.Params, "table_name")),
	}
}

func decodeMQTTArgs(tc ToolCall) mqttArgs {
	payload := firstNonEmptyToolString(tc.Payload, toolArgString(tc.Params, "payload"))
	if payload == "" {
		payload = firstNonEmptyToolString(tc.Message, toolArgString(tc.Params, "message"))
	}
	if payload == "" {
		payload = firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content"))
	}
	req := mqttArgs{
		Topic:   firstNonEmptyToolString(tc.Topic, toolArgString(tc.Params, "topic")),
		Payload: payload,
		QoS:     firstNonEmptyInt(tc.QoS, toolArgInt(tc.Params, 0, "qos")),
		Retain:  tc.Retain,
		Limit:   firstNonEmptyInt(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
	}
	if retain, ok := toolArgBool(tc.Params, "retain"); ok {
		req.Retain = retain
	}
	return req
}
