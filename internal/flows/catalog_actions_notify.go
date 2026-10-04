package flows

import (
	"context"
	"strings"
)

// Notification node types.
const (
	TypeTelegram = "notify.telegram"
	TypeEmail    = "notify.email"
	TypePush     = "notify.push"
	TypeDiscord  = "notify.discord"
)

func registerNotifyNodes(reg *Registry, env CatalogEnv) error {
	for _, def := range []*NodeDef{telegramDef(env), emailDef(env), pushDef(env), discordDef(env)} {
		if err := reg.Register(def); err != nil {
			return err
		}
	}
	return nil
}

// notificationFailure reports an error when no channel of a send_notification-style
// answer sent the message. One sent channel is a success even when others failed
// (they are all in the results the node returns). Only the status "sent" counts as
// sent: an entry with any other status, or one that is not even an object, did not
// send. An answer with no results at all is not judged here (callers have checked the
// status already; the real tools always list their channels). The message is the
// channels' own text, cleaned and bounded by notifyText; it is not wrapped in quotes,
// so that a plain "chat not found" stays "chat not found".
func notificationFailure(out map[string]any) error {
	raw, present := out["results"]
	if !present || raw == nil {
		return nil
	}
	results, isList := notifyResults(out)
	if !isList {
		return NewNodeError("FLOW_NOTIFY_FAILED", "the notification tool returned a result list that cannot be read")
	}
	if len(results) == 0 {
		return nil
	}
	var failed []string
	for _, r := range results {
		if r.sent() {
			return nil
		}
		failed = append(failed, firstNonEmpty(r.detail, r.channel, r.status, "a channel failed"))
	}
	return &NodeError{Code: "FLOW_NOTIFY_FAILED", Message: truncateRunes(strings.Join(failed, "; "), maxToolMessageRunes)}
}

// requiredText is the trimmed text of a parameter, or FLOW_PARAM_INVALID with msg
// when it is blank. The notification nodes read their parameters with the stricter
// readers of catalog_notify_args.go; this one stays for nodes that only need a
// non-blank text.
func requiredText(in ExecInput, param, msg string) (string, error) {
	v := strings.TrimSpace(Stringify(in.Params[param]))
	if v == "" {
		return "", NewNodeError("FLOW_PARAM_INVALID", "%s", msg)
	}
	return v, nil
}

// notifyDef is the common part of the notification nodes. The message, the title, the
// subject and the body are not sensitive sinks: they carry the content to a person,
// which is the point of the node, as the content of doc.pdf_create does. The
// parameters that decide where a message goes or what is attached are. The outputs
// are made from the node's own parameters and the cleaned results of the tool, so
// they are trusted.
func notifyDef(typ, icon, tool string, env CatalogEnv) *NodeDef {
	def := actionDef(typ, "notify", icon, tool, env)
	def.Effects = []Effect{EffectSendsMessage}
	def.OutputFields = []FieldSpec{{Name: "sent", Type: "bool", Primary: true}}
	return def
}

// telegramDef defines notify.telegram. The real send_telegram sends to the one chat
// that is configured (telegram_user_id): there is no recipient parameter. The file is
// a sensitive sink, an untrusted path would post any workspace file to the chat.
//
// Known limit, for the tool invoker (plan 1c): today send_telegram reads only message,
// title and priority and drops file_path without a word, so a file is not sent. The
// invoker has to send it (the send_document tool takes a path) or refuse the call;
// the node cannot tell from the answer.
func telegramDef(env CatalogEnv) *NodeDef {
	def := notifyDef(TypeTelegram, "brand-telegram", "send_telegram", env)
	def.PrimaryInput = "message"
	def.Params = []ParamSpec{
		{Name: "message", Kind: ParamTextarea, LabelKey: "easydrag.param.message", Required: true, Templatable: true},
		{Name: "title", Kind: ParamText, LabelKey: "easydrag.param.title", Templatable: true},
		{Name: "file", Kind: ParamFile, LabelKey: "easydrag.param.attachment", Templatable: true, SensitiveSink: true},
	}
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		issues := literalIssueIfSet(n, "message", telegramMessage)
		issues = append(issues, literalIssueIfSet(n, "title", notifyTitle)...)
		return append(issues, literalIssueIfSet(n, "file", attachmentParam)...)
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		msg, err := telegramMessage(in.Params["message"])
		if err != nil {
			return ExecResult{}, err
		}
		title, err := notifyTitle(in.Params["title"])
		if err != nil {
			return ExecResult{}, err
		}
		file, err := attachmentParam(in.Params["file"])
		if err != nil {
			return ExecResult{}, err
		}
		if err := telegramFits(title, msg); err != nil {
			return ExecResult{}, err
		}
		args := map[string]any{"message": msg}
		setArg(args, "title", title)
		setArg(args, "file_path", file)
		out, err := callTool(ctx, in, "send_telegram", args)
		if err != nil {
			return ExecResult{}, err
		}
		if err := requireSuccess(out, "Telegram tool"); err != nil {
			return ExecResult{}, err
		}
		if err := notificationFailure(out); err != nil {
			return ExecResult{}, err
		}
		return ExecResult{Output: map[string]any{"sent": true}}, nil
	}
	return def
}

