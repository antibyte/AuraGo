import re

with open('internal/agent/native_tools_execution.go', 'r', encoding='utf-8') as f:
    text = f.read()

repl = r'''\1
                        tool("toml_editor",
                                "Read, modify, and validate TOML files using dot-path notation. Get/set/delete values at any depth.",
                                schema(map[string]interface{}{
                                        "operation": map[string]interface{}{
                                                "type":        "string",
                                                "description": "TOML operation to perform",
                                                "enum":        []string{"get", "set", "delete"},
                                        },
                                        "file_path": prop("string", "Path to the TOML file"),
                                        "toml_path": prop("string", "Dot-separated path to the target value"),
                                        "set_value": map[string]interface{}{"description": "Value to set (any type). Required for 'set'."},
                                }, "operation", "file_path"),
                        ),'''

# Replace yaml_editor with itself plus toml_editor
text = re.sub(r'(\s*tool\("yaml_editor",[\s\S]+?\}, "operation", "file_path"\),\n)', repl, text)

with open('internal/agent/native_tools_execution.go', 'w', encoding='utf-8') as f:
    f.write(text)
