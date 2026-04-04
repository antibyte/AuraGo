import pathlib
p = pathlib.Path('internal/services/optimizer/db.go')
text = p.read_text(encoding='utf-8')
import re
text = re.sub(r'if _, err := db\.Exec\(schema\); err != nil \{\s*\}', 'if _, err := db.Exec(schema); err != nil {\\n\\t\\treturn nil, fmt.Errorf(\"failed to initialize schema: %w\", err)\\n\\t}', text)
text = text.replace('CREATE INDEX IF NOT EXISTS idx_prompt_overrides_tool_status ON prompt_overrides(tool_name, active, shadow);', 'CREATE INDEX IF NOT EXISTS idx_prompt_overrides_tool_status ON prompt_overrides(tool_name, active, shadow);\\n\\tCREATE INDEX IF NOT EXISTS idx_traces_tool_version ON tool_traces(tool_name, prompt_version);\\n\\tCREATE INDEX IF NOT EXISTS idx_traces_timestamp ON tool_traces(timestamp);')
p.write_text(text, encoding='utf-8')
