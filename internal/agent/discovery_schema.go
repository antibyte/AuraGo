package agent

import (
	"bytes"
	"encoding/json"
	"sort"
	"sync"

	"aurago/internal/security"
)

// schemaLeafKind says how a string of a JSON schema reaches the model.
type schemaLeafKind int

const (
	// schemaLeafStructure is a keyword value the model reads exactly: type,
	// format, $ref, a required name.
	schemaLeafStructure schemaLeafKind = iota
	// schemaLeafValue is a value the model may send back, or a pattern it has to
	// match: enum, const, default, examples, pattern.
	schemaLeafValue
	// schemaLeafText is prose: description, title, $comment and every keyword
	// this walker does not know.
	schemaLeafText
	// schemaLeafFlagged is a structure or value string whose keyword the group
	// check judged hostile (or could not judge): it is handled like prose.
	schemaLeafFlagged
)

// schemaWalk visits the strings of a JSON schema. The walk returns a copy in
// which leaf may replace every string; names (property names and the keys of
// value objects) are only reported through name and always stay in place.
//
// The walk is default-deny: a string under a keyword that is not a known
// structure or value keyword counts as prose.
//
// A structure or value keyword is judged as a whole: group receives all strings
// the keyword holds (an enum, an examples list, a default object) and a true
// verdict hands each of them to leaf as schemaLeafFlagged. What a string looks
// like decides nothing: the verdict comes from a scan of the keyword's text.
type schemaWalk struct {
	name  func(name string)
	leaf  func(text string, kind schemaLeafKind) string
	group func(texts []string, kind schemaLeafKind) bool
}

// schema walks one schema node. A node that is not an object (a boolean schema,
// a malformed one) is handled as data.
func (w schemaWalk) schema(node interface{}) interface{} {
	object, ok := node.(map[string]interface{})
	if !ok {
		return w.data(node, schemaLeafText)
	}
	properties, _ := object["properties"].(map[string]interface{})
	out := make(map[string]interface{}, len(object))
	for key, value := range object {
		switch key {
		case "required":
			out[key] = w.required(value, properties)
		case "type", "format", "$ref", "$schema", "$id", "$anchor", "$dynamicRef", "$dynamicAnchor", "dependentRequired":
			out[key] = w.data(value, schemaLeafStructure)
		case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas", "dependencies":
			out[key] = w.named(value)
		case "items", "additionalItems", "additionalProperties", "unevaluatedItems", "unevaluatedProperties", "contains",
			"propertyNames", "not", "if", "then", "else", "contentSchema", "allOf", "anyOf", "oneOf", "prefixItems":
			out[key] = w.subschemas(value)
		case "enum", "const", "default", "examples", "example", "pattern":
			out[key] = w.data(value, schemaLeafValue)
		default:
			out[key] = w.data(value, schemaLeafText)
		}
	}
	return out
}

// required keeps the names that a sibling property declares; any other entry is
// a structure string, judged together with the other undeclared entries.
func (w schemaWalk) required(value interface{}, properties map[string]interface{}) interface{} {
	list, ok := value.([]interface{})
	if !ok {
		return w.data(value, schemaLeafStructure)
	}
	declared := func(item interface{}) bool {
		name, ok := item.(string)
		if !ok {
			return false
		}
		_, ok = properties[name]
		return ok
	}
	var undeclared []interface{}
	for _, item := range list {
		if !declared(item) {
			undeclared = append(undeclared, item)
		}
	}
	walked, _ := w.data(undeclared, schemaLeafStructure).([]interface{})
	out := make([]interface{}, len(list))
	for i, item := range list {
		if declared(item) {
			out[i] = item
			continue
		}
		out[i], walked = walked[0], walked[1:]
	}
	return out
}

// named walks an object that maps names to schemas (properties, $defs). The
// legacy dependencies keyword may map a name to a list of names instead.
func (w schemaWalk) named(value interface{}) interface{} {
	object, ok := value.(map[string]interface{})
	if !ok {
		return w.data(value, schemaLeafText)
	}
	out := make(map[string]interface{}, len(object))
	for name, sub := range object {
		if w.name != nil {
			w.name(name)
		}
		if _, isList := sub.([]interface{}); isList {
			out[name] = w.data(sub, schemaLeafStructure)
			continue
		}
		out[name] = w.schema(sub)
	}
	return out
}

// subschemas walks a keyword whose value is a schema or a list of schemas.
func (w schemaWalk) subschemas(value interface{}) interface{} {
	if list, ok := value.([]interface{}); ok {
		out := make([]interface{}, len(list))
		for i, item := range list {
			out[i] = w.schema(item)
		}
		return out
	}
	return w.schema(value)
}

