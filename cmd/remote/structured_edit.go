//go:build !remote_minimal

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/beevik/etree"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v3"
)

// jsonEdit performs JSON file operations on the remote device.
func (e *Executor) jsonEdit(path, op, jsonPath string, setValue interface{}) (string, error) {
	switch op {
	case "get":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if !gjson.ValidBytes(data) {
			return "", fmt.Errorf("file is not valid JSON")
		}
		if jsonPath == "" {
			return string(data), nil
		}
		result := gjson.GetBytes(data, jsonPath)
		if !result.Exists() {
			return "", fmt.Errorf("path '%s' not found", jsonPath)
		}
		return result.Raw, nil

	case "set":
		if jsonPath == "" {
			return "", fmt.Errorf("json_path is required for set")
		}
		var data []byte
		if _, err := os.Stat(path); err == nil {
			data, err = os.ReadFile(path)
			if err != nil {
				return "", err
			}
			if !gjson.ValidBytes(data) {
				return "", fmt.Errorf("file is not valid JSON")
			}
		} else {
			data = []byte("{}")
		}
		updated, err := sjson.SetBytes(data, jsonPath, setValue)
		if err != nil {
			return "", fmt.Errorf("set failed: %w", err)
		}
		var pretty json.RawMessage = updated
		formatted, err := json.MarshalIndent(pretty, "", "  ")
		if err != nil {
			formatted = updated
		}
		if err := os.WriteFile(path, append(formatted, '\n'), 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("set '%s' successfully", jsonPath), nil

	case "delete":
		if jsonPath == "" {
			return "", fmt.Errorf("json_path is required for delete")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if !gjson.ValidBytes(data) {
			return "", fmt.Errorf("file is not valid JSON")
		}
		updated, err := sjson.DeleteBytes(data, jsonPath)
		if err != nil {
			return "", fmt.Errorf("delete failed: %w", err)
		}
		var pretty json.RawMessage = updated
		formatted, err := json.MarshalIndent(pretty, "", "  ")
		if err != nil {
			formatted = updated
		}
		if err := os.WriteFile(path, append(formatted, '\n'), 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("deleted '%s' successfully", jsonPath), nil

	case "keys":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if !gjson.ValidBytes(data) {
			return "", fmt.Errorf("file is not valid JSON")
		}
		target := string(data)
		if jsonPath != "" {
			r := gjson.Get(target, jsonPath)
			if !r.Exists() {
				return "", fmt.Errorf("path '%s' not found", jsonPath)
			}
			target = r.Raw
		}
		var keys []string
		gjson.Parse(target).ForEach(func(key, _ gjson.Result) bool {
			keys = append(keys, key.String())
			return true
		})
		out, _ := json.Marshal(keys)
		return string(out), nil

	case "validate":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if gjson.ValidBytes(data) {
			return `{"valid":true}`, nil
		}
		return `{"valid":false}`, nil

	case "format":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if !gjson.ValidBytes(data) {
			return "", fmt.Errorf("file is not valid JSON")
		}
		var raw json.RawMessage = data
		formatted, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			return "", fmt.Errorf("format failed: %w", err)
		}
		if err := os.WriteFile(path, append(formatted, '\n'), 0o644); err != nil {
			return "", err
		}
		return "formatted successfully", nil

	default:
		return "", fmt.Errorf("unknown json_edit operation: %s", op)
	}
}

// yamlEdit performs YAML file operations on the remote device.
func (e *Executor) yamlEdit(path, op, dotPath string, setValue interface{}) (string, error) {
	switch op {
	case "get":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		var doc interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return "", fmt.Errorf("invalid YAML: %w", err)
		}
		if dotPath == "" {
			return string(data), nil
		}
		val, err := yamlNavigateRemote(doc, dotPath)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(val)
		return string(out), nil

	case "set":
		if dotPath == "" {
			return "", fmt.Errorf("json_path is required for set")
		}
		var root yaml.Node
		data, _ := os.ReadFile(path)
		if len(data) > 0 {
			if err := yaml.Unmarshal(data, &root); err != nil {
				return "", fmt.Errorf("invalid YAML: %w", err)
			}
		} else {
			root.Kind = yaml.DocumentNode
			root.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
		}
		parts := strings.Split(dotPath, ".")
		if err := yamlNodeSetRemote(root.Content[0], parts, setValue); err != nil {
			return "", err
		}
		out, err := yaml.Marshal(&root)
		if err != nil {
			return "", fmt.Errorf("marshal failed: %w", err)
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("set '%s' successfully", dotPath), nil

	case "delete":
		if dotPath == "" {
			return "", fmt.Errorf("json_path is required for delete")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		var root yaml.Node
		if err := yaml.Unmarshal(data, &root); err != nil {
			return "", fmt.Errorf("invalid YAML: %w", err)
		}
		parts := strings.Split(dotPath, ".")
		if err := yamlNodeDeleteRemote(root.Content[0], parts); err != nil {
			return "", err
		}
		out, err := yaml.Marshal(&root)
		if err != nil {
			return "", fmt.Errorf("marshal failed: %w", err)
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("deleted '%s' successfully", dotPath), nil

	case "keys":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		var doc interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return "", fmt.Errorf("invalid YAML: %w", err)
		}
		target := doc
		if dotPath != "" {
			val, err := yamlNavigateRemote(doc, dotPath)
			if err != nil {
				return "", err
			}
			target = val
		}
		m, ok := target.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("value at path is not a mapping")
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		out, _ := json.Marshal(keys)
		return string(out), nil

	case "validate":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		var doc interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return `{"valid":false,"error":"` + err.Error() + `"}`, nil
		}
		return `{"valid":true}`, nil

	default:
		return "", fmt.Errorf("unknown yaml_edit operation: %s", op)
	}
}

// yamlNavigateRemote traverses a decoded YAML value by dot-path.
func yamlNavigateRemote(doc interface{}, dotPath string) (interface{}, error) {
	parts := strings.Split(dotPath, ".")
	current := doc
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("path '%s' not found", dotPath)
			}
			current = val
		default:
			return nil, fmt.Errorf("path '%s' not found (not a mapping)", dotPath)
		}
	}
	return current, nil
}

