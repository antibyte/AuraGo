package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/beevik/etree"
)

// XmlEditorResult is the JSON response returned for xml_editor operations.
type XmlEditorResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ExecuteXmlEditor handles XML file editing operations, sandboxed to workspaceDir.
// xpath uses etree's path syntax: e.g. "//server", "./config/database[@name='main']"
func ExecuteXmlEditor(operation, filePath, xpath string, value interface{}, workspaceDir string) string {
	encode := func(r XmlEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if filePath == "" {
		return encode(XmlEditorResult{Status: "error", Message: "'file_path' is required"})
	}

	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return encode(XmlEditorResult{Status: "error", Message: err.Error()})
	}

	switch operation {
	case "get":
		return xmlGet(resolved, xpath, encode)
	case "set_text":
		return xmlSetText(resolved, xpath, value, encode)
	case "set_attribute":
		return xmlSetAttribute(resolved, xpath, value, encode)
	case "add_element":
		return xmlAddElement(resolved, xpath, value, encode)
	case "delete":
		return xmlDelete(resolved, xpath, encode)
	case "validate":
		return xmlValidate(resolved, encode)
	case "format":
		return xmlFormat(resolved, encode)
	default:
		return encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("Unknown xml_editor operation '%s'. Valid: get, set_text, set_attribute, add_element, delete, validate, format", operation)})
	}
}

func xmlLoadDoc(resolved string) (*etree.Document, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromFile(resolved); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}
	return doc, nil
}

func xmlSaveDoc(doc *etree.Document, resolved string) error {
	doc.Indent(2)
	data, err := doc.WriteToBytes()
	if err != nil {
		return fmt.Errorf("failed to serialize XML: %w", err)
	}
	return writeFileAtomic(resolved, data)
}

func xmlGet(resolved, xpath string, encode func(XmlEditorResult) string) string {
	if xpath == "" {
		return encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for get"})
	}

	doc, err := xmlLoadDoc(resolved)
	if err != nil {
		return encode(XmlEditorResult{Status: "error", Message: err.Error()})
	}

	elements := doc.FindElements(xpath)
	if len(elements) == 0 {
		return encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No elements found for path '%s'", xpath)})
	}

	var results []map[string]interface{}
	for _, el := range elements {
		entry := map[string]interface{}{
			"tag":  el.Tag,
			"text": strings.TrimSpace(el.Text()),
		}
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
		if len(el.ChildElements()) > 0 {
			var children []string
			for _, ch := range el.ChildElements() {
				children = append(children, ch.Tag)
			}
			entry["children"] = children
		}
		results = append(results, entry)
	}

	if len(results) == 1 {
		return encode(XmlEditorResult{Status: "success", Data: results[0]})
	}
	return encode(XmlEditorResult{Status: "success", Data: results})
}

func xmlSetText(resolved, xpath string, value interface{}, encode func(XmlEditorResult) string) string {
	if xpath == "" {
		return encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for set_text"})
	}

	text, ok := value.(string)
	if !ok {
		if value == nil {
			text = ""
		} else {
			text = fmt.Sprintf("%v", value)
		}
	}

	doc, err := xmlLoadDoc(resolved)
	if err != nil {
		return encode(XmlEditorResult{Status: "error", Message: err.Error()})
	}

	elements := doc.FindElements(xpath)
	if len(elements) == 0 {
		return encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No elements found for path '%s'", xpath)})
	}

	for _, el := range elements {
		el.SetText(text)
	}

	if err := xmlSaveDoc(doc, resolved); err != nil {
		return encode(XmlEditorResult{Status: "error", Message: err.Error()})
	}

	return encode(XmlEditorResult{Status: "success", Message: fmt.Sprintf("Set text on %d element(s)", len(elements))})
}

