package agent

import (
	"aurago/internal/meshcore"
	"aurago/internal/prompts"
	"strings"
)

func appendMeshCoreNotice(cfg *RunConfig) func() {
	noop := func() {}
	if cfg == nil || cfg.Config == nil || !cfg.Config.MeshCore.Enabled || cfg.IsCoAgent || cfg.IsMission || cfg.IsMaintenance {
		return noop
	}
	switch strings.ToLower(strings.TrimSpace(cfg.MessageSource)) {
	case "web_chat", "telegram", "discord", "sms", "rocketchat", "agodesk_chat", "virtual_desktop_chat":
	default:
		return noop
	}
	m := meshcore.DefaultManager()
	if m == nil {
		return noop
	}
	notice, ids, err := m.PendingNotice()
	if err != nil || notice == "" {
		return noop
	}
	cfg.TrustedPromptAddenda = append(cfg.TrustedPromptAddenda, prompts.PromptAddendum{ID: "meshcore_inbox_notice", Text: notice})
	return func() {
		if err := m.MarkNotified(ids); err != nil && cfg.Logger != nil {
			cfg.Logger.Warn("MeshCore notice acknowledgement failed")
		}
	}
}
