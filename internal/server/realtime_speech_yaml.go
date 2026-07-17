package server

import (
	"bytes"
	"fmt"

	"aurago/internal/config"

	"gopkg.in/yaml.v3"
)

func marshalConfigWithRealtimeSpeech(data []byte, realtime config.RealtimeSpeechConfig) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if len(document.Content) == 0 {
		document.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config root must be a YAML mapping")
	}

	node := realtimeSpeechYAMLNode(realtime)
	replaced := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Kind == yaml.ScalarNode && root.Content[i].Value == "realtime_speech" {
			root.Content[i+1] = node
			replaced = true
			break
		}
	}
	if !replaced {
		root.Content = append(root.Content, yamlStringScalar("realtime_speech"), node)
	}

	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		_ = encoder.Close()
		return nil, fmt.Errorf("failed to save realtime speech config: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize realtime speech config: %w", err)
	}
	return output.Bytes(), nil
}

func realtimeSpeechYAMLNode(realtime config.RealtimeSpeechConfig) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	appendYAMLBoolField(node, "enabled", realtime.Enabled)
	appendYAMLStringField(node, "default_profile", realtime.DefaultProfile)
	appendYAMLNodeField(node, "park_after_seconds", &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: fmt.Sprintf("%d", realtime.ParkAfterSeconds),
	})
	profiles := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, profile := range realtime.Profiles {
		entry := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		appendYAMLStringField(entry, "id", profile.ID)
		appendYAMLStringField(entry, "name", profile.Name)
		appendYAMLStringField(entry, "provider", profile.Provider)
		appendYAMLStringField(entry, "model", profile.Model)
		appendYAMLStringField(entry, "voice", profile.Voice)
		appendYAMLBoolField(entry, "enabled", profile.Enabled)
		profiles.Content = append(profiles.Content, entry)
	}
	appendYAMLNodeField(node, "profiles", profiles)
	return node
}
