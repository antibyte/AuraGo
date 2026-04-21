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

type secretVaultArgs struct {
	Action    string
	Operation string
	Key       string
	Value     string
}

type cronScheduleArgs struct {
	Operation  string
	ID         string
	CronExpr   string
	TaskPrompt string
}

type documentCreatorArgs struct {
	Operation   string
	Title       string
	Content     string
	URL         string
	Filename    string
	PaperSize   string
	Landscape   bool
	Sections    string
	SourceFiles string
}

type archiveArgs struct {
	Operation   string
	FilePath    string
	Destination string
	SourceFiles string
	Format      string
}

type pdfOperationArgs struct {
	Operation     string
	FilePath      string
	OutputFile    string
	Pages         string
	Password      string
	WatermarkText string
	SourceFiles   string
}

type imageProcessingArgs struct {
	Operation    string
	FilePath     string
	OutputFile   string
	OutputFormat string
	Width        int
	Height       int
	QualityPct   int
	CropX        int
	CropY        int
	CropWidth    int
	CropHeight   int
	Angle        int
}

type apiRequestArgs struct {
	Method  string
	URL     string
	Body    string
	Headers map[string]string
}

type filesystemArgs struct {
	Operation   string
	FilePath    string
	Destination string
	Content     string
	Items       []map[string]interface{}
	Limit       int
	Offset      int
}

type fileEditorArgs struct {
	Operation string
	FilePath  string
	Old       string
	New       string
	Marker    string
	Content   string
	StartLine int
	EndLine   int
	LineCount int
	Pattern   string
}

type jsonEditorArgs struct {
	Operation string
	FilePath  string
	JsonPath  string
	SetValue  interface{}
	Content   string
}

type yamlEditorArgs struct {
	Operation string
	FilePath  string
	JsonPath  string
	SetValue  interface{}
}

type xmlEditorArgs struct {
	Operation string
	FilePath  string
	XPath     string
	SetValue  interface{}
}

type textDiffArgs struct {
	Operation string
	File1     string
	File2     string
	Text1     string
	Text2     string
}

type fileSearchArgs struct {
	Operation  string
	Pattern    string
	FilePath   string
	Glob       string
	OutputMode string
}

type advancedFileReadArgs struct {
	Operation string
	FilePath  string
	Pattern   string
	StartLine int
	EndLine   int
	LineCount int
}

type smartFileReadArgs struct {
	Operation        string
	FilePath         string
	Query            string
	SamplingStrategy string
	MaxTokens        int
	LineCount        int
}

type cloudStorageArgs struct {
	Operation   string
	FilePath    string
	Destination string
	Content     string
	MaxResults  int
}

type imageGenerationArgs struct {
	Prompt        string
	EnhancePrompt *bool
	Model         string
	Size          string
	Quality       string
	Style         string
	SourceImage   string
}

type inventoryQueryArgs struct {
	Tag        string
	DeviceType string
	Hostname   string
}

type remoteShellArgs struct {
	ServerID string
	Command  string
}

type remoteFileTransferArgs struct {
	ServerID   string
	Direction  string
	LocalPath  string
	RemotePath string
}

type memoryReflectArgs struct {
	Scope string
}