func xmlSetAttribute(resolved, xpath string, value interface{}, encode func(XmlEditorResult) string) string {
	if xpath == "" {
		return encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for set_attribute"})
	}

	// value should be a map with "name" and "value" keys
	attrs, ok := value.(map[string]interface{})
	if !ok {
		return encode(XmlEditorResult{Status: "error", Message: "set_value must be an object with 'name' and 'value' keys for set_attribute"})
	}
	attrName, _ := attrs["name"].(string)
	if attrName == "" {
		return encode(XmlEditorResult{Status: "error", Message: "set_value.name is required for set_attribute"})
	}
	attrValue := fmt.Sprintf("%v", attrs["value"])

	doc, err := xmlLoadDoc(resolved)
	if err != nil {
		return encode(XmlEditorResult{Status: "error", Message: err.Error()})
	}

	elements := doc.FindElements(xpath)
	if len(elements) == 0 {
		return encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No elements found for path '%s'", xpath)})
	}

	for _, el := range elements {
		el.CreateAttr(attrName, attrValue)
	}

	if err := xmlSaveDoc(doc, resolved); err != nil {
		return encode(XmlEditorResult{Status: "error", Message: err.Error()})
	}

	return encode(XmlEditorResult{Status: "success", Message: fmt.Sprintf("Set attribute '%s' on %d element(s)", attrName, len(elements))})
}

func xmlAddElement(resolved, xpath string, value interface{}, encode func(XmlEditorResult) string) string {
	if xpath == "" {
		return encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for add_element (selects parent)"})
	}

	// value should be a map with "tag", optional "text", optional "attributes"
	spec, ok := value.(map[string]interface{})
	if !ok {
		return encode(XmlEditorResult{Status: "error", Message: "set_value must be an object with 'tag' (and optionally 'text', 'attributes') for add_element"})
	}
	tag, _ := spec["tag"].(string)
	if tag == "" {
		return encode(XmlEditorResult{Status: "error", Message: "set_value.tag is required for add_element"})
	}

	doc, err := xmlLoadDoc(resolved)
	if err != nil {
		return encode(XmlEditorResult{Status: "error", Message: err.Error()})
	}

	parents := doc.FindElements(xpath)
	if len(parents) == 0 {
		return encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No parent elements found for path '%s'", xpath)})
	}

	for _, parent := range parents {
		child := parent.CreateElement(tag)
		if text, ok := spec["text"].(string); ok {
			child.SetText(text)
		}
		if attrs, ok := spec["attributes"].(map[string]interface{}); ok {
			for k, v := range attrs {
				child.CreateAttr(k, fmt.Sprintf("%v", v))
			}
		}
	}

	if err := xmlSaveDoc(doc, resolved); err != nil {
		return encode(XmlEditorResult{Status: "error", Message: err.Error()})
	}

	return encode(XmlEditorResult{Status: "success", Message: fmt.Sprintf("Added <%s> to %d parent(s)", tag, len(parents))})
}

func xmlDelete(resolved, xpath string, encode func(XmlEditorResult) string) string {
	if xpath == "" {
		return encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for delete"})
	}

	doc, err := xmlLoadDoc(resolved)
	if err != nil {
		return encode(XmlEditorResult{Status: "error", Message: err.Error()})
	}

	elements := doc.FindElements(xpath)
	if len(elements) == 0 {
		return encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No elements found for path '%s'", xpath)})
	}

	count := 0
	for _, el := range elements {
		parent := el.Parent()
		if parent != nil {
			parent.RemoveChild(el)
			count++
		}
	}

	if err := xmlSaveDoc(doc, resolved); err != nil {
		return encode(XmlEditorResult{Status: "error", Message: err.Error()})
	}

	return encode(XmlEditorResult{Status: "success", Message: fmt.Sprintf("Deleted %d element(s)", count)})
}

func xmlValidate(resolved string, encode func(XmlEditorResult) string) string {
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(data); err != nil {
		return encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("Invalid XML: %v", err)})
	}

	root := doc.Root()
	if root == nil {
		return encode(XmlEditorResult{Status: "error", Message: "XML document has no root element"})
	}

	// Count elements
	var countElements func(el *etree.Element) int
	countElements = func(el *etree.Element) int {
		n := 1
		for _, ch := range el.ChildElements() {
			n += countElements(ch)
		}
		return n
	}

	return encode(XmlEditorResult{
		Status:  "success",
		Message: "Valid XML",
		Data: map[string]interface{}{
			"root_tag":      root.Tag,
			"element_count": countElements(root),
		},
	})
}

func xmlFormat(resolved string, encode func(XmlEditorResult) string) string {
	doc, err := xmlLoadDoc(resolved)
	if err != nil {
		return encode(XmlEditorResult{Status: "error", Message: err.Error()})
	}

	if err := xmlSaveDoc(doc, resolved); err != nil {
		return encode(XmlEditorResult{Status: "error", Message: err.Error()})
	}

	return encode(XmlEditorResult{Status: "success", Message: "File formatted"})
}
