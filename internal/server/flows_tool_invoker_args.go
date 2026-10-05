package server

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/security"
)

// Per-tool argument handling of the flow tool invoker: types, secrets, logging and the
// Telegram text that must not be sent twice.

// flowNormalizeMQTTArgs gives mqtt_publish its qos as an int and retain as a bool. The
// agent's decoder (decodeMQTTArgs) reads qos with toolArgInt, which takes int and float64,
// and retain with toolArgBool, which takes only a bool: a "true" in text would publish
// without retain. The curated mqtt.publish node already passes int and bool; this keeps any
// other shape (a JSON number, text) from changing what is published. Values it cannot read
// are left for the tool, which falls back to the configured QoS. args is the invoker's own
// copy of the node's arguments.
func flowNormalizeMQTTArgs(args map[string]any) {
	switch q := args["qos"].(type) {
	case float64:
		if q == math.Trunc(q) && q >= 0 && q <= 2 {
			args["qos"] = int(q)
		}
	case int64:
		if q >= 0 && q <= 2 {
			args["qos"] = int(q)
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(q)); err == nil && n >= 0 && n <= 2 {
			args["qos"] = n
		}
	}
	if s, ok := args["retain"].(string); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(s)); err == nil {
			args["retain"] = b
		}
	}
}

// flowSecretMinBytes is the shortest secret the invoker redacts: shorter values (an ntfy
// topic such as "alerts") would cut ordinary words out of tool output. The global scrubber
// uses the same bound (security.RegisterSensitive).
const flowSecretMinBytes = 8

// flowCallSecrets lists the configured secrets a call of tool reads: the Telegram bot token,
// the ntfy topic and token, the Pushover keys, the Discord bot token, the Telnyx key, the Home
// Assistant token, the MQTT and email passwords and the Brave key, as far as the tool uses
// them. They come from the vault when the configuration is loaded.
func flowCallSecrets(cfg *config.Config, tool string) []string {
	notify := func() []string {
		return []string{cfg.Telegram.BotToken, cfg.Notifications.Ntfy.Topic, cfg.Notifications.Ntfy.Token,
			cfg.Notifications.Pushover.UserKey, cfg.Notifications.Pushover.AppToken, cfg.Discord.BotToken, cfg.Telnyx.APIKey}
	}
	switch tool {
	case "send_telegram", "send_notification":
		return notify()
	case "send_discord":
		return []string{cfg.Discord.BotToken}
	case "home_assistant":
		return []string{cfg.HomeAssistant.AccessToken}
	case "mqtt_publish":
		return []string{cfg.MQTT.Password}
	case "send_email":
		secrets := []string{cfg.Email.Password}
		for _, account := range cfg.EmailAccounts {
			secrets = append(secrets, account.Password)
		}
		return secrets
	case flows.BraveSearchTool:
		return []string{cfg.BraveSearch.APIKey}
	}
	return nil
}

// flowRegisterSecrets registers the call's secrets with the global output scrubber
// (security.Scrub, which agent.DispatchToolCallResult applies to the raw result) and returns
// those long enough to redact.
func flowRegisterSecrets(cfg *config.Config, tool string) []string {
	var secrets []string
	for _, s := range flowCallSecrets(cfg, tool) {
		if len(s) < flowSecretMinBytes {
			continue
		}
		security.RegisterSensitive(s)
		secrets = append(secrets, s)
	}
	return secrets
}

// flowRedactSecrets replaces the call's secrets in a tool output. The dispatcher already
// scrubbed the raw result; this pass also covers what the dispatcher added and keeps the
// guarantee independent of it. It is an exact match: cheap even for large outputs.
func flowRedactSecrets(output string, secrets []string) string {
	for _, s := range secrets {
		if strings.Contains(output, s) {
			output = strings.ReplaceAll(output, s, security.RedactedText(""))
		}
	}
	return output
}

// flowTextSentRetention is how long the invoker remembers that a Telegram text went out
// while its document failed: the longest run plus an hour.
var flowTextSentRetention = time.Duration(flows.MaxRunSecondsLimit)*time.Second + time.Hour

