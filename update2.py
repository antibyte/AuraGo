with open('internal/agent/native_tools_execution.go', 'r', encoding='utf-8') as f:
    text = f.read()

s2 = '''                        tool("yaml_editor",
                                "Read and validate YAML files using dot-path notation. Get values at any depth, list keys, or validate syntax (read-only — filesystem writes are disabled).",
                                schema(map[string]interface{}{
                                        "operation": map[string]interface{}{
                                                "type":        "string",
                                                "description": "Read-only YAML operation to perform",
                                                "enum":        []string{"get", "keys", "validate"},
                                        },
                                        "file_path": prop("string", "Path to the YAML file"),
                                        "json_path": prop("string", "Dot-separated path to the target value"),
                                }, "operation", "file_path"),
                        ),'''

r2 = s2 + '''
                        tool("toml_editor",
                                "Read and validate TOML files using dot-path notation. Get values at any depth (read-only — filesystem writes are disabled).",
                                schema(map[string]interface{}{
                                        "operation": map[string]interface{}{
                                                "type":        "string",
                                                "description": "Read-only TOML operation to perform",
                                                "enum":        []string{"get"},
                                        },
                                        "file_path": prop("string", "Path to the TOML file"),
                                        "toml_path": prop("string", "Dot-separated path to the target value"),
                                }, "operation", "file_path"),
                        ),'''

if s2 in text:
    text = text.replace(s2, r2)
    with open('internal/agent/native_tools_execution.go', 'w', encoding='utf-8') as f:
        f.write(text)
    print("Replaced read-only toml_editor")
else:
    print("Could not find read-only yaml_editor string")
