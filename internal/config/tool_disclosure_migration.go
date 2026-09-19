package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

var disclosureMovedSettings = [][2]string{
	{"agent.max_tool_calls", "circuit_breaker.max_tool_calls"},
	{"skill_manager.read_only", "tools.skill_manager.readonly"},
	{"skill_manager", "tools.skill_manager"},
	{"tools.skill_manager.read_only", "tools.skill_manager.readonly"},
	{"agent.budget", "budget"},
	{"composio.preferred_capabilities", "mcp.preferred_capabilities"},
}

var disclosureRemovedSettings = []string{
	"agent.discover_tools_snapshot_ttl_minutes", "agent.output_compression.repetitive_substitution.ltsc_lite_enabled",
	"memory_analysis.enabled", "memory_analysis.preset", "memory_analysis.real_time", "memory_analysis.query_expansion",
	"memory_analysis.llm_reranking", "memory_analysis.unified_memory_block", "memory_analysis.effectiveness_tracking", "memory_analysis.weekly_reflection",
}

// NormalizeToolDisclosureConfig migrates obsolete template paths without
// overriding explicit canonical values. Removed switches have no save path.
// YAML nodes preserve comments; running the migration twice is a no-op.
func NormalizeToolDisclosureConfig(data []byte) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse tool configuration migration: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return data, nil
	}
	root := doc.Content[0]
	changed := false
	for _, move := range disclosureMovedSettings {
		if value := disclosureYAMLPath(root, strings.Split(move[0], "."), false); value != nil {
			destination := strings.Split(move[1], ".")
			parent := disclosureYAMLPath(root, destination[:len(destination)-1], true)
			if parent == nil || parent.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("configuration path %s must be a mapping", strings.Join(destination[:len(destination)-1], "."))
			}
			key := destination[len(destination)-1]
			if existing := disclosureYAMLPath(parent, []string{key}, false); existing == nil {
				parent.Content = append(parent.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
			} else {
				mergeMissingDisclosureYAML(existing, value)
			}
			removeDisclosureYAML(root, strings.Split(move[0], "."))
			changed = true
		}
	}
	for _, path := range disclosureRemovedSettings {
		if removeDisclosureYAML(root, strings.Split(path, ".")) {
			changed = true
		}
	}
	if !changed {
		return data, nil
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, fmt.Errorf("encode tool configuration migration: %w", err)
	}
	return out, nil
}

// ToolDisclosureMigrationNotices contains field paths and known boolean opt-outs, never secrets.
func ToolDisclosureMigrationNotices(data []byte) []string {
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 {
		return nil
	}
	var result []string
	for _, move := range disclosureMovedSettings {
		if disclosureYAMLPath(doc.Content[0], strings.Split(move[0], "."), false) != nil {
			result = append(result, move[0]+" → "+move[1])
		}
	}
	for _, path := range disclosureRemovedSettings {
		if disclosureYAMLPath(doc.Content[0], strings.Split(path, "."), false) != nil {
			result = append(result, path+" (removed: no runtime consumer)")
		}
	}
	for _, path := range []string{"agent.output_compression.reversible.enabled", "agent.output_compression.smart_crusher.enabled", "agent.importance_scoring.enabled", "agent.auto_learning.enabled"} {
		if node := disclosureYAMLPath(doc.Content[0], strings.Split(path, "."), false); node != nil && node.Tag == "!!bool" && node.Value == "false" {
			result = append(result, path+" = false (explicit opt-out respected)")
		}
	}
	return result
}

// Old files migrate, but stale browser patches must not recreate removed controls.
func ValidateToolDisclosurePatch(patch map[string]interface{}) error {
	has := func(path string) (interface{}, bool) {
		var current interface{} = patch
		for _, key := range strings.Split(path, ".") {
			m, ok := current.(map[string]interface{})
			if !ok {
				return nil, false
			}
			current, ok = m[key]
			if !ok {
				return nil, false
			}
		}
		return current, true
	}
	for _, move := range disclosureMovedSettings {
		if _, ok := has(move[0]); ok {
			return fmt.Errorf("setting %s moved to %s; reload the configuration page", move[0], move[1])
		}
	}
	for _, path := range disclosureRemovedSettings {
		if _, ok := has(path); ok {
			return fmt.Errorf("setting %s was removed because it has no runtime effect; reload the configuration page", path)
		}
	}
	for _, path := range []string{"agent.output_compression.reversible.enabled", "agent.output_compression.smart_crusher.enabled", "agent.importance_scoring.enabled", "agent.auto_learning.enabled"} {
		if value, exists := has(path); exists {
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("setting %s must be true or false", path)
			}
		}
	}
	return nil
}

func disclosureYAMLPath(node *yaml.Node, path []string, create bool) *yaml.Node {
	for _, key := range path {
		if node == nil || node.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				next = node.Content[i+1]
				break
			}
		}
		if next == nil {
			if !create {
				return nil
			}
			next = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, next)
		}
		node = next
	}
	return node
}

func removeDisclosureYAML(root *yaml.Node, path []string) bool {
	parent := disclosureYAMLPath(root, path[:len(path)-1], false)
	if parent == nil || parent.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == path[len(path)-1] {
			parent.Content = append(parent.Content[:i], parent.Content[i+2:]...)
			return true
		}
	}
	return false
}

func mergeMissingDisclosureYAML(destination, source *yaml.Node) {
	if destination.Kind != yaml.MappingNode || source.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(source.Content); i += 2 {
		key, value := source.Content[i], source.Content[i+1]
		if existing := disclosureYAMLPath(destination, []string{key.Value}, false); existing != nil {
			mergeMissingDisclosureYAML(existing, value)
		} else {
			destination.Content = append(destination.Content, key, value)
		}
	}
}