// flowTextSentKey identifies one node of one run, or "" when either id is missing.
func flowTextSentKey(req flows.ToolRequest) string {
	if req.RunID == "" || req.NodeID == "" {
		return ""
	}
	return req.RunID + "\x00" + req.NodeID
}

// textAlreadySent reports whether an earlier attempt of this node in this run sent the text
// of a Telegram message whose document then failed.
func (i *flowToolInvoker) textAlreadySent(key string) bool {
	if key == "" {
		return false
	}
	i.sentMu.Lock()
	defer i.sentMu.Unlock()
	_, sent := i.textSent[key]
	return sent
}

// rememberTextSent records such an attempt and forgets entries older than the retention.
func (i *flowToolInvoker) rememberTextSent(key string, now time.Time) {
	if key == "" {
		return
	}
	i.sentMu.Lock()
	defer i.sentMu.Unlock()
	if i.textSent == nil {
		i.textSent = map[string]time.Time{}
	}
	for k, at := range i.textSent {
		if now.Sub(at) > flowTextSentRetention {
			delete(i.textSent, k)
		}
	}
	i.textSent[key] = now
}

// flowTelegramTextSent reports whether a send_telegram answer says the text went out as a
// message of its own while the call failed (tools.SendTelegramFile sets "text_sent": true).
func flowTelegramTextSent(output string) bool {
	if len(output) > flowStatusParseLimit {
		return false
	}
	out, _ := flows.ParseToolOutput(output)
	sent, _ := out["text_sent"].(bool)
	status, _ := out["status"].(string)
	return sent && !strings.EqualFold(strings.TrimSpace(status), "success")
}

// flowLogHandler bounds what the dispatcher logs while it runs a flow's tool call: every
// string value is cut to flowLogRunes runes, a "url" value loses its user info, query and
// fragment (api_request logs the URL at Info, and a flow's URL can hold a key in its query),
// and values under argument-like keys (headers, body, args, params, payload) are left out.
type flowLogHandler struct{ inner slog.Handler }

// flowDispatchLogger wraps base (slog.Default when nil) in a flowLogHandler.
func flowDispatchLogger(base *slog.Logger) *slog.Logger {
	if base == nil {
		base = slog.Default()
	}
	return slog.New(flowLogHandler{inner: base.Handler()})
}

func (h flowLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h flowLogHandler) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, flowBoundRunes(r.Message, 2*flowLogRunes), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(flowLogAttr(a))
		return true
	})
	return h.inner.Handle(ctx, out)
}

func (h flowLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	bounded := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		bounded[i] = flowLogAttr(a)
	}
	return flowLogHandler{inner: h.inner.WithAttrs(bounded)}
}

func (h flowLogHandler) WithGroup(name string) slog.Handler {
	return flowLogHandler{inner: h.inner.WithGroup(name)}
}

// flowLogOmittedKeys are attribute keys whose values are tool arguments as a whole.
var flowLogOmittedKeys = map[string]bool{"headers": true, "header": true, "body": true, "args": true, "arguments": true,
	"params": true, "payload": true, "authorization": true}

func flowLogAttr(a slog.Attr) slog.Attr {
	key := strings.ToLower(a.Key)
	if flowLogOmittedKeys[key] {
		return slog.String(a.Key, "[omitted]")
	}
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		s := v.String()
		if key == "url" {
			s = flowRedactURL(s)
		}
		return slog.String(a.Key, flowBoundRunes(s, flowLogRunes))
	case slog.KindGroup:
		group := v.Group()
		bounded := make([]any, len(group))
		for i, g := range group {
			bounded[i] = flowLogAttr(g)
		}
		return slog.Group(a.Key, bounded...)
	case slog.KindAny:
		return slog.String(a.Key, flowBoundRunes(fmt.Sprint(v.Any()), flowLogRunes))
	}
	return slog.Attr{Key: a.Key, Value: v}
}

// flowRedactURL keeps scheme, host and path of a URL for the log.
func flowRedactURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "[url]"
	}
	query := u.RawQuery != "" || u.ForceQuery
	u.User, u.RawQuery, u.ForceQuery, u.Fragment, u.RawFragment = nil, "", false, "", ""
	if query {
		return u.String() + "?[redacted]"
	}
	return u.String()
}