// emailDef defines notify.email. The recipients, the attachment and the account are
// sensitive sinks: untrusted data must not pick who gets a mail, which file goes
// with it, or which of the configured mailboxes it is sent from.
//
// send_email writes the recipients into the To header as they are, splits them at the
// commas, and sends one text part. Known limit, for the tool invoker (plan 1c): it
// reads no attachments (its schema has no such argument and the dispatcher drops
// them), so a file is not sent; the invoker has to implement it or refuse the call.
// Also, a transport error after the server took the mail (the answer is lost, QUIT
// fails) is reported as a failure, and a retry sends the mail again.
func emailDef(env CatalogEnv) *NodeDef {
	def := notifyDef(TypeEmail, "mail", "send_email", env)
	def.PrimaryInput = "body"
	def.Params = []ParamSpec{
		{Name: "to", Kind: ParamText, LabelKey: "easydrag.param.email_to", Required: true, Templatable: true, SensitiveSink: true},
		{Name: "subject", Kind: ParamText, LabelKey: "easydrag.param.email_subject", Templatable: true},
		{Name: "body", Kind: ParamTextarea, LabelKey: "easydrag.param.email_body", Required: true, Templatable: true},
		{Name: "attachment", Kind: ParamFile, LabelKey: "easydrag.param.attachment", Templatable: true, SensitiveSink: true},
		{Name: "account", Kind: ParamSelect, LabelKey: "easydrag.param.email_account", OptionsSource: "email_accounts", SensitiveSink: true},
	}
	def.OutputFields = append(def.OutputFields, FieldSpec{Name: "to", Type: "text"})
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		issues := literalIssueIfSet(n, "to", emailRecipients)
		issues = append(issues, literalIssueIfSet(n, "subject", emailSubject)...)
		issues = append(issues, literalIssueIfSet(n, "body", emailBody)...)
		issues = append(issues, literalIssueIfSet(n, "attachment", attachmentParam)...)
		return append(issues, literalIssueIfSet(n, "account", emailAccount)...)
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		to, err := emailRecipients(in.Params["to"])
		if err != nil {
			return ExecResult{}, err
		}
		subject, err := emailSubject(in.Params["subject"])
		if err != nil {
			return ExecResult{}, err
		}
		account, err := emailAccount(in.Params["account"])
		if err != nil {
			return ExecResult{}, err
		}
		body, err := emailBody(in.Params["body"])
		if err != nil {
			return ExecResult{}, err
		}
		attachment, err := attachmentParam(in.Params["attachment"])
		if err != nil {
			return ExecResult{}, err
		}
		args := map[string]any{"to": to, "body": body}
		setArg(args, "subject", subject)
		setArg(args, "account", account)
		if attachment != "" {
			args["attachments"] = []any{attachment}
		}
		out, err := callTool(ctx, in, "send_email", args)
		if err != nil {
			return ExecResult{}, err
		}
		// A refusal that came as plain text is not a sent mail.
		if err := requireSuccess(out, "email tool"); err != nil {
			return ExecResult{}, err
		}
		return ExecResult{Output: map[string]any{"sent": true, "to": to}}, nil
	}
	return def
}

