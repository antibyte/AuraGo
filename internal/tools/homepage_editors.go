package tools

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/beevik/etree"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v3"
)

// ─── Structured File Editors ──────────────────────────────────────────────

// extractDockerOutput extracts the output text from a DockerExec JSON response.
func extractDockerOutput(result string) string {
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resp); err == nil {
		if status, ok := resp["status"].(string); ok && status == "error" {
			return ""
		}
		if output, ok := resp["output"].(string); ok {
			return output
		}
	}
	return result
}

// HomepageJsonEdit edits a JSON file inside the homepage container (or local workspace).
func HomepageJsonEdit(cfg HomepageConfig, path, operation, jsonPath string, setValue interface{}, content string, logger *slog.Logger) string {
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	logger.Info("[Homepage] JsonEdit", "path", path, "op", operation)

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		fullPath, err := resolveHomepagePath(cfg.WorkspacePath, path)
		if err != nil {
			return errJSON("%v", err)
		}
		return ExecuteJsonEditor(operation, fullPath, jsonPath, setValue, content, cfg.WorkspacePath)
	}

	// Docker: read from container
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	readResult := DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cat /workspace/%s", path), "")
	fileContent := extractDockerOutput(readResult)
	if fileContent == "" && operation != "set" {
		return errJSON("could not read file from container")
	}

	// Apply JSON operation on content
	result, edited, err := applyHomepageJsonEdit(fileContent, operation, jsonPath, setValue)
	if err != "" {
		return err
	}

	// For read-only operations, return the result
	switch operation {
	case "get", "keys", "validate":
		return result
	}

	// Write edited content back
	encoded := base64.StdEncoding.EncodeToString([]byte(edited))
	dir := filepath.Dir(path)
	writeCmd := fmt.Sprintf("mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s", dir, encoded, path)
	DockerExec(dockerCfg, homepageContainerName, writeCmd, "")
	return result
}

// applyHomepageJsonEdit performs JSON operations on in-memory content.
// Returns (resultJSON, editedContent, errorJSON). errorJSON is empty on success.
func applyHomepageJsonEdit(content, operation, jsonPath string, setValue interface{}) (string, string, string) {
	encode := func(r JsonEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	switch operation {
	case "get":
		if !gjson.Valid(content) {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "not valid JSON"})
		}
		if jsonPath == "" {
			return encode(JsonEditorResult{Status: "ok", Data: json.RawMessage(content)}), "", ""
		}
		r := gjson.Get(content, jsonPath)
		if !r.Exists() {
			return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("path '%s' not found", jsonPath)})
		}
		return encode(JsonEditorResult{Status: "ok", Data: json.RawMessage(r.Raw)}), "", ""

	case "set":
		if jsonPath == "" {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "'json_path' is required for set"})
		}
		data := content
		if data == "" {
			data = "{}"
		}
		if !gjson.Valid(data) {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "not valid JSON"})
		}
		updated, err := sjson.Set(data, jsonPath, setValue)
		if err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: err.Error()})
		}
		var raw json.RawMessage = []byte(updated)
		formatted, _ := json.MarshalIndent(raw, "", "  ")
		return encode(JsonEditorResult{Status: "ok", Message: fmt.Sprintf("set '%s'", jsonPath)}), string(formatted) + "\n", ""

	case "delete":
		if jsonPath == "" {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "'json_path' is required for delete"})
		}
		if !gjson.Valid(content) {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "not valid JSON"})
		}
		updated, err := sjson.Delete(content, jsonPath)
		if err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: err.Error()})
		}
		var raw json.RawMessage = []byte(updated)
		formatted, _ := json.MarshalIndent(raw, "", "  ")
		return encode(JsonEditorResult{Status: "ok", Message: fmt.Sprintf("deleted '%s'", jsonPath)}), string(formatted) + "\n", ""

	case "keys":
		if !gjson.Valid(content) {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "not valid JSON"})
		}
		target := content
		if jsonPath != "" {
			r := gjson.Get(content, jsonPath)
			if !r.Exists() {
				return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("path '%s' not found", jsonPath)})
			}
			target = r.Raw
		}
		var keys []string
		gjson.Parse(target).ForEach(func(key, _ gjson.Result) bool {
			keys = append(keys, key.String())
			return true
		})
		return encode(JsonEditorResult{Status: "ok", Data: keys}), "", ""

	case "validate":
		if gjson.Valid(content) {
			return encode(JsonEditorResult{Status: "ok", Message: "valid JSON", Data: true}), "", ""
		}
		return encode(JsonEditorResult{Status: "ok", Message: "invalid JSON", Data: false}), "", ""

	case "format":
		if !gjson.Valid(content) {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "not valid JSON"})
		}
		var raw json.RawMessage = []byte(content)
		formatted, _ := json.MarshalIndent(raw, "", "  ")
		return encode(JsonEditorResult{Status: "ok", Message: "formatted"}), string(formatted) + "\n", ""

	default:
		return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("unknown operation: %s", operation)})
	}
}

