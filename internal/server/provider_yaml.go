package server

import (
	"bytes"
	"fmt"
	"strconv"

	"aurago/internal/config"
	"gopkg.in/yaml.v3"
)

func marshalConfigWithProviderEntries(data []byte, entries []config.ProviderEntry) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config root must be a YAML mapping")
	}

	providersNode := providerEntriesYAMLNode(entries)
	replaced := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Kind == yaml.ScalarNode && root.Content[i].Value == "providers" {
			root.Content[i+1] = providersNode
			replaced = true
			break
		}
	}
	if !replaced {
		root.Content = append(root.Content, yamlStringScalar("providers"), providersNode)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		_ = enc.Close()
		return nil, fmt.Errorf("failed to save config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize config: %w", err)
	}
	return buf.Bytes(), nil
}

func providerEntriesYAMLNode(entries []config.ProviderEntry) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, entry := range entries {
		seq.Content = append(seq.Content, providerEntryYAMLNode(entry))
	}
	return seq
}

func providerEntryYAMLNode(entry config.ProviderEntry) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	appendYAMLStringField(node, "id", entry.ID)
	appendYAMLStringField(node, "name", entry.Name)
	appendYAMLStringField(node, "type", entry.Type)
	appendYAMLStringField(node, "base_url", entry.BaseURL)
	appendYAMLStringField(node, "model", entry.Model)
	if entry.AccountID != "" {
		appendYAMLStringField(node, "account_id", entry.AccountID)
	}
	if len(entry.Models) > 0 {
		models := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, model := range entry.Models {
			modelNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			appendYAMLStringField(modelNode, "name", model.Name)
			appendYAMLFloatField(modelNode, "input_per_million", model.InputPerMillion)
			appendYAMLFloatField(modelNode, "output_per_million", model.OutputPerMillion)
			models.Content = append(models.Content, modelNode)
		}
		appendYAMLNodeField(node, "models", models)
	}
	if providerCapabilitiesConfigured(entry.Capabilities) {
		caps := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		appendYAMLBoolField(caps, "auto", entry.Capabilities.AutoEnabled())
		appendYAMLBoolField(caps, "tool_calling", entry.Capabilities.ToolCalling)
		appendYAMLBoolField(caps, "structured_outputs", entry.Capabilities.StructuredOutputs)
		appendYAMLBoolField(caps, "multimodal", entry.Capabilities.Multimodal)
		if entry.Capabilities.DetectedModel != "" {
			appendYAMLStringField(caps, "detected_model", entry.Capabilities.DetectedModel)
		}
		if entry.Capabilities.Source != "" {
			appendYAMLStringField(caps, "source", entry.Capabilities.Source)
		}
		appendYAMLNodeField(node, "capabilities", caps)
	}
	if entry.AuthType != "" && entry.AuthType != "api_key" {
		appendYAMLStringField(node, "auth_type", entry.AuthType)
		appendYAMLStringField(node, "oauth_auth_url", entry.OAuthAuthURL)
		appendYAMLStringField(node, "oauth_token_url", entry.OAuthTokenURL)
		appendYAMLStringField(node, "oauth_client_id", entry.OAuthClientID)
		appendYAMLStringField(node, "oauth_scopes", entry.OAuthScopes)
	}
	return node
}

func appendYAMLStringField(node *yaml.Node, key, value string) {
	appendYAMLNodeField(node, key, yamlStringScalar(value))
}

func appendYAMLBoolField(node *yaml.Node, key string, value bool) {
	appendYAMLNodeField(node, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(value)})
}

func appendYAMLFloatField(node *yaml.Node, key string, value float64) {
	appendYAMLNodeField(node, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: strconv.FormatFloat(value, 'f', -1, 64)})
}

func appendYAMLNodeField(node *yaml.Node, key string, value *yaml.Node) {
	node.Content = append(node.Content, yamlStringScalar(key), value)
}

func yamlStringScalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}
