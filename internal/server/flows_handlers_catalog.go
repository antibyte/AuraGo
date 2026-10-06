package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/tools"
)

type flowOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
}

// flowsNodeTypes serves /node-types and /node-types/{type}/options/{param}.
func (s *Server) flowsNodeTypes(w http.ResponseWriter, r *http.Request, rest []string) {
	if r.Method != http.MethodGet {
		flowsMethodNotAllowed(w)
		return
	}
	lang := s.flowsLang(r)
	tr := flowsTranslator(lang)
	switch {
	case len(rest) == 0:
		infos := flows.DescribeNodeTypes(s.Flows.Registry(), tr)
		flowsJSON(w, http.StatusOK, map[string]any{"node_types": infos, "categories": flowCategories(infos, tr)})
	case len(rest) == 3 && rest[1] == "options":
		def, ok := s.Flows.Registry().Lookup(rest[0])
		if !ok {
			flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown node type")
			return
		}
		source := ""
		for _, p := range def.Params {
			if p.Name == rest[2] {
				source = p.OptionsSource
			}
		}
		if source == "" {
			flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "the parameter has no dynamic options")
			return
		}
		opts, err := s.flowOptions(r.Context(), source, lang)
		if err != nil {
			flowsError(w, http.StatusBadGateway, "FLOW_OPTIONS_UNAVAILABLE", err.Error())
			return
		}
		flowsJSON(w, http.StatusOK, map[string]any{"options": opts})
	default:
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown node type route")
	}
}

// flowCategories lists the curated categories in order, then the generic tool categories.
func flowCategories(infos []flows.NodeTypeInfo, tr func(string) string) []map[string]string {
	out := make([]map[string]string, 0, len(flows.CategoryOrder)+8)
	for _, c := range flows.CategoryOrder {
		out = append(out, map[string]string{"id": c, "label": tr("easydrag.category." + c)})
	}
	seen := map[string]bool{}
	for _, info := range infos {
		id, isTool := strings.CutPrefix(info.Category, "tool:")
		if !isTool || seen[id] {
			continue
		}
		seen[id] = true
		key := "easydrag.tool_category." + id
		label := tr(key)
		if label == key {
			label = id
		}
		out = append(out, map[string]string{"id": info.Category, "label": label})
	}
	return out
}

// flowOptions returns the choices of a dynamic select parameter.
func (s *Server) flowOptions(ctx context.Context, source, lang string) ([]flowOption, error) {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return nil, errors.New("the configuration is not ready")
	}
	tr := flowsTranslator(lang)
	out := []flowOption{}
	switch source {
	case "webhooks":
		if s.WebhookManager != nil {
			for _, h := range s.WebhookManager.List() {
				label := h.Name
				if label == "" {
					label = h.Slug
				}
				out = append(out, flowOption{Value: h.ID, Label: label, Hint: h.Slug})
			}
		}
	case "email_accounts":
		out = append(out, flowOption{Value: "", Label: tr("easydrag.option.email_account_default")})
		for _, a := range cfg.EmailAccounts {
			if a.Disabled || a.ReadOnly {
				continue
			}
			label := a.Name
			if label == "" {
				label = a.FromAddress
			}
			out = append(out, flowOption{Value: a.ID, Label: label, Hint: a.FromAddress})
		}
	case "notification_channels":
		out = append(out, flowOption{Value: "all", Label: tr("easydrag.option.channel_all")},
			flowOption{Value: "push", Label: tr("easydrag.option.channel_push")})
		if cfg.Telegram.BotToken != "" && cfg.Telegram.UserID != 0 {
			out = append(out, flowOption{Value: "telegram", Label: "Telegram"})
		}
		if cfg.Discord.Enabled {
			out = append(out, flowOption{Value: "discord", Label: "Discord"})
		}
		if cfg.Notifications.Ntfy.Enabled {
			out = append(out, flowOption{Value: "ntfy", Label: "ntfy"})
		}
		if cfg.Notifications.Pushover.Enabled {
			out = append(out, flowOption{Value: "pushover", Label: "Pushover"})
		}
	case "ai_models":
		label := tr("easydrag.option.ai_model_default")
		if cfg.LLM.Model != "" {
			label += " (" + cfg.LLM.Model + ")"
		}
		out = append(out, flowOption{Value: "", Label: label})
		for _, p := range cfg.Providers {
			name := p.Name
			if name == "" {
				name = p.ID
			}
			out = append(out, flowOption{Value: p.ID, Label: name, Hint: p.Model})
		}
	case "missions":
		if s.MissionManagerV2 != nil {
			for _, m := range s.MissionManagerV2.List() {
				label := m.Name
				if label == "" {
					label = m.ID
				}
				out = append(out, flowOption{Value: m.ID, Label: label, Hint: string(m.ExecutionType)})
			}
		}
	case "ha_entities":
		return s.flowHAEntities(ctx, cfg)
	default:
		return nil, fmt.Errorf("unknown options source %q", source)
	}
	return out, nil
}

// flowHAEntities lists Home Assistant entities (empty when Home Assistant is not set up).
func (s *Server) flowHAEntities(ctx context.Context, cfg *config.Config) ([]flowOption, error) {
	ha := cfg.HomeAssistant
	if !ha.Enabled || strings.TrimSpace(ha.URL) == "" || ha.AccessToken == "" {
		return []flowOption{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	raw := tools.HAGetStatesContext(ctx, tools.HAConfig{URL: ha.URL, AccessToken: ha.AccessToken, ReadOnly: true}, "")
	var res struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		States  []struct {
			EntityID string `json:"entity_id"`
			State    string `json:"state"`
			Name     string `json:"friendly_name"`
		} `json:"states"`
	}
	if err := json.Unmarshal([]byte(raw), &res); err != nil || res.Status != "success" {
		msg := res.Message
		if msg == "" {
			msg = "Home Assistant did not answer"
		}
		return nil, errors.New(msg)
	}
	out := make([]flowOption, 0, len(res.States))
	for _, st := range res.States {
		label := st.Name
		if label == "" {
			label = st.EntityID
		}
		out = append(out, flowOption{Value: st.EntityID, Label: label, Hint: st.EntityID + " · " + st.State})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Value < out[j].Value })
	if len(out) > 2000 {
		out = out[:2000]
	}
	return out, nil
}

func (s *Server) flowsTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		flowsMethodNotAllowed(w)
		return
	}
	tr := flowsTranslator(s.flowsLang(r))
	list := []map[string]any{}
	for _, info := range flows.Templates() {
		list = append(list, map[string]any{"id": info.ID, "name": tr(info.NameKey),
			"description": tr(info.DescriptionKey), "categories": info.Categories})
	}
	flowsJSON(w, http.StatusOK, map[string]any{"templates": list})
}

func (s *Server) flowsValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		flowsMethodNotAllowed(w)
		return
	}
	var body struct {
		Doc  json.RawMessage `json:"doc"`
		Mode string          `json:"mode"`
	}
	if !flowsDecode(w, r, &body, flowsDocBodyLimit, false) {
		return
	}
	doc, err := flows.ParseFlow(body.Doc)
	if err != nil {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", err.Error())
		return
	}
	mode := flows.ModeDraft
	if body.Mode == "publish" {
		mode = flows.ModePublish
	}
	issues := nonNilIssues(s.Flows.Validate(doc, mode))
	flowsJSON(w, http.StatusOK, map[string]any{"valid": !flows.HasErrors(issues), "issues": issues})
}
