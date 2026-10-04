package flows

import (
	"fmt"
	"sort"
	"strings"
)

// TemplateInfo describes a starter template in the gallery.
type TemplateInfo struct {
	ID             string   `json:"id"`
	NameKey        string   `json:"name_key"`
	DescriptionKey string   `json:"description_key"`
	Categories     []string `json:"categories"`
	textKeys       []string
}

type templateNode struct {
	key    string
	typ    string
	params map[string]any
	x, y   float64
}

type templateEdge struct {
	from, port, to string
}

type templateSpec struct {
	info  TemplateInfo
	nodes []templateNode
	edges []templateEdge
}

func templateKey(id, part string) string { return "easydrag.template." + id + "." + part }

// templateUntrustedRunes is how much text of an untrusted source (a webhook body, the
// titles of a feed) a template puts into an ai.step prompt. The truncate filter counts
// runes and the node caps the prompt in bytes (maxAIPromptBytes, 256 KiB), so the limit
// must hold for the worst case of four bytes per rune: 40000 runes and the ellipsis are at
// most 160003 bytes, which leaves about 100 KB of the cap for the translated instruction
// text. (100000 runes could be 400 KB and fail the node with a large body.)
const templateUntrustedRunes = 40000

// templateSpecs builds the templates with texts translated by tr. Node keys are fixed
// English identifiers so that template strings can reference them in every language.
//
// A translated text is trusted template text. The language files of the editor
// (ui/lang/easydrag) ship with the repository, and some of their texts hold expressions on
// purpose: the reminder message reads {{trigger.data.title}}, the budget message reads
// {{trigger.data.percentage}}. The texts are therefore used as they are, as a node
// parameter, and an expression in one must read data the trigger or a node before it really
// produces, like any other. (The validator does not check trigger fields; the hardening
// tests check the two texts above against the trigger samples.) The name and the
// description of a template and the labels of its nodes are plain text.
func templateSpecs(tr func(string) string) []templateSpec {
	text := func(id string, n int) string { return tr(templateKey(id, fmt.Sprintf("text_%d", n))) }
	info := func(id string, texts int, categories ...string) TemplateInfo {
		ti := TemplateInfo{ID: id, NameKey: templateKey(id, "name"), DescriptionKey: templateKey(id, "description"), Categories: categories}
		for i := 1; i <= texts; i++ {
			ti.textKeys = append(ti.textKeys, templateKey(id, fmt.Sprintf("text_%d", i)))
		}
		return ti
	}
	chain := func(keys ...string) []templateEdge {
		var edges []templateEdge
		for i := 1; i < len(keys); i++ {
			edges = append(edges, templateEdge{keys[i-1], PortOut, keys[i]})
		}
		return edges
	}
	col := func(i int) float64 { return 80 + float64(i)*300 }
	// untrusted is the expression that puts a bounded part of an untrusted value into a prompt.
	untrusted := func(expr string) string {
		return fmt.Sprintf("{{%s | truncate(%d)}}", expr, templateUntrustedRunes)
	}
	return []templateSpec{
		{
			info: info("ai_news_pdf_telegram", 4, "trigger", "web", "ai", "documents", "notify"),
			nodes: []templateNode{
				{"schedule", TypeTriggerSchedule, map[string]any{"mode": "weekdays", "time": "07:00"}, col(0), 200},
				{"search", TypeWebSearch, map[string]any{"query": text("ai_news_pdf_telegram", 1), "count": 5.0}, col(1), 200},
				{"summary", TypeAIStep, map[string]any{"prompt": text("ai_news_pdf_telegram", 2) +
					"\n\n{{search.results | pluck(\"snippet\") | join(\"\\n\")}}"}, col(2), 200},
				{"pdf", TypePDFCreate, map[string]any{"title": text("ai_news_pdf_telegram", 3) +
					" {{run.started_at | date(\"DD.MM.YYYY\")}}", "content": "{{summary.text}}"}, col(3), 200},
				{"telegram", TypeTelegram, map[string]any{"message": text("ai_news_pdf_telegram", 4), "file": "{{pdf.file}}"}, col(4), 200},
			},
			edges: chain("schedule", "search", "summary", "pdf", "telegram"),
		},
		{
			info: info("webhook_summary_email", 2, "trigger", "ai", "notify"),
			nodes: []templateNode{
				{"hook", TypeTriggerWebhook, map[string]any{}, col(0), 200},
				{"summary", TypeAIStep, map[string]any{"prompt": text("webhook_summary_email", 1) + "\n\n" +
					untrusted("trigger.data.raw")}, col(1), 200},
				{"mail", TypeEmail, map[string]any{"to": "", "subject": text("webhook_summary_email", 2), "body": "{{summary.text}}"}, col(2), 200},
			},
			edges: chain("hook", "summary", "mail"),
		},
		{
			info: info("appointment_reminder", 1, "trigger", "notify"),
			nodes: []templateNode{
				{"due", TypeTriggerPlanner, map[string]any{"event": "appointment_due"}, col(0), 200},
				{"telegram", TypeTelegram, map[string]any{"message": text("appointment_reminder", 1)}, col(1), 200},
			},
			edges: chain("due", "telegram"),
		},
		{
			info: info("leaving_home", 0, "trigger", "logic", "smart_home"),
			nodes: []templateNode{
				{"presence", TypeTriggerHAState, map[string]any{"entity": "", "state_equals": "not_home"}, col(0), 200},
				{"check", TypeIf, map[string]any{"condition": map[string]any{"match": "all", "rows": []any{
					map[string]any{"left": "{{trigger.data.new_state}}", "op": "eq", "right": "not_home"}}}}, col(1), 200},
				{"lights", TypeHomeAssistant, map[string]any{"operation": "call_service", "entity": "", "service": "turn_off"}, col(2), 140},
			},
			edges: []templateEdge{{"presence", PortOut, "check"}, {"check", PortTrue, "lights"}},
		},
		{
			info: info("budget_guard", 2, "trigger", "notify"),
			nodes: []templateNode{
				{"budget", TypeTriggerBudget, map[string]any{"event": "warning"}, col(0), 200},
				{"push", TypePush, map[string]any{"channel": "all", "title": text("budget_guard", 1), "message": text("budget_guard", 2)}, col(1), 200},
			},
			edges: chain("budget", "push"),
		},
		{
			info: info("rss_digest", 2, "trigger", "web", "ai", "notify"),
			nodes: []templateNode{
				{"schedule", TypeTriggerSchedule, map[string]any{"mode": "daily", "time": "18:00"}, col(0), 200},
				{"feed", TypeWebRead, map[string]any{"url": "", "mode": "rss"}, col(1), 200},
				{"summary", TypeAIStep, map[string]any{"prompt": text("rss_digest", 1) + "\n\n" +
					untrusted(`feed.items | pluck("title") | join("\n")`)}, col(2), 200},
				{"mail", TypeEmail, map[string]any{"to": "", "subject": text("rss_digest", 2), "body": "{{summary.text}}"}, col(3), 200},
			},
			edges: chain("schedule", "feed", "summary", "mail"),
		},
	}
}

