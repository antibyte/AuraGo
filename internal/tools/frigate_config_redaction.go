package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// redactFrigateConfig uses a conservative projection instead of a credential
// name blacklist: unknown scalar values and all strings remain confidential.
// Re-encoding also discards comments. Aliases/tags/multiple documents fail closed.
func redactFrigateConfig(data []byte, raw bool) ([]byte, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var doc, extra yaml.Node
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("invalid configuration")
	}
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("multiple configuration documents")
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("configuration must be a mapping")
	}
	remaining := 20000
	var project func(*yaml.Node, string, int) (interface{}, error)
	project = func(node *yaml.Node, key string, depth int) (interface{}, error) {
		remaining--
		if remaining < 0 || depth > 48 {
			return nil, fmt.Errorf("configuration too complex")
		}
		switch node.Kind {
		case yaml.MappingNode:
			if node.Tag != "!!map" || len(node.Content)%2 != 0 {
				return nil, fmt.Errorf("unsupported mapping")
			}
			out := map[string]interface{}{}
			for i := 0; i < len(node.Content); i += 2 {
				k := node.Content[i]
				if k.Kind != yaml.ScalarNode || k.Tag != "!!str" || len(k.Value) > 256 {
					return nil, fmt.Errorf("unsupported key")
				}
				if _, exists := out[k.Value]; exists {
					return nil, fmt.Errorf("duplicate key")
				}
				name := strings.ToLower(k.Value)
				// Dynamic environment/header/stream keys can themselves be secrets.
				switch name {
				case "environment_vars", "environment", "headers", "auth", "go2rtc", "mqtt", "tls":
					out[k.Value] = "[REDACTED]"
					continue
				}
				value, err := project(node.Content[i+1], name, depth+1)
				if err != nil {
					return nil, err
				}
				out[k.Value] = value
			}
			return out, nil
		case yaml.SequenceNode:
			if node.Tag != "!!seq" {
				return nil, fmt.Errorf("unsupported sequence")
			}
			out := make([]interface{}, 0, len(node.Content))
			for _, child := range node.Content {
				value, err := project(child, key, depth+1)
				if err != nil {
					return nil, err
				}
				out = append(out, value)
			}
			return out, nil
		case yaml.ScalarNode:
			if node.Tag != "!!str" && node.Tag != "!!int" && node.Tag != "!!float" && node.Tag != "!!bool" && node.Tag != "!!null" {
				return nil, fmt.Errorf("unsupported scalar")
			}
			switch key {
			case "enabled", "width", "height", "fps", "days", "pre_capture", "post_capture", "threshold", "min_score", "max_disappeared":
				if node.Tag == "!!int" || node.Tag == "!!float" || node.Tag == "!!bool" {
					var value interface{}
					if err := node.Decode(&value); err != nil {
						return nil, err
					}
					return value, nil
				}
			}
			return "[REDACTED]", nil
		default:
			return nil, fmt.Errorf("unsupported configuration node")
		}
	}
	clean, err := project(doc.Content[0], "", 0)
	if err != nil {
		return nil, err
	}
	if !raw {
		return json.Marshal(clean)
	}
	yamlData, err := yaml.Marshal(clean)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]interface{}{"status": "ok", "redacted": true, "data": string(yamlData)})
}
