package flows

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// ErrUnknownTemplate is the error of TemplateFlow for an id that is not a starter template.
var ErrUnknownTemplate = errors.New("unknown starter template")

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

// templateDef is a starter template: its gallery entry, and a builder for its nodes and
// edges. The builder gets text, which returns the translated text n (1 to N) of this
// template, and runs only for the template that is built, so building one template
// translates only its own texts.
type templateDef struct {
	info  TemplateInfo
	build func(text func(n int) string) ([]templateNode, []templateEdge)
}

func templateKey(id, part string) string { return "easydrag.template." + id + "." + part }

// templateUntrustedRunes is how much text of an untrusted source (a webhook body, the
// titles of a feed) a template puts into an ai.step prompt. The truncate filter counts
// runes and the node caps the prompt in bytes (maxAIPromptBytes, 256 KiB), so the limit
// must hold for the worst case of four bytes per rune: 40000 runes and the ellipsis are at
// most 160003 bytes, which leaves about 100 KB of the cap for the translated instruction
// text. (100000 runes could be 400 KB and fail the node with a large body.)
const templateUntrustedRunes = 40000

// boundedExpr is the expression that puts at most templateUntrustedRunes runes of an
// untrusted value into a prompt.
func boundedExpr(expr string) string {
	return fmt.Sprintf("{{%s | truncate(%d)}}", expr, templateUntrustedRunes)
}

// newTemplateInfo builds the gallery entry of a template with texts text_1 to text_N.
func newTemplateInfo(id string, texts int, categories ...string) TemplateInfo {
	ti := TemplateInfo{ID: id, NameKey: templateKey(id, "name"), DescriptionKey: templateKey(id, "description"), Categories: categories}
	for i := 1; i <= texts; i++ {
		ti.textKeys = append(ti.textKeys, templateKey(id, fmt.Sprintf("text_%d", i)))
	}
	return ti
}

// chainEdges connects the nodes with the given keys one after the other, from the default
// output port to the input port.
func chainEdges(keys ...string) []templateEdge {
	var edges []templateEdge
	for i := 1; i < len(keys); i++ {
		edges = append(edges, templateEdge{keys[i-1], PortOut, keys[i]})
	}
	return edges
}

// templateCol is the x position of the i-th column of nodes on the canvas.
func templateCol(i int) float64 { return 80 + float64(i)*300 }

// starterTemplates are the six starter templates in gallery order. Node keys are fixed
// English identifiers so that template strings can reference them in every language.
//
// A translated text is trusted template text. The language files of the editor
// (ui/lang/easydrag) ship with the repository, and some of their texts hold expressions on
// purpose: the reminder message reads the title and the time of the appointment, the budget
// message the amounts spent and allowed. The texts are therefore used as they are, as a
// node parameter, and an expression in one must read data the trigger or a node before it
// really produces, like any other. The validator does not check trigger fields. The
// hardening tests check the fixture tplShipped, which mirrors those texts of plan 1c,
// against the trigger samples; the language files themselves are created and checked by
// plan 1c. The name and the description of a template and the labels of its nodes are
// plain text.
var starterTemplates = []templateDef{
	{
		info: newTemplateInfo("ai_news_pdf_telegram", 4, "trigger", "web", "ai", "documents", "notify"),
		build: func(text func(int) string) ([]templateNode, []templateEdge) {
			return []templateNode{
				{"schedule", TypeTriggerSchedule, map[string]any{"mode": "weekdays", "time": "07:00"}, templateCol(0), 200},
				{"search", TypeWebSearch, map[string]any{"query": text(1), "count": 5.0}, templateCol(1), 200},
				{"summary", TypeAIStep, map[string]any{"prompt": text(2) +
					"\n\n{{search.results | pluck(\"snippet\") | join(\"\\n\")}}"}, templateCol(2), 200},
				{"pdf", TypePDFCreate, map[string]any{"title": text(3) +
					" {{run.started_at | date(\"DD.MM.YYYY\")}}", "content": "{{summary.text}}"}, templateCol(3), 200},
				{"telegram", TypeTelegram, map[string]any{"message": text(4), "file": "{{pdf.file}}"}, templateCol(4), 200},
			}, chainEdges("schedule", "search", "summary", "pdf", "telegram")
		},
	},
	{
		info: newTemplateInfo("webhook_summary_email", 2, "trigger", "ai", "notify"),
		build: func(text func(int) string) ([]templateNode, []templateEdge) {
			return []templateNode{
				{"hook", TypeTriggerWebhook, map[string]any{}, templateCol(0), 200},
				{"summary", TypeAIStep, map[string]any{"prompt": text(1) + "\n\n" + boundedExpr("trigger.data.raw")}, templateCol(1), 200},
				{"mail", TypeEmail, map[string]any{"to": "", "subject": text(2), "body": "{{summary.text}}"}, templateCol(2), 200},
			}, chainEdges("hook", "summary", "mail")
		},
	},
	{
		info: newTemplateInfo("appointment_reminder", 1, "trigger", "notify"),
		build: func(text func(int) string) ([]templateNode, []templateEdge) {
			return []templateNode{
				{"due", TypeTriggerPlanner, map[string]any{"event": "appointment_due"}, templateCol(0), 200},
				{"telegram", TypeTelegram, map[string]any{"message": text(1)}, templateCol(1), 200},
			}, chainEdges("due", "telegram")
		},
	},
	{
		info: newTemplateInfo("leaving_home", 0, "trigger", "logic", "smart_home"),
		build: func(func(int) string) ([]templateNode, []templateEdge) {
			nodes := []templateNode{
				{"presence", TypeTriggerHAState, map[string]any{"entity": "", "state_equals": "not_home"}, templateCol(0), 200},
				{"check", TypeIf, map[string]any{"condition": map[string]any{"match": "all", "rows": []any{
					map[string]any{"left": "{{trigger.data.new_state}}", "op": "eq", "right": "not_home"}}}}, templateCol(1), 200},
				{"lights", TypeHomeAssistant, map[string]any{"operation": "call_service", "entity": "", "service": "turn_off"}, templateCol(2), 140},
			}
			return nodes, []templateEdge{{"presence", PortOut, "check"}, {"check", PortTrue, "lights"}}
		},
	},
	{
		info: newTemplateInfo("budget_guard", 2, "trigger", "notify"),
		build: func(text func(int) string) ([]templateNode, []templateEdge) {
			return []templateNode{
				{"budget", TypeTriggerBudget, map[string]any{"event": "warning"}, templateCol(0), 200},
				{"push", TypePush, map[string]any{"channel": "all", "title": text(1), "message": text(2)}, templateCol(1), 200},
			}, chainEdges("budget", "push")
		},
	},
	{
		info: newTemplateInfo("rss_digest", 2, "trigger", "web", "ai", "notify"),
		build: func(text func(int) string) ([]templateNode, []templateEdge) {
			return []templateNode{
				{"schedule", TypeTriggerSchedule, map[string]any{"mode": "daily", "time": "18:00"}, templateCol(0), 200},
				{"feed", TypeWebRead, map[string]any{"url": "", "mode": "rss"}, templateCol(1), 200},
				{"summary", TypeAIStep, map[string]any{"prompt": text(1) + "\n\n" +
					boundedExpr(`feed.items | pluck("title") | join("\n")`)}, templateCol(2), 200},
				{"mail", TypeEmail, map[string]any{"to": "", "subject": text(2), "body": "{{summary.text}}"}, templateCol(3), 200},
			}, chainEdges("schedule", "feed", "summary", "mail")
		},
	},
}

