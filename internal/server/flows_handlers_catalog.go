package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/llm/catalog"
	"aurago/internal/security"
	"aurago/internal/tools"
)

type flowOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
}

// flowsNodeTypes serves /node-types and /node-types/{type}/options/{param}.
//
// The palette (GET /node-types) is a hint, not a safety claim: the Effects and Risky of a
// NodeTypeInfo describe a node with default parameters (a generic tool node shows its
// worst case), so a node can do more than its palette entry says once its parameters are
// set. What a flow does is shown at publish time from the real parameters
// (flows.CollectEffects), and at run time each tool call passes the tool's own gates
// (read-only modes, permissions, allowed services); those are the guarantee.
func (s *Server) flowsNodeTypes(w http.ResponseWriter, r *http.Request, rest []string) {
	if r.Method != http.MethodGet {
		flowsMethodNotAllowed(w)
		return
	}
	lang := s.flowsLang(r)
	switch {
	case len(rest) == 0:
		s.serveFlowNodeTypes(w, r, lang)
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
		list, err := s.flowOptionList(r.Context(), source, lang)
		if err != nil {
			if ctxErr := r.Context().Err(); ctxErr != nil {
				s.flowsErrorFrom(w, r, ctxErr)
				return
			}
			// The cause can be a remote service's answer (Home Assistant's error text):
			// untrusted and of any length.
			msg := flowCapRunes(security.RedactSensitiveInfo(security.Scrub(err.Error())), flowOptionsErrorRunes)
			s.Logger.Debug("Flow parameter options are unavailable", "source", source, "error", msg)
			flowsError(w, http.StatusBadGateway, "FLOW_OPTIONS_UNAVAILABLE", msg)
			return
		}
		answer := map[string]any{"options": list.options}
		if list.truncated {
			answer["truncated"] = true
		}
		flowsJSON(w, http.StatusOK, answer)
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

// flowNodeTypesCacheLangs bounds the cached palette answers: one per language. That is at
// most the 16 that i18n.NormalizeLang returns (English included), plus one spare.
const flowNodeTypesCacheLangs = 17

// flowNodeTypesCache keeps the encoded GET /node-types answer and its ETag per language.
// DescribeNodeTypes runs every definition hook, and the answer of a full configuration is
// several hundred KB of JSON, so without the cache every palette open would describe and
// encode the whole registry again.
//
//   - Key: the registry's Generation, read before describing (see
//     flows.Registry.Generation), and the configuration snapshot (tool availability
//     follows it; saving the configuration swaps the pointer). Nothing else that an
//     answer holds changes at run time: translations, trigger samples and definitions are
//     fixed once registered.
//   - A put for another key drops every cached language. A put for an older generation,
//     or for a configuration that is no longer the current one, is not kept, so a slow
//     request cannot replace a newer answer with its stale one.
//   - Bound: one answer per language, at most flowNodeTypesCacheLangs.
//   - Locking: mu guards all fields and is never held across a describe or an encode.
//     Two requests that miss at the same time both build the answer; the later put wins.
//
// The zero value is ready to use; Server.flowNodeTypes holds the one of the API.
type flowNodeTypesCache struct {
	mu      sync.Mutex
	gen     uint64
	cfg     *config.Config
	answers map[string]flowNodeTypesAnswer
	builds  int // answers built since start; tests read it
}

// flowNodeTypesAnswer is an encoded palette answer and its entity tag.
type flowNodeTypesAnswer struct {
	body []byte
	etag string
}

// get returns the cached answer of lang for the key (gen, cfg).
func (c *flowNodeTypesCache) get(gen uint64, cfg *config.Config, lang string) (flowNodeTypesAnswer, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.answers == nil || c.gen != gen || c.cfg != cfg {
		return flowNodeTypesAnswer{}, false
	}
	a, ok := c.answers[lang]
	return a, ok
}

// build describes and encodes the palette of reg in lang (flowNodeTypesBody) and counts it.
func (c *flowNodeTypesCache) build(reg *flows.Registry, lang string) (flowNodeTypesAnswer, error) {
	c.mu.Lock()
	c.builds++
	c.mu.Unlock()
	return flowNodeTypesBody(reg, lang)
}

// put caches answer for lang under the key (gen, cfg), unless gen is older than the cached
// key or cfg is not the current configuration.
func (c *flowNodeTypesCache) put(gen uint64, cfg, current *config.Config, lang string, answer flowNodeTypesAnswer) {
	if cfg == nil || cfg != current {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.answers != nil && gen < c.gen {
		return
	}
	if c.answers == nil || gen != c.gen || cfg != c.cfg {
		c.gen, c.cfg, c.answers = gen, cfg, make(map[string]flowNodeTypesAnswer, 2)
	}
	if _, ok := c.answers[lang]; !ok && len(c.answers) >= flowNodeTypesCacheLangs {
		return
	}
	c.answers[lang] = answer
}

// flowNodeTypesBody encodes the palette answer {"node_types", "categories"} of reg in lang,
// with a final line break as flowsJSON writes it. Its entity tag is weak (W/"…": the first
// 16 bytes of the body's SHA-256, hex): the gzip middleware compresses the answer and keeps
// the header, and a strong tag would have to differ between the plain and the gzip form
// (RFC 9110, 8.8.3). If-None-Match compares weakly anyway, so a 304 works for both.
func flowNodeTypesBody(reg *flows.Registry, lang string) (flowNodeTypesAnswer, error) {
	tr := flowsTranslator(lang)
	infos := flows.DescribeNodeTypes(reg, tr)
	data, err := json.Marshal(map[string]any{"node_types": infos, "categories": flowCategories(infos, tr)})
	if err != nil {
		return flowNodeTypesAnswer{}, err
	}
	data = append(data, '\n')
	sum := sha256.Sum256(data)
	return flowNodeTypesAnswer{body: data, etag: `W/"` + hex.EncodeToString(sum[:16]) + `"`}, nil
}

// serveFlowNodeTypes answers GET /node-types from flowNodeTypesCache, with an ETag and
// "Cache-Control: private, no-cache" (a browser may keep the answer but must revalidate
// it; flowsJSON's no-store does not apply here). A request whose If-None-Match names the
// current tag gets 304 without a body.
func (s *Server) serveFlowNodeTypes(w http.ResponseWriter, r *http.Request, lang string) {
	reg := s.Flows.Registry()
	gen := reg.Generation() // before the registry is described
	cfg := s.ConfigSnapshot()
	answer, ok := s.flowNodeTypes.get(gen, cfg, lang)
	if !ok {
		var err error
		if answer, err = s.flowNodeTypes.build(reg, lang); err != nil {
			s.Logger.Warn("The flow node catalog could not be encoded", "lang", lang, "error", flowsErrorText(err))
			flowsWriteEncodeFailure(w)
			return
		}
		s.flowNodeTypes.put(gen, cfg, s.ConfigSnapshot(), lang, answer)
	}
	h := w.Header()
	h.Set("Cache-Control", "private, no-cache")
	h.Set("ETag", answer.etag)
	if flowETagMatches(r.Header.Get("If-None-Match"), answer.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(answer.body)
}

// flowETagMatches reports whether an If-None-Match header names etag. It compares weakly,
// as RFC 9110 prescribes for If-None-Match (a W/ prefix on either side is ignored), and "*"
// matches any tag.
func flowETagMatches(header, etag string) bool {
	if strings.TrimSpace(header) == "" {
		return false
	}
	want := strings.TrimPrefix(etag, "W/")
	for _, tag := range strings.Split(header, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "*" || strings.TrimPrefix(tag, "W/") == want {
			return true
		}
	}
	return false
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
		list, err := s.flowHAEntities(ctx, cfg)
		if err != nil {
			return flowOptionList{}, err
		}
		return flowBoundOptions(list), nil
	default:
		return flowOptionList{}, fmt.Errorf("unknown options source %q", source)
	}
	return flowBoundOptions(flowOptionList{options: out}), nil
}

// flowBoundOptions returns a copy of list with every label and hint bounded to
// flowOptionTextRunes runes. It never writes into list, whose options may be shared
// (flowHACache).
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
// flowProviderEntry accepts before it looks at credentials. ProviderEntry has no purpose
// field, but its type can rule chat out: media providers (image generation, vision-only,
// Agnes image and video models) and unknown types are left out, as are an entry without
// a model, Workers AI without an account id, an entry without an id (its value would be
// the default route's) and the reserved managed local provider (cfg.FindProvider refuses
// it). Credentials are not checked: a missing key is fixed in the configuration, not in the
// flow, and the run reports it (FLOW_AI_UNAVAILABLE). An embedding or speech model on a chat
// provider type cannot be told apart and fails at run time with the provider's error.
func flowChatProvider(p *config.ProviderEntry) bool {
	id := strings.TrimSpace(p.ID)
	if id == "" || strings.EqualFold(id, config.LocalLLMProviderID) {
		return false
	}
	if strings.TrimSpace(p.Type) == "" {
		return strings.TrimSpace(p.Model) != ""
	}
	if ok, _ := config.SpeechLabChatProviderEligibility(p); !ok {
		return false
	}
	return catalog.NormalizeProviderID(p.Type) != "workers-ai" || strings.TrimSpace(p.AccountID) != ""
}

// flowHACache keeps the last Home Assistant entity list (flowHAEntities) for
// flowHAEntitiesTTL, for one configuration snapshot. Only lists Home Assistant answered are
// kept; after an error the next request asks again. The cached options are shared and
// read-only. The zero value is ready to use; Server.flowHACache holds the one of the API.
type flowHACache struct {
	mu   sync.Mutex
	cfg  *config.Config
	at   time.Time
	list flowOptionList
	now  func() time.Time // nil means time.Now; tests set it
}

func (c *flowHACache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// get returns the cached list of cfg while it is younger than flowHAEntitiesTTL.
func (c *flowHACache) get(cfg *config.Config) (flowOptionList, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg == nil || c.cfg != cfg {
		return flowOptionList{}, false
	}
	if age := c.clock().Sub(c.at); age < 0 || age >= flowHAEntitiesTTL {
		return flowOptionList{}, false
	}
	return c.list, true
}

// put caches the list Home Assistant answered for cfg.
func (c *flowHACache) put(cfg *config.Config, list flowOptionList) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg, c.at, c.list = cfg, c.clock(), list
}

// flowHAEntities lists Home Assistant entities (empty when Home Assistant is not set up),
// sorted by entity id and cut to flowHAEntitiesMax. tools.HAGetStatesContext reads at most
// 10 MiB of states (readHTTPResponseBody); a larger answer is an error. The list is reused
// for flowHAEntitiesTTL (flowHACache); two requests that miss at the same time both ask.
func (s *Server) flowHAEntities(ctx context.Context, cfg *config.Config) (flowOptionList, error) {
	ha := cfg.HomeAssistant
	if !ha.Enabled || strings.TrimSpace(ha.URL) == "" || ha.AccessToken == "" {
		return flowOptionList{options: []flowOption{}}, nil
	}
	if list, ok := s.flowHACache.get(cfg); ok {
		return list, nil
	}
	ctx, cancel := context.WithTimeout(ctx, flowHAEntitiesTimeout)
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
	s.flowHACache.put(cfg, list)
	return list, nil
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
	var mode flows.ValidationMode
	switch body.Mode {
	case "", "draft":
		mode = flows.ModeDraft
	case "publish":
		mode = flows.ModePublish
	default:
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", `mode must be "draft" or "publish"`)
		return
	}
	doc, err := flows.ParseFlow(body.Doc)
	if err != nil {
		flowsDocumentError(w, err)
		return
	}
	issues := nonNilIssues(s.Flows.Validate(doc, mode))
	flowsJSON(w, http.StatusOK, map[string]any{"valid": !flows.HasErrors(issues), "issues": issues})
}