type memoryOrchestratorArgs struct {
	Preview         bool
	ThresholdLow    int
	ThresholdMedium int
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

func toolArgRaw(args map[string]interface{}, keys ...string) (interface{}, bool) {
	for _, key := range keys {
		raw, ok := args[key]
		if ok {
			return raw, true
		}
	}
	return nil, false
}

func toolArgItems(args map[string]interface{}, keys ...string) []map[string]interface{} {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok {
			continue
		}
		switch values := raw.(type) {
		case []map[string]interface{}:
			result := make([]map[string]interface{}, 0, len(values))
			for _, value := range values {
				item := make(map[string]interface{}, len(value))
				for k, v := range value {
					item[k] = v
				}
				result = append(result, item)
			}
			return result
		case []interface{}:
			result := make([]map[string]interface{}, 0, len(values))
			for _, value := range values {
				item, ok := value.(map[string]interface{})
				if !ok {
					continue
				}
				clone := make(map[string]interface{}, len(item))
				for k, v := range item {
					clone[k] = v
				}
				result = append(result, clone)
			}
			if len(result) > 0 {
				return result
			}
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
	// Some models (e.g. glm-5.1) emit "action_type" instead of "operation".
	// Resolve operation with fallback priority: operation → action_type → params.
	operation := firstNonEmptyToolString(tc.Operation, tc.ActionType, toolArgString(tc.Params, "operation", "action_type"))
	return coreMemoryArgs{
		Operation:   operation,
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

func decodeSecretVaultArgs(tc ToolCall) secretVaultArgs {
	return secretVaultArgs{
		Action:    firstNonEmptyToolString(tc.Action, toolArgString(tc.Params, "action")),
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Key:       firstNonEmptyToolString(tc.Key, toolArgString(tc.Params, "key")),
		Value:     firstNonEmptyToolString(tc.Value, toolArgString(tc.Params, "value")),
	}
}

func decodeCronScheduleArgs(tc ToolCall) cronScheduleArgs {
	return cronScheduleArgs{
		Operation:  firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ID:         firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		CronExpr:   firstNonEmptyToolString(tc.CronExpr, toolArgString(tc.Params, "cron_expr")),
		TaskPrompt: firstNonEmptyToolString(tc.TaskPrompt, tc.Content, toolArgString(tc.Params, "task_prompt", "content")),
	}
}

func decodeDocumentCreatorArgs(tc ToolCall) documentCreatorArgs {
	req := documentCreatorArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Title:       firstNonEmptyToolString(tc.Title, toolArgString(tc.Params, "title")),
		Content:     firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		URL:         firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		Filename:    firstNonEmptyToolString(tc.Filename, toolArgString(tc.Params, "filename")),
		PaperSize:   firstNonEmptyToolString(tc.PaperSize, toolArgString(tc.Params, "paper_size")),
		Landscape:   tc.Landscape,
		Sections:    firstNonEmptyToolString(string(tc.Sections), toolArgJSONText(tc.Params, "sections")),
		SourceFiles: firstNonEmptyToolString(string(tc.SourceFiles), toolArgJSONText(tc.Params, "source_files")),
	}
	if landscape, ok := toolArgBool(tc.Params, "landscape"); ok {
		req.Landscape = landscape
	}
	return req
}

func decodeArchiveArgs(tc ToolCall) archiveArgs {
	return archiveArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		FilePath:    firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		Destination: firstNonEmptyToolString(tc.Destination, tc.Dest, toolArgString(tc.Params, "destination", "dest")),
		SourceFiles: firstNonEmptyToolString(string(tc.SourceFiles), toolArgJSONText(tc.Params, "source_files")),
		Format:      firstNonEmptyToolString(tc.Format, toolArgString(tc.Params, "format")),
	}
}

func decodePDFOperationArgs(tc ToolCall) pdfOperationArgs {
	return pdfOperationArgs{
		Operation:     firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		FilePath:      firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		OutputFile:    firstNonEmptyToolString(tc.OutputFile, tc.Destination, tc.Dest, toolArgString(tc.Params, "output_file", "destination", "dest")),
		Pages:         firstNonEmptyToolString(tc.Pages, toolArgString(tc.Params, "pages")),
		Password:      firstNonEmptyToolString(tc.Password, toolArgString(tc.Params, "password")),
		WatermarkText: firstNonEmptyToolString(tc.WatermarkText, toolArgString(tc.Params, "watermark_text")),
		SourceFiles:   firstNonEmptyToolString(string(tc.SourceFiles), toolArgJSONText(tc.Params, "source_files")),
	}
}

func decodeImageProcessingArgs(tc ToolCall) imageProcessingArgs {
	return imageProcessingArgs{
		Operation:    firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		FilePath:     firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		OutputFile:   firstNonEmptyToolString(tc.OutputFile, tc.Destination, tc.Dest, toolArgString(tc.Params, "output_file", "destination", "dest")),
		OutputFormat: firstNonEmptyToolString(tc.OutputFormat, toolArgString(tc.Params, "output_format")),
		Width:        firstNonEmptyInt(tc.Width, toolArgInt(tc.Params, 0, "width")),
		Height:       firstNonEmptyInt(tc.Height, toolArgInt(tc.Params, 0, "height")),
		QualityPct:   firstNonEmptyInt(tc.QualityPct, toolArgInt(tc.Params, 0, "quality_pct")),
		CropX:        firstNonEmptyInt(tc.CropX, toolArgInt(tc.Params, 0, "crop_x")),
		CropY:        firstNonEmptyInt(tc.CropY, toolArgInt(tc.Params, 0, "crop_y")),
		CropWidth:    firstNonEmptyInt(tc.CropWidth, toolArgInt(tc.Params, 0, "crop_width")),
		CropHeight:   firstNonEmptyInt(tc.CropHeight, toolArgInt(tc.Params, 0, "crop_height")),
		Angle:        firstNonEmptyInt(tc.Angle, toolArgInt(tc.Params, 0, "angle")),
	}
}

func decodeAPIRequestArgs(tc ToolCall) apiRequestArgs {
	req := apiRequestArgs{
		Method:  firstNonEmptyToolString(tc.Method, toolArgString(tc.Params, "method")),
		URL:     firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		Body:    firstNonEmptyToolString(tc.Body, tc.Content, toolArgString(tc.Params, "body", "content")),
		Headers: tc.Headers,
	}
	if len(req.Headers) == 0 {
		req.Headers = toolArgStringMap(tc.Params, "headers")
	}
	return req
}

func decodeFilesystemArgs(tc ToolCall) filesystemArgs {
	req := filesystemArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		FilePath:    firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		Destination: firstNonEmptyToolString(tc.Destination, tc.Dest, toolArgString(tc.Params, "destination", "dest")),
		Content:     firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		Limit:       toolArgInt(tc.Params, 0, "limit"),
		Offset:      toolArgInt(tc.Params, 0, "offset"),
	}
	if len(tc.Items) > 0 {
		req.Items = append([]map[string]interface{}(nil), tc.Items...)
	} else {
		req.Items = toolArgItems(tc.Params, "items")
	}
	return req
}

func decodeFileEditorArgs(tc ToolCall) fileEditorArgs {
	return fileEditorArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		FilePath:  firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		Old:       toolArgString(tc.Params, "old"),
		New:       toolArgString(tc.Params, "new"),
		Marker:    toolArgString(tc.Params, "marker"),
		Content:   firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		StartLine: toolArgInt(tc.Params, 0, "start_line"),
		EndLine:   toolArgInt(tc.Params, 0, "end_line"),
		LineCount: toolArgInt(tc.Params, 0, "line_count"),
		Pattern:   toolArgString(tc.Params, "pattern", "glob"),
	}
}