// Templates lists the starter templates in gallery order. The result is a copy.
func Templates() []TemplateInfo {
	out := make([]TemplateInfo, len(starterTemplates))
	for i, def := range starterTemplates {
		out[i] = def.info
		out[i].Categories = slices.Clone(def.info.Categories)
		out[i].textKeys = slices.Clone(def.info.textKeys)
	}
	return out
}

// TemplateFlow builds a new flow document from a template. tr translates i18n keys; only
// the keys of the requested template and the labels of its nodes are asked for. A key
// without a translation (tr returns "" or the key) falls back to the template id for the
// name and the description, to the label's node key for a label and to the key itself for
// a text, so that no name or parameter is blank. An unknown id is ErrUnknownTemplate.
func TemplateFlow(id string, tr func(string) string) (*Flow, error) {
	if tr == nil {
		tr = func(k string) string { return k }
	}
	for _, def := range starterTemplates {
		if def.info.ID != id {
			continue
		}
		text := func(n int) string { return localize(tr, templateKey(id, fmt.Sprintf("text_%d", n)), "") }
		nodes, edges := def.build(text)
		f := &Flow{Schema: SchemaVersion, ID: NewFlowID(), Kind: KindFlow, Name: localize(tr, def.info.NameKey, id),
			Description: localize(tr, def.info.DescriptionKey, id)}
		ids := map[string]string{}
		for _, tn := range nodes {
			nodeID := NewNodeID()
			ids[tn.key] = nodeID
			labelKey := "easydrag.node." + strings.ReplaceAll(tn.typ, ".", "_") + ".label"
			f.Nodes = append(f.Nodes, Node{ID: nodeID, Key: tn.key, Type: tn.typ, TypeVersion: 1,
				Label: localize(tr, labelKey, tn.key), Position: Point{X: tn.x, Y: tn.y}, Params: cloneJSONMap(tn.params)})
		}
		for _, te := range edges {
			f.Edges = append(f.Edges, Edge{ID: NewEdgeID(), Source: PortRef{Node: ids[te.from], Port: te.port}, Target: PortRef{Node: ids[te.to], Port: PortIn}})
		}
		f.Normalize()
		return f, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnknownTemplate, quoteForError(id))
}

// CatalogI18nKeys lists every i18n key the catalog and the templates use (generic tool
// nodes have none). Plan 1c checks that ui/lang/easydrag/*.json contains all of them.
//
// The keys of the output fields are collected the way DescribeNodeTypes asks for them: from
// the fields of a sample node of the definition (FieldsOf, which is the static OutputFields
// when the definition has no OutputFieldsFunc), under a recover, so a hook that panics adds
// no key, as it adds no field to the description.
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
		var fields []FieldSpec
		if describeHook(def, "OutputFieldsFunc", func() { fields = def.FieldsOf(sampleNode(def)) }) {
			for _, f := range fields {
				add(f.DescriptionKey)
			}
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
