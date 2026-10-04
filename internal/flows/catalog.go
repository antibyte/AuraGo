package flows

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sort"
)

// CategoryOrder is the palette order of the curated categories; generic tool
// categories ("tool:<name>") follow alphabetically.
var CategoryOrder = []string{"trigger", "logic", "ai", "web", "documents", "notify", "smart_home", "planner"}

// RegisterCatalog registers all phase-1 node types: logic, triggers, AI and curated actions.
func RegisterCatalog(reg *Registry, env CatalogEnv) error {
	if err := RegisterLogicNodes(reg); err != nil {
		return err
	}
	if err := RegisterTriggerNodes(reg); err != nil {
		return err
	}
	if err := RegisterAINodes(reg); err != nil {
		return err
	}
	return RegisterActionNodes(reg, env)
}

// NodeTypeInfo is the editor's view of a node type (labels already translated).
//
// A NodeTypeInfo shares no memory with the definition it describes: the slices, maps
// and pointers in it are copies, so a caller may modify it freely.
type NodeTypeInfo struct {
	Type           string         `json:"type"`
	Version        int            `json:"version"`
	Category       string         `json:"category"`
	Icon           string         `json:"icon,omitempty"`
	Color          string         `json:"color,omitempty"`
	Label          string         `json:"label"`
	Description    string         `json:"description,omitempty"`
	Summary        string         `json:"summary,omitempty"`
	Trigger        bool           `json:"trigger,omitempty"`
	Inputs         []string       `json:"inputs"`
	Outputs        []string       `json:"outputs"`
	DynamicOutputs string         `json:"dynamic_outputs,omitempty"`
	DynamicFields  string         `json:"dynamic_fields,omitempty"`
	Params         []ParamInfo    `json:"params"`
	OutputFields   []FieldInfo    `json:"output_fields,omitempty"`
	PrimaryInput   string         `json:"primary_input,omitempty"`
	Tool           string         `json:"tool,omitempty"`
	Effects        []Effect       `json:"effects,omitempty"`
	Risky          bool           `json:"risky,omitempty"`
	Untrusted      bool           `json:"untrusted,omitempty"`
	Availability   Availability   `json:"availability"`
	Sample         map[string]any `json:"sample,omitempty"`
}

// ParamInfo is a translated parameter description.
type ParamInfo struct {
	Name          string       `json:"name"`
	Kind          string       `json:"kind"`
	Label         string       `json:"label"`
	Help          string       `json:"help,omitempty"`
	Required      bool         `json:"required,omitempty"`
	Templatable   bool         `json:"templatable,omitempty"`
	Default       any          `json:"default,omitempty"`
	Options       []OptionInfo `json:"options,omitempty"`
	OptionsSource string       `json:"options_source,omitempty"`
	VisibleIf     *Visibility  `json:"visible_if,omitempty"`
	SensitiveSink bool         `json:"sensitive_sink,omitempty"`
	Fields        []ParamInfo  `json:"fields,omitempty"`
}

// OptionInfo is a translated option.
type OptionInfo struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// FieldInfo is a translated output field.
type FieldInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Primary     bool   `json:"primary,omitempty"`
}

// dynamicOutputs and dynamicFields tell the editor which parameter drives ports and
// fields. The value names the driving parameter.
//
//   - logic.switch: "cases" is a list, and every case adds a port.
//   - ai.step: "fields" is a list, and every usable entry is an output field.
//   - logic.merge: "mode" is a segmented value, not a list. The fields of a merge node
//     depend on it, so the editor must compute them from the mode: "append" outputs
//     items (list, the primary field) and count (number); every other mode, wait_all
//     (the default) included, declares no field, because it outputs one entry per
//     upstream node key and those are only known from the connections. The
//     OutputFields of the merge's NodeTypeInfo describe the default mode, which is none.
var (
	dynamicOutputs = map[string]string{TypeSwitch: "cases"}
	dynamicFields  = map[string]string{TypeAIStep: "fields", TypeMerge: "mode"}
)

