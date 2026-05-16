package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aurago/internal/agentmail"
)

type agentMailArgs struct {
	Operation    string
	InboxID      string
	MessageID    string
	ThreadID     string
	DraftID      string
	AttachmentID string
	Limit        int
	Cursor       string
	After        string
	Labels       []string
	AddLabels    []string
	RemoveLabels []string
	To           []string
	CC           []string
	BCC          []string
	Subject      string
	Text         string
	HTML         string
	Username     string
	Domain       string
	DisplayName  string
	Attachments  []agentmail.AttachmentInput
}

var agentMailMutatingOperations = map[string]struct{}{
	"create_inbox":          {},
	"update_inbox":          {},
	"delete_inbox":          {},
	"update_message_labels": {},
	"delete_message":        {},
	"send_message":          {},
	"reply_message":         {},
	"reply_all_message":     {},
	"forward_message":       {},
	"create_draft":          {},
	"update_draft":          {},
	"delete_draft":          {},
	"send_draft":            {},
}

func dispatchAgentMailCases(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	if dc == nil || dc.Cfg == nil {
		return agentMailError("config is not available"), true
	}
	cfg := dc.Cfg
	if !cfg.AgentMail.Enabled {
		return agentMailError("AgentMail is not enabled. Enable agentmail.enabled in config.yaml."), true
	}
	req := decodeAgentMailArgs(tc, cfg.AgentMail.InboxID)
	if req.Operation == "" {
		return agentMailError("'operation' is required"), true
	}
	if cfg.AgentMail.ReadOnly {
		if _, mutating := agentMailMutatingOperations[req.Operation]; mutating {
			return agentMailError("AgentMail is in read-only mode. Disable agentmail.readonly to allow this operation."), true
		}
	}
	amCfg := agentmail.ConfigFromAppConfig(cfg.AgentMail)
	client, err := agentmail.NewClient(agentmail.ClientConfig{
		BaseURL: amCfg.BaseURL,
		APIKey:  amCfg.APIKey,
	})
	if err != nil {
		return agentMailError(err.Error()), true
	}
	opCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	result, err := executeAgentMailOperation(opCtx, client, amCfg, req, cfg.Directories.WorkspaceDir)
	if err != nil {
		return agentMailError(err.Error()), true
	}
	return agentMailSuccess(result), true
}

