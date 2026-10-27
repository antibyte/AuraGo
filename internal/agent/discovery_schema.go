package agent

import (
	"bytes"
	"encoding/json"
	"sync"
)

// schemaLeafKind says how a string of a JSON schema reaches the model.
type schemaLeafKind int

const (
	// schemaLeafStructure is a keyword value the model reads exactly: type,
	// format, $ref, a required name.
	schemaLeafStructure schemaLeafKind = iota
	// schemaLeafValue is a value the model may send back: enum, const, default,
	// examples.
	schemaLeafValue
	// schemaLeafText is prose: description, title, $comment and every keyword
	// this walker does not know.
	schemaLeafText
)

const (
	// schemaStructureTokenLimit and schemaValueTokenLimit bound a plain token.
	schemaStructureTokenLimit = 200
	schemaValueTokenLimit     = 48
)

// plainSchemaToken reports whether a string is one short token without spaces,
// quotes or markup: a type name, a format, a $ref, an enum member such as
// "read". Such a string cannot carry an instruction, and fencing it would
// change the value the model has to send back.
func plainSchemaToken(text string, limit int) bool {
	if text == "" || len(text) > limit {
		return false
	}
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '_' || c == '-' || c == '.' || c == ':' || c == '/' || c == '@' || c == '+' || c == '#' || c == '$' || c == '=' || c == ',' || c == '%' || c == '*' || c == '~':
		default:
			return false
		}
	}
	return true
}

// schemaWalk visits the strings of a JSON schema. The walk returns a copy in
// which leaf may replace every string; names (property names and the keys of
// value objects) are only reported through name and always stay in place.
//
// The walk is default-deny: a string under a keyword that is not a known
// structure or value keyword counts as prose.
type schemaWalk struct {
	name func(name string)
	leaf func(text string, kind schemaLeafKind) string
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
		case "enum", "const", "default", "examples", "example":
			out[key] = w.data(value, schemaLeafValue)
		default:
			out[key] = w.data(value, schemaLeafText)
		}
	}
	return out
}

// required keeps the names that a sibling property declares; any other entry is
// an ordinary structure string.
func (w schemaWalk) required(value interface{}, properties map[string]interface{}) interface{} {
	list, ok := value.([]interface{})
	if !ok {
		return w.data(value, schemaLeafStructure)
	}
	out := make([]interface{}, len(list))
	for i, item := range list {
		if name, ok := item.(string); ok {
			if _, declared := properties[name]; declared {
				out[i] = name
				continue
			}
		}
		out[i] = w.data(item, schemaLeafStructure)
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

// data walks a plain JSON value: every string in it has the given kind.
func (w schemaWalk) data(value interface{}, kind schemaLeafKind) interface{} {
	switch v := value.(type) {
	case string:
		if w.leaf == nil {
			return v
		}
		return w.leaf(v, kind)
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = w.data(item, kind)
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for key, item := range v {
			if w.name != nil {
				w.name(key)
			}
			out[key] = w.data(item, kind)
		}
		return out
	default:
		return value
	}
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

// fenceSchemaLeaf isolates the prose of a tool schema. Structure keywords and
// plain value tokens stay as they are, so the schema keeps its meaning.
func fenceSchemaLeaf(text string, kind schemaLeafKind) string {
	switch kind {
	case schemaLeafStructure:
		if plainSchemaToken(text, schemaStructureTokenLimit) {
			return text
		}
	case schemaLeafValue:
		if plainSchemaToken(text, schemaValueTokenLimit) {
			return text
		}
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
func fenceToolSchema(entry *ToolCatalogEntry, params map[string]interface{}) map[string]interface{} {
	if entry != nil {
		if _, builtin := firstPartyToolNames()[entry.Name]; builtin {
			return params
		}
	}
	return fenceSchemaAnnotations(params)
}

// fenceSchemaAnnotations returns a copy of a tool's input schema in which every
// string a third party can use for prose (description, title, free-text enum,
// default and example values, unknown keywords) is wrapped by
// security.IsolateExternalData. Keys, type, required, properties and plain
// tokens stay raw, so the JSON keeps its shape and stays parseable. The copy
// leaves the catalog schema, which becomes the function definition after an
// activation, untouched. A schema that cannot be encoded yields nil.
func fenceSchemaAnnotations(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}
	plain, ok := plainSchemaCopy(params)
	if !ok {
		return nil
	}
	fenced, _ := schemaWalk{leaf: fenceSchemaLeaf}.schema(plain).(map[string]interface{})
	return fenced
}
