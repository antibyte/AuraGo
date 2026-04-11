#!/usr/bin/env python3
"""Fix concrete tool name examples in agent_loop.go recovery feedback."""

filepath = 'internal/agent/agent_loop.go'

with open(filepath, 'rb') as f:
    content = f.read()

# Check if already patched
if b'CHANGE LOG 2026-04-11: Removed concrete example tool names' in content:
    print("Already patched")
    exit(0)

# Find the start of the section
idx_start = content.find(b'incompleteToolCallCount >= 2 {')
if idx_start < 0:
    print("ERROR: incompleteToolCallCount >= 2 { not found")
    exit(1)

# Find the Example: read_file marker
idx_read_file = content.find(b'Example: {\\"action\\": \\"read_file\\", \\"file_path\\": \\"/etc/caddy/Caddyfile\\"}"')
if idx_read_file < 0:
    print("ERROR: Example: read_file not found")
    exit(1)

# Find the Example: system_metrics marker  
idx_sys_metrics = content.find(b'Example: {\\"action\\": \\"system_metrics\\"}"')
if idx_sys_metrics < 0:
    print("ERROR: Example: system_metrics not found")
    exit(1)

print(f"read_file at {idx_read_file}, system_metrics at {idx_sys_metrics}")

# The section goes from idx_start to the end of the system_metrics line
# Find the closing quote after system_metrics
# Then find the newline after that quote
idx_closing_quote = content.find(b'"', idx_sys_metrics + 50)
if idx_closing_quote < 0:
    print("ERROR: closing quote not found")
    exit(1)

# Find the newline after the closing quote
idx_newline = content.find(b'\n', idx_closing_quote + 1)
if idx_newline < 0:
    print("ERROR: newline not found")
    exit(1)

# section_end is the character after the newline
section_end = idx_newline + 1

print(f"Section from {idx_start} to {section_end}")

old_section = content[idx_start:section_end]
print(f"Old section length: {len(old_section)} bytes")

# Verify
if b'incompleteToolCallCount >= 2' not in old_section:
    print("ERROR: incompleteToolCallCount not in section")
    exit(1)
if b'read_file' not in old_section:
    print("ERROR: read_file not in section")
    exit(1)
if b'system_metrics' not in old_section:
    print("ERROR: system_metrics not in section")
    exit(1)

# Now construct the replacement
new_section = (
    b'\t\t\t} else if incompleteToolCallCount >= 2 {\n'
    b'\t\t\t\t// Escalate on second attempt - be very explicit about tool_call tags\n'
    b'\t\t\t\t// CHANGE LOG 2026-04-11: Removed concrete example tool names to prevent hallucination.\n'
    b'\t\t\t\t// Models that understand the format don\'t need examples; models that don\'t won\'t be helped.\n'
    b'\t\t\t\tfeedbackMsg = "CRITICAL ERROR: You sent \'<tool_call>\' as raw text again. '
    b'This is not a valid tool call format. Do NOT output any XML tags at all. '
    b'Output a raw JSON object starting with \'{\'."\n'
    b'\t\t\t} else {\n'
    b'\t\t\t\t// CHANGE LOG 2026-04-11: Removed concrete example tool names to prevent hallucination.\n'
    b'\t\t\t\tfeedbackMsg = "ERROR: You emitted a bare <tool_call> tag but did not include the JSON body. '
    b'Do NOT output XML tags. Output ONLY the raw JSON tool call object - no XML tags, no explanation, no preamble."\n'
    b'\t\t\t}\n'
)

print(f"New section length: {len(new_section)} bytes")

# Replace
new_content = content[:idx_start] + new_section + content[section_end:]

# Write back
with open(filepath, 'wb') as f:
    f.write(new_content)

# Write output to file
with open('disposable/fix_output.txt', 'w') as f:
    f.write(f"Old section length: {len(old_section)} bytes\n")
    f.write(f"New section length: {len(new_section)} bytes\n")
    f.write(f"File size change: {len(content)} -> {len(new_content)} bytes ({len(new_content) - len(content):+d})\n")
    f.write("Successfully patched agent_loop.go\n")