func decodeJSONEditorArgs(tc ToolCall) jsonEditorArgs {
	req := jsonEditorArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		FilePath:  firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		JsonPath:  toolArgString(tc.Params, "json_path"),
		Content:   firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
	}
	if value, ok := toolArgRaw(tc.Params, "set_value"); ok {
		req.SetValue = value
	}
	return req
}

type tomlEditorArgs struct {
	Operation string
	FilePath  string
	TomlPath  string
	SetValue  interface{}
}

func decodeTOMLEditorArgs(tc ToolCall) tomlEditorArgs {
	req := tomlEditorArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		FilePath:  firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		TomlPath:  firstNonEmptyToolString(toolArgString(tc.Params, "toml_path"), toolArgString(tc.Params, "json_path")),
	}
	if value, ok := toolArgRaw(tc.Params, "set_value"); ok {
		req.SetValue = value
	}
	return req
}

func decodeYAMLEditorArgs(tc ToolCall) yamlEditorArgs {
	req := yamlEditorArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		FilePath:  firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		JsonPath:  toolArgString(tc.Params, "json_path"),
	}
	if value, ok := toolArgRaw(tc.Params, "set_value"); ok {
		req.SetValue = value
	}
	return req
}

func decodeXMLEditorArgs(tc ToolCall) xmlEditorArgs {
	req := xmlEditorArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		FilePath:  firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		XPath:     firstNonEmptyToolString(toolArgString(tc.Params, "xpath"), toolArgString(tc.Params, "json_path")),
	}
	if value, ok := toolArgRaw(tc.Params, "set_value"); ok {
		req.SetValue = value
	}
	return req
}

func decodeTextDiffArgs(tc ToolCall) textDiffArgs {
	return textDiffArgs{
		Operation: toolArgString(tc.Params, "operation"),
		File1:     toolArgString(tc.Params, "file1"),
		File2:     toolArgString(tc.Params, "file2"),
		Text1:     toolArgString(tc.Params, "text1"),
		Text2:     toolArgString(tc.Params, "text2"),
	}
}

func decodeFileSearchArgs(tc ToolCall) fileSearchArgs {
	return fileSearchArgs{
		Operation:  firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Pattern:    toolArgString(tc.Params, "pattern"),
		FilePath:   firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		Glob:       toolArgString(tc.Params, "glob"),
		OutputMode: toolArgString(tc.Params, "output_mode"),
	}
}

func decodeAdvancedFileReadArgs(tc ToolCall) advancedFileReadArgs {
	return advancedFileReadArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		FilePath:  firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		Pattern:   toolArgString(tc.Params, "pattern"),
		StartLine: toolArgInt(tc.Params, 0, "start_line"),
		EndLine:   toolArgInt(tc.Params, 0, "end_line"),
		LineCount: toolArgInt(tc.Params, 0, "line_count"),
	}
}

