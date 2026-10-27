package dockerstacks

import "gopkg.in/yaml.v3"

// maxAliasDepth and maxMergedPairs bound alias and merge-key chains so a hostile file ("billion laughs" built from
// <<: [*a, *a, ...]) cannot make the walkers loop or explode.
const (
	maxAliasDepth  = 8
	maxMergedPairs = 10000
)

// yamlPair is one key/value pair of a mapping node.
type yamlPair struct{ Key, Value *yaml.Node }

// resolveAlias follows alias nodes to the node they point at.
func resolveAlias(n *yaml.Node) *yaml.Node {
	for i := 0; n != nil && n.Kind == yaml.AliasNode && i < maxAliasDepth; i++ {
		n = n.Alias
	}
	return n
}

// mapPairs returns the pairs of a mapping node: its own keys first, then the keys that "<<" merges in (an own key
// wins over a merged one). Aliases are resolved. A node that is not a mapping yields nil.
func mapPairs(n *yaml.Node) []yamlPair {
	budget := maxMergedPairs
	return collectPairs(n, 0, &budget)
}

func collectPairs(n *yaml.Node, depth int, budget *int) []yamlPair {
	n = resolveAlias(n)
	if n == nil || n.Kind != yaml.MappingNode || depth > maxAliasDepth || *budget <= 0 {
		return nil
	}
	*budget -= len(n.Content) / 2
	var own []yamlPair
	var merges []*yaml.Node
	seen := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.Kind == yaml.ScalarNode && (k.Tag == "!!merge" || (k.Value == "<<" && k.Style == 0)) {
			merges = append(merges, v)
			continue
		}
		own = append(own, yamlPair{k, resolveAlias(v)})
		seen[k.Value] = true
	}
	for _, m := range merges {
		sources := []*yaml.Node{m}
		if r := resolveAlias(m); r != nil && r.Kind == yaml.SequenceNode {
			sources = r.Content
		}
		for _, src := range sources {
			for _, p := range collectPairs(src, depth+1, budget) {
				if !seen[p.Key.Value] {
					seen[p.Key.Value] = true
					own = append(own, p)
				}
			}
		}
	}
	return own
}

// mapGet returns the key and (alias-resolved) value node of key in a mapping.
func mapGet(n *yaml.Node, key string) (k, v *yaml.Node) {
	for _, p := range mapPairs(n) {
		if p.Key.Kind == yaml.ScalarNode && p.Key.Value == key {
			return p.Key, p.Value
		}
	}
	return nil, nil
}

// mapValue is mapGet without the key node.
func mapValue(n *yaml.Node, key string) *yaml.Node {
	_, v := mapGet(n, key)
	return v
}

// seqItems returns the alias-resolved items of a sequence node, nil for anything else.
func seqItems(n *yaml.Node) []*yaml.Node {
	n = resolveAlias(n)
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]*yaml.Node, 0, len(n.Content))
	for _, c := range n.Content {
		if r := resolveAlias(c); r != nil {
			out = append(out, r)
		}
	}
	return out
}

// scalarText returns the text of a scalar node and whether n is a non-null scalar.
func scalarText(n *yaml.Node) (string, bool) {
	n = resolveAlias(n)
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return "", false
	}
	return n.Value, true
}

// scalarBool reports whether n is the boolean true: a YAML bool that decodes to true or the quoted string "true".
// Other spellings ("yes", "on") are not guessed.
func scalarBool(n *yaml.Node) bool {
	n = resolveAlias(n)
	if n == nil || n.Kind != yaml.ScalarNode {
		return false
	}
	var b bool
	if n.Tag == "!!bool" {
		return n.Decode(&b) == nil && b
	}
	return n.Value == "true"
}

// walkScalars calls fn for every scalar node below n in document order (keys and values). Alias nodes are not
// followed, so an anchored value is visited once, where it is defined.
func walkScalars(n *yaml.Node, fn func(*yaml.Node)) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.ScalarNode:
		fn(n)
	case yaml.DocumentNode, yaml.MappingNode, yaml.SequenceNode:
		for _, c := range n.Content {
			walkScalars(c, fn)
		}
	}
}