func executeAgentMailOperation(ctx context.Context, client *agentmail.Client, cfg agentmail.Config, req agentMailArgs, workspaceDir string) (map[string]interface{}, error) {
	inboxID := strings.TrimSpace(req.InboxID)
	requireInbox := func() (string, error) {
		if inboxID == "" {
			return "", fmt.Errorf("'inbox_id' is required for %s (or set agentmail.inbox_id)", req.Operation)
		}
		return inboxID, nil
	}
	requireMessage := func() (string, error) {
		if strings.TrimSpace(req.MessageID) == "" {
			return "", fmt.Errorf("'message_id' is required for %s", req.Operation)
		}
		return strings.TrimSpace(req.MessageID), nil
	}
	prepareAttachments := func(inputs []agentmail.AttachmentInput) ([]agentmail.OutgoingAttachment, error) {
		if len(inputs) == 0 {
			return nil, nil
		}
		if strings.TrimSpace(workspaceDir) == "" {
			workspaceDir = "."
		}
		return agentmail.PrepareAttachments(workspaceDir, cfg.MaxAttachmentMB, inputs)
	}

	switch req.Operation {
	case "test_connection":
		res, err := client.ListInboxes(ctx, agentmail.ListInboxesOptions{Limit: 1})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"message": "Connection successful", "inbox_count": len(res.Inboxes), "inboxes": res.Inboxes}, nil
	case "list_inboxes":
		res, err := client.ListInboxes(ctx, agentmail.ListInboxesOptions{Limit: req.Limit, Cursor: req.Cursor})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"inboxes": res.Inboxes, "next_cursor": res.NextCursor}, nil
	case "get_inbox":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		inbox, err := client.GetInbox(ctx, id)
		return map[string]interface{}{"inbox": inbox}, err
	case "create_inbox":
		inbox, err := client.CreateInbox(ctx, agentmail.CreateInboxRequest{Username: req.Username, Domain: req.Domain, DisplayName: req.DisplayName})
		return map[string]interface{}{"inbox": inbox}, err
	case "update_inbox":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		inbox, err := client.UpdateInbox(ctx, id, agentmail.UpdateInboxRequest{DisplayName: req.DisplayName})
		return map[string]interface{}{"inbox": inbox}, err
	case "delete_inbox":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"deleted": id}, client.DeleteInbox(ctx, id)
	case "list_messages":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		res, err := client.ListMessages(ctx, id, agentmail.ListMessagesOptions{Limit: req.Limit, Cursor: req.Cursor, After: req.After, Labels: req.Labels, Thread: req.ThreadID})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"messages": res.Messages, "next_cursor": res.NextCursor}, nil
	case "get_message":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		msgID, err := requireMessage()
		if err != nil {
			return nil, err
		}
		msg, err := client.GetMessage(ctx, id, msgID)
		return map[string]interface{}{"message": msg}, err
	case "update_message_labels":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		msgID, err := requireMessage()
		if err != nil {
			return nil, err
		}
		msg, err := client.UpdateMessage(ctx, id, msgID, agentmail.UpdateMessageRequest{AddLabels: req.AddLabels, RemoveLabels: req.RemoveLabels})
		return map[string]interface{}{"message": msg}, err
	case "delete_message":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		msgID, err := requireMessage()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"deleted": msgID}, client.DeleteMessage(ctx, id, msgID)
	case "send_message":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		attachments, err := prepareAttachments(req.Attachments)
		if err != nil {
			return nil, err
		}
		msg, err := client.SendMessage(ctx, id, agentmail.SendMessageRequest{To: req.To, CC: req.CC, BCC: req.BCC, Subject: req.Subject, Text: req.Text, HTML: req.HTML, Attachments: attachments})
		return map[string]interface{}{"message": msg}, err
	case "reply_message", "reply_all_message":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		msgID, err := requireMessage()
		if err != nil {
			return nil, err
		}
		attachments, err := prepareAttachments(req.Attachments)
		if err != nil {
			return nil, err
		}
		reply := agentmail.ReplyMessageRequest{Text: req.Text, HTML: req.HTML, Attachments: attachments}
		var msg *agentmail.Message
		if req.Operation == "reply_all_message" {
			msg, err = client.ReplyAllMessage(ctx, id, msgID, reply)
		} else {
			msg, err = client.ReplyMessage(ctx, id, msgID, reply)
		}
		return map[string]interface{}{"message": msg}, err
	case "forward_message":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		msgID, err := requireMessage()
		if err != nil {
			return nil, err
		}
		attachments, err := prepareAttachments(req.Attachments)
		if err != nil {
			return nil, err
		}
		msg, err := client.ForwardMessage(ctx, id, msgID, agentmail.ForwardMessageRequest{To: req.To, CC: req.CC, BCC: req.BCC, Text: req.Text, HTML: req.HTML, Attachments: attachments})
		return map[string]interface{}{"message": msg}, err
	case "get_raw_message":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		msgID, err := requireMessage()
		if err != nil {
			return nil, err
		}
		raw, err := client.GetRawMessage(ctx, id, msgID)
		return map[string]interface{}{"raw": string(raw)}, err
	case "get_attachment":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		msgID, err := requireMessage()
		if err != nil {
			return nil, err
		}
		if req.AttachmentID == "" {
			return nil, fmt.Errorf("'attachment_id' is required for get_attachment")
		}
		attachment, err := client.GetAttachment(ctx, id, msgID, req.AttachmentID)
		return map[string]interface{}{"attachment": attachment}, err
	case "list_threads":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		res, err := client.ListThreads(ctx, id, agentmail.ListThreadsOptions{Limit: req.Limit, Cursor: req.Cursor})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"threads": res.Threads, "next_cursor": res.NextCursor}, nil
	case "get_thread":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		if req.ThreadID == "" {
			return nil, fmt.Errorf("'thread_id' is required for get_thread")
		}
		thread, err := client.GetThread(ctx, id, req.ThreadID)
		return map[string]interface{}{"thread": thread}, err
	case "list_drafts":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		res, err := client.ListDrafts(ctx, id, agentmail.ListDraftsOptions{Limit: req.Limit, Cursor: req.Cursor})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"drafts": res.Drafts, "next_cursor": res.NextCursor}, nil
	case "get_draft":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		if req.DraftID == "" {
			return nil, fmt.Errorf("'draft_id' is required for get_draft")
		}
		draft, err := client.GetDraft(ctx, id, req.DraftID)
		return map[string]interface{}{"draft": draft}, err
	case "create_draft", "update_draft":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		attachments, err := prepareAttachments(req.Attachments)
		if err != nil {
			return nil, err
		}
		draftReq := agentmail.Draft{To: req.To, CC: req.CC, BCC: req.BCC, Subject: req.Subject, Text: req.Text, HTML: req.HTML, Attachments: attachments}
		var draft *agentmail.Draft
		if req.Operation == "update_draft" {
			if req.DraftID == "" {
				return nil, fmt.Errorf("'draft_id' is required for update_draft")
			}
			draft, err = client.UpdateDraft(ctx, id, req.DraftID, draftReq)
		} else {
			draft, err = client.CreateDraft(ctx, id, draftReq)
		}
		return map[string]interface{}{"draft": draft}, err
	case "delete_draft":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		if req.DraftID == "" {
			return nil, fmt.Errorf("'draft_id' is required for delete_draft")
		}
		return map[string]interface{}{"deleted": req.DraftID}, client.DeleteDraft(ctx, id, req.DraftID)
	case "send_draft":
		id, err := requireInbox()
		if err != nil {
			return nil, err
		}
		if req.DraftID == "" {
			return nil, fmt.Errorf("'draft_id' is required for send_draft")
		}
		msg, err := client.SendDraft(ctx, id, req.DraftID)
		return map[string]interface{}{"message": msg}, err
	default:
		return nil, fmt.Errorf("unsupported AgentMail operation %q", req.Operation)
	}
}