// DescribeNodeTypes describes every registered node type for the editor, ordered by
// CategoryOrder (generic categories last, alphabetically) and then by label. Equal
// labels keep the order of Registry.All, which is sorted by type, so the result is the
// same on every call (the sort is stable; TestDescribeNodeTypesOrderIsStable pins it).
// tr translates i18n keys and returns the key itself when it has no translation.
//
// The description calls definition hooks (ports, effects, fields, availability and
// the trigger sample) on a sample node. Hooks are not trusted: a hook that panics is
// logged and replaced by a safe fallback, so one faulty definition cannot take the
// whole palette down. See describeNodeType for the fallbacks.
func DescribeNodeTypes(reg *Registry, tr func(key string) string) []NodeTypeInfo {
	if reg == nil {
		return []NodeTypeInfo{}
	}
	if tr == nil {
		tr = func(key string) string { return key }
	}
	defs := reg.All()
	out := make([]NodeTypeInfo, 0, len(defs))
	for _, def := range defs {
		out = append(out, describeNodeType(def, tr))
	}
	sort.SliceStable(out, func(i, j int) bool {
		ci, cj := categoryRank(out[i].Category), categoryRank(out[j].Category)
		if ci != cj {
			return ci < cj
		}
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Label < out[j].Label
	})
	return out
}

func categoryRank(category string) int {
	for i, c := range CategoryOrder {
		if c == category {
			return i
		}
	}
	return len(CategoryOrder)
}

// localize returns the translation of key; without one it returns raw, and without raw
// the key itself, so missing translations stay visible instead of rendering empty.
func localize(tr func(string) string, key, raw string) string {
	if key != "" {
		if v := tr(key); v != "" && v != key {
			return v
		}
	}
	if raw != "" {
		return raw
	}
	return key
}

// hookFailedAvailability is what the palette shows for a node type whose availability
// check panicked. The node cannot be used (blocked, not "needs setup": no setting fixes
// a crash), and the reason is fixed text: the panic value stays in the log.
var hookFailedAvailability = Availability{State: BlockedState, Reason: "the availability check of this node type failed"}

// describeHook calls fn, a call into a hook of def, and reports whether it returned
// normally. A panic is logged (type, hook, bounded panic value) and reported as false;
// the caller then uses its fallback.
func describeHook(def *NodeDef, hook string, fn func()) bool {
	p := catchPanic(fn)
	if p == nil {
		return true
	}
	slog.Warn("flows: a node definition hook panicked while describing the node type; using a fallback",
		"type", def.Type, "hook", hook, "panic", truncateForError(fmt.Sprint(p)))
	return false
}

// sampleParams returns the parameters of a sample node of def: the parameter defaults,
// and nothing else. Every call returns copies of the defaults, so a hook that modifies
// its node cannot change the definition, nor what the next hook sees.
func sampleParams(def *NodeDef) map[string]any {
	params := withDefaults(def, map[string]any{})
	for k, v := range params {
		params[k] = cloneParamValue(v)
	}
	return params
}

// sampleNode is the node the description hands to the hooks of def.
func sampleNode(def *NodeDef) *Node {
	return &Node{Type: def.Type, Params: sampleParams(def)}
}

// effectsSampleNode is the node whose effects the palette shows. It is sampleNode,
// except that a generic tool node is shown without its operation: whatever default the
// operation parameter had, the effects of the palette entry must be what any operation
// can do (the worst case, which makes a tool that can delete or run code risky), not
// those of one preselected operation. Generic parameters carry no defaults today, so
// this only keeps that from changing silently.
func effectsSampleNode(def *NodeDef) *Node {
	n := sampleNode(def)
	if isGenericDef(def) {
		if op, _ := genericOperation(def.Params); op != "" {
			delete(n.Params, op)
		}
	}
	return n
}

