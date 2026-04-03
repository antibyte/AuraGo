with open('internal/agent/native_tools_execution.go', 'r', encoding='utf-8') as f:
    text = f.read()

s1 = '''                        tool("yaml_editor",
                                "Read, modify, and validate YAML files using dot-path notation. Get/set/delete values at any depth, list keys, or validate syntax. Preserves YAML structure.",
                                schema(map[string]interface{}{
                                        "operation": map[string]interface{}{
                                                "type":        "string",
                                                "description": "YAML operation to perform",
                                                "enum":        []string{"get", "set", "delete", "keys", "validate"},
                                        },
                                        "file_path": prop("string", "Path to the YAML file"),
                                        "json_path": prop("string", "Dot-separated path to the target value (e.g. 'server.port', 'database.host')"),
                                        "set_value": map[string]interface{}{"description": "Value to set (any type). Required for 'set'."},
                                }, "operation", "file_path"),
                        ),'''

r1 = s1 + '''
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

# Wait, udget_status and certificate_manager before the conditionally-included built-in tools
s3 = '''        // -- Conditionally-included built-in tools --------------------------------'''

r3 = '''        tools = append(tools, tool("budget_status",
                "Returns the agent's current token budget status and usage statistics.",
                schema(map[string]interface{}{}),
        ))

        tools = append(tools, tool("certificate_manager",
                "Manage agent-specific SSL/TLS certificates. Generates, inspects, and renews self-signed certificates. Keys are securely stored in the Vault under the restrictive 'agent_cert_' namespace.",
                schema(map[string]interface{}{
                        "operation": map[string]interface{}{
                                "type":        "string",
                                "description": "Operation to perform: 'generate_self_signed', 'check_expiry', 'inspect', or 'renew'",
                                "enum":        []string{"generate_self_signed", "check_expiry", "inspect", "renew"},
                        },
                        "cert_name": prop("string", "Name of the certificate in the Vault (must start with 'agent_cert_')"),
                }, "operation", "cert_name"),
        ))

        // -- Conditionally-included built-in tools --------------------------------'''

text = text.replace(s1, r1)
text = text.replace(s3, r3)

with open('internal/agent/native_tools_execution.go', 'w', encoding='utf-8') as f:
    f.write(text)