// pushDef defines notify.push. The channel is a sensitive sink: it decides which of
// the configured channels (SMS included) the text goes to, and "all" is a broadcast.
// One channel that sent is a success even if others failed; every entry is in the
// results of the output.
func pushDef(env CatalogEnv) *NodeDef {
	def := notifyDef(TypePush, "bell", "send_notification", env)
	def.PrimaryInput = "message"
	def.Params = []ParamSpec{
		{Name: "channel", Kind: ParamSelect, LabelKey: "easydrag.param.push_channel", Default: "all", OptionsSource: "notification_channels", SensitiveSink: true},
		{Name: "title", Kind: ParamText, LabelKey: "easydrag.param.title", Templatable: true},
		{Name: "message", Kind: ParamTextarea, LabelKey: "easydrag.param.message", Required: true, Templatable: true},
		{Name: "priority", Kind: ParamSelect, LabelKey: "easydrag.param.priority", Default: "normal",
			Options: []Option{option("low", "priority_low"), option("normal", "priority_normal"), option("high", "priority_high"), option("critical", "priority_critical")}},
	}
	def.OutputFields = append(def.OutputFields, FieldSpec{Name: "results", Type: "list"})
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		issues := literalIssueIfSet(n, "channel", pushChannel)
		issues = append(issues, literalIssueIfSet(n, "title", notifyTitle)...)
		issues = append(issues, literalIssueIfSet(n, "message", pushMessage)...)
		return append(issues, choiceIssue(n, "priority", "normal", notifyPriorities)...)
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		msg, err := pushMessage(in.Params["message"])
		if err != nil {
			return ExecResult{}, err
		}
		channel, err := pushChannel(in.Params["channel"])
		if err != nil {
			return ExecResult{}, err
		}
		title, err := notifyTitle(in.Params["title"])
		if err != nil {
			return ExecResult{}, err
		}
		priority, ok := choiceParam(in.Params["priority"], "normal", notifyPriorities...)
		if !ok {
			return ExecResult{}, choiceError("priority", notifyPriorities)
		}
		args := map[string]any{"channel": channel, "message": msg, "priority": priority}
		setArg(args, "title", title)
		out, err := callTool(ctx, in, "send_notification", args)
		if err != nil {
			return ExecResult{}, err
		}
		if err := requireSuccess(out, "notification tool"); err != nil {
			return ExecResult{}, err
		}
		if err := notificationFailure(out); err != nil {
			return ExecResult{}, err
		}
		results := []any{}
		entries, _ := notifyResults(out)
		for _, r := range entries {
			entry := map[string]any{"channel": r.channel, "status": r.status}
			if r.detail != "" {
				entry["detail"] = r.detail
			}
			results = append(results, entry)
		}
		return ExecResult{Output: map[string]any{"sent": true, "results": results}}, nil
	}
	return def
}

// discordDef defines notify.discord. The channel id is a sensitive sink; without one
// the tool uses the channel that is configured as the default.
func discordDef(env CatalogEnv) *NodeDef {
	def := notifyDef(TypeDiscord, "brand-discord", "send_discord", env)
	def.PrimaryInput = "message"
	def.Params = []ParamSpec{
		{Name: "message", Kind: ParamTextarea, LabelKey: "easydrag.param.message", Required: true, Templatable: true},
		{Name: "channel_id", Kind: ParamText, LabelKey: "easydrag.param.discord_channel", SensitiveSink: true},
	}
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		issues := literalIssueIfSet(n, "message", discordMessage)
		return append(issues, literalIssueIfSet(n, "channel_id", discordChannelID)...)
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		msg, err := discordMessage(in.Params["message"])
		if err != nil {
			return ExecResult{}, err
		}
		channelID, err := discordChannelID(in.Params["channel_id"])
		if err != nil {
			return ExecResult{}, err
		}
		args := map[string]any{"message": msg}
		setArg(args, "channel_id", channelID)
		out, err := callTool(ctx, in, "send_discord", args)
		if err != nil {
			return ExecResult{}, err
		}
		if err := requireSuccess(out, "Discord tool"); err != nil {
			return ExecResult{}, err
		}
		return ExecResult{Output: map[string]any{"sent": true}}, nil
	}
	return def
}
