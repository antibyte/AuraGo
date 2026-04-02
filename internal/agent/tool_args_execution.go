package agent

type callWebhookArgs struct {
	WebhookName string
	Parameters  map[string]interface{}
}

type saveToolArgs struct {
	Name        string
	Description string
	Code        string
}

type runToolArgs struct {
	Name          string
	Args          []string
	Background    bool
	VaultKeys     []string
	CredentialIDs []string
}

type sandboxExecutionArgs struct {
	Code          string
	Language      string
	Libraries     []string
	VaultKeys     []string
	CredentialIDs []string
}

type pythonExecutionArgs struct {
	Code          string
	Background    bool
	VaultKeys     []string
	CredentialIDs []string
}

type shellExecutionArgs struct {
	Command    string
	Background bool
}

type sudoExecutionArgs struct {
	Command string
}

type installPackageArgs struct {
	Package string
}

type processControlArgs struct {
	PID int
}

type updateManagementArgs struct {
	Operation string
}

type knowledgeGraphArgs struct {
	Operation   string
	ID          string
	Label       string
	Properties  map[string]string
	Source      string
	Target      string
	Relation    string
	NewRelation string
	Limit       int
	Depth       int
	Content     string
}

type coreMemoryArgs struct {
	Operation   string
	Fact        string
	Key         string
	Value       string
	ID          string
	MemoryKey   string
	MemoryValue string
	Content     string
}

type cheatsheetArgs struct {
	Operation    string
	ID           string
	Name         string
	Content      string
	Active       *bool
	Filename     string
	Source       string
	AttachmentID string
}

func toolArgInterfaceMap(args map[string]interface{}, keys ...string) map[string]interface{} {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok {
			continue
		}
		switch values := raw.(type) {
		case map[string]interface{}:
			result := make(map[string]interface{}, len(values))
			for k, v := range values {
				result[k] = v
			}
			return result
		}
	}
	return nil
}

func toolArgBool(args map[string]interface{}, keys ...string) (bool, bool) {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok {
			continue
		}
		if value, ok := raw.(bool); ok {
			return value, true
		}
	}
	return false, false
}

func toolArgBoolPtr(args map[string]interface{}, keys ...string) *bool {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok {
			continue
		}
		if value, ok := raw.(bool); ok {
			v := value
			return &v
		}
	}
	return nil
}

func toolArgStringsFromRaw(raw interface{}) []string {
	switch values := raw.(type) {
	case nil:
		return nil
	case []string:
		return append([]string(nil), values...)
	case []interface{}:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if s, ok := value.(string); ok {
				result = append(result, s)
			}
		}
		if len(result) > 0 {
			return result
		}
	case string:
		if values != "" {
			return []string{values}
		}
	}
	return nil
}

func decodeCallWebhookArgs(tc ToolCall) callWebhookArgs {
	req := callWebhookArgs{
		WebhookName: firstNonEmptyToolString(tc.WebhookName, toolArgString(tc.Params, "webhook_name")),
	}
	if parameters, ok := tc.Parameters.(map[string]interface{}); ok && len(parameters) > 0 {
		req.Parameters = make(map[string]interface{}, len(parameters))
		for key, value := range parameters {
			req.Parameters[key] = value
		}
	} else {
		req.Parameters = toolArgInterfaceMap(tc.Params, "parameters")
	}
	if req.Parameters == nil {
		req.Parameters = map[string]interface{}{}
	}
	return req
}

func decodeSaveToolArgs(tc ToolCall) saveToolArgs {
	return saveToolArgs{
		Name:        firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Description: firstNonEmptyToolString(tc.Description, toolArgString(tc.Params, "description")),
		Code:        firstNonEmptyToolString(tc.Code, toolArgString(tc.Params, "code")),
	}
}

func decodeRunToolArgs(tc ToolCall) runToolArgs {
	req := runToolArgs{
		Name: firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Args: tc.GetArgs(),
	}
	if len(req.Args) == 0 {
		req.Args = toolArgStringsFromRaw(tc.Params["args"])
	}
	if tc.Background {
		req.Background = true
	} else if value, ok := toolArgBool(tc.Params, "background"); ok {
		req.Background = value
	}
	if len(tc.VaultKeys) > 0 {
		req.VaultKeys = append([]string(nil), tc.VaultKeys...)
	} else {
		req.VaultKeys = toolArgStringSlice(tc.Params, "vault_keys")
	}
	if len(tc.CredentialIDs) > 0 {
		req.CredentialIDs = append([]string(nil), tc.CredentialIDs...)
	} else {
		req.CredentialIDs = toolArgStringSlice(tc.Params, "credential_ids")
	}
	return req
}