func decodeSmartFileReadArgs(tc ToolCall) smartFileReadArgs {
	return smartFileReadArgs{
		Operation:        firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		FilePath:         firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		Query:            firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		SamplingStrategy: toolArgString(tc.Params, "sampling_strategy"),
		MaxTokens:        toolArgInt(tc.Params, 0, "max_tokens"),
		LineCount:        toolArgInt(tc.Params, 0, "line_count"),
	}
}

func decodeCloudStorageArgs(tc ToolCall) cloudStorageArgs {
	return cloudStorageArgs{
		Operation:   firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"), tc.Action),
		FilePath:    firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		Destination: firstNonEmptyToolString(tc.Destination, tc.Dest, toolArgString(tc.Params, "destination", "dest")),
		Content:     firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		MaxResults:  firstNonEmptyInt(tc.MaxResults, toolArgInt(tc.Params, 0, "max_results")),
	}
}

func decodeImageGenerationArgs(tc ToolCall) imageGenerationArgs {
	req := imageGenerationArgs{
		Prompt: firstNonEmptyToolString(
			tc.Prompt,
			toolArgString(tc.Params, "prompt"),
			tc.Content,
			toolArgString(tc.Params, "content"),
		),
		EnhancePrompt: tc.EnhancePrompt,
		Model:         firstNonEmptyToolString(tc.Model, toolArgString(tc.Params, "model")),
		Size:          firstNonEmptyToolString(tc.Size, toolArgString(tc.Params, "size")),
		Quality:       firstNonEmptyToolString(tc.Quality, toolArgString(tc.Params, "quality")),
		Style:         firstNonEmptyToolString(tc.Style, toolArgString(tc.Params, "style")),
		SourceImage:   firstNonEmptyToolString(tc.SourceImage, toolArgString(tc.Params, "source_image")),
	}
	if req.EnhancePrompt == nil {
		req.EnhancePrompt = toolArgBoolPtr(tc.Params, "enhance_prompt")
	}
	return req
}

func decodeInventoryQueryArgs(tc ToolCall) inventoryQueryArgs {
	tag := firstNonEmptyToolString(
		tc.Tag,
		tc.Tags,
		toolArgString(tc.Params, "tag", "tags"),
	)
	return inventoryQueryArgs{
		Tag:        tag,
		DeviceType: firstNonEmptyToolString(tc.DeviceType, toolArgString(tc.Params, "device_type")),
		Hostname:   firstNonEmptyToolString(tc.Hostname, toolArgString(tc.Params, "hostname", "name")),
	}
}

func decodeRemoteShellArgs(tc ToolCall) remoteShellArgs {
	return remoteShellArgs{
		ServerID: firstNonEmptyToolString(tc.ServerID, toolArgString(tc.Params, "server_id", "id", "name")),
		Command:  firstNonEmptyToolString(tc.Command, toolArgString(tc.Params, "command")),
	}
}

func decodeRemoteFileTransferArgs(tc ToolCall) remoteFileTransferArgs {
	return remoteFileTransferArgs{
		ServerID:   firstNonEmptyToolString(tc.ServerID, toolArgString(tc.Params, "server_id", "id", "name")),
		Direction:  firstNonEmptyToolString(tc.Direction, toolArgString(tc.Params, "direction")),
		LocalPath:  firstNonEmptyToolString(tc.LocalPath, toolArgString(tc.Params, "local_path")),
		RemotePath: firstNonEmptyToolString(tc.RemotePath, toolArgString(tc.Params, "remote_path")),
	}
}

func decodeMemoryReflectArgs(tc ToolCall) memoryReflectArgs {
	return memoryReflectArgs{
		Scope: firstNonEmptyToolString(tc.Scope, toolArgString(tc.Params, "scope")),
	}
}

func decodeMemoryOrchestratorArgs(tc ToolCall) memoryOrchestratorArgs {
	req := memoryOrchestratorArgs{
		Preview:         tc.Preview,
		ThresholdLow:    firstNonEmptyInt(tc.ThresholdLow, toolArgInt(tc.Params, 0, "threshold_low")),
		ThresholdMedium: firstNonEmptyInt(tc.ThresholdMedium, toolArgInt(tc.Params, 0, "threshold_medium")),
	}
	if value, ok := toolArgBool(tc.Params, "preview"); ok {
		req.Preview = value
	}
	return req
}
