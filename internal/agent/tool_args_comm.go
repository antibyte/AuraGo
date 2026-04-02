package agent

import "strings"

type followUpArgs struct {
	TaskPrompt         string
	DelaySeconds       int
	TimeoutSecs        int
	NotifyOnCompletion bool
}

type waitForEventArgs struct {
	EventType          string
	TaskPrompt         string
	URL                string
	Host               string
	Port               int
	FilePath           string
	PID                int
	TimeoutSecs        int
	IntervalSecs       int
	NotifyOnCompletion bool
}

type toolManualArgs struct {
	ToolName string
}

type lifeboatHandoverArgs struct {
	TaskPrompt string
}

type systemMetricsArgs struct {
	Target string
}

type planManagementArgs struct {
	Operation       string
	ID              string
	TaskID          string
	Title           string
	Description     string
	Content         string
	Priority        int
	Status          string
	Limit           int
	IncludeArchived bool
	Result          string
	Error           string
	Reason          string
	FilePath        string
	URL             string
	ArtifactType    string
	Label           string
	Items           []map[string]interface{}
}

type notesManagementArgs struct {
	Operation string
	Category  string
	Title     string
	Content   string
	Priority  int
	DueDate   string
	Done      int
	NoteID    int64
}

type journalManagementArgs struct {
	Operation  string
	Title      string
	Content    string
	EntryType  string
	Importance int
	Tags       string
	Limit      int
	Query      string
	FromDate   string
	ToDate     string
	EntryID    int64
}

type telnyxSMSArgs struct {
	Operation string
	To        string
	Message   string
	MessageID string
	MediaURLs []string
}

type telnyxCallArgs struct {
	Operation     string
	To            string
	CallControlID string
	Text          string
	AudioURL      string
	MaxDigits     int
	TimeoutSecs   int
}

type telnyxManageArgs struct {
	Operation string
	Limit     int
	Port      int
}

type addressBookArgs struct {
	Operation      string
	ID             string
	Query          string
	Name           string
	Email          string
	Phone          string
	Mobile         string
	ContactAddress string
	Relationship   string
	Notes          string
}

