package agent

import (
	"encoding/json"
	"strings"
)

type webhookParameterArgs struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

type manageOutgoingWebhooksArgs struct {
	Operation    string
	ID           string
	Name         string
	Description  string
	Method       string
	URL          string
	PayloadType  string
	BodyTemplate string
	Headers      map[string]string
	Parameters   []webhookParameterArgs
}

type createSkillFromTemplateArgs struct {
	Template     string
	Name         string
	Description  string
	URL          string
	Dependencies []string
	VaultKeys    []string
}

type googleWorkspaceArgs struct {
	Operation    string
	MessageID    string
	To           string
	Subject      string
	Body         string
	AddLabels    []string
	RemoveLabels []string
	Query        string
	MaxResults   int
	EventID      string
	Title        string
	StartTime    string
	EndTime      string
	Description  string
	FileID       string
	DocumentID   string
	Range        string
	Values       [][]interface{}
}

type processAnalyzerArgs struct {
	Operation string
	Name      string
	PID       int
	Limit     int
}

type webCaptureArgs struct {
	Operation string
	URL       string
	Selector  string
	FullPage  bool
	OutputDir string
}

type webPerformanceAuditArgs struct {
	URL      string
	Viewport string
}

type networkPingArgs struct {
	Host    string
	Count   int
	Timeout int
}

type detectFileTypeArgs struct {
	FilePath  string
	Recursive bool
}

type dnsLookupArgs struct {
	Host       string
	RecordType string
}

type portScannerArgs struct {
	Host      string
	PortRange string
	TimeoutMs int
}

type siteCrawlerArgs struct {
	URL            string
	MaxDepth       int
	MaxPages       int
	AllowedDomains string
	Selector       string
}

type whoisLookupArgs struct {
	Host       string
	URL        string
	IncludeRaw bool
}

type siteMonitorArgs struct {
	Operation string
	MonitorID string
	URL       string
	Selector  string
	Interval  string
	Limit     int
}

type formAutomationArgs struct {
	Operation     string
	URL           string
	Fields        string
	Selector      string
	ScreenshotDir string
}

type upnpScanArgs struct {
	SearchTarget      string
	TimeoutSecs       int
	AutoRegister      bool
	RegisterType      string
	RegisterTags      []string
	OverwriteExisting bool
}

type manageProcessesArgs struct {
	Operation string
	PID       int
}

type registerDeviceArgs struct {
	Hostname       string
	DeviceType     string
	IPAddress      string
	Port           int
	Username       string
	Password       string
	PrivateKeyPath string
	Description    string
	Tags           string
	MACAddress     string
}

type wakeOnLANArgs struct {
	ServerID   string
	MACAddress string
	IPAddress  string
}

type pinMessageArgs struct {
	ID     string
	Pinned bool
}

type discordMessageArgs struct {
	ChannelID string
	Message   string
	Limit     int
}

type missionArgs struct {
	Operation      string
	ID             string
	Title          string
	Command        string
	CronExpr       string
	Priority       int
	Locked         bool
	LockedProvided bool
}

type notificationArgs struct {
	Channel  string
	Title    string
	Message  string
	Priority string
}

type emailFetchArgs struct {
	Account string
	Folder  string
	Limit   int
}

type emailSendArgs struct {
	Account string
	To      string
	Subject string
	Body    string
}