func decodeSandboxExecutionArgs(tc ToolCall) sandboxExecutionArgs {
	req := sandboxExecutionArgs{
		Code:     firstNonEmptyToolString(tc.Code, toolArgString(tc.Params, "code")),
		Language: firstNonEmptyToolString(tc.SandboxLang, tc.Language, toolArgString(tc.Params, "sandbox_lang", "language")),
	}
	if len(tc.Libraries) > 0 {
		req.Libraries = append([]string(nil), tc.Libraries...)
	} else {
		req.Libraries = toolArgStringSlice(tc.Params, "libraries")
	}
	if len(tc.VaultKeys) > 0 {
		req.VaultKeys = append([]string(nil), tc.VaultKeys...)
	} else {
		req.VaultKeys = toolArgStringSlice(tc.Params, "vault_keys")
	}
	if len(tc.CredentialIDs) > 0 {
		req.CredentialIDs = append([]string(nil), tc.CredentialIDs...)
	} else {
		req.CredentialIDs = toolArgStringSlice(tc.Params, "credential_ids")
	}
	return req
}

func decodePythonExecutionArgs(tc ToolCall) pythonExecutionArgs {
	req := pythonExecutionArgs{
		Code:       firstNonEmptyToolString(tc.Code, toolArgString(tc.Params, "code")),
		Background: tc.Background,
	}
	if background, ok := toolArgBool(tc.Params, "background"); ok {
		req.Background = background
	}
	if len(tc.VaultKeys) > 0 {
		req.VaultKeys = append([]string(nil), tc.VaultKeys...)
	} else {
		req.VaultKeys = toolArgStringSlice(tc.Params, "vault_keys")
	}
	if len(tc.CredentialIDs) > 0 {
		req.CredentialIDs = append([]string(nil), tc.CredentialIDs...)
	} else {
		req.CredentialIDs = toolArgStringSlice(tc.Params, "credential_ids")
	}
	return req
}

func decodeShellExecutionArgs(tc ToolCall) shellExecutionArgs {
	req := shellExecutionArgs{
		Command:    firstNonEmptyToolString(tc.Command, toolArgString(tc.Params, "command")),
		Background: tc.Background,
	}
	if background, ok := toolArgBool(tc.Params, "background"); ok {
		req.Background = background
	}
	return req
}

func decodeSudoExecutionArgs(tc ToolCall) sudoExecutionArgs {
	return sudoExecutionArgs{
		Command: firstNonEmptyToolString(tc.Command, toolArgString(tc.Params, "command")),
	}
}

func decodeInstallPackageArgs(tc ToolCall) installPackageArgs {
	return installPackageArgs{
		Package: firstNonEmptyToolString(tc.Package, toolArgString(tc.Params, "package")),
	}
}

func decodeProcessControlArgs(tc ToolCall) processControlArgs {
	return processControlArgs{
		PID: max(tc.PID, toolArgInt(tc.Params, 0, "pid")),
	}
}

func decodeUpdateManagementArgs(tc ToolCall) updateManagementArgs {
	return updateManagementArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
	}
}

func decodeKnowledgeGraphArgs(tc ToolCall) knowledgeGraphArgs {
	req := knowledgeGraphArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ID:          firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		Label:       firstNonEmptyToolString(tc.Label, toolArgString(tc.Params, "label")),
		Properties:  tc.Properties,
		Source:      firstNonEmptyToolString(tc.Source, toolArgString(tc.Params, "source")),
		Target:      firstNonEmptyToolString(tc.Target, toolArgString(tc.Params, "target")),
		Relation:    firstNonEmptyToolString(tc.Relation, toolArgString(tc.Params, "relation")),
		NewRelation: firstNonEmptyToolString(tc.NewRelation, toolArgString(tc.Params, "new_relation")),
		Limit:       firstNonEmptyInt(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
		Depth:       firstNonEmptyInt(tc.Depth, toolArgInt(tc.Params, 0, "depth")),
		Content:     firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
	}
	if len(req.Properties) == 0 {
		req.Properties = toolArgStringMap(tc.Params, "properties")
	}
	return req
}

func decodeCoreMemoryArgs(tc ToolCall) coreMemoryArgs {
	return coreMemoryArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Fact:        firstNonEmptyToolString(tc.Fact, toolArgString(tc.Params, "fact")),
		Key:         firstNonEmptyToolString(tc.Key, toolArgString(tc.Params, "key")),
		Value:       firstNonEmptyToolString(tc.Value, toolArgString(tc.Params, "value")),
		ID:          firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		MemoryKey:   firstNonEmptyToolString(tc.MemoryKey, toolArgString(tc.Params, "memory_key")),
		MemoryValue: firstNonEmptyToolString(tc.MemoryValue, toolArgString(tc.Params, "memory_value")),
		Content:     firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
	}
}

func decodeCheatsheetArgs(tc ToolCall) cheatsheetArgs {
	req := cheatsheetArgs{
		Operation:    firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ID:           firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		Name:         firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Content:      firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		Active:       tc.Active,
		Filename:     firstNonEmptyToolString(tc.Filename, toolArgString(tc.Params, "filename")),
		Source:       firstNonEmptyToolString(tc.Source, toolArgString(tc.Params, "source")),
		AttachmentID: firstNonEmptyToolString(tc.AttachmentID, toolArgString(tc.Params, "attachment_id")),
	}
	if req.Active == nil {
		req.Active = toolArgBoolPtr(tc.Params, "active")
	}
	return req
}