func decodeAgentMailArgs(tc ToolCall, defaultInboxID string) agentMailArgs {
	params := tc.Params
	req := agentMailArgs{
		Operation:    strings.ToLower(strings.TrimSpace(firstNonEmptyToolString(tc.Operation, tc.ActionType, toolArgString(params, "operation")))),
		InboxID:      firstNonEmptyToolString(toolArgString(params, "inbox_id"), defaultInboxID),
		MessageID:    firstNonEmptyToolString(tc.ID, toolArgString(params, "message_id", "id")),
		ThreadID:     firstNonEmptyToolString(toolArgString(params, "thread_id")),
		DraftID:      firstNonEmptyToolString(toolArgString(params, "draft_id")),
		AttachmentID: firstNonEmptyToolString(toolArgString(params, "attachment_id")),
		Limit:        toolArgInt(params, tc.Limit, "limit"),
		Cursor:       toolArgString(params, "cursor"),
		After:        toolArgString(params, "after"),
		Labels:       toolArgStringSlice(params, "labels"),
		AddLabels:    toolArgStringSlice(params, "add_labels"),
		RemoveLabels: toolArgStringSlice(params, "remove_labels"),
		To:           stringListFromToolArg(params, tc.To, "to"),
		CC:           stringListFromToolArg(params, tc.CC, "cc"),
		BCC:          stringListFromToolArg(params, "", "bcc"),
		Subject:      firstNonEmptyToolString(tc.Subject, toolArgString(params, "subject")),
		Text:         firstNonEmptyToolString(tc.Body, tc.Message, tc.Content, toolArgString(params, "text", "body", "message", "content")),
		HTML:         toolArgString(params, "html"),
		Username:     toolArgString(params, "username"),
		Domain:       toolArgString(params, "domain"),
		DisplayName:  toolArgString(params, "display_name"),
	}
	for _, item := range toolArgItemMaps(params, "attachments") {
		req.Attachments = append(req.Attachments, agentmail.AttachmentInput{
			Path:        strings.TrimSpace(asString(item["path"])),
			Filename:    strings.TrimSpace(asString(item["filename"])),
			ContentType: strings.TrimSpace(asString(item["content_type"])),
			Base64:      strings.TrimSpace(firstNonEmptyToolString(asString(item["base64"]), asString(item["content_base64"]))),
		})
	}
	return req
}

func stringListFromToolArg(params map[string]interface{}, fallback string, keys ...string) []string {
	values := toolArgStringSlice(params, keys...)
	if len(values) > 0 {
		return values
	}
	if strings.TrimSpace(fallback) == "" {
		return nil
	}
	parts := strings.Split(fallback, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func agentMailSuccess(data map[string]interface{}) string {
	if data == nil {
		data = map[string]interface{}{}
	}
	data["status"] = "success"
	raw, _ := json.Marshal(data)
	return "Tool Output: " + string(raw)
}

func agentMailError(message string) string {
	raw, _ := json.Marshal(map[string]interface{}{"status": "error", "message": message})
	return "Tool Output: " + string(raw)
}