func toolArgStringSlice(args map[string]interface{}, keys ...string) []string {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok {
			continue
		}
		switch values := raw.(type) {
		case []string:
			return append([]string(nil), values...)
		case []interface{}:
			result := make([]string, 0, len(values))
			for _, value := range values {
				if s, ok := value.(string); ok && s != "" {
					result = append(result, s)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return nil
}

func toolArgStringMap(args map[string]interface{}, keys ...string) map[string]string {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok {
			continue
		}
		switch values := raw.(type) {
		case map[string]string:
			result := make(map[string]string, len(values))
			for k, v := range values {
				result[k] = v
			}
			return result
		case map[string]interface{}:
			result := make(map[string]string, len(values))
			for k, v := range values {
				if s, ok := v.(string); ok {
					result[k] = s
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return nil
}

func toolArgMatrix(args map[string]interface{}, keys ...string) [][]interface{} {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok {
			continue
		}
		switch values := raw.(type) {
		case [][]interface{}:
			result := make([][]interface{}, len(values))
			copy(result, values)
			return result
		case []interface{}:
			var result [][]interface{}
			for _, row := range values {
				if cells, ok := row.([]interface{}); ok {
					result = append(result, cells)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return nil
}

func toolArgWebhookParameters(args map[string]interface{}, keys ...string) []webhookParameterArgs {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok {
			continue
		}
		values, ok := raw.([]interface{})
		if !ok {
			continue
		}
		result := make([]webhookParameterArgs, 0, len(values))
		for _, value := range values {
			entry, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			param := webhookParameterArgs{
				Name:        toolArgString(entry, "name"),
				Type:        toolArgString(entry, "type"),
				Description: toolArgString(entry, "description"),
			}
			if required, ok := entry["required"].(bool); ok {
				param.Required = required
			}
			result = append(result, param)
		}
		if len(result) > 0 {
			return result
		}
	}
	return nil
}

func toolArgCSV(args map[string]interface{}, keys ...string) string {
	if value := toolArgString(args, keys...); value != "" {
		return value
	}
	values := toolArgStringSlice(args, keys...)
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ",")
}

func toolArgJSONText(args map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok || raw == nil {
			continue
		}
		if text, ok := raw.(string); ok {
			if text != "" {
				return text
			}
			continue
		}
		encoded, err := json.Marshal(raw)
		if err == nil && len(encoded) > 0 && string(encoded) != "null" {
			return string(encoded)
		}
	}
	return ""
}

func decodeManageOutgoingWebhooksArgs(tc ToolCall) manageOutgoingWebhooksArgs {
	req := manageOutgoingWebhooksArgs{
		Operation:    firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ID:           firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		Name:         firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Description:  firstNonEmptyToolString(tc.Description, toolArgString(tc.Params, "description")),
		Method:       firstNonEmptyToolString(tc.Method, toolArgString(tc.Params, "method")),
		URL:          firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		PayloadType:  firstNonEmptyToolString(tc.PayloadType, toolArgString(tc.Params, "payload_type")),
		BodyTemplate: firstNonEmptyToolString(tc.BodyTemplate, toolArgString(tc.Params, "body_template")),
	}
	if len(tc.Headers) > 0 {
		req.Headers = make(map[string]string, len(tc.Headers))
		for key, value := range tc.Headers {
			req.Headers[key] = value
		}
	} else {
		req.Headers = toolArgStringMap(tc.Params, "headers")
	}
	if req.Headers == nil {
		req.Headers = map[string]string{}
	}
	req.Parameters = toolArgWebhookParameters(tc.Params, "parameters")
	if len(req.Parameters) == 0 {
		switch raw := tc.Parameters.(type) {
		case []interface{}:
			req.Parameters = toolArgWebhookParameters(map[string]interface{}{"parameters": raw}, "parameters")
		}
	}
	return req
}

func (req manageOutgoingWebhooksArgs) rawParameters() []interface{} {
	if len(req.Parameters) == 0 {
		return nil
	}
	raw := make([]interface{}, 0, len(req.Parameters))
	for _, parameter := range req.Parameters {
		raw = append(raw, map[string]interface{}{
			"name":        parameter.Name,
			"type":        parameter.Type,
			"description": parameter.Description,
			"required":    parameter.Required,
		})
	}
	return raw
}

func decodeCreateSkillFromTemplateArgs(tc ToolCall) createSkillFromTemplateArgs {
	req := createSkillFromTemplateArgs{
		Template:    firstNonEmptyToolString(tc.Template, toolArgString(tc.Params, "template")),
		Name:        firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Description: firstNonEmptyToolString(tc.Description, toolArgString(tc.Params, "description")),
		URL:         firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
	}
	if len(tc.VaultKeys) > 0 {
		req.VaultKeys = append([]string(nil), tc.VaultKeys...)
	} else {
		req.VaultKeys = toolArgStringSlice(tc.Params, "vault_keys")
	}
	req.Dependencies = toolArgStringSlice(tc.Params, "dependencies")
	return req
}

func decodeProcessAnalyzerArgs(tc ToolCall) processAnalyzerArgs {
	return processAnalyzerArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Name:      firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		PID:       max(tc.PID, toolArgInt(tc.Params, 0, "pid")),
		Limit:     max(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
	}
}

func decodeWebCaptureArgs(tc ToolCall) webCaptureArgs {
	req := webCaptureArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		URL:       firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		Selector:  firstNonEmptyToolString(tc.Selector, toolArgString(tc.Params, "selector")),
		OutputDir: firstNonEmptyToolString(tc.OutputDir, toolArgString(tc.Params, "output_dir")),
		FullPage:  tc.FullPage,
	}
	if fullPage, ok := toolArgBool(tc.Params, "full_page"); ok {
		req.FullPage = fullPage
	}
	return req
}

func decodeWebPerformanceAuditArgs(tc ToolCall) webPerformanceAuditArgs {
	return webPerformanceAuditArgs{
		URL:      firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		Viewport: firstNonEmptyToolString(tc.Viewport, toolArgString(tc.Params, "viewport")),
	}
}

func decodeNetworkPingArgs(tc ToolCall) networkPingArgs {
	return networkPingArgs{
		Host:    firstNonEmptyToolString(tc.Host, toolArgString(tc.Params, "host", "target")),
		Count:   max(tc.Count, toolArgInt(tc.Params, 0, "count")),
		Timeout: max(tc.Timeout, toolArgInt(tc.Params, 0, "timeout")),
	}
}

func decodeDetectFileTypeArgs(tc ToolCall) detectFileTypeArgs {
	req := detectFileTypeArgs{
		FilePath:  firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		Recursive: tc.Recursive,
	}
	if recursive, ok := toolArgBool(tc.Params, "recursive"); ok {
		req.Recursive = recursive
	}
	return req
}

func decodeDNSLookupArgs(tc ToolCall) dnsLookupArgs {
	return dnsLookupArgs{
		Host:       firstNonEmptyToolString(tc.Host, toolArgString(tc.Params, "host", "domain")),
		RecordType: firstNonEmptyToolString(tc.RecordType, toolArgString(tc.Params, "record_type")),
	}
}

func decodePortScannerArgs(tc ToolCall) portScannerArgs {
	return portScannerArgs{
		Host:      firstNonEmptyToolString(tc.Host, toolArgString(tc.Params, "host", "target")),
		PortRange: firstNonEmptyToolString(tc.PortRange, toolArgString(tc.Params, "port_range")),
		TimeoutMs: max(tc.TimeoutMs, toolArgInt(tc.Params, 0, "timeout_ms")),
	}
}

func decodeSiteCrawlerArgs(tc ToolCall) siteCrawlerArgs {
	return siteCrawlerArgs{
		URL:            firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		MaxDepth:       max(tc.MaxDepth, toolArgInt(tc.Params, 0, "max_depth")),
		MaxPages:       max(tc.MaxPages, toolArgInt(tc.Params, 0, "max_pages")),
		AllowedDomains: firstNonEmptyToolString(tc.AllowedDomains, toolArgCSV(tc.Params, "allowed_domains")),
		Selector:       firstNonEmptyToolString(tc.Selector, toolArgString(tc.Params, "selector")),
	}
}

func decodeWhoisLookupArgs(tc ToolCall) whoisLookupArgs {
	req := whoisLookupArgs{
		Host:       firstNonEmptyToolString(tc.Host, toolArgString(tc.Params, "host", "domain")),
		URL:        firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		IncludeRaw: tc.IncludeRaw,
	}
	if includeRaw, ok := toolArgBool(tc.Params, "include_raw"); ok {
		req.IncludeRaw = includeRaw
	}
	return req
}

func (req whoisLookupArgs) domain() string {
	return firstNonEmptyToolString(req.Host, req.URL)
}

func decodeSiteMonitorArgs(tc ToolCall) siteMonitorArgs {
	return siteMonitorArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		MonitorID: firstNonEmptyToolString(tc.MonitorID, toolArgString(tc.Params, "monitor_id")),
		URL:       firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		Selector:  firstNonEmptyToolString(tc.Selector, toolArgString(tc.Params, "selector")),
		Interval:  firstNonEmptyToolString(tc.Interval, toolArgString(tc.Params, "interval")),
		Limit:     max(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
	}
}

func decodeFormAutomationArgs(tc ToolCall) formAutomationArgs {
	return formAutomationArgs{
		Operation:     firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		URL:           firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		Fields:        firstNonEmptyToolString(tc.Fields, toolArgJSONText(tc.Params, "fields")),
		Selector:      firstNonEmptyToolString(tc.Selector, toolArgString(tc.Params, "selector")),
		ScreenshotDir: firstNonEmptyToolString(tc.ScreenshotDir, toolArgString(tc.Params, "screenshot_dir")),
	}
}

func decodeUPnPScanArgs(tc ToolCall) upnpScanArgs {
	req := upnpScanArgs{
		SearchTarget:      firstNonEmptyToolString(tc.SearchTarget, toolArgString(tc.Params, "search_target")),
		TimeoutSecs:       max(tc.TimeoutSecs, toolArgInt(tc.Params, 0, "timeout_secs")),
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

func decodeManageProcessesArgs(tc ToolCall) manageProcessesArgs {
	return manageProcessesArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		PID:       max(tc.PID, toolArgInt(tc.Params, 0, "pid")),
	}
}

func decodeRegisterDeviceArgs(tc ToolCall) registerDeviceArgs {
	return registerDeviceArgs{
		Hostname:       firstNonEmptyToolString(tc.Hostname, toolArgString(tc.Params, "hostname", "host", "server_id")),
		DeviceType:     firstNonEmptyToolString(tc.DeviceType, toolArgString(tc.Params, "device_type")),
		IPAddress:      firstNonEmptyToolString(tc.IPAddress, toolArgString(tc.Params, "ip_address", "ip")),
		Port:           max(tc.Port, toolArgInt(tc.Params, 0, "port")),
		Username:       firstNonEmptyToolString(tc.Username, toolArgString(tc.Params, "username", "user")),
		Password:       firstNonEmptyToolString(tc.Password, toolArgString(tc.Params, "password", "pass")),
		PrivateKeyPath: firstNonEmptyToolString(tc.PrivateKeyPath, toolArgString(tc.Params, "private_key_path", "key_path")),
		Description:    firstNonEmptyToolString(tc.Description, toolArgString(tc.Params, "description")),
		Tags:           firstNonEmptyToolString(tc.Tags, toolArgCSV(tc.Params, "tags")),
		MACAddress:     firstNonEmptyToolString(tc.MACAddress, toolArgString(tc.Params, "mac_address")),
	}
}

func decodeWakeOnLANArgs(tc ToolCall) wakeOnLANArgs {
	return wakeOnLANArgs{
		ServerID:   firstNonEmptyToolString(tc.ServerID, toolArgString(tc.Params, "server_id")),
		MACAddress: firstNonEmptyToolString(tc.MACAddress, toolArgString(tc.Params, "mac_address")),
		IPAddress:  firstNonEmptyToolString(tc.IPAddress, toolArgString(tc.Params, "ip_address", "ip")),
	}
}

func decodePinMessageArgs(tc ToolCall) pinMessageArgs {
	req := pinMessageArgs{
		ID:     firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		Pinned: tc.Pinned,
	}
	if pinned, ok := toolArgBool(tc.Params, "pinned"); ok {
		req.Pinned = pinned
	}
	return req
}

func decodeDiscordMessageArgs(tc ToolCall) discordMessageArgs {
	return discordMessageArgs{
		ChannelID: firstNonEmptyToolString(tc.ChannelID, toolArgString(tc.Params, "channel_id")),
		Message:   firstNonEmptyToolString(tc.Message, tc.Content, tc.Body, toolArgString(tc.Params, "message", "content", "body")),
		Limit:     max(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
	}
}

func decodeMissionArgs(tc ToolCall) missionArgs {
	req := missionArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ID:        firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		Title:     firstNonEmptyToolString(tc.Title, toolArgString(tc.Params, "title", "name")),
		Command:   firstNonEmptyToolString(tc.Command, toolArgString(tc.Params, "command", "prompt")),
		CronExpr:  firstNonEmptyToolString(tc.CronExpr, toolArgString(tc.Params, "cron_expr")),
		Priority:  max(tc.Priority, toolArgInt(tc.Params, 0, "priority")),
		Locked:    tc.Locked,
	}
	if locked, ok := toolArgBool(tc.Params, "locked"); ok {
		req.Locked = locked
		req.LockedProvided = true
	} else if strings.Contains(tc.RawJSON, `"locked"`) {
		req.LockedProvided = true
	}
	return req
}

func decodeNotificationArgs(tc ToolCall) notificationArgs {
	req := notificationArgs{
		Channel:  firstNonEmptyToolString(tc.Channel, toolArgString(tc.Params, "channel")),
		Title:    firstNonEmptyToolString(tc.Title, toolArgString(tc.Params, "title")),
		Message:  firstNonEmptyToolString(tc.Message, tc.Content, tc.Body, toolArgString(tc.Params, "message", "content", "body")),
		Priority: firstNonEmptyToolString(tc.Tag, toolArgString(tc.Params, "tag", "priority")),
	}
	switch firstNonEmptyToolString(tc.Action, tc.ToolName) {
	case "send_push_notification", "web_push":
		req.Channel = "push"
	}
	return req
}

func decodeEmailFetchArgs(tc ToolCall) emailFetchArgs {
	return emailFetchArgs{
		Account: firstNonEmptyToolString(tc.Account, toolArgString(tc.Params, "account")),
		Folder:  firstNonEmptyToolString(tc.Folder, toolArgString(tc.Params, "folder")),
		Limit:   max(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
	}
}

func decodeEmailSendArgs(tc ToolCall) emailSendArgs {
	return emailSendArgs{
		Account: firstNonEmptyToolString(tc.Account, toolArgString(tc.Params, "account")),
		To:      firstNonEmptyToolString(tc.To, toolArgString(tc.Params, "to")),
		Subject: firstNonEmptyToolString(tc.Subject, toolArgString(tc.Params, "subject")),
		Body:    firstNonEmptyToolString(tc.Body, tc.Content, toolArgString(tc.Params, "body", "content")),
	}
}

func decodeGoogleWorkspaceArgsFromMap(args map[string]interface{}) googleWorkspaceArgs {
	return googleWorkspaceArgs{
		Operation:    toolArgString(args, "operation"),
		MessageID:    toolArgString(args, "message_id"),
		To:           toolArgString(args, "to"),
		Subject:      toolArgString(args, "subject"),
		Body:         toolArgString(args, "body"),
		AddLabels:    toolArgStringSlice(args, "add_labels"),
		RemoveLabels: toolArgStringSlice(args, "remove_labels"),
		Query:        toolArgString(args, "query"),
		MaxResults:   toolArgInt(args, 0, "max_results"),
		EventID:      toolArgString(args, "event_id"),
		Title:        toolArgString(args, "title"),
		StartTime:    toolArgString(args, "start_time"),
		EndTime:      toolArgString(args, "end_time"),
		Description:  toolArgString(args, "description"),
		FileID:       toolArgString(args, "file_id"),
		DocumentID:   toolArgString(args, "document_id"),
		Range:        toolArgString(args, "range"),
		Values:       toolArgMatrix(args, "values"),
	}
}

func decodeGoogleWorkspaceArgs(tc ToolCall) googleWorkspaceArgs {
	req := decodeGoogleWorkspaceArgsFromMap(tc.Params)
	req.Operation = firstNonEmptyToolString(tc.Operation, req.Operation)
	req.MessageID = firstNonEmptyToolString(tc.MessageID, req.MessageID)
	req.To = firstNonEmptyToolString(tc.To, req.To)
	req.Subject = firstNonEmptyToolString(tc.Subject, req.Subject)
	req.Body = firstNonEmptyToolString(tc.Body, req.Body)
	if len(tc.AddLabels) > 0 {
		req.AddLabels = append([]string(nil), tc.AddLabels...)
	}
	if len(tc.RemoveLabels) > 0 {
		req.RemoveLabels = append([]string(nil), tc.RemoveLabels...)
	}
	req.Query = firstNonEmptyToolString(tc.Query, req.Query)
	if tc.MaxResults > 0 {
		req.MaxResults = tc.MaxResults
	}
	req.EventID = firstNonEmptyToolString(tc.EventID, req.EventID)
	req.Title = firstNonEmptyToolString(tc.Title, req.Title)
	req.StartTime = firstNonEmptyToolString(tc.StartTime, req.StartTime)
	req.EndTime = firstNonEmptyToolString(tc.EndTime, req.EndTime)
	req.Description = firstNonEmptyToolString(tc.Description, req.Description)
	req.FileID = firstNonEmptyToolString(tc.FileID, req.FileID)
	req.DocumentID = firstNonEmptyToolString(tc.DocumentID, req.DocumentID)
	req.Range = firstNonEmptyToolString(tc.Range, req.Range)
	if len(tc.Values) > 0 {
		req.Values = append([][]interface{}(nil), tc.Values...)
	}
	return req
}

func (req googleWorkspaceArgs) params() map[string]interface{} {
	params := make(map[string]interface{})
	if req.MessageID != "" {
		params["message_id"] = req.MessageID
	}
	if req.To != "" {
		params["to"] = req.To
	}
	if req.Subject != "" {
		params["subject"] = req.Subject
	}
	if req.Body != "" {
		params["body"] = req.Body
	}
	if len(req.AddLabels) > 0 {
		params["add_labels"] = req.AddLabels
	}
	if len(req.RemoveLabels) > 0 {
		params["remove_labels"] = req.RemoveLabels
	}
	if req.Query != "" {
		params["query"] = req.Query
	}
	if req.MaxResults > 0 {
		params["max_results"] = req.MaxResults
	}
	if req.EventID != "" {
		params["event_id"] = req.EventID
	}
	if req.Title != "" {
		params["title"] = req.Title
	}
	if req.StartTime != "" {
		params["start_time"] = req.StartTime
	}
	if req.EndTime != "" {
		params["end_time"] = req.EndTime
	}
	if req.Description != "" {
		params["description"] = req.Description
	}
	if req.FileID != "" {
		params["file_id"] = req.FileID
	}
	if req.DocumentID != "" {
		params["document_id"] = req.DocumentID
	}
	if req.Range != "" {
		params["range"] = req.Range
	}
	if len(req.Values) > 0 {
		params["values"] = req.Values
	}
	return params
}

func firstNonEmptyToolString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
