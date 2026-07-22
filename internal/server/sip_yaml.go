package server

import (
	"bytes"
	"fmt"

	"aurago/internal/config"

	"gopkg.in/yaml.v3"
)

func marshalConfigWithSIP(data []byte, sipConfig config.SIPConfig) ([]byte, error) {
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
	var node yaml.Node
	if err := node.Encode(sipConfig); err != nil {
		return nil, fmt.Errorf("encode SIP config: %w", err)
	}
	replaced := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Kind == yaml.ScalarNode && root.Content[i].Value == "sip" {
			root.Content[i+1] = &node
			replaced = true
			break
		}
	}
	if !replaced {
		root.Content = append(root.Content, yamlStringScalar("sip"), &node)
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		_ = encoder.Close()
		return nil, fmt.Errorf("failed to save SIP config: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize SIP config: %w", err)
	}
	return output.Bytes(), nil
}