// HomepageYamlEdit edits a YAML file inside the homepage container (or local workspace).
func HomepageYamlEdit(cfg HomepageConfig, path, operation, dotPath string, setValue interface{}, logger *slog.Logger) string {
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	logger.Info("[Homepage] YamlEdit", "path", path, "op", operation)

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		fullPath, err := resolveHomepagePath(cfg.WorkspacePath, path)
		if err != nil {
			return errJSON("%v", err)
		}
		return ExecuteYamlEditor(operation, fullPath, dotPath, setValue, cfg.WorkspacePath)
	}

	// Docker: read from container
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	readResult := DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cat /workspace/%s", path), "")
	fileContent := extractDockerOutput(readResult)
	if fileContent == "" && operation != "set" {
		return errJSON("could not read file from container")
	}

	// Apply YAML operation on content
	result, edited, err := applyHomepageYamlEdit(fileContent, operation, dotPath, setValue)
	if err != "" {
		return err
	}

	// For read-only operations, return the result
	switch operation {
	case "get", "keys", "validate":
		return result
	}

	// Write edited content back
	encoded := base64.StdEncoding.EncodeToString([]byte(edited))
	dir := filepath.Dir(path)
	writeCmd := fmt.Sprintf("mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s", dir, encoded, path)
	DockerExec(dockerCfg, homepageContainerName, writeCmd, "")
	return result
}

// applyHomepageYamlEdit performs YAML operations on in-memory content.
// Returns (resultJSON, editedContent, errorJSON). errorJSON is empty on success.
func applyHomepageYamlEdit(content, operation, dotPath string, setValue interface{}) (string, string, string) {
	encode := func(r JsonEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	switch operation {
	case "get":
		var doc interface{}
		if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "invalid YAML: " + err.Error()})
		}
		if dotPath == "" {
			return encode(JsonEditorResult{Status: "ok", Data: doc}), "", ""
		}
		val, found := yamlNavigate(doc, strings.Split(dotPath, "."))
		if !found {
			return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("path '%s' not found", dotPath)})
		}
		return encode(JsonEditorResult{Status: "ok", Data: val}), "", ""

	case "set":
		if dotPath == "" {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "'json_path' is required for set"})
		}
		var node yaml.Node
		data := []byte(content)
		if len(data) > 0 {
			if err := yaml.Unmarshal(data, &node); err != nil {
				return "", "", encode(JsonEditorResult{Status: "error", Message: "invalid YAML: " + err.Error()})
			}
		}
		if node.Kind == 0 {
			node = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
		}
		parts := strings.Split(dotPath, ".")
		if err := yamlNodeSet(&node, parts, setValue); err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: err.Error()})
		}
		out, err := yaml.Marshal(&node)
		if err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(JsonEditorResult{Status: "ok", Message: fmt.Sprintf("set '%s'", dotPath)}), string(out), ""

	case "delete":
		if dotPath == "" {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "'json_path' is required for delete"})
		}
		var node yaml.Node
		if err := yaml.Unmarshal([]byte(content), &node); err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "invalid YAML: " + err.Error()})
		}
		parts := strings.Split(dotPath, ".")
		if !yamlNodeDelete(&node, parts) {
			return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("path '%s' not found", dotPath)})
		}
		out, err := yaml.Marshal(&node)
		if err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(JsonEditorResult{Status: "ok", Message: fmt.Sprintf("deleted '%s'", dotPath)}), string(out), ""

	case "keys":
		var doc interface{}
		if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "invalid YAML: " + err.Error()})
		}
		target := doc
		if dotPath != "" {
			val, found := yamlNavigate(doc, strings.Split(dotPath, "."))
			if !found {
				return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("path '%s' not found", dotPath)})
			}
			target = val
		}
		m, ok := target.(map[string]interface{})
		if !ok {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "value at path is not a mapping"})
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		return encode(JsonEditorResult{Status: "ok", Data: keys}), "", ""

	case "validate":
		var doc interface{}
		if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
			return encode(JsonEditorResult{Status: "ok", Message: "invalid YAML", Data: false}), "", ""
		}
		return encode(JsonEditorResult{Status: "ok", Message: "valid YAML", Data: true}), "", ""

	default:
		return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("unknown operation: %s", operation)})
	}
}