// Templates lists the starter templates.
func Templates() []TemplateInfo {
	specs := templateSpecs(func(k string) string { return k })
	out := make([]TemplateInfo, len(specs))
	for i, s := range specs {
		out[i] = s.info
	}
	return out
}

// TemplateFlow builds a new flow document from a template. tr translates i18n keys.
func TemplateFlow(id string, tr func(string) string) (*Flow, error) {
	if tr == nil {
		tr = func(k string) string { return k }
	}
	for _, spec := range templateSpecs(tr) {
		if spec.info.ID != id {
			continue
		}
		f := &Flow{Schema: SchemaVersion, ID: NewFlowID(), Kind: KindFlow, Name: tr(spec.info.NameKey),
			Description: tr(spec.info.DescriptionKey)}
		ids := map[string]string{}
		for _, tn := range spec.nodes {
			nodeID := NewNodeID()
			ids[tn.key] = nodeID
			labelKey := "easydrag.node." + strings.ReplaceAll(tn.typ, ".", "_") + ".label"
			f.Nodes = append(f.Nodes, Node{ID: nodeID, Key: tn.key, Type: tn.typ, TypeVersion: 1,
				Label: localize(tr, labelKey, tn.key), Position: Point{X: tn.x, Y: tn.y}, Params: cloneJSONMap(tn.params)})
		}
		for _, te := range spec.edges {
			f.Edges = append(f.Edges, Edge{ID: NewEdgeID(), Source: PortRef{Node: ids[te.from], Port: te.port}, Target: PortRef{Node: ids[te.to], Port: PortIn}})
		}
		f.Normalize()
		return f, nil
	}
	return nil, fmt.Errorf("unknown template %q", id)
}

// CatalogI18nKeys lists every i18n key the catalog and the templates use (generic tool
// nodes have none). Plan 1c checks that ui/lang/easydrag/*.json contains all of them.
func CatalogI18nKeys(reg *Registry) []string {
	if reg == nil {
		reg = NewRegistry()
	}
	seen := map[string]bool{}
	add := func(k string) {
		if k != "" {
			seen[k] = true
		}
	}
	var addParam func(p ParamSpec)
	addParam = func(p ParamSpec) {
		add(p.LabelKey)
		add(p.HelpKey)
		for _, o := range p.Options {
			add(o.LabelKey)
		}
		for _, f := range p.Fields {
			addParam(f)
		}
	}
	for _, def := range reg.All() {
		if isGenericDef(def) {
			continue
		}
		add(def.LabelKey)
		add(def.DescriptionKey)
		add(def.SummaryKey)
		for _, p := range def.Params {
			addParam(p)
		}
		for _, f := range def.OutputFields {
			add(f.DescriptionKey)
		}
	}
	for _, c := range CategoryOrder {
		add("easydrag.category." + c)
	}
	for _, info := range Templates() {
		add(info.NameKey)
		add(info.DescriptionKey)
		for _, k := range info.textKeys {
			add(k)
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
