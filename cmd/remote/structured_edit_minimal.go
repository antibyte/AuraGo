//go:build remote_minimal

package main

import "fmt"

func (e *Executor) jsonEdit(path, op, jsonPath string, setValue interface{}) (string, error) {
	return "", fmt.Errorf("json_edit is not available in remote_minimal builds")
}

func (e *Executor) yamlEdit(path, op, dotPath string, setValue interface{}) (string, error) {
	return "", fmt.Errorf("yaml_edit is not available in remote_minimal builds")
}

func (e *Executor) xmlEdit(path, op, xpath string, setValue interface{}) (string, error) {
	return "", fmt.Errorf("xml_edit is not available in remote_minimal builds")
}
