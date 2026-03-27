with open('internal/agent/agent_loop.go', 'r', encoding='utf-8') as f:
    content = f.read()

t = '\t'
old = (
    t*3 + '// Persist tool call to history: native path synthesizes a text representation\n' +
    t*3 + 'histContent := content\n' +
    '\n' +
    t*3 + '// Decide if this message should be hidden from the UI history endpoint.\n' +
    t*3 + '// Hide it if it\'s purely a synthetic JSON string (e.g. no text, only tool call),\n' +
    t*3 + '// but show it if the LLM provided conversational text.\n' +
    t*3 + 'isMsgInternal := true\n' +
    t*3 + 'if strings.TrimSpace(content) != "" && !strings.HasPrefix(strings.TrimSpace(content), "{") {\n' +
    t*4 + 'isMsgInternal = false\n' +
    t*3 + '}'
)

new = (
    t*3 + '// Persist tool call to history: native path synthesizes a text representation\n' +
    t*3 + 'histContent := content\n' +
    '\n' +
    t*3 + '// Strip <think> reasoning blocks — never store them in history.\n' +
    t*3 + 'histContent = security.StripThinkingTags(histContent)\n' +
    '\n' +
    t*3 + '// When the LLM mixes conversational text with a trailing JSON tool call\n' +
    t*3 + '// (e.g. "Done!\\n\\n{\\"tool_call\\":\\"deploy\\"}"), keep only the text portion\n' +
    t*3 + '// so the raw JSON never appears as a chat message in history.\n' +
    t*3 + 'if !useNativePath {\n' +
    t*4 + 'if jsonIdx := strings.Index(histContent, "{"); jsonIdx > 0 {\n' +
    t*5 + 'textPart := strings.TrimSpace(histContent[:jsonIdx])\n' +
    t*5 + 'if textPart != "" {\n' +
    t*6 + 'histContent = textPart\n' +
    t*5 + '}\n' +
    t*4 + '}\n' +
    t*3 + '}\n' +
    '\n' +
    t*3 + '// Decide if this message should be hidden from the UI history endpoint.\n' +
    t*3 + '// Hide it if it\'s purely a synthetic JSON string (e.g. no text, only tool call),\n' +
    t*3 + '// but show it if the LLM provided conversational text.\n' +
    t*3 + 'isMsgInternal := true\n' +
    t*3 + 'if strings.TrimSpace(histContent) != "" && !strings.HasPrefix(strings.TrimSpace(histContent), "{") {\n' +
    t*4 + 'isMsgInternal = false\n' +
    t*3 + '}'
)

if old in content:
    content = content.replace(old, new, 1)
    with open('internal/agent/agent_loop.go', 'w', encoding='utf-8') as f:
        f.write(content)
    print('REPLACED OK')
else:
    print('NOT FOUND - context:')
    idx = content.find('isMsgInternal := true')
    if idx >= 0:
        print(repr(content[idx-300:idx+200]))
    else:
        print('isMsgInternal not found either')
