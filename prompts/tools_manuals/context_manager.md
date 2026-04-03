# context_manager — Conversation Context Management

Allows managing the agent's short-term context window, pinning important information, and summarizing.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `pin` | Pin a specific text to context (always included) | `content` (optional: `topic`) |
| `unpin` | Remove a pinned topic from context | `topic` |
| `list_pins` | List currently pinned topics | (none) |
| `summarize` | Summarize the current conversation to save tokens | (none) |
| `clear_history`| Clear the unpinned conversation history | (none) |

## Key Behaviors

- **pin**: Keeps vital facts (like an API key shape or user preference) in the context window indefinitely.
- **summarize**: Condenses the previous turns into a short summary and clears the raw messages, preventing context window limits.

## Examples

```
# Pin a fact
context_manager(operation="pin", topic="database_schema", content="Users table has (id, name, email)")

# List pins
context_manager(operation="list_pins")

# Summarize and clear history safely
context_manager(operation="summarize")
```

## Tips
- Use `pin` for specific task rules that you shouldn't forget but don't belong in the global system prompt.
- If you feel you are running out of context or losing track, use `summarize`.