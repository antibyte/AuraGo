package ecalc

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// decodeStrict decodes the first YAML document of data into v and rejects unknown fields.
func decodeStrict(data []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(v)
}

// decodeNode decodes one YAML node into v and rejects unknown fields (yaml.Node.Decode alone would not).
func decodeNode(n *yaml.Node, v any) error {
	data, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	return decodeStrict(data, v)
}

type mapEntry struct {
	Key string
	Val *yaml.Node
}

// orderedMap returns the entries of a YAML mapping in file order. An absent or null node is an empty mapping;
// anything else that is not a mapping, a non-scalar key or a duplicate key is an error.
func orderedMap(n *yaml.Node, what string) ([]mapEntry, error) {
	if n.Kind == 0 || (n.Kind == yaml.ScalarNode && n.Tag == "!!null") {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must be a mapping (line %d)", what, n.Line)
	}
	seen := map[string]bool{}
	out := make([]mapEntry, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if k.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%s: keys must be plain names (line %d)", what, k.Line)
		}
		if seen[k.Value] {
			return nil, fmt.Errorf("%s: duplicate key %q (line %d)", what, k.Value, k.Line)
		}
		seen[k.Value] = true
		out = append(out, mapEntry{Key: k.Value, Val: n.Content[i+1]})
	}
	return out, nil
}