// yamlNodeSetRemote sets a value in a YAML node tree.
func yamlNodeSetRemote(node *yaml.Node, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}
	key := parts[0]
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node")
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			if len(parts) == 1 {
				var valNode yaml.Node
				valNode.Encode(value)
				*node.Content[i+1] = valNode
				return nil
			}
			return yamlNodeSetRemote(node.Content[i+1], parts[1:], value)
		}
	}
	if len(parts) == 1 {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"}
		var valNode yaml.Node
		valNode.Encode(value)
		node.Content = append(node.Content, keyNode, &valNode)
		return nil
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"}
	newMapping := &yaml.Node{Kind: yaml.MappingNode}
	node.Content = append(node.Content, keyNode, newMapping)
	return yamlNodeSetRemote(newMapping, parts[1:], value)
}

// yamlNodeDeleteRemote deletes a key from a YAML node tree.
func yamlNodeDeleteRemote(node *yaml.Node, parts []string) error {
	if len(parts) == 0 || node.Kind != yaml.MappingNode {
		return fmt.Errorf("cannot delete: invalid path or node type")
	}
	key := parts[0]
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			if len(parts) == 1 {
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				return nil
			}
			return yamlNodeDeleteRemote(node.Content[i+1], parts[1:])
		}
	}
	return fmt.Errorf("key '%s' not found", key)
}

