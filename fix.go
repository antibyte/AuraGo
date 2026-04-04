package main
import \"os\"
import \"strings\"
import \"fmt\"

func main() {
b, _ := os.ReadFile(\"internal/services/optimizer/db.go\")
s := string(b)
s = strings.ReplaceAll(s, \"if _, err := db.Exec(schema); err != nil {\\n\\t}\", \"if _, err := db.Exec(schema); err != nil {\\n\\t\\treturn nil, fmt.Errorf(\\\"failed to initialize schema: %w\\\", err)\\n\\t}\")
s = strings.ReplaceAll(s, \"idx_prompt_overrides_tool_status ON prompt_overrides(tool_name, active, shadow);\\\\n\\\\tCREATE INDEX IF NOT EXISTS idx_traces_tool_version\", \"idx_prompt_overrides_tool_status ON prompt_overrides(tool_name, active, shadow);\\n\\tCREATE INDEX IF NOT EXISTS idx_traces_tool_version\")

s = strings.ReplaceAll(s, \"prompt_version);\\\\n\\\\tCREATE INDEX IF NOT EXISTS idx_traces_timestamp ON tool_traces(timestamp);\", \"prompt_version);\\n\\tCREATE INDEX IF NOT EXISTS idx_traces_timestamp ON tool_traces(timestamp);\")
os.WriteFile(\"internal/services/optimizer/db.go\", []byte(s), 0644)
}
