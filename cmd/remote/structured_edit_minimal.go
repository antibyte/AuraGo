//go:build remote_minimal

package main

import (
	"fmt"

	"aurago/internal/remote"
)

func clientCapabilities() []string {
	out := []string{}
	for _, op := range remote.ClassicCapabilities() {
		if op != remote.OpJsonEdit && op != remote.OpYamlEdit && op != remote.OpXmlEdit {
			out = append(out, op)
		}
	}
	return out
}

func (e *fileOperation) jsonEdit(path, op, jsonPath string, setValue interface{}) (string, error) {
	return "", fmt.Errorf("json_edit is not available in remote_minimal builds")
}

func (e *fileOperation) yamlEdit(path, op, dotPath string, setValue interface{}) (string, error) {
	return "", fmt.Errorf("yaml_edit is not available in remote_minimal builds")
}

func (e *fileOperation) xmlEdit(path, op, xpath string, setValue interface{}) (string, error) {
	return "", fmt.Errorf("xml_edit is not available in remote_minimal builds")
}
