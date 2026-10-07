package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/sync/singleflight"

	"aurago/internal/config"
	"aurago/internal/tools"
)

type flowOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
}

// Options of dynamic select parameters (GET node-types/{type}/options/{param}).
const (
	// flowOptionTextRunes bounds the label and the hint of an option, the ellipsis of a cut
	// included. Names come from the configuration, from Mission Control (the agent names
	// missions) and from Home Assistant (integrations name entities), so they can be of any
	// length.
	flowOptionTextRunes = 120
	// flowOptionsErrorRunes bounds the reason of a FLOW_OPTIONS_UNAVAILABLE answer.
	flowOptionsErrorRunes = 200
	// flowHAEntitiesMax caps the Home Assistant entities of one answer (the first ones by
	// entity id); a longer list is cut and the answer says "truncated": true.
	flowHAEntitiesMax = 2000
	// flowHAEntitiesTTL is how long an entity list is reused for the same configuration
	// snapshot, so a flow with several Home Assistant selects asks Home Assistant once.
	flowHAEntitiesTTL = 30 * time.Second
	// flowHAEntitiesErrorTTL is how long a failed Home Assistant request is reused, so an
	// unreachable Home Assistant is not asked (and waited for) once per select.
	flowHAEntitiesErrorTTL = 5 * time.Second
	// flowHAEntitiesTimeout bounds one request to Home Assistant.
	flowHAEntitiesTimeout = 10 * time.Second
)

// flowOptionList is the answer of an options route: the choices, and whether the list
// was cut.
type flowOptionList struct {
	options   []flowOption
	truncated bool
}

// flowOptions returns the choices of a dynamic select parameter (flowOptionList without
// the truncated flag).
func (s *Server) flowOptions(ctx context.Context, source, lang string) ([]flowOption, error) {
	list, err := s.flowOptionList(ctx, source, lang)
	return list.options, err
}

