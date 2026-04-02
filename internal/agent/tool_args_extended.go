package agent

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