// describeNodeType describes one node type. Hooks run on sample nodes through
// describeHook; when one panics:
//
//   - AvailabilityFunc: the node is blocked (hookFailedAvailability), so the palette does
//     not offer it as usable;
//   - EffectsFunc: the node counts as risky (fail closed) and lists no effects;
//   - OutputsFunc: the static ports of the definition (Outputs, or "out");
//   - OutputFieldsFunc and the trigger sample: none.
func describeNodeType(def *NodeDef, tr func(string) string) NodeTypeInfo {
	info := NodeTypeInfo{
		Type: def.Type, Version: def.Version, Category: def.Category, Icon: def.Icon, Color: def.Color,
		Label:          firstNonEmpty(localize(tr, def.LabelKey, def.Label), def.Type),
		Description:    localize(tr, def.DescriptionKey, def.Description),
		Summary:        localize(tr, def.SummaryKey, ""),
		Trigger:        def.Trigger,
		Inputs:         nonNilStrings(slices.Clone(def.InputPorts())),
		DynamicOutputs: dynamicOutputs[def.Type],
		DynamicFields:  dynamicFields[def.Type],
		Params:         []ParamInfo{},
		PrimaryInput:   def.PrimaryInput,
		Tool:           def.Tool,
		Untrusted:      def.UntrustedOutput,
		Availability:   hookFailedAvailability,
	}

	var outputs []string
	if !describeHook(def, "OutputsFunc", func() { outputs = def.OutputPorts(sampleNode(def)) }) {
		outputs = def.OutputPorts(nil)
	}
	info.Outputs = nonNilStrings(outputs)

	var effects []Effect
	if describeHook(def, "EffectsFunc", func() { effects = def.EffectsOf(effectsSampleNode(def)) }) {
		info.Effects = slices.Clone(effects)
		info.Risky = IsRisky(effects)
	} else {
		info.Risky = true
	}

	var availability Availability
	if describeHook(def, "AvailabilityFunc", func() { availability = def.Availability() }) {
		info.Availability = availability
	}

	for _, p := range def.Params {
		info.Params = append(info.Params, describeParam(p, tr))
	}

	var fields []FieldSpec
	if describeHook(def, "OutputFieldsFunc", func() { fields = def.FieldsOf(sampleNode(def)) }) {
		for _, f := range fields {
			info.OutputFields = append(info.OutputFields, FieldInfo{Name: f.Name, Type: f.Type,
				Description: localize(tr, f.DescriptionKey, ""), Primary: f.Primary})
		}
	}

	if def.Trigger {
		var sample map[string]any
		if describeHook(def, "TriggerSample", func() { sample = TriggerSample(sampleNode(def)) }) {
			info.Sample = sample
		}
	}
	return info
}

func describeParam(p ParamSpec, tr func(string) string) ParamInfo {
	info := ParamInfo{
		Name: p.Name, Kind: p.Kind, Label: firstNonEmpty(localize(tr, p.LabelKey, p.Label), p.Name),
		Help: localize(tr, p.HelpKey, p.Help), Required: p.Required, Templatable: p.Templatable,
		Default: cloneParamValue(p.Default), OptionsSource: p.OptionsSource, VisibleIf: cloneVisibility(p.VisibleIf),
		SensitiveSink: p.SensitiveSink,
	}
	for _, o := range p.Options {
		info.Options = append(info.Options, OptionInfo{Value: o.Value, Label: firstNonEmpty(localize(tr, o.LabelKey, o.Label), o.Value)})
	}
	for _, f := range p.Fields {
		info.Fields = append(info.Fields, describeParam(f, tr))
	}
	return info
}

// cloneVisibility copies v, so the Equals list of the definition is not shared.
func cloneVisibility(v *Visibility) *Visibility {
	if v == nil {
		return nil
	}
	return &Visibility{Param: v.Param, Equals: slices.Clone(v.Equals)}
}

// cloneParamValue returns a copy of a parameter default that shares no memory with it.
// Scalars are returned as they are. Lists and objects (a default can be a []any or a
// map[string]any, and a definition may use other types) are copied through JSON, which
// is how the editor receives them; a value JSON cannot encode becomes nil rather than
// being shared.
func cloneParamValue(v any) any {
	switch v.(type) {
	case nil, bool, string, float64, float32, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, json.Number:
		return v
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	if json.Unmarshal(data, &out) != nil {
		return nil
	}
	return out
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// CatalogI18nKeys returns every i18n key the node definitions of reg refer to, sorted and
// without duplicates: labels, descriptions and summaries, parameter labels and help
// (nested fields included), option labels and output field descriptions. A test of the
// translations can check that each of them has a text.
//
// Generic tool nodes (GenericTypePrefix) carry literal text, not keys, and are skipped.
// The keys of dynamic output fields are read from the sample node like in
// DescribeNodeTypes, and a hook that panics contributes none.
func CatalogI18nKeys(reg *Registry) []string {
	if reg == nil {
		return []string{}
	}
	seen := map[string]bool{}
	add := func(key string) {
		if key != "" {
			seen[key] = true
		}
	}
	var addParams func(params []ParamSpec)
	addParams = func(params []ParamSpec) {
		for _, p := range params {
			add(p.LabelKey)
			add(p.HelpKey)
			for _, o := range p.Options {
				add(o.LabelKey)
			}
			addParams(p.Fields)
		}
	}
	for _, def := range reg.All() {
		if isGenericDef(def) {
			continue
		}
		add(def.LabelKey)
		add(def.DescriptionKey)
		add(def.SummaryKey)
		addParams(def.Params)
		for _, f := range def.OutputFields {
			add(f.DescriptionKey)
		}
		var fields []FieldSpec
		if catchPanic(func() { fields = def.FieldsOf(sampleNode(def)) }) == nil {
			for _, f := range fields {
				add(f.DescriptionKey)
			}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
