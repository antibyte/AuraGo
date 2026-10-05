package server

import (
	"math"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/security"
)

// Per-tool argument handling of the flow tool invoker: types, secrets and the Telegram text
// that must not be sent twice. The log handler is in flows_tool_invoker_log.go.

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

// flowCallSecrets lists the configured secrets a call of tool reads. credentials are true
// credentials (the Telegram and Discord bot tokens, the ntfy token, the Pushover keys, the
// Telnyx key, the Home Assistant token, the MQTT and email passwords and the Brave key);
// private are values that are only private, not credentials (the ntfy topic). All come from
// the configuration (the vault fills the credentials when it is loaded).
func flowCallSecrets(cfg *config.Config, tool string) (credentials, private []string) {
	switch tool {
	case "send_telegram", "send_notification":
		return []string{cfg.Telegram.BotToken, cfg.Notifications.Ntfy.Token, cfg.Notifications.Pushover.UserKey,
			cfg.Notifications.Pushover.AppToken, cfg.Discord.BotToken, cfg.Telnyx.APIKey}, []string{cfg.Notifications.Ntfy.Topic}
	case "send_discord":
		return []string{cfg.Discord.BotToken}, nil
	case "home_assistant":
		return []string{cfg.HomeAssistant.AccessToken}, nil
	case "mqtt_publish":
		return []string{cfg.MQTT.Password}, nil
	case "send_email":
		credentials := []string{cfg.Email.Password}
		for _, account := range cfg.EmailAccounts {
			credentials = append(credentials, account.Password)
		}
		return credentials, nil
	case flows.BraveSearchTool:
		return []string{cfg.BraveSearch.APIKey}, nil
	}
	return nil, nil
}

// flowRegisterSecrets registers the call's secrets with the output scrubber (security.Scrub,
// which agent.DispatchToolCallResult applies to the raw result) and returns those long enough
// to redact, with the release of the scoped ones. Credentials are registered for good, like
// every other integration registers its credentials. Private values that are no credentials
// (the ntfy topic) are registered only for the call (security.RegisterScopedSensitiveExact),
// so ordinary values do not pile up in the process-wide scrubber.
func flowRegisterSecrets(cfg *config.Config, tool string) (secrets []string, release func()) {
	credentials, private := flowCallSecrets(cfg, tool)
	var releases []func()
	for _, s := range credentials {
		if len(s) >= flowSecretMinBytes {
			security.RegisterSensitive(s)
			secrets = append(secrets, s)
		}
	}
	for _, s := range private {
		if len(s) >= flowSecretMinBytes {
			releases = append(releases, security.RegisterScopedSensitiveExact(s))
			secrets = append(secrets, s)
		}
	}
	return secrets, func() {
		for _, r := range releases {
			r()
		}
	}
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
