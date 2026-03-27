import sys

with open('internal/agent/agent_loop.go', 'r', encoding='utf-8') as f:
    content = f.read()

t = '\t'
old = (
    t*6 + '// MiniMaxFix mode: suppress any chunk that contains tool call JSON\n' +
    t*6 + '// (MiniMax puts tool calls in delta.Content, not delta.ToolCalls).\n' +
    t*6 + '// The heuristic checks: starts with \'{\' AND contains a tool keyword.\n' +
    t*6 + 'trimmed := strings.TrimLeft(delta.Content, " \\t\\r\\n")\n' +
    t*6 + 'isLikelyToolCallJSON := len(trimmed) > 0 && trimmed[0] == \'{\' &&\n' +
    t*7 + '(strings.Contains(trimmed, `"action"`) || strings.Contains(trimmed, `"command"`) ||\n' +
    t*8 + 'strings.Contains(trimmed, `"operation"`) || strings.Contains(trimmed, `"tool"`) ||\n' +
    t*8 + 'strings.Contains(trimmed, `"name"`) || strings.Contains(trimmed, `"arguments"`))\n' +
    t*6 + 'suppressForMiniMax := cfg.LLM.MiniMaxFix && isLikelyToolCallJSON'
)

new = (
    t*6 + '// Suppress JSON tool-call chunks so they never render as chat text.\n' +
    t*6 + '// {"tool_call":...} / {"tool_name":...} are always suppressed (MiniMax format).\n' +
    t*6 + '// Broader JSON heuristics (action/command/...) only suppressed when MiniMaxFix=true.\n' +
    t*6 + 'trimmed := strings.TrimLeft(delta.Content, " \\t\\r\\n")\n' +
    t*6 + 'isToolCallJSON := len(trimmed) > 0 && trimmed[0] == \'{\' &&\n' +
    t*7 + '(strings.Contains(trimmed, `"tool_call"`) || strings.Contains(trimmed, `"tool_name"`))\n' +
    t*6 + 'isLikelyToolCallJSON := len(trimmed) > 0 && trimmed[0] == \'{\' &&\n' +
    t*7 + '(strings.Contains(trimmed, `"action"`) || strings.Contains(trimmed, `"command"`) ||\n' +
    t*8 + 'strings.Contains(trimmed, `"operation"`) || strings.Contains(trimmed, `"tool_call"`) ||\n' +
    t*8 + 'strings.Contains(trimmed, `"tool"`) || strings.Contains(trimmed, `"name"`) ||\n' +
    t*8 + 'strings.Contains(trimmed, `"arguments"`))\n' +
    t*6 + 'suppressForMiniMax := isToolCallJSON || (cfg.LLM.MiniMaxFix && isLikelyToolCallJSON)'
)

if old in content:
    content = content.replace(old, new, 1)
    with open('internal/agent/agent_loop.go', 'w', encoding='utf-8') as f:
        f.write(content)
    print('REPLACED OK')
else:
    print('NOT FOUND - showing diff context:')
    idx = content.find('suppressForMiniMax := cfg.LLM.MiniMaxFix')
    print(repr(content[idx-200:idx+50]))