// flowOptionList returns the choices of a dynamic select parameter from the current
// configuration, every label and hint bounded to flowOptionTextRunes runes. Each source
// offers only values its node can use:
//
//   - webhooks: every webhook, as Mission Control's webhook picker lists them; the hint of a
//     disabled one says so (its trigger fires only once the webhook is enabled again).
//   - email_accounts: the default account, then the accounts that may send (neither
//     disabled nor read-only).
//   - notification_channels: "all" and "push", then the channels tools.SendNotification
//     sends to: Telegram with a bot token and a user, Discord when it is enabled, not
//     read-only and has a default channel (SendNotification refuses the others), ntfy and
//     Pushover when enabled.
//   - ai_models: the default route (flows.ai_provider, else the main model), then the
//     providers that can answer an ai.step (flowChatProvider).
//   - missions: every Mission Control mission; the hint is its execution type ("flow",
//     "manual", "scheduled", "triggered"), "agent" for an older agent mission without one.
//     A flow's own mission is offered too: publishing refuses it as its own trigger.
//   - ha_entities: flowHAEntities.
func (s *Server) flowOptionList(ctx context.Context, source, lang string) (flowOptionList, error) {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return flowOptionList{}, errors.New("the configuration is not ready")
	}
	tr := flowsTranslator(lang)
	out := []flowOption{}
	switch source {
	case "webhooks":
		if s.WebhookManager != nil {
			disabled := tr("easydrag.option.webhook_disabled")
			for _, h := range s.WebhookManager.List() {
				label := h.Name
				if label == "" {
					label = h.Slug
				}
				hint := h.Slug
				if !h.Enabled {
					hint = strings.TrimPrefix(hint+" · "+disabled, " · ")
				}
				out = append(out, flowOption{Value: h.ID, Label: label, Hint: hint})
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
		if cfg.Discord.Enabled && !cfg.Discord.ReadOnly && strings.TrimSpace(cfg.Discord.DefaultChannelID) != "" {
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
		if model := flowDefaultAIModel(cfg); model != "" {
			label += " (" + model + ")"
		}
		out = append(out, flowOption{Value: "", Label: label})
		for i := range cfg.Providers {
			p := &cfg.Providers[i]
			if !flowChatProvider(p) {
				continue
			}
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
				kind := string(m.ExecutionType)
				if kind == "" {
					kind = "agent"
				}
				out = append(out, flowOption{Value: m.ID, Label: label, Hint: kind})
			}
		}
	case "ha_entities":
		// Bounded once, before it was cached; the options are shared and read-only.
		return s.flowHAEntities(ctx, cfg)
	default:
		return flowOptionList{}, fmt.Errorf("unknown options source %q", source)
	}
	return flowBoundOptions(flowOptionList{options: out}), nil
}

// flowBoundOptions returns a copy of list with every label and hint bounded to
// flowOptionTextRunes runes. It never writes into list.
func flowBoundOptions(list flowOptionList) flowOptionList {
	out := make([]flowOption, len(list.options))
	for i, o := range list.options {
		out[i] = flowOption{Value: o.Value, Label: flowOptionText(o.Label), Hint: flowOptionText(o.Hint)}
	}
	return flowOptionList{options: out, truncated: list.truncated}
}

// flowOptionText bounds an option text to flowOptionTextRunes runes (flowCapRunes).
func flowOptionText(text string) string {
	return flowCapRunes(text, flowOptionTextRunes)
}

// flowCapRunes cuts text to at most maxRunes runes, the ellipsis that marks a cut included
// (flowBoundRunes adds it after maxRunes runes), so a capped text is never longer than
// maxRunes and capping it again changes nothing.
func flowCapRunes(text string, maxRunes int) string {
	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	return flowBoundRunes(text, maxRunes-1)
}

// flowDefaultAIModel names the model an ai.step without a model uses (flowLLM.route): the
// model of the flows.ai_provider entry when one is set ("" when it names no entry), else
// the main model. It reads cfg.Providers directly, because cfg.FindProvider writes into the
// configuration for one synthetic id.
func flowDefaultAIModel(cfg *config.Config) string {
	id := strings.TrimSpace(cfg.Flows.AIProvider)
	if id == "" {
		return cfg.LLM.Model
	}
	for i := range cfg.Providers {
		if cfg.Providers[i].ID == id {
			return cfg.Providers[i].Model
		}
	}
	return ""
}

// flowChatProvider reports whether a provider entry can answer an ai.step: what
// flowProviderEntry accepts before it looks at credentials (flowChatProviderEntryOK, the
// same function). ProviderEntry has no purpose field, but its type can rule chat out:
// media providers (image generation, vision-only, Agnes image and video models) and
// unknown types are left out. Credentials are not checked: a missing key is fixed in the
// configuration, not in the flow, and the run reports it (FLOW_AI_UNAVAILABLE). An
// embedding or speech model on a chat provider type cannot be told apart and fails at run
// time with the provider's error.
func flowChatProvider(p *config.ProviderEntry) bool {
	ok, _ := flowChatProviderEntryOK(p)
	return ok
}

// flowHACache keeps the last Home Assistant entity answer (flowHAEntities) of one
// configuration snapshot: a list for flowHAEntitiesTTL, an error for flowHAEntitiesErrorTTL.
// The cached options are bounded (flowBoundOptions), shared and read-only. fetches coalesces
// the requests that miss at the same time into one Home Assistant request per configuration
// snapshot. The zero value is ready to use; Server.flowHACache holds the one of the API.
type flowHACache struct {
	mu      sync.Mutex
	cfg     *config.Config
	at      time.Time
	list    flowOptionList
	err     error
	now     func() time.Time // nil means time.Now; tests set it
	fetches singleflight.Group
}

func (c *flowHACache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// get returns the cached answer of cfg (a list, or the error of the last request) while it
// is younger than its time to live; ok is false on a miss.
func (c *flowHACache) get(cfg *config.Config) (list flowOptionList, err error, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg == nil || c.cfg != cfg {
		return flowOptionList{}, nil, false
	}
	ttl := flowHAEntitiesTTL
	if c.err != nil {
		ttl = flowHAEntitiesErrorTTL
	}
	if age := c.clock().Sub(c.at); age < 0 || age >= ttl {
		return flowOptionList{}, nil, false
	}
	return c.list, c.err, true
}

// put caches the answer Home Assistant gave for cfg, unless cfg is no longer the current
// configuration (as flowNodeTypesCache.put does).
func (c *flowHACache) put(cfg, current *config.Config, list flowOptionList, err error) {
	if cfg == nil || cfg != current {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg, c.at, c.list, c.err = cfg, c.clock(), list, err
}

// flowHAEntities lists Home Assistant entities (empty when Home Assistant is not set up),
// sorted by entity id, cut to flowHAEntitiesMax and bounded (flowBoundOptions).
// tools.HAGetStatesContext reads at most 10 MiB of states (readHTTPResponseBody); a larger
// answer is an error. Answers are reused (flowHACache). Requests that miss at the same time
// share one Home Assistant request, which is detached from the request that started it
// (context.WithoutCancel, bounded by flowHAEntitiesTimeout), so a caller that goes away does
// not fail the others; such a caller gets its own context's error at once.
func (s *Server) flowHAEntities(ctx context.Context, cfg *config.Config) (flowOptionList, error) {
	ha := cfg.HomeAssistant
	if !ha.Enabled || strings.TrimSpace(ha.URL) == "" || ha.AccessToken == "" {
		return flowOptionList{options: []flowOption{}}, nil
	}
	if list, err, ok := s.flowHACache.get(cfg); ok {
		return list, err
	}
	detached := context.WithoutCancel(ctx)
	results := s.flowHACache.fetches.DoChan(fmt.Sprintf("%p", cfg), func() (any, error) {
		// A request that finished between the miss above and this call has filled the cache.
		if list, err, ok := s.flowHACache.get(cfg); ok {
			return list, err
		}
		fetchCtx, cancel := context.WithTimeout(detached, flowHAEntitiesTimeout)
		defer cancel()
		list, err := fetchFlowHAEntities(fetchCtx, cfg)
		s.flowHACache.put(cfg, s.ConfigSnapshot(), list, err)
		return list, err
	})
	select {
	case <-ctx.Done():
		return flowOptionList{}, ctx.Err()
	case res := <-results:
		list, _ := res.Val.(flowOptionList)
		return list, res.Err
	}
}

// fetchFlowHAEntities asks Home Assistant for its entities (see flowHAEntities).
func fetchFlowHAEntities(ctx context.Context, cfg *config.Config) (flowOptionList, error) {
	ha := cfg.HomeAssistant
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
		return flowOptionList{}, errors.New(msg)
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
	list := flowOptionList{options: out}
	if len(out) > flowHAEntitiesMax {
		list = flowOptionList{options: out[:flowHAEntitiesMax], truncated: true}
	}
	return flowBoundOptions(list), nil
}