// xmlEdit performs XML editing operations on the remote host.
func (e *Executor) xmlEdit(path, op, xpath string, setValue interface{}) (string, error) {
	switch op {
	case "get":
		if xpath == "" {
			return "", fmt.Errorf("xpath is required for get")
		}
		doc := etree.NewDocument()
		if err := doc.ReadFromFile(path); err != nil {
			return "", fmt.Errorf("failed to parse XML: %w", err)
		}
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", fmt.Errorf("no elements found for path '%s'", xpath)
		}
		var results []map[string]interface{}
		for _, el := range elements {
			entry := map[string]interface{}{"tag": el.Tag, "text": strings.TrimSpace(el.Text())}
			if len(el.Attr) > 0 {
				attrs := make(map[string]string)
				for _, a := range el.Attr {
					key := a.Key
					if a.Space != "" {
						key = a.Space + ":" + key
					}
					attrs[key] = a.Value
				}
				entry["attributes"] = attrs
			}
			results = append(results, entry)
		}
		if len(results) == 1 {
			out, _ := json.Marshal(results[0])
			return string(out), nil
		}
		out, _ := json.Marshal(results)
		return string(out), nil

	case "set_text":
		if xpath == "" {
			return "", fmt.Errorf("xpath is required for set_text")
		}
		text := fmt.Sprintf("%v", setValue)
		doc := etree.NewDocument()
		if err := doc.ReadFromFile(path); err != nil {
			return "", fmt.Errorf("failed to parse XML: %w", err)
		}
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", fmt.Errorf("no elements found for path '%s'", xpath)
		}
		for _, el := range elements {
			el.SetText(text)
		}
		doc.Indent(2)
		if err := doc.WriteToFile(path); err != nil {
			return "", fmt.Errorf("failed to write XML: %w", err)
		}
		out, _ := json.Marshal(map[string]interface{}{"updated": len(elements)})
		return string(out), nil

	case "set_attribute":
		if xpath == "" {
			return "", fmt.Errorf("xpath is required for set_attribute")
		}
		attrs, ok := setValue.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("set_value must be {name, value} for set_attribute")
		}
		attrName, _ := attrs["name"].(string)
		if attrName == "" {
			return "", fmt.Errorf("set_value.name is required for set_attribute")
		}
		attrValue := fmt.Sprintf("%v", attrs["value"])
		doc := etree.NewDocument()
		if err := doc.ReadFromFile(path); err != nil {
			return "", fmt.Errorf("failed to parse XML: %w", err)
		}
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", fmt.Errorf("no elements found for path '%s'", xpath)
		}
		for _, el := range elements {
			el.CreateAttr(attrName, attrValue)
		}
		doc.Indent(2)
		if err := doc.WriteToFile(path); err != nil {
			return "", fmt.Errorf("failed to write XML: %w", err)
		}
		out, _ := json.Marshal(map[string]interface{}{"updated": len(elements)})
		return string(out), nil

	case "add_element":
		if xpath == "" {
			return "", fmt.Errorf("xpath is required for add_element (selects parent)")
		}
		spec, ok := setValue.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("set_value must be {tag, text?, attributes?} for add_element")
		}
		tag, _ := spec["tag"].(string)
		if tag == "" {
			return "", fmt.Errorf("set_value.tag is required for add_element")
		}
		doc := etree.NewDocument()
		if err := doc.ReadFromFile(path); err != nil {
			return "", fmt.Errorf("failed to parse XML: %w", err)
		}
		parents := doc.FindElements(xpath)
		if len(parents) == 0 {
			return "", fmt.Errorf("no parent elements found for path '%s'", xpath)
		}
		for _, parent := range parents {
			child := parent.CreateElement(tag)
			if text, ok := spec["text"].(string); ok {
				child.SetText(text)
			}
			if childAttrs, ok := spec["attributes"].(map[string]interface{}); ok {
				for k, v := range childAttrs {
					child.CreateAttr(k, fmt.Sprintf("%v", v))
				}
			}
		}
		doc.Indent(2)
		if err := doc.WriteToFile(path); err != nil {
			return "", fmt.Errorf("failed to write XML: %w", err)
		}
		out, _ := json.Marshal(map[string]interface{}{"added_to": len(parents)})
		return string(out), nil

	case "delete":
		if xpath == "" {
			return "", fmt.Errorf("xpath is required for delete")
		}
		doc := etree.NewDocument()
		if err := doc.ReadFromFile(path); err != nil {
			return "", fmt.Errorf("failed to parse XML: %w", err)
		}
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", fmt.Errorf("no elements found for path '%s'", xpath)
		}
		count := 0
		for _, el := range elements {
			if p := el.Parent(); p != nil {
				p.RemoveChild(el)
				count++
			}
		}
		doc.Indent(2)
		if err := doc.WriteToFile(path); err != nil {
			return "", fmt.Errorf("failed to write XML: %w", err)
		}
		out, _ := json.Marshal(map[string]interface{}{"deleted": count})
		return string(out), nil

	case "validate":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		doc := etree.NewDocument()
		if err := doc.ReadFromBytes(data); err != nil {
			return "", fmt.Errorf("invalid XML: %w", err)
		}
		root := doc.Root()
		if root == nil {
			return "", fmt.Errorf("XML document has no root element")
		}
		out, _ := json.Marshal(map[string]interface{}{"valid": true, "root_tag": root.Tag})
		return string(out), nil

	case "format":
		doc := etree.NewDocument()
		if err := doc.ReadFromFile(path); err != nil {
			return "", fmt.Errorf("failed to parse XML: %w", err)
		}
		doc.Indent(2)
		if err := doc.WriteToFile(path); err != nil {
			return "", fmt.Errorf("failed to write XML: %w", err)
		}
		return `{"formatted":true}`, nil

	default:
		return "", fmt.Errorf("unknown xml_editor operation: %s", op)
	}
}
