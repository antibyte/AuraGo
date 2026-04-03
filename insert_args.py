with open('internal/agent/tool_args_execution.go', 'r', encoding='utf-8') as f:
    text = f.read()

args = '''type tomlEditorArgs struct {
        Operation string
        FilePath  string
        TomlPath  string
        SetValue  interface{}
}

func decodeTOMLEditorArgs(tc ToolCall) tomlEditorArgs {
        req := tomlEditorArgs{
                Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
                FilePath:  firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "file_path", "path")),
                TomlPath:  firstNonEmptyToolString(toolArgString(tc.Params, "toml_path"), toolArgString(tc.Params, "json_path")),
        }
        if value, ok := toolArgRaw(tc.Params, "set_value"); ok {
                req.SetValue = value
        }
        return req
}

'''

idx = text.find('func decodeYAMLEditorArgs')
if idx != -1:
    text = text[:idx] + args + text[idx:]
    with open('internal/agent/tool_args_execution.go', 'w', encoding='utf-8') as f:
        f.write(text)
    print("Added TOML args mapping")
else:
    print("Could not find decodeYAMLEditorArgs")