// HomepageXmlEdit performs XML editing operations on homepage project files.
func HomepageXmlEdit(cfg HomepageConfig, path, operation, xpath string, setValue interface{}, logger *slog.Logger) string {
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	logger.Info("[Homepage] XmlEdit", "path", path, "op", operation)

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		fullPath, err := resolveHomepagePath(cfg.WorkspacePath, path)
		if err != nil {
			return errJSON("%v", err)
		}
		return ExecuteXmlEditor(operation, fullPath, xpath, setValue, cfg.WorkspacePath)
	}

	// Docker: read from container
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	readResult := DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cat /workspace/%s", path), "")
	fileContent := extractDockerOutput(readResult)
	if fileContent == "" && operation != "add_element" {
		return errJSON("could not read file from container")
	}

	// Apply XML operation on content
	result, edited, errMsg := applyHomepageXmlEdit(fileContent, operation, xpath, setValue)
	if errMsg != "" {
		return errMsg
	}

	// For read-only operations, return the result
	switch operation {
	case "get", "validate":
		return result
	}

	// Write edited content back
	encoded := base64.StdEncoding.EncodeToString([]byte(edited))
	dir := filepath.Dir(path)
	writeCmd := fmt.Sprintf("mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s", dir, encoded, path)
	DockerExec(dockerCfg, homepageContainerName, writeCmd, "")
	return result
}

// applyHomepageXmlEdit performs XML operations on in-memory content.
func applyHomepageXmlEdit(content, operation, xpath string, setValue interface{}) (string, string, string) {
	encode := func(r XmlEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	doc := etree.NewDocument()
	if content != "" {
		if err := doc.ReadFromString(content); err != nil {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "invalid XML: " + err.Error()})
		}
	}

	switch operation {
	case "get":
		if xpath == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for get"})
		}
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", "", encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No elements found for path '%s'", xpath)})
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
			return encode(XmlEditorResult{Status: "success", Data: results[0]}), "", ""
		}
		return encode(XmlEditorResult{Status: "success", Data: results}), "", ""

	case "set_text":
		if xpath == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for set_text"})
		}
		text := fmt.Sprintf("%v", setValue)
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", "", encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No elements found for path '%s'", xpath)})
		}
		for _, el := range elements {
			el.SetText(text)
		}
		doc.Indent(2)
		output, err := doc.WriteToString()
		if err != nil {
			return "", "", encode(XmlEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(XmlEditorResult{Status: "success", Message: fmt.Sprintf("Set text on %d element(s)", len(elements))}), output, ""

	case "set_attribute":
		if xpath == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for set_attribute"})
		}
		attrs, ok := setValue.(map[string]interface{})
		if !ok {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "set_value must be {name, value}"})
		}
		attrName, _ := attrs["name"].(string)
		if attrName == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "set_value.name is required"})
		}
		attrValue := fmt.Sprintf("%v", attrs["value"])
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", "", encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No elements found for path '%s'", xpath)})
		}
		for _, el := range elements {
			el.CreateAttr(attrName, attrValue)
		}
		doc.Indent(2)
		output, err := doc.WriteToString()
		if err != nil {
			return "", "", encode(XmlEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(XmlEditorResult{Status: "success", Message: fmt.Sprintf("Set attribute '%s' on %d element(s)", attrName, len(elements))}), output, ""

	case "add_element":
		if xpath == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for add_element"})
		}
		spec, ok := setValue.(map[string]interface{})
		if !ok {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "set_value must be {tag, text?, attributes?}"})
		}
		tag, _ := spec["tag"].(string)
		if tag == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "set_value.tag is required"})
		}
		parents := doc.FindElements(xpath)
		if len(parents) == 0 {
			return "", "", encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No parent elements found for path '%s'", xpath)})
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
		output, err := doc.WriteToString()
		if err != nil {
			return "", "", encode(XmlEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(XmlEditorResult{Status: "success", Message: fmt.Sprintf("Added <%s> to %d parent(s)", tag, len(parents))}), output, ""

	case "delete":
		if xpath == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for delete"})
		}
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", "", encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No elements found for path '%s'", xpath)})
		}
		count := 0
		for _, el := range elements {
			if p := el.Parent(); p != nil {
				p.RemoveChild(el)
				count++
			}
		}
		doc.Indent(2)
		output, err := doc.WriteToString()
		if err != nil {
			return "", "", encode(XmlEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(XmlEditorResult{Status: "success", Message: fmt.Sprintf("Deleted %d element(s)", count)}), output, ""

	case "validate":
		root := doc.Root()
		if root == nil {
			return encode(XmlEditorResult{Status: "error", Message: "XML document has no root element"}), "", ""
		}
		return encode(XmlEditorResult{Status: "success", Message: "Valid XML", Data: map[string]interface{}{"root_tag": root.Tag}}), "", ""

	case "format":
		doc.Indent(2)
		output, err := doc.WriteToString()
		if err != nil {
			return "", "", encode(XmlEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(XmlEditorResult{Status: "success", Message: "File formatted"}), output, ""

	default:
		return "", "", encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("unknown operation: %s", operation)})
	}
}
