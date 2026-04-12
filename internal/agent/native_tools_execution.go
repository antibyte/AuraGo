package agent

import openai "github.com/sashabaranov/go-openai"

func appendExecutionToolSchemas(tools []openai.Tool, ff ToolFeatureFlags, executePythonDesc string) []openai.Tool {
	if ff.VirusTotalEnabled {
		tools = append(tools, tool("virustotal_scan",
			"Scan a URL, domain, IP address, file hash, or local file using VirusTotal threat intelligence. For local files, you can hash only or upload the file.",
			schema(map[string]interface{}{
				"resource":  prop("string", "The URL, domain, IP address, or file hash to scan with VirusTotal"),
				"file_path": prop("string", "Optional local file path to hash or upload to VirusTotal"),
				"mode": map[string]interface{}{
					"type":        "string",
					"description": "Optional scan mode for local files: auto=hash lookup then upload if unknown, hash=only calculate and look up hashes, upload=force file upload",
					"enum":        []string{"auto", "hash", "upload"},
				},
			}),
		))
	}

	if ff.GolangciLintEnabled {
		tools = append(tools, tool("golangci_lint",
			"Run golangci-lint static analysis on Go source code. Returns a structured list of lint issues. golangci-lint is auto-installed if not present.",
			schema(map[string]interface{}{
				"path":   prop("string", "Package path or directory to lint (e.g. './...', './internal/agent', './cmd/aurago'). Defaults to './...' if omitted."),
				"config": prop("string", "Optional path to a .golangci.yml config file relative to the workspace root. Uses golangci-lint auto-detection if omitted."),
			}),
		))
	}

	// ── Conditionally-included built-in tools ────────────────────────────────

	if ff.AllowFilesystemWrite {
		tools = append(tools,
			tool("file_editor",
				"Precisely edit text files: replace exact strings, insert lines relative to anchors, append/prepend content, or delete line ranges. Safer than write_file for targeted edits because it validates matches.",
				schema(map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "Edit operation to perform",
						"enum":        []string{"str_replace", "str_replace_all", "insert_after", "insert_before", "append", "prepend", "delete_lines"},
					},
					"file_path":  prop("string", "Path to the file to edit"),
					"old":        prop("string", "Exact text to find (required for str_replace/str_replace_all). Must match uniquely for str_replace."),
					"new":        prop("string", "Replacement text (for str_replace/str_replace_all)"),
					"marker":     prop("string", "Anchor text — the line containing this text is the reference point (for insert_after/insert_before). Must match exactly one line."),
					"content":    prop("string", "Text to insert (for insert_after/insert_before/append/prepend)"),
					"start_line": prop("integer", "First line to delete, 1-based (for delete_lines)"),
					"end_line":   prop("integer", "Last line to delete, 1-based inclusive (for delete_lines)"),
				}, "operation", "file_path"),
			),
			tool("json_editor",
				"Read, modify, and validate JSON files using dot-path notation. Get/set/delete values at any depth, list keys, validate syntax, or reformat.",
				schema(map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "JSON operation to perform",
						"enum":        []string{"get", "set", "delete", "keys", "validate", "format"},
					},
					"file_path": prop("string", "Path to the JSON file"),
					"json_path": prop("string", "Dot-separated path to the target value (e.g. 'server.port', 'users.0.name')"),
					"set_value": map[string]interface{}{"description": "Value to set (any JSON type: string, number, boolean, object, array, null). Required for 'set'."},
				}, "operation", "file_path"),
			),
			tool("yaml_editor",
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
			),
			tool("xml_editor",
				"Read, modify, and validate XML files using XPath. Get elements, set text/attributes, add/delete elements, validate, or format.",
				schema(map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "XML operation to perform",
						"enum":        []string{"get", "set_text", "set_attribute", "add_element", "delete", "validate", "format"},
					},
					"file_path": prop("string", "Path to the XML file"),
					"xpath":     prop("string", "XPath expression to select elements (e.g. '//server', './config/database')"),
					"set_value": map[string]interface{}{"description": "Value to set. For set_text: string. For set_attribute: {name, value}. For add_element: {tag, text?, attributes?}."},
				}, "operation", "file_path"),
			),
		)
	} else {
		// Read-only variants: json/yaml/xml editors exposed with read-only operations only
		tools = append(tools,
			tool("json_editor",
				"Read and validate JSON files using dot-path notation. Get values at any depth, list keys, validate syntax, or reformat (read-only — filesystem writes are disabled).",
				schema(map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "Read-only JSON operation to perform",
						"enum":        []string{"get", "keys", "validate", "format"},
					},
					"file_path": prop("string", "Path to the JSON file"),
					"json_path": prop("string", "Dot-separated path to the target value (e.g. 'server.port', 'users.0.name')"),
				}, "operation", "file_path"),
			),
			tool("yaml_editor",
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
			),
			tool("xml_editor",
				"Read and validate XML files using XPath. Get elements, validate, or format (read-only — filesystem writes are disabled).",
				schema(map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "Read-only XML operation to perform",
						"enum":        []string{"get", "validate", "format"},
					},
					"file_path": prop("string", "Path to the XML file"),
					"xpath":     prop("string", "XPath expression to select elements (e.g. '//server', './config/database')"),
				}, "operation", "file_path"),
			),
		)
	}

	if ff.AllowShell {
		tools = append(tools, tool("execute_shell",
			"Run a shell command on the local system. Use for file operations, system info, running programs, etc.",
			schema(map[string]interface{}{
				"command":    prop("string", "The shell command to execute"),
				"background": prop("boolean", "Run as background process (default false)"),
			}, "command"),
		))
	}

	if ff.AllowPython {
		execPythonProps := map[string]interface{}{
			"code":        prop("string", "The complete Python code to execute"),
			"description": prop("string", "Brief description of what this script does"),
			"background":  prop("boolean", "Run as background process (default false)"),
		}
		if ff.PythonSecretInjectionEnabled {
			execPythonProps["vault_keys"] = map[string]interface{}{
				"type":        "array",
				"description": "List of vault secret key names to inject as AURAGO_SECRET_<KEY> environment variables. Only user/agent-created secrets are accessible.",
				"items":       map[string]interface{}{"type": "string"},
			}
			execPythonProps["credential_ids"] = map[string]interface{}{
				"type":        "array",
				"description": "List of credential UUIDs to inject as AURAGO_CRED_<NAME>_USERNAME / _PASSWORD / _TOKEN environment variables. Only credentials with 'allow_python' enabled are accessible.",
				"items":       map[string]interface{}{"type": "string"},
			}
		}
		tools = append(tools, tool("execute_python",
			executePythonDesc,
			schema(execPythonProps, "code"),
		))

		tools = append(tools, tool("save_tool",
			"Save a new Python tool/script to the tools directory and register it in the manifest.",
			schema(map[string]interface{}{
				"name":        prop("string", "Filename for the tool (e.g. 'my_tool.py')"),
				"description": prop("string", "What this tool does"),
				"code":        prop("string", "Complete Python code for the tool"),
			}, "name", "description", "code"),
		))

		tools = append(tools, tool("list_skill_templates",
			"List available skill templates that can be used with create_skill_from_template. Templates provide ready-made Python skill scaffolding for common patterns.",
			schema(map[string]interface{}{}),
		))

		tools = append(tools, tool("create_skill_from_template",
			"Create a new Python skill from a built-in template. The skill is immediately usable via execute_skill. Use list_skill_templates to see all available templates.",
			schema(map[string]interface{}{
				"template":     prop("string", "Template name from list_skill_templates (e.g. api_client, data_transformer, monitor_check, docker_manager, daemon_monitor)"),
				"name":         prop("string", "Unique name for the new skill (e.g. 'weather_api', 'log_parser')"),
				"description":  prop("string", "What this skill does"),
				"url":          prop("string", "Base URL for the API (api_client template only)"),
				"dependencies": map[string]interface{}{"type": "array", "description": "Additional pip packages to install", "items": map[string]interface{}{"type": "string"}},
				"vault_keys":   map[string]interface{}{"type": "array", "description": "Vault secret keys this skill needs at runtime (e.g. API_KEY)", "items": map[string]interface{}{"type": "string"}},
			}, "template", "name"),
		))
	}

	if ff.AllowRemoteShell {
		tools = append(tools, tool("remote_execution",
			"Execute a command on a remote SSH server registered in the inventory.",
			schema(map[string]interface{}{
				"server_id": prop("string", "Server ID or hostname from the inventory"),
				"command":   prop("string", "Shell command to run on the remote server"),
			}, "server_id", "command"),
		))
		tools = append(tools, tool("transfer_remote_file",
			"Transfer a file to or from a remote SSH server registered in the inventory via SFTP. "+
				"The local path must be within the agent workspace.",
			schema(map[string]interface{}{
				"server_id": prop("string", "Server ID or hostname from the inventory"),
				"direction": map[string]interface{}{
					"type":        "string",
					"description": "Transfer direction: 'upload' sends local file to server, 'download' fetches remote file to local workspace",
					"enum":        []string{"upload", "download"},
				},
				"local_path":  prop("string", "Local file path within the agent workspace (source for upload, destination for download)"),
				"remote_path": prop("string", "Remote file path on the target server (destination for upload, source for download)"),
			}, "server_id", "direction", "local_path", "remote_path"),
		))
	}

	if ff.AllowNetworkRequests {
		tools = append(tools, tool("api_request",
			"Make an HTTP request to an external API endpoint.",
			schema(map[string]interface{}{
				"url":    prop("string", "The full URL to request"),
				"method": map[string]interface{}{"type": "string", "description": "HTTP method", "enum": []string{"GET", "POST", "PUT", "PATCH", "DELETE"}},
				"headers": map[string]interface{}{
					"type":        "object",
					"description": "HTTP headers as key-value string pairs",
				},
				"body": prop("string", "Request body (for POST/PUT/PATCH)"),
			}, "url"),
		))
	}

	return tools
}