// data walks a plain JSON value: every string in it has the given kind. The
// strings of a structure or value keyword are judged by group first.
func (w schemaWalk) data(value interface{}, kind schemaLeafKind) interface{} {
	if w.group != nil && (kind == schemaLeafStructure || kind == schemaLeafValue) {
		if texts := schemaStrings(value, nil); len(texts) > 0 && w.group(texts, kind) {
			kind = schemaLeafFlagged
		}
	}
	return w.leaves(value, kind)
}

func (w schemaWalk) leaves(value interface{}, kind schemaLeafKind) interface{} {
	switch v := value.(type) {
	case string:
		if w.leaf == nil {
			return v
		}
		return w.leaf(v, kind)
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = w.leaves(item, kind)
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for key, item := range v {
			if w.name != nil {
				w.name(key)
			}
			out[key] = w.leaves(item, kind)
		}
		return out
	default:
		return value
	}
}

// schemaStrings appends the strings of a plain JSON value in document order for
// lists; the values of an object follow its sorted keys, as encoding/json
// orders them, which is the order the model reads. Keys are names and are not
// part of the text.
func schemaStrings(value interface{}, texts []string) []string {
	switch v := value.(type) {
	case string:
		return append(texts, v)
	case []interface{}:
		for _, item := range v {
			texts = schemaStrings(item, texts)
		}
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			texts = schemaStrings(v[key], texts)
		}
	}
	return texts
}

// plainSchemaCopy returns a deep copy of a schema that holds only plain JSON
// types (objects, lists, strings, json.Number, booleans, nil). Native schemas
// carry []string and other Go types the walk would pass over unseen. It reports
// false when the schema cannot be encoded.
func plainSchemaCopy(schema interface{}) (map[string]interface{}, bool) {
	if schema == nil {
		return nil, true
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, false
	}
	return decodePlainSchema(raw, true)
}

// decodePlainSchema decodes an encoded schema into plain JSON values. Numbers
// stay exact json.Number values for output that is only encoded again; without
// exactNumbers they are float64, the type a decoded MCP schema has.
func decodePlainSchema(raw []byte, exactNumbers bool) (map[string]interface{}, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if exactNumbers {
		decoder.UseNumber()
	}
	var out map[string]interface{}
	if err := decoder.Decode(&out); err != nil {
		return nil, false
	}
	return out, true
}

// fenceSchemaLeaf isolates the prose of a tool schema and every structure or
// value string that the group check flagged. All other structure and value
// strings stay byte-exact: they are what the model has to send back.
func fenceSchemaLeaf(text string, kind schemaLeafKind) string {
	switch kind {
	case schemaLeafStructure, schemaLeafValue:
		return text
	}
	return fenceDiscoveryText(text)
}

// firstPartyToolNames holds the name of every tool schema that is compiled into
// AuraGo. Their schemas are static first-party text; every third-party entry
// has a reserved prefix (mcp__, skill__, tool__, package__) and cannot take
// such a name. A built-in tool missing from the set is merely fenced.
var firstPartyToolNames = sync.OnceValue(allBuiltinToolNameSet)

// fenceToolSchema is the get_tool_info form of a tool's parameter schema:
// fenced for everything but the schemas compiled into AuraGo. Fencing those
// would only cost size (every leaf grows by about 55 bytes: game_maker_project
// would grow from 40 KB to 58 KB and exceed the default 50 KB tool output limit)
// and protect nothing.
func fenceToolSchema(guardian *security.Guardian, entry *ToolCatalogEntry, params map[string]interface{}) map[string]interface{} {
	if entry != nil {
		if _, builtin := firstPartyToolNames()[entry.Name]; builtin {
			return params
		}
	}
	return fenceSchemaAnnotations(guardian, params)
}

// fenceSchemaAnnotations returns a copy of a tool's input schema in which every
// string a third party can use for prose (description, title, unknown keywords)
// is wrapped by security.IsolateExternalData. The strings of a structure or
// value keyword (type, format, enum, const, default, examples, pattern) stay
// byte-exact, because they are what the model sends back or has to match: they
// are fenced, all strings of the keyword together, only when a scan of the
// keyword's text (the run's Guardian, ThreatHigh or above; a list read joined,
// an identifier read as words) flags it or cannot cover it within the budget.
// Keys, type, required and properties stay raw, so the JSON keeps its shape and
// stays parseable. The copy leaves the catalog schema, which becomes the function
// definition after an activation, untouched. A schema that cannot be encoded
// yields nil.
func fenceSchemaAnnotations(guardian *security.Guardian, params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}
	plain, ok := plainSchemaCopy(params)
	if !ok {
		return nil
	}
	scan := newSchemaScan(guardian, schemaScanBudget)
	walk := schemaWalk{
		leaf: fenceSchemaLeaf,
		group: func(texts []string, _ schemaLeafKind) bool {
			flagged, complete := scan.group(texts)
			return flagged || !complete
		},
	}
	fenced, _ := walk.schema(plain).(map[string]interface{})
	return fenced
}