func toolArgItemMaps(args map[string]interface{}, keys ...string) []map[string]interface{} {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok {
			continue
		}
		switch values := raw.(type) {
		case []map[string]interface{}:
			result := make([]map[string]interface{}, 0, len(values))
			for _, item := range values {
				cp := make(map[string]interface{}, len(item))
				for k, v := range item {
					cp[k] = v
				}
				result = append(result, cp)
			}
			return result
		case []interface{}:
			result := make([]map[string]interface{}, 0, len(values))
			for _, item := range values {
				if m, ok := item.(map[string]interface{}); ok {
					cp := make(map[string]interface{}, len(m))
					for k, v := range m {
						cp[k] = v
					}
					result = append(result, cp)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return nil
}

func decodeFollowUpArgs(tc ToolCall) followUpArgs {
	req := followUpArgs{
		TaskPrompt:         firstNonEmptyToolString(tc.TaskPrompt, toolArgString(tc.Params, "task_prompt", "prompt")),
		DelaySeconds:       toolArgInt(tc.Params, tc.DelaySeconds, "delay_seconds"),
		TimeoutSecs:        toolArgInt(tc.Params, tc.TimeoutSecs, "timeout_secs", "timeout_seconds"),
		NotifyOnCompletion: tc.NotifyOnCompletion,
	}
	if notify, ok := toolArgBool(tc.Params, "notify_on_completion"); ok {
		req.NotifyOnCompletion = notify
	}
	return req
}

func decodeWaitForEventArgs(tc ToolCall) waitForEventArgs {
	req := waitForEventArgs{
		EventType:          firstNonEmptyToolString(tc.EventType, toolArgString(tc.Params, "event_type")),
		TaskPrompt:         firstNonEmptyToolString(tc.TaskPrompt, toolArgString(tc.Params, "task_prompt", "prompt")),
		URL:                firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		Host:               firstNonEmptyToolString(tc.Host, toolArgString(tc.Params, "host")),
		Port:               toolArgInt(tc.Params, tc.Port, "port"),
		FilePath:           firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		PID:                toolArgInt(tc.Params, tc.PID, "pid"),
		TimeoutSecs:        toolArgInt(tc.Params, tc.TimeoutSecs, "timeout_secs", "timeout_seconds"),
		IntervalSecs:       toolArgInt(tc.Params, tc.IntervalSecs, "interval_secs", "poll_interval", "poll_interval_seconds"),
		NotifyOnCompletion: tc.NotifyOnCompletion,
	}
	if notify, ok := toolArgBool(tc.Params, "notify_on_completion"); ok {
		req.NotifyOnCompletion = notify
	}
	return req
}

func decodeToolManualArgs(tc ToolCall) toolManualArgs {
	return toolManualArgs{
		ToolName: firstNonEmptyToolString(tc.ToolName, tc.Name, toolArgString(tc.Params, "tool_name", "name")),
	}
}

func decodeLifeboatHandoverArgs(tc ToolCall) lifeboatHandoverArgs {
	return lifeboatHandoverArgs{
		TaskPrompt: firstNonEmptyToolString(tc.TaskPrompt, tc.Content, toolArgString(tc.Params, "task_prompt", "content", "prompt")),
	}
}

func decodeSystemMetricsArgs(tc ToolCall) systemMetricsArgs {
	return systemMetricsArgs{
		Target: firstNonEmptyToolString(tc.Target, toolArgString(tc.Params, "target")),
	}
}

func decodePlanManagementArgs(tc ToolCall) planManagementArgs {
	req := planManagementArgs{
		Operation:       firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ID:              firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		TaskID:          firstNonEmptyToolString(tc.TaskID, toolArgString(tc.Params, "task_id")),
		Title:           firstNonEmptyToolString(tc.Title, toolArgString(tc.Params, "title")),
		Description:     firstNonEmptyToolString(tc.Description, toolArgString(tc.Params, "description")),
		Content:         firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		Priority:        toolArgInt(tc.Params, tc.Priority, "priority"),
		Status:          firstNonEmptyToolString(tc.Status, toolArgString(tc.Params, "status")),
		Limit:           toolArgInt(tc.Params, tc.Limit, "limit"),
		IncludeArchived: tc.IncludeArchived,
		Result:          firstNonEmptyToolString(tc.Result, toolArgString(tc.Params, "result")),
		Error:           firstNonEmptyToolString(tc.Error, toolArgString(tc.Params, "error")),
		Reason:          firstNonEmptyToolString(tc.Reason, toolArgString(tc.Params, "reason")),
		FilePath:        firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
		URL:             firstNonEmptyToolString(tc.URL, toolArgString(tc.Params, "url")),
		ArtifactType:    firstNonEmptyToolString(tc.ArtifactType, toolArgString(tc.Params, "artifact_type")),
		Label:           firstNonEmptyToolString(tc.Label, toolArgString(tc.Params, "label")),
	}
	if includeArchived, ok := toolArgBool(tc.Params, "include_archived"); ok {
		req.IncludeArchived = includeArchived
	}
	if len(tc.Items) > 0 {
		req.Items = append([]map[string]interface{}(nil), tc.Items...)
	} else {
		req.Items = toolArgItemMaps(tc.Params, "items")
	}
	return req
}

func decodeNotesManagementArgs(tc ToolCall) notesManagementArgs {
	return notesManagementArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Category:  firstNonEmptyToolString(tc.Category, toolArgString(tc.Params, "category")),
		Title:     firstNonEmptyToolString(tc.Title, toolArgString(tc.Params, "title")),
		Content:   firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		Priority:  toolArgInt(tc.Params, tc.Priority, "priority"),
		DueDate:   firstNonEmptyToolString(tc.DueDate, toolArgString(tc.Params, "due_date")),
		Done:      toolArgInt(tc.Params, tc.Done, "done"),
		NoteID:    toolArgInt64(tc.Params, "note_id"),
	}
}

func decodeJournalManagementArgs(tc ToolCall) journalManagementArgs {
	return journalManagementArgs{
		Operation:  firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Title:      firstNonEmptyToolString(tc.Title, toolArgString(tc.Params, "title")),
		Content:    firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content")),
		EntryType:  firstNonEmptyToolString(tc.EntryType, toolArgString(tc.Params, "entry_type")),
		Importance: toolArgInt(tc.Params, tc.Importance, "importance"),
		Tags:       firstNonEmptyToolString(tc.Tags, toolArgString(tc.Params, "tags")),
		Limit:      toolArgInt(tc.Params, tc.Limit, "limit"),
		Query:      firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		FromDate:   firstNonEmptyToolString(tc.FromDate, toolArgString(tc.Params, "from_date")),
		ToDate:     firstNonEmptyToolString(tc.ToDate, toolArgString(tc.Params, "to_date")),
		EntryID:    firstNonEmptyToolInt64(tc.EntryID, toolArgInt64(tc.Params, "entry_id")),
	}
}

func decodeTelnyxSMSArgs(tc ToolCall) telnyxSMSArgs {
	req := telnyxSMSArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		To:        firstNonEmptyToolString(tc.To, toolArgString(tc.Params, "to")),
		Message:   firstNonEmptyToolString(tc.Message, tc.Text, tc.Content, toolArgString(tc.Params, "message", "text", "content")),
		MessageID: firstNonEmptyToolString(tc.ID, tc.MessageID, toolArgString(tc.Params, "id", "message_id")),
	}
	if len(tc.MediaURLs) > 0 {
		req.MediaURLs = append([]string(nil), tc.MediaURLs...)
	} else {
		req.MediaURLs = toolArgStringSlice(tc.Params, "media_urls")
	}
	return req
}

func decodeTelnyxCallArgs(tc ToolCall) telnyxCallArgs {
	return telnyxCallArgs{
		Operation:     firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		To:            firstNonEmptyToolString(tc.To, toolArgString(tc.Params, "to")),
		CallControlID: firstNonEmptyToolString(tc.CallControlID, toolArgString(tc.Params, "call_control_id")),
		Text:          firstNonEmptyToolString(tc.Text, tc.Message, tc.Content, toolArgString(tc.Params, "text", "message", "content")),
		AudioURL:      firstNonEmptyToolString(tc.AudioURL, toolArgString(tc.Params, "audio_url")),
		MaxDigits:     toolArgInt(tc.Params, tc.MaxDigits, "max_digits"),
		TimeoutSecs:   toolArgInt(tc.Params, tc.TimeoutSecs, "timeout_secs", "timeout_seconds"),
	}
}

func decodeTelnyxManageArgs(tc ToolCall) telnyxManageArgs {
	return telnyxManageArgs{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Limit:     toolArgInt(tc.Params, tc.Limit, "limit"),
		Port:      toolArgInt(tc.Params, tc.Port, "port"),
	}
}

func decodeAddressBookArgs(tc ToolCall) addressBookArgs {
	return addressBookArgs{
		Operation:      firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ID:             firstNonEmptyToolString(tc.ID, toolArgString(tc.Params, "id")),
		Query:          firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query")),
		Name:           firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name")),
		Email:          firstNonEmptyToolString(tc.Email, toolArgString(tc.Params, "email")),
		Phone:          firstNonEmptyToolString(tc.Phone, toolArgString(tc.Params, "phone")),
		Mobile:         firstNonEmptyToolString(tc.Mobile, toolArgString(tc.Params, "mobile")),
		ContactAddress: firstNonEmptyToolString(tc.ContactAddress, toolArgString(tc.Params, "address")),
		Relationship:   firstNonEmptyToolString(tc.Relationship, toolArgString(tc.Params, "relationship")),
		Notes:          firstNonEmptyToolString(tc.Notes, toolArgString(tc.Params, "notes")),
	}
}

func (req journalManagementArgs) normalizedTags() []string {
	if req.Tags == "" {
		return nil
	}
	parts := strings.Split(req.Tags, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func firstNonEmptyToolInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
