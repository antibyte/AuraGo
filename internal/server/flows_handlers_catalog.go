package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/security"
)

// flowsNodeTypes serves /node-types and /node-types/{type}/options/{param}.
//
// The palette (GET /node-types) is a hint, not a safety claim: the Effects and Risky of a
// NodeTypeInfo describe a node with default parameters (a generic tool node shows its
// worst case), so a node can do more than its palette entry says once its parameters are
// set. What a flow does is shown at publish time from the real parameters
// (flows.CollectEffects), and at run time each tool call passes the tool's own gates
// (read-only modes, permissions, allowed services); those are the guarantee.
//
// HEAD is served for the palette only: the answer of GET without the body.
func (s *Server) flowsNodeTypes(w http.ResponseWriter, r *http.Request, rest []string) {
	if r.Method != http.MethodGet && (r.Method != http.MethodHead || len(rest) != 0) {
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
// The zero value is ready to use; Server.flowNodeTypesCache holds the one of the API.
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
// current tag gets 304 without a body. HEAD gets the headers of GET (with the Content-Length
// of the plain body) and no body.
func (s *Server) serveFlowNodeTypes(w http.ResponseWriter, r *http.Request, lang string) {
	reg := s.Flows.Registry()
	gen := reg.Generation() // before the registry is described
	cfg := s.ConfigSnapshot()
	answer, ok := s.flowNodeTypesCache.get(gen, cfg, lang)
	if !ok {
		var err error
		if answer, err = s.flowNodeTypesCache.build(reg, lang); err != nil {
			s.Logger.Warn("The flow node catalog could not be encoded", "lang", lang, "error", flowsErrorText(err))
			flowsWriteEncodeFailure(w)
			return
		}
		s.flowNodeTypesCache.put(gen, cfg, s.ConfigSnapshot(), lang, answer)
	}
	h := w.Header()
	h.Set("Cache-Control", "private, no-cache")
	h.Set("ETag", answer.etag)
	if flowETagMatches(r.Header.Get("If-None-Match"), answer.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodHead {
		h.Set("Content-Length", strconv.Itoa(len(answer.body)))
		w.WriteHeader(http.StatusOK)
		return
	}
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
